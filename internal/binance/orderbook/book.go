// Package orderbook maintains a sequence-checked local depth book.
package orderbook

import (
	"errors"
	"fmt"
	"sort"
	"sync"

	"github.com/bubblenexus/bnx/internal/binance/models"
	"github.com/shopspring/decimal"
)

const maxBufferedEvents = 20_000

var (
	ErrResyncRequired = errors.New("order book sequence gap requires resynchronization")
	ErrBufferOverflow = errors.New("order book synchronization buffer overflow")
)

type Diff struct {
	FirstUpdateID int64          `json:"U"`
	FinalUpdateID int64          `json:"u"`
	Bids          []models.Level `json:"b"`
	Asks          []models.Level `json:"a"`
}

type Book struct {
	mu          sync.RWMutex
	lastID      int64
	initialized bool
	synced      bool
	bids        []models.Level // sorted descending by price
	asks        []models.Level // sorted ascending by price
	pending     []Diff
}

func New() *Book { return &Book{} }

// BeginSync clears the current snapshot and starts buffering stream events.
func (book *Book) BeginSync() {
	book.mu.Lock()
	defer book.mu.Unlock()
	book.resetLocked()
}

// Apply applies a depth update or buffers it until a snapshot is installed.
// Any sequence gap returns ErrResyncRequired and invalidates the local book.
func (book *Book) Apply(update Diff) error {
	book.mu.Lock()
	defer book.mu.Unlock()
	if update.FirstUpdateID <= 0 || update.FinalUpdateID < update.FirstUpdateID {
		book.resetLocked()
		return fmt.Errorf("invalid depth update range [%d,%d]: %w", update.FirstUpdateID, update.FinalUpdateID, ErrResyncRequired)
	}
	if !book.initialized {
		if len(book.pending) >= maxBufferedEvents {
			book.resetLocked()
			return ErrBufferOverflow
		}
		book.pending = append(book.pending, cloneDiff(update))
		return nil
	}
	if update.FinalUpdateID <= book.lastID {
		return nil
	}
	if !book.synced {
		next := book.lastID + 1
		if update.FirstUpdateID > next || update.FinalUpdateID < next {
			book.resetLocked()
			return fmt.Errorf("first update [%d,%d] does not bridge snapshot id %d: %w", update.FirstUpdateID, update.FinalUpdateID, book.lastID, ErrResyncRequired)
		}
		book.applyLocked(update)
		book.synced = true
		return nil
	}
	if update.FirstUpdateID != book.lastID+1 {
		previous := book.lastID
		book.resetLocked()
		return fmt.Errorf("expected update %d after %d, got [%d,%d]: %w", previous+1, previous, update.FirstUpdateID, update.FinalUpdateID, ErrResyncRequired)
	}
	book.applyLocked(update)
	return nil
}

// InstallSnapshot installs the REST snapshot and replays buffered events. The
// first replayed event must bridge lastUpdateId+1, subsequent events must be
// exactly contiguous, and out-of-order buffered events are sorted by sequence.
func (book *Book) InstallSnapshot(snapshot models.Depth) error {
	book.mu.Lock()
	defer book.mu.Unlock()
	if snapshot.LastUpdateID < 0 {
		book.resetLocked()
		return fmt.Errorf("negative snapshot update id: %w", ErrResyncRequired)
	}
	book.lastID = snapshot.LastUpdateID
	book.initialized = true
	book.synced = false
	book.bids = cloneLevels(snapshot.Bids)
	book.asks = cloneLevels(snapshot.Asks)
	sort.Slice(book.bids, func(i, j int) bool { return book.bids[i].Price.GreaterThan(book.bids[j].Price) })
	sort.Slice(book.asks, func(i, j int) bool { return book.asks[i].Price.LessThan(book.asks[j].Price) })
	pending := book.pending
	book.pending = nil
	sort.SliceStable(pending, func(i, j int) bool {
		if pending[i].FirstUpdateID == pending[j].FirstUpdateID {
			return pending[i].FinalUpdateID < pending[j].FinalUpdateID
		}
		return pending[i].FirstUpdateID < pending[j].FirstUpdateID
	})
	for _, update := range pending {
		if update.FinalUpdateID <= book.lastID {
			continue
		}
		if !book.synced {
			next := book.lastID + 1
			if update.FirstUpdateID > next || update.FinalUpdateID < next {
				book.resetLocked()
				return fmt.Errorf("buffer has no bridge at snapshot id %d: %w", snapshot.LastUpdateID, ErrResyncRequired)
			}
			book.applyLocked(update)
			book.synced = true
			continue
		}
		if update.FirstUpdateID != book.lastID+1 {
			previous := book.lastID
			book.resetLocked()
			return fmt.Errorf("buffered update [%d,%d] is not contiguous after %d: %w", update.FirstUpdateID, update.FinalUpdateID, previous, ErrResyncRequired)
		}
		book.applyLocked(update)
	}
	return nil
}

