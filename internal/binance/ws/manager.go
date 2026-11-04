// Package ws provides bounded, reconnecting Binance market stream subscriptions.
package ws

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand/v2"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/coder/websocket"
)

const maxStreamsPerConnection = 1024

var errTopologyChanged = errors.New("websocket shard topology changed")

type State string

const (
	Connecting   State = "connecting"
	Live         State = "live"
	Degraded     State = "degraded"
	Reconnecting State = "reconnecting"
)

type Policy uint8

const (
	DropOldest Policy = iota
	ResyncOnOverflow
)

type Options struct {
	URLs                   []string
	HTTPClient             *http.Client
	MaxStreams             int
	MinBackoff, MaxBackoff time.Duration
	RotateAfter            time.Duration
}

type Stats struct {
	State         State     `json:"state"`
	Messages      uint64    `json:"messages"`
	LastMessage   time.Time `json:"lastMessage"`
	Reconnects    uint64    `json:"reconnects"`
	ActiveStreams int       `json:"activeStreams"`
}

type Event struct {
	Stream string          `json:"stream"`
	Data   json.RawMessage `json:"data"`
	Err    error           `json:"-"`
}

type subscriber struct {
	ch     chan Event
	resync chan struct{}
	policy Policy
}

type Subscription struct {
	manager    *Manager
	stream     string
	subscriber *subscriber
	Messages   <-chan Event
	Resync     <-chan struct{}
	once       sync.Once
}

func (s *Subscription) Close() { s.once.Do(func() { s.manager.unsubscribe(s.stream, s.subscriber) }) }

type Manager struct {
	opts       Options
	mu         sync.RWMutex
	subs       map[string]map[*subscriber]struct{}
	changes    chan struct{}
	state      atomic.Value
	messages   atomic.Uint64
	reconnects atomic.Uint64
	last       atomic.Int64
}

func New(options Options) (*Manager, error) {
	if len(options.URLs) == 0 {
		return nil, errors.New("at least one Binance WebSocket URL is required")
	}
	for _, raw := range options.URLs {
		u, err := url.Parse(raw)
		if err != nil || u.Scheme != "wss" || u.Host == "" {
			return nil, fmt.Errorf("invalid mainnet WebSocket URL %q", raw)
		}
		if strings.Contains(strings.ToLower(u.Host), "testnet") {
			return nil, errors.New("testnet WebSocket URL is forbidden")
		}
	}
	if options.MaxStreams <= 0 || options.MaxStreams > maxStreamsPerConnection {
		options.MaxStreams = maxStreamsPerConnection
	}
	if options.MinBackoff <= 0 {
		options.MinBackoff = 250 * time.Millisecond
	}
	if options.MaxBackoff <= 0 {
		options.MaxBackoff = 15 * time.Second
	}
	if options.RotateAfter <= 0 || options.RotateAfter >= 24*time.Hour {
		options.RotateAfter = 23*time.Hour + 50*time.Minute
	}
	m := &Manager{opts: options, subs: make(map[string]map[*subscriber]struct{}), changes: make(chan struct{}, 1)}
	m.state.Store(Connecting)
	return m, nil
}

func (m *Manager) Subscribe(stream string, buffer int, policy Policy) (*Subscription, error) {
	stream = strings.ToLower(strings.TrimSpace(stream))
	if stream == "" || strings.ContainsAny(stream, " /?#") {
		return nil, fmt.Errorf("invalid stream name %q", stream)
	}
	if buffer < 1 {
		buffer = 1
	}
	s := &subscriber{ch: make(chan Event, buffer), resync: make(chan struct{}, 1), policy: policy}
	m.mu.Lock()
	group := m.subs[stream]
	if group == nil {
		group = make(map[*subscriber]struct{})
		m.subs[stream] = group
	}
	group[s] = struct{}{}
	m.mu.Unlock()
	m.signal()
	return &Subscription{manager: m, stream: stream, subscriber: s, Messages: s.ch, Resync: s.resync}, nil
}

func (m *Manager) unsubscribe(stream string, s *subscriber) {
	m.mu.Lock()
	if group := m.subs[stream]; group != nil {
		delete(group, s)
		if len(group) == 0 {
			delete(m.subs, stream)
		}
	}
	close(s.ch)
	close(s.resync)
	m.mu.Unlock()
	m.signal()
}

func (m *Manager) signal() {
	select {
	case m.changes <- struct{}{}:
	default:
	}
}

func (m *Manager) Stats() Stats {
	m.mu.RLock()
	active := len(m.subs)
	m.mu.RUnlock()
	var last time.Time
	if n := m.last.Load(); n > 0 {
		last = time.Unix(0, n)
	}
	state, ok := m.state.Load().(State)
	if !ok {
		state = Connecting
	}
	return Stats{State: state, Messages: m.messages.Load(), LastMessage: last, Reconnects: m.reconnects.Load(), ActiveStreams: active}
}

// Run owns all connection and reader goroutines until ctx is cancelled.
func (m *Manager) Run(ctx context.Context) error {
	backoff := m.opts.MinBackoff
	urlIndex := -1
	for ctx.Err() == nil {
		streams := m.streamList()
		if len(streams) == 0 {
			m.state.Store(Connecting)
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-m.changes:
			}
			continue
		}
		m.state.Store(Connecting)
		urlIndex = (urlIndex + 1) % len(m.opts.URLs)
		err := m.runShards(ctx, m.opts.URLs[urlIndex], streams)
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if errors.Is(err, errTopologyChanged) {
			continue
		}
		if err != nil {
			m.state.Store(Reconnecting)
			m.reconnects.Add(1)
		}
		delay := jitter(backoff)
		if err := sleep(ctx, delay); err != nil {
			return err
		}
		backoff = min(backoff*2, m.opts.MaxBackoff)
	}
	return ctx.Err()
}

func (m *Manager) runShards(ctx context.Context, base string, streams []string) error {
	chunks := make([][]string, 0, (len(streams)+m.opts.MaxStreams-1)/m.opts.MaxStreams)
	for i := 0; i < len(streams); i += m.opts.MaxStreams {
		end := min(i+m.opts.MaxStreams, len(streams))
		chunks = append(chunks, streams[i:end])
	}
	child, cancel := context.WithCancel(ctx)
	defer cancel()
	errCh := make(chan error, len(chunks))
	controls := make([]chan []string, len(chunks))
	var wg sync.WaitGroup
	for index, chunk := range chunks {
		assigned := append([]string(nil), chunk...)
		controls[index] = make(chan []string, 1)
		wg.Add(1)
		go func(streams []string, control <-chan []string) {
			defer wg.Done()
			errCh <- m.runShard(child, base, streams, control)
		}(assigned, controls[index])
	}
	for {
		select {
		case <-ctx.Done():
			cancel()
			wg.Wait()
			return ctx.Err()
		case err := <-errCh:
			cancel()
			wg.Wait()
			return err
		case <-m.changes:
			desired := m.streamList()
			newChunks := make([][]string, 0, (len(desired)+m.opts.MaxStreams-1)/m.opts.MaxStreams)
			for i := 0; i < len(desired); i += m.opts.MaxStreams {
				newChunks = append(newChunks, desired[i:min(i+m.opts.MaxStreams, len(desired))])
			}
			if len(newChunks) != len(controls) {
				cancel()
				wg.Wait()
				return errTopologyChanged
			}
			for index, next := range newChunks {
				select {
				case controls[index] <- append([]string(nil), next...):
				default:
					select {
					case <-controls[index]:
					default:
					}
					controls[index] <- append([]string(nil), next...)
				}
			}
		}
	}
}

func (m *Manager) runShard(ctx context.Context, base string, streams []string, controls <-chan []string) error {
	u, err := url.Parse(base)
	if err != nil {
		return err
	}
	u.Path = strings.TrimRight(u.Path, "/") + "/stream"
	u.RawQuery = "streams=" + url.QueryEscape(strings.Join(streams, "/"))
	u.RawQuery = strings.ReplaceAll(u.RawQuery, "%2F", "/")
	conn, response, err := websocket.Dial(ctx, u.String(), &websocket.DialOptions{HTTPClient: m.opts.HTTPClient})
	if response != nil && response.Body != nil {
		_ = response.Body.Close()
	}
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close(websocket.StatusNormalClosure, "reconnect") }()
	m.state.Store(Live)
	readCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	frames := make(chan []byte, 64)
	readErr := make(chan error, 1)
	go func() {
		defer close(frames)
		for {
			_, b, e := conn.Read(readCtx)
			if e != nil {
				readErr <- e
				return
			}
			select {
			case frames <- b:
			case <-readCtx.Done():
				return
			}
		}
	}()
	rotate := time.NewTimer(m.opts.RotateAfter)
	defer rotate.Stop()
	active := make(map[string]struct{}, len(streams))
	for _, s := range streams {
		active[s] = struct{}{}
	}
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-rotate.C:
			return errors.New("planned websocket rotation")
		case err := <-readErr:
			return err
		case desired := <-controls:
			for _, s := range desired {
				if _, ok := active[s]; !ok {
					if err := writeControl(ctx, conn, "SUBSCRIBE", []string{s}); err != nil {
						return err
					}
					active[s] = struct{}{}
				}
			}
			for s := range active {
				if !contains(desired, s) {
					if err := writeControl(ctx, conn, "UNSUBSCRIBE", []string{s}); err != nil {
						return err
					}
					delete(active, s)
				}
			}
		case frame, ok := <-frames:
			if !ok {
				return errors.New("websocket reader stopped")
			}
			var event Event
			if err := json.Unmarshal(frame, &event); err != nil {
				m.state.Store(Degraded)
				continue
			}
			if event.Stream == "" {
				continue
			}
			m.messages.Add(1)
			m.last.Store(time.Now().UnixNano())
			m.fanout(event)
		}
	}
}

func writeControl(ctx context.Context, c *websocket.Conn, method string, streams []string) error {
	b, err := json.Marshal(struct {
		Method string   `json:"method"`
		Params []string `json:"params"`
		ID     int      `json:"id"`
	}{method, streams, 1})
	if err != nil {
		return err
	}
	return c.Write(ctx, websocket.MessageText, b)
}

func (m *Manager) streamList() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]string, 0, len(m.subs))
	for s := range m.subs {
		out = append(out, s)
	}
	sort.Strings(out)
	return out
}
func contains(values []string, target string) bool {
	i := sort.SearchStrings(values, target)
	return i < len(values) && values[i] == target
}
func (m *Manager) fanout(event Event) {
	m.mu.RLock()
	group := make([]*subscriber, 0, len(m.subs[event.Stream]))
	for s := range m.subs[event.Stream] {
		group = append(group, s)
	}
	for _, s := range group {
		select {
		case s.ch <- event:
		default:
			if s.policy == ResyncOnOverflow {
				select {
				case s.resync <- struct{}{}:
				default:
				}
			} else {
				select {
				case <-s.ch:
				default:
				}
				select {
				case s.ch <- event:
				default:
				}
			}
		}
	}
	m.mu.RUnlock()
}
func jitter(d time.Duration) time.Duration {
	if d <= 0 {
		return d
	}
	return d/2 + time.Duration(rand.Int64N(int64(d/2)+1))
}
func sleep(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}