func (book *Book) Synced() bool        { book.mu.RLock(); defer book.mu.RUnlock(); return book.synced }
func (book *Book) LastUpdateID() int64 { book.mu.RLock(); defer book.mu.RUnlock(); return book.lastID }

func (book *Book) BestBidAsk() (bid, ask models.Level, ok bool) {
	book.mu.RLock()
	defer book.mu.RUnlock()
	if !book.synced || len(book.bids) == 0 || len(book.asks) == 0 {
		return models.Level{}, models.Level{}, false
	}
	return book.bids[0], book.asks[0], true
}

func (book *Book) SpreadMid() (spread, mid decimal.Decimal, ok bool) {
	bid, ask, ok := book.BestBidAsk()
	if !ok {
		return decimal.Zero, decimal.Zero, false
	}
	spread = ask.Price.Sub(bid.Price)
	mid = ask.Price.Add(bid.Price).Div(decimal.NewFromInt(2))
	return spread, mid, true
}

func (book *Book) Snapshot(limit int) (bids, asks []models.Level, updateID int64, ok bool) {
	book.mu.RLock()
	defer book.mu.RUnlock()
	if !book.synced {
		return nil, nil, book.lastID, false
	}
	if limit < 0 {
		limit = 0
	}
	bidEnd, askEnd := min(limit, len(book.bids)), min(limit, len(book.asks))
	return cloneLevels(book.bids[:bidEnd]), cloneLevels(book.asks[:askEnd]), book.lastID, true
}

func (book *Book) CumulativeDepth(bidSide bool, limit int) []models.Level {
	book.mu.RLock()
	defer book.mu.RUnlock()
	if !book.synced || limit <= 0 {
		return nil
	}
	levels := book.asks
	if bidSide {
		levels = book.bids
	}
	result := make([]models.Level, 0, min(limit, len(levels)))
	total := decimal.Zero
	for _, level := range levels[:min(limit, len(levels))] {
		total = total.Add(level.Quantity)
		result = append(result, models.Level{Price: level.Price, Quantity: total})
	}
	return result
}

func (book *Book) Imbalance(limit int) (decimal.Decimal, bool) {
	book.mu.RLock()
	defer book.mu.RUnlock()
	if !book.synced || limit <= 0 {
		return decimal.Zero, false
	}
	bidTotal, askTotal := decimal.Zero, decimal.Zero
	for _, level := range book.bids[:min(limit, len(book.bids))] {
		bidTotal = bidTotal.Add(level.Quantity)
	}
	for _, level := range book.asks[:min(limit, len(book.asks))] {
		askTotal = askTotal.Add(level.Quantity)
	}
	total := bidTotal.Add(askTotal)
	if total.IsZero() {
		return decimal.Zero, false
	}
	return bidTotal.Sub(askTotal).Div(total), true
}

func (book *Book) applyLocked(update Diff) {
	for _, level := range update.Bids {
		book.bids = updateLevel(book.bids, level, true)
	}
	for _, level := range update.Asks {
		book.asks = updateLevel(book.asks, level, false)
	}
	book.lastID = update.FinalUpdateID
}

func updateLevel(levels []models.Level, next models.Level, bidSide bool) []models.Level {
	index, exists := findLevel(levels, next.Price, bidSide)
	if exists {
		if next.Quantity.IsZero() {
			copy(levels[index:], levels[index+1:])
			return levels[:len(levels)-1]
		}
		levels[index] = next
		return levels
	}
	if next.Quantity.IsZero() {
		return levels
	}
	levels = append(levels, models.Level{})
	copy(levels[index+1:], levels[index:])
	levels[index] = next
	return levels
}

func findLevel(levels []models.Level, price decimal.Decimal, bidSide bool) (int, bool) {
	index := sort.Search(len(levels), func(index int) bool {
		if bidSide {
			return !levels[index].Price.GreaterThan(price)
		}
		return !levels[index].Price.LessThan(price)
	})
	return index, index < len(levels) && levels[index].Price.Equal(price)
}

func cloneDiff(update Diff) Diff {
	update.Bids = cloneLevels(update.Bids)
	update.Asks = cloneLevels(update.Asks)
	return update
}

func cloneLevels(levels []models.Level) []models.Level { return append([]models.Level(nil), levels...) }

func (book *Book) resetLocked() {
	book.lastID = 0
	book.initialized = false
	book.synced = false
	book.bids = nil
	book.asks = nil
	book.pending = nil
}
