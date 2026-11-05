// Package tui implements the keyboard-driven, read-only terminal UI.
package tui

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/bubblenexus/bnx/internal/binance/models"
	marketdata "github.com/bubblenexus/bnx/internal/core/market"
	"github.com/bubblenexus/bnx/internal/store"
	"github.com/bubblenexus/bnx/internal/tui/components"
	"github.com/bubblenexus/bnx/internal/tui/theme"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/shopspring/decimal"
)

type Service interface {
	Ticker(context.Context, marketdata.Product, string, string) ([]models.Ticker24h, error)
	BookTickers(context.Context, marketdata.Product) ([]models.BookTicker, error)
	ExchangeInfo(context.Context, marketdata.Product, string) (models.ExchangeInfo, error)
	Klines(context.Context, marketdata.Product, string, string, int, int64, int64) ([]models.Kline, error)
	Book(context.Context, marketdata.Product, string, int) (models.Depth, error)
	Trades(context.Context, marketdata.Product, string, int) ([]models.Trade, error)
}

type View int

const (
	Dashboard View = iota
	Markets
	Symbol
	Futures
	Account
	Alerts
	Logs
	Help
	SpotMarket
)

var viewNames = [...]string{"Dashboard", "Markets", "Symbol", "Futures", "Account", "Alerts", "Logs", "Help", "Spot Market"}
var products = [...]marketdata.Product{marketdata.Spot, marketdata.USDM, marketdata.COINM, marketdata.Options}
var intervals = [...]string{"1m", "5m", "15m", "1h", "4h", "1d", "1w", "1M"}
var quoteFilters = [...]string{"ALL", "USDT", "USDC", "BTC", "FDUSD", "BNB"}

type Model struct {
	ctx          context.Context
	service      Service
	local        *store.Store
	profile      string
	product      marketdata.Product
	view         View
	width        int
	height       int
	spotHeatmap  components.HeatmapRenderer
	spotMode     components.HeatmapMode
	spotGridTop  int
	spotQuote    string
	spotPage     int
	symbol       string
	interval     int
	zoom         int
	page         int
	selected     int
	search       string
	searching    bool
	paletteOpen  bool
	paletteQuery string
	quoteIndex   int
	sortBy       int
	themeIndex   int
	favorites    map[string]bool
	showOnly     bool
	indicators   bool
	loading      bool
	lastUpdate   time.Time
	lastError    string
	tickers      map[string]models.Ticker24h
	bookTickers  map[string]models.BookTicker
	symbols      []models.SymbolInfo
	rows         []models.SymbolInfo
	klines       []models.Kline
	book         models.Depth
	trades       []models.Trade
	funding      []models.FundingRate
	futures      []models.FuturesData
	alerts       []store.Alert
	toast        string
	storageError string
	accountData  *accountDataMsg
	userEvents   <-chan json.RawMessage
	userDataLive bool
	detailErr    string
	sparklines   map[string][]models.Kline
}

type marketDataMsg struct {
	product     marketdata.Product
	tickers     []models.Ticker24h
	bookTickers []models.BookTicker
	symbols     []models.SymbolInfo
	options     []models.OptionSymbol
	warning     error
	err         error
}
type detailDataMsg struct {
	product  marketdata.Product
	symbol   string
	interval string
	klines   []models.Kline
	book     models.Depth
	trades   []models.Trade
	err      error
}
type refreshMsg time.Time
type sparklineDataMsg struct {
	product marketdata.Product
	data    map[string][]models.Kline
}
type localDataMsg struct {
	watchlist []string
	alerts    []store.Alert
	err       error
}
type favoriteSavedMsg error
type futuresDataMsg struct {
	funding []models.FundingRate
	ratios  []models.FuturesData
	alerts  []store.Alert
	err     error
}
type alertsTriggeredMsg struct{ alerts []store.Alert }
type toastClearMsg struct{}
type accountDataMsg struct {
	snapshot marketdata.AccountSnapshot
	orders   []models.AccountOrder
	err      error
}
type userDataStreamMsg struct{ open bool }

func New(ctx context.Context, service Service, product marketdata.Product, symbol string) *Model {
	return NewWithStore(ctx, service, product, symbol, nil, "main")
}

func NewWithStore(ctx context.Context, service Service, product marketdata.Product, symbol string, local *store.Store, profile string) *Model {
	if ctx == nil {
		ctx = context.Background()
	}
	if product == "" {
		product = marketdata.Spot
	}
	view := Dashboard
	if symbol != "" {
		view = Symbol
	}
	if profile == "" {
		profile = "main"
	}
	return &Model{ctx: ctx, service: service, local: local, profile: profile, product: product, view: view, symbol: strings.ToUpper(symbol), interval: 3,
		favorites: make(map[string]bool), tickers: make(map[string]models.Ticker24h), bookTickers: make(map[string]models.BookTicker),
		sparklines: make(map[string][]models.Kline), loading: true, width: 80, height: 24, spotQuote: "USDT"}
}

func Run(ctx context.Context, service Service, product marketdata.Product, symbol string, input io.Reader, output io.Writer) error {
	return RunProfile(ctx, service, product, symbol, "main", input, output)
}

func RunProfile(ctx context.Context, service Service, product marketdata.Product, symbol, profile string, input io.Reader, output io.Writer) error {
	return RunProfileWithUserData(ctx, service, product, symbol, profile, nil, input, output)
}

func RunProfileWithUserData(ctx context.Context, service Service, product marketdata.Product, symbol, profile string, userEvents <-chan json.RawMessage, input io.Reader, output io.Writer) error {
	dbPath, dbErr := store.DefaultPath()
	var local *store.Store
	if dbErr == nil {
		local, dbErr = store.Open(dbPath)
	}
	if local != nil {
		defer func() { _ = local.Close() }()
	}
	model := NewWithStore(ctx, service, product, symbol, local, profile)
	model.userEvents = userEvents
	if dbErr != nil {
		model.storageError = "local persistence unavailable: " + dbErr.Error()
	}
	options := []tea.ProgramOption{tea.WithAltScreen(), tea.WithMouseCellMotion(), tea.WithInput(input), tea.WithOutput(output), tea.WithContext(ctx)}
	program := tea.NewProgram(model, options...)
	_, err := program.Run()
	return err
}

func (m *Model) Init() tea.Cmd {
	commands := []tea.Cmd{m.loadMarket(), m.loadSparklines(), refreshAfter(5 * time.Second)}
	if m.local != nil {
		commands = append(commands, m.loadLocalData())
	}
	if m.userEvents != nil {
		commands = append(commands, waitUserDataEvent(m.ctx, m.userEvents))
	}
	return tea.Batch(commands...)
}

func (m *Model) Update(message tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := message.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		return m, nil
	case marketDataMsg:
		if msg.product != m.product {
			return m, nil
		}
		m.loading = false
		m.lastUpdate = time.Now()
		if msg.err != nil {
			m.lastError = msg.err.Error()
			return m, nil
		}
		m.lastError = ""
		if msg.warning != nil {
			m.lastError = msg.warning.Error()
		}
		if len(msg.symbols) > 0 || len(msg.options) > 0 {
			m.symbols = msg.symbols
		}
		if m.product == marketdata.Options && len(msg.options) > 0 {
			m.symbols = make([]models.SymbolInfo, 0, len(msg.options))
			for _, option := range msg.options {
				m.symbols = append(m.symbols, models.SymbolInfo{Symbol: option.Symbol, BaseAsset: option.Symbol, QuoteAsset: "OPTION", Status: option.Status})
			}
		}
		m.tickers = make(map[string]models.Ticker24h, len(msg.tickers))
		for _, ticker := range msg.tickers {
			m.tickers[ticker.Symbol] = ticker
		}
		m.bookTickers = make(map[string]models.BookTicker, len(msg.bookTickers))
		for _, ticker := range msg.bookTickers {
			m.bookTickers[ticker.Symbol] = ticker
		}
		if m.product == marketdata.Options && len(m.symbols) == 0 {
			// Options exchange metadata uses optionSymbols rather than symbols.
			// Keep the market usable from its ticker response when metadata is absent.
			for symbol := range m.tickers {
				m.symbols = append(m.symbols, models.SymbolInfo{Symbol: symbol, BaseAsset: symbol, QuoteAsset: "OPTION", Status: "TRADING"})
			}
		}
		m.filterRows()
		if m.view == Symbol {
			return m, m.loadDetail()
		}
		return m, nil
	case detailDataMsg:
		if msg.product != m.product || msg.symbol != m.symbol || msg.interval != intervals[m.interval] {
			return m, nil
		}
		m.klines, m.book, m.trades = msg.klines, msg.book, msg.trades
		if msg.err != nil {
			m.detailErr = msg.err.Error()
		} else {
			m.detailErr = ""
		}
		return m, nil
	case sparklineDataMsg:
		if msg.product == m.product {
			m.sparklines = msg.data
		}
		return m, nil
	case localDataMsg:
		if msg.err != nil {
			m.storageError = "local persistence unavailable: " + msg.err.Error()
			return m, nil
		}
		for _, symbol := range msg.watchlist {
			m.favorites[symbol] = true
		}
		m.alerts = msg.alerts
		m.filterRows()
		return m, nil
	case favoriteSavedMsg:
		if msg != nil {
			m.lastError = "watchlist save failed: " + msg.Error()
		}
		return m, nil
	case futuresDataMsg:
		m.funding, m.futures = msg.funding, msg.ratios
		if msg.err != nil {
			m.detailErr = msg.err.Error()
		} else {
			m.detailErr = ""
		}
		if len(msg.alerts) > 0 {
			m.toast = fmt.Sprintf("Funding alert: %s %s", msg.alerts[0].Symbol, msg.alerts[0].Condition)
			return m, tea.Batch(clearToastAfter(3*time.Second), tea.Printf("\a"))
		}
		return m, nil
	case accountDataMsg:
		m.accountData = &msg
		if msg.err != nil {
			m.detailErr = msg.err.Error()
		} else {
			m.detailErr = ""
		}
		return m, nil
	case userDataStreamMsg:
		if !msg.open {
			m.userEvents = nil
			m.userDataLive = false
			return m, nil
		}
		m.userDataLive = true
		commands := []tea.Cmd{waitUserDataEvent(m.ctx, m.userEvents)}
		if m.view == Account {
			commands = append(commands, m.loadAccount())
		}
		return m, tea.Batch(commands...)
	case alertsTriggeredMsg:
		if len(msg.alerts) > 0 {
			m.toast = fmt.Sprintf("Alert: %s %s %s", msg.alerts[0].Symbol, msg.alerts[0].Condition, msg.alerts[0].Target)
			return m, clearToastAfter(3 * time.Second)
		}
		return m, nil
	case toastClearMsg:
		m.toast = ""
		return m, nil
	case refreshMsg:
		commands := []tea.Cmd{m.loadMarket(), refreshAfter(5 * time.Second)}
		if m.view == Symbol {
			commands = append(commands, m.loadDetail())
		}
		if m.view == Futures {
			commands = append(commands, m.loadFutures())
		}
		if m.view == Account {
			commands = append(commands, m.loadAccount())
		}
		if m.local != nil {
			commands = append(commands, m.evaluateAlerts())
		}
		return m, tea.Batch(commands...)
	case tea.MouseMsg:
		if msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonLeft && msg.Y <= 2 {
			row := max(0, msg.Y-1)
			start := row * 4
			count := min(4, len(viewNames)-start)
			index := start + min(count-1, msg.X/max(1, m.width/count))
			if index >= 0 && index < len(viewNames) {
				return m, m.selectView(View(index))
			}
		}
		return m, nil
	case tea.KeyMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

func (m *Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.paletteOpen {
		switch msg.String() {
		case "esc", "ctrl+c":
			m.paletteOpen, m.paletteQuery = false, ""
			return m, nil
		case "backspace":
			if len(m.paletteQuery) > 0 {
				m.paletteQuery = m.paletteQuery[:len(m.paletteQuery)-1]
			}
		case "enter":
			command := m.choosePaletteAction()
			return m, command
		default:
			if msg.Type == tea.KeyRunes {
				m.paletteQuery += string(msg.Runes)
			}
		}
		return m, nil
	}
	if m.searching {
		switch msg.String() {
		case "esc", "enter":
			m.searching = false
		case "backspace":
			if len(m.search) > 0 {
				m.search = m.search[:len(m.search)-1]
			}
		default:
			if msg.Type == tea.KeyRunes {
				m.search += strings.ToUpper(string(msg.Runes))
			}
		}
		m.filterRows()
		return m, nil
	}
	if msg.String() == "ctrl+c" || msg.String() == "ctrl+q" || msg.String() == "q" && m.view != SpotMarket {
		return m, tea.Quit
	}
	switch msg.String() {
	case "1", "2", "3", "4", "5", "6", "7", "8", "9":
		return m, m.selectView(View(msg.String()[0] - '1'))
	case "?":
		m.view = Help
	case "ctrl+p", ":":
		m.paletteOpen, m.paletteQuery = true, ""
	case "esc":
		switch {
		case m.view == Symbol:
			m.view = Markets
		case m.search != "":
			m.search = ""
			m.filterRows()
		default:
			m.view = Dashboard
		}
	case "/":
		m.searching = true
	case "j", "down":
		if m.view == SpotMarket && components.UsesHeatmapGrid(m.width, m.height, m.spotMode) {
			m.spotGridTop++
			return m, nil
		}
		m.moveSelection(1)
	case "k", "up":
		if m.view == SpotMarket && components.UsesHeatmapGrid(m.width, m.height, m.spotMode) {
			m.spotGridTop = max(0, m.spotGridTop-1)
			return m, nil
		}
		m.moveSelection(-1)
	case "g":
		m.selected, m.page = 0, 0
	case "G":
		if len(m.rows) > 0 {
			m.selected = len(m.rows) - 1
			m.page = m.selected / m.pageSize()
		}
	case "pgdown", "ctrl+d":
		m.changePage(1)
	case "pgup", "ctrl+u":
		m.changePage(-1)
	case "enter":
		if m.view == Markets && m.selected >= 0 && m.selected < len(m.rows) {
			m.symbol = m.rows[m.selected].Symbol
			m.view = Symbol
			return m, m.loadDetail()
		}
	case "s":
		m.sortBy = (m.sortBy + 1) % 3
		m.filterRows()
	case "f":
		if m.selected >= 0 && m.selected < len(m.rows) {
			symbol := m.rows[m.selected].Symbol
			m.favorites[symbol] = !m.favorites[symbol]
			var save tea.Cmd
			if m.local != nil {
				save = m.saveFavorite(symbol, m.favorites[symbol])
			}
			if m.showOnly {
				m.filterRows()
			}
			return m, save
		}
	case "F":
		m.showOnly = !m.showOnly
		m.filterRows()
	case "t":
		m.themeIndex = (m.themeIndex + 1) % len(theme.Names)
	case "v":
		if m.view == SpotMarket {
			m.spotMode = (m.spotMode + 1) % 3
			m.spotGridTop = 0
		}
	case "a":
		m.indicators = !m.indicators
	case "q", "Q":
		if m.view == SpotMarket {
			m.cycleSpotQuote()
			return m, nil
		}
		if msg.String() == "Q" {
			m.quoteIndex = (m.quoteIndex + 1) % len(quoteFilters)
			m.filterRows()
		}
	case "m":
		if m.view == SpotMarket {
			return m, nil
		}
		for index, product := range products {
			if product == m.product {
				m.product = products[(index+1)%len(products)]
				m.loading = true
				m.symbols = nil
				m.tickers = make(map[string]models.Ticker24h)
				m.bookTickers = make(map[string]models.BookTicker)
				m.sparklines = make(map[string][]models.Kline)
				return m, tea.Batch(m.loadMarket(), m.loadSparklines())
			}
		}
	case "[", "]":
		if m.view == SpotMarket {
			_, total := m.rankedSpotMarketTiles()
			pages := max(1, (total+spotMarketPageSize-1)/spotMarketPageSize)
			if msg.String() == "[" {
				m.spotPage = max(0, m.spotPage-1)
			} else {
				m.spotPage = min(pages-1, m.spotPage+1)
			}
			m.spotGridTop = 0
			return m, nil
		}
		if msg.String() == "[" {
			m.interval = (m.interval + len(intervals) - 1) % len(intervals)
		} else {
			m.interval = (m.interval + 1) % len(intervals)
		}
		if m.view == Symbol {
			return m, m.loadDetail()
		}
	case "+", "=":
		m.zoom = min(5, m.zoom+1)
	case "-", "_":
		m.zoom = max(0, m.zoom-1)
	case "r":
		m.loading = true
		return m, m.loadMarket()
	}
	return m, nil
}

func (m *Model) View() string {
	if m.width < 1 {
		return "Starting bnx…"
	}
	contentHeight := max(4, m.height-7)
	spotBodyHeight := max(1, m.height-lipgloss.Height(m.header())-lipgloss.Height(m.statusBar())-lipgloss.Height(m.footer()))
	var body string
	switch m.view {
	case Dashboard:
		body = m.dashboard(contentHeight)
	case Markets:
		body = m.marketsView(contentHeight)
	case Symbol:
		body = m.symbolView(contentHeight)
	case Futures:
		body = m.futuresView(contentHeight)
	case Account:
		body = m.accountView(contentHeight)
	case Alerts:
		body = m.alertsView()
	case Logs:
		body = m.diagnosticsView()
	case Help:
		body = m.helpView()
	case SpotMarket:
		body = m.spotMarketView(spotBodyHeight)
	}
	joined := lipgloss.JoinVertical(lipgloss.Left, m.header(), body, m.statusBar(), m.footer())
	if m.paletteOpen {
		joined = lipgloss.JoinVertical(lipgloss.Left, m.header(), m.paletteView(), m.statusBar(), m.footer())
	}
	lines := strings.Split(joined, "\n")
	if len(lines) > m.height {
		lines = lines[:m.height]
	}
	for index := range lines {
		lines[index] = truncate(lines[index], m.width)
	}
	return strings.Join(lines, "\n")
}

func (m *Model) paletteView() string {
	var matches []string
	query := strings.ToLower(strings.TrimSpace(m.paletteQuery))
	for i, name := range viewNames {
		candidate := fmt.Sprintf("%d  Open %s", i+1, name)
		if query == "" || strings.Contains(strings.ToLower(candidate), query) {
			matches = append(matches, candidate)
		}
	}
	for _, symbol := range m.rows {
		candidate := "Open symbol " + symbol.Symbol
		if query == "" || strings.Contains(strings.ToLower(candidate), query) {
			matches = append(matches, candidate)
		}
		if len(matches) >= 10 {
			break
		}
	}
	if len(matches) > 9 {
		matches = matches[:9]
	}
	return roundedPanel("COMMAND PALETTE  ·  type to filter · enter to run · esc to close", "› "+m.paletteQuery+"\n"+strings.Join(matches, "\n"), min(m.width-4, 72), m.palette())
}

func (m *Model) choosePaletteAction() tea.Cmd {
	query := strings.ToLower(strings.TrimSpace(m.paletteQuery))
	for i, name := range viewNames {
		if query == "" || strings.Contains(strings.ToLower(name), query) || query == strconv.Itoa(i+1) || query == strings.ToLower("open "+name) {
			m.paletteOpen, m.paletteQuery = false, ""
			return m.selectView(View(i))
		}
	}
	for _, symbol := range m.rows {
		if query == "" || strings.Contains(strings.ToLower(symbol.Symbol), query) || query == strings.ToLower("open symbol "+symbol.Symbol) {
			m.symbol, m.view = symbol.Symbol, Symbol
			m.paletteOpen, m.paletteQuery = false, ""
			return m.loadDetail()
		}
	}
	return nil
}

func (m *Model) selectView(view View) tea.Cmd {
	m.view = view
	if view == SpotMarket && m.product != marketdata.Spot {
		m.product = marketdata.Spot
		m.loading = true
		m.lastError = ""
		m.symbols = nil
		m.rows = nil
		m.tickers = make(map[string]models.Ticker24h)
		m.bookTickers = make(map[string]models.BookTicker)
		m.sparklines = make(map[string][]models.Kline)
		return m.loadMarket()
	}
	switch view {
	case Symbol:
		if m.symbol != "" {
			return m.loadDetail()
		}
	case Futures:
		return m.loadFutures()
	case Account:
		return m.loadAccount()
	case Dashboard, Markets, Alerts, Logs, Help, SpotMarket:
		return nil
	}
	return nil
}

func (m *Model) header() string {
	items := make([]string, len(viewNames))
	for index, name := range viewNames {
		label := fmt.Sprintf("%d %s", index+1, name)
		style := lipgloss.NewStyle().Padding(0, 1).Foreground(m.palette().Muted)
		if View(index) == m.view {
			style = style.Bold(true).Foreground(m.palette().Accent).Underline(true)
		}
		items[index] = style.Render(label)
	}
	return lipgloss.JoinVertical(lipgloss.Left,
		theme.Title(m.palette()).Render("BNX  ·  BINANCE MAINNET  ·  "+theme.Names[m.themeIndex]),
		lipgloss.JoinHorizontal(lipgloss.Top, items[:4]...),
		lipgloss.JoinHorizontal(lipgloss.Top, items[4:]...),
	)
}

func (m *Model) statusBar() string {
	state := "REST polling"
	if m.loading {
		state = "loading…"
	}
	updated := "waiting for data"
	if !m.lastUpdate.IsZero() {
		updated = "updated " + m.lastUpdate.Format("15:04:05")
	}
	line := fmt.Sprintf(" %s  ·  %s  ·  %s", strings.ToUpper(string(m.product)), state, updated)
	if m.lastError != "" {
		line += "  ·  " + m.lastError
	}
	if m.storageError != "" {
		line += "  ·  " + m.storageError
	}
	if m.userDataLive {
		line += "  ·  ACCOUNT WS LIVE"
	}
	if m.toast != "" {
		line += "  ·  " + m.toast
	}
	return lipgloss.NewStyle().Width(m.width).Foreground(m.palette().Muted).Render(truncate(line, m.width))
}

func (m *Model) footer() string {
	text := "1-9 views  j/k move  / search  s sort  f favorite  F favorites  Q quote  t theme  m market  r refresh  ? help  ctrl+c/q quit"
	if m.view == SpotMarket {
		text = "1-9 views  j/k move  / search  q quote  v layout  [ ] range  r refresh  ? help  ctrl+c quit"
	}
	if m.searching {
		text = "Search: " + m.search + "_  ·  enter accept  esc cancel"
	}
	return lipgloss.NewStyle().Width(m.width).Foreground(m.palette().Muted).Render(truncate(text, m.width))
}

func (m *Model) diagnosticsView() string {
	updated := "waiting for market response"
	if !m.lastUpdate.IsZero() {
		updated = m.lastUpdate.Format(time.RFC3339)
	}
	stream := "REST polling"
	if m.userDataLive {
		stream = "futures account stream live"
	}
	lastError := m.lastError
	if lastError == "" {
		lastError = "none"
	}
	storage := m.storageError
	if storage == "" {
		storage = "available"
	}
	lines := []string{
		"Market: " + strings.ToUpper(string(m.product)),
		"Connection: " + stream,
		"Last update: " + updated,
		fmt.Sprintf("Symbols loaded: %d", len(m.tickers)),
		fmt.Sprintf("Watchlist entries: %d  ·  Alerts: %d", len(m.favorites), len(m.alerts)),
		"Local storage: " + storage,
		"Last request error: " + lastError,
		"Logs are redacted and written to the configured log file.",
		"Export diagnostics: bnx diagnostics --file bnx-diagnostics.zip",
	}
	return roundedPanel("DIAGNOSTICS  ·  LIVE STATUS", strings.Join(lines, "\n"), m.width-4, m.palette())
}

func (m *Model) dashboard(height int) string {
	analysis := m.analysisTickers()
	advancers, volume := 0, decimal.Zero
	for _, ticker := range analysis {
		if ticker.PriceChangePercent.IsPositive() {
			advancers++
		}
		volume = volume.Add(ticker.QuoteVolume)
	}
	breadth := "—"
	if len(analysis) > 0 {
		breadth = fmt.Sprintf("%d%%", advancers*100/len(analysis))
	}
	var output strings.Builder
	_, _ = fmt.Fprintf(&output, "  MARKET PULSE  ·  %s  ·  24H OVERVIEW\n", strings.ToUpper(string(m.product)))
	metrics := fmt.Sprintf("Pairs  %s   │   Advancing  %s   │   24h quote volume  $%s   │   Updated  %s",
		commaInt(len(analysis)), breadth, compactDecimal(volume), m.updateLabel())
	output.WriteString(roundedPanel("MARKET SUMMARY", metrics, m.width-4, m.palette()))
	output.WriteString("\n")

	watch := m.watchPanel(height)
	movers := m.moversPanel(height)
	if m.width >= 112 {
		leftWidth := (m.width - 7) / 2
		rightWidth := m.width - leftWidth - 5
		output.WriteString(lipgloss.JoinHorizontal(lipgloss.Top,
			roundedPanel("MAJOR PAIRS  ·  1H TREND", watch, leftWidth, m.palette()), " ",
			roundedPanel("TOP 24H MOVERS", movers, rightWidth, m.palette())))
		output.WriteString("\n")
		output.WriteString(roundedPanel("HIGHEST QUOTE VOLUME", m.volumePanel(5), m.width-4, m.palette()))
	} else {
		output.WriteString(roundedPanel("MAJOR PAIRS  ·  1H TREND", watch, m.width-4, m.palette()))
		output.WriteString("\n")
		output.WriteString(roundedPanel("TOP 24H MOVERS", movers, m.width-4, m.palette()))
		if height > 20 {
			output.WriteString("\n")
			output.WriteString(roundedPanel("HIGHEST QUOTE VOLUME", m.volumePanel(3), m.width-4, m.palette()))
		}
	}
	if len(analysis) == 0 {
		if m.loading {
			output.WriteString("\n  Fetching Binance market overview…")
		} else if m.lastError != "" {
			output.WriteString("\n  Market data unavailable: " + m.lastError)
		}
	}
	return output.String()
}

func (m *Model) watchPanel(height int) string {
	var output strings.Builder
	rows := 5
	if height > 30 {
		rows = 7
	} else if height < 22 {
		rows = 3
	}
	for index, symbol := range dashboardSymbols(m.product) {
		if index >= rows {
			break
		}
		ticker, ok := m.tickers[symbol]
		if !ok {
			if m.loading {
				_, _ = fmt.Fprintf(&output, "%-10s  loading\n", symbol)
			}
			continue
		}
		change := signedPercent(ticker.PriceChangePercent)
		trendColor := m.palette().Green
		if ticker.PriceChangePercent.IsNegative() {
			trendColor = m.palette().Red
		}
		style := lipgloss.NewStyle().Foreground(trendColor)
		graph := components.Sparkline(m.sparklines[symbol], 18, trendColor)
		bid, ask := m.bestPrices(ticker)
		_, _ = fmt.Fprintf(&output, "%-10s %13s %9s  %s  %s / %s\n", symbol, displayPrice(ticker.LastPrice), style.Render(change), graph, bid, ask)
	}
	if output.Len() == 0 {
		output.WriteString("Waiting for major-pair market data")
	}
	return strings.TrimRight(output.String(), "\n")
}

func (m *Model) moversPanel(height int) string {
	rows := m.analysisTickers()
	if len(rows) == 0 {
		return "Waiting for quote-asset tickers"
	}
	gainers := append([]models.Ticker24h(nil), rows...)
	losers := append([]models.Ticker24h(nil), rows...)
	sort.Slice(gainers, func(i, j int) bool { return gainers[i].PriceChangePercent.GreaterThan(gainers[j].PriceChangePercent) })
	sort.Slice(losers, func(i, j int) bool { return losers[i].PriceChangePercent.LessThan(losers[j].PriceChangePercent) })
	limit := 3
	if height < 18 {
		limit = 1
	} else if height < 25 {
		limit = 2
	}
	var output strings.Builder
	output.WriteString("GAINERS\n")
	for index := 0; index < min(limit, len(gainers)); index++ {
		ticker := gainers[index]
		style := lipgloss.NewStyle().Foreground(m.palette().Green)
		_, _ = fmt.Fprintf(&output, "  %-14s %12s  %s\n", ticker.Symbol, displayPrice(ticker.LastPrice), style.Render(signedPercent(ticker.PriceChangePercent)))
	}
	if height >= 18 {
		output.WriteByte('\n')
	}
	output.WriteString("LOSERS\n")
	for index := 0; index < min(limit, len(losers)); index++ {
		ticker := losers[index]
		style := lipgloss.NewStyle().Foreground(m.palette().Red)
		_, _ = fmt.Fprintf(&output, "  %-14s %12s  %s\n", ticker.Symbol, displayPrice(ticker.LastPrice), style.Render(signedPercent(ticker.PriceChangePercent)))
	}
	return strings.TrimRight(output.String(), "\n")
}

func (m *Model) volumePanel(limit int) string {
	rows := m.analysisTickers()
	sort.Slice(rows, func(i, j int) bool { return rows[i].QuoteVolume.GreaterThan(rows[j].QuoteVolume) })
	var output strings.Builder
	for index := 0; index < min(limit, len(rows)); index++ {
		ticker := rows[index]
		bar := volumeBar(ticker.QuoteVolume, rows[0].QuoteVolume, max(4, m.width/14))
		_, _ = fmt.Fprintf(&output, "  %-14s %12s  %s\n", ticker.Symbol, "$"+compactDecimal(ticker.QuoteVolume), lipgloss.NewStyle().Foreground(m.palette().Accent).Render(bar))
	}
	return strings.TrimRight(output.String(), "\n")
}

func (m *Model) analysisTickers() []models.Ticker24h {
	quotes := map[string]bool{"USDT": true, "USDC": true, "FDUSD": true, "BUSD": true}
	allowed := make(map[string]bool, len(m.symbols))
	for _, symbol := range m.symbols {
		marketEligible := quotes[symbol.QuoteAsset]
		if m.product == marketdata.COINM || m.product == marketdata.Options {
			marketEligible = true
		}
		if symbol.Status == "TRADING" && marketEligible {
			allowed[symbol.Symbol] = true
		}
	}
	rows := make([]models.Ticker24h, 0, len(allowed))
	for symbol := range allowed {
		if ticker, ok := m.tickers[symbol]; ok {
			rows = append(rows, ticker)
		}
	}
	if len(rows) == 0 && len(m.symbols) == 0 {
		for _, ticker := range m.tickers {
			if strings.HasSuffix(ticker.Symbol, "USDT") || strings.HasSuffix(ticker.Symbol, "USDC") {
				rows = append(rows, ticker)
			}
		}
	}
	return rows
}

func (m *Model) spotMarketView(height int) string {
	_, total := m.rankedSpotMarketTiles()
	pageCount := max(1, (total+spotMarketPageSize-1)/spotMarketPageSize)
	m.spotPage = min(max(0, m.spotPage), pageCount-1)
	tiles := m.spotMarketTiles()
	var output strings.Builder
	output.WriteString("  SPOT MARKET  /  24H MOVERS\n")
	gridMode := components.UsesHeatmapGrid(m.width, m.height, m.spotMode)
	start := 0
	if total > 0 {
		start = min(m.spotPage*spotMarketPageSize, total)
	}
	end := min(start+len(tiles), total)
	modeDescription := "SIZE = 24H CHANGE"
	if gridMode {
		modeDescription = "GRID MODE"
	}
	_, _ = fmt.Fprintf(&output, "  QUOTE %s (Q)  ·  PAIRS %d-%d / %d ([ ])  ·  %s\n", m.spotQuote, min(start+1, total), end, total, modeDescription)
	headerLines := 2
	if gridMode {
		if m.spotMode == components.HeatmapAuto {
			output.WriteString("  Terminal small: grid mode (resize to 120x35 for treemap)\n")
			headerLines++
		}
	}
	if len(tiles) == 0 {
		switch {
		case m.loading:
			output.WriteString("\n  Loading Binance Spot symbols and tickers…")
		case m.lastError != "":
			output.WriteString("\n  Spot data unavailable: " + m.lastError)
		default:
			output.WriteString("\n  No " + m.spotQuote + " Spot pairs are available in the current Binance response.")
		}
		return output.String()
	}
	footerLines := 1
	if gridMode {
		footerLines = 2
	}
	heatmapHeight := max(1, height-headerLines-footerLines)
	output.WriteString(m.spotHeatmap.Render(tiles, max(1, m.width), heatmapHeight, m.width, m.height, m.palette(), m.spotMode, m.spotGridTop))
	green := lipgloss.NewStyle().Foreground(m.palette().HeatPositiveStrong).Bold(true).Render("+ GAIN")
	red := lipgloss.NewStyle().Foreground(m.palette().HeatNegativeStrong).Bold(true).Render("− LOSS")
	neutral := lipgloss.NewStyle().Foreground(m.palette().Muted).Render("0 FLAT")
	if gridMode {
		_, _ = fmt.Fprintf(&output, "\n  %s   %s   %s\n  %s  ·  v LAYOUT  ·  j/k SCROLL", green, red, neutral, components.HeatmapGridStatus(len(tiles), m.width, heatmapHeight, m.spotGridTop))
	} else {
		_, _ = fmt.Fprintf(&output, "\n  %s   %s   %s   ·   v LAYOUT   Q QUOTE   [ ] RANGE   r REFRESH", green, red, neutral)
	}
	return output.String()
}

const spotMarketPageSize = 100

func (m *Model) spotMarketTiles() []components.HeatmapTile {
	coins, _ := m.rankedSpotMarketTiles()
	start := min(m.spotPage*spotMarketPageSize, len(coins))
	end := min(start+spotMarketPageSize, len(coins))
	coins = coins[start:end]
	tiles := make([]components.HeatmapTile, 0, len(coins))
	rawWeights := make([]float64, len(coins))
	for index, coin := range coins {
		change := coin.change.Abs().InexactFloat64()
		rawWeights[index] = max(0.25, change)
	}
	weights := components.CapHeatmapWeights(rawWeights, 0.15)
	for index, coin := range coins {
		tiles = append(tiles, components.HeatmapTile{
			Name:   coin.base,
			Detail: signedPercent(coin.change),
			Weight: weights[index],
			Change: coin.change.InexactFloat64(),
		})
	}
	return tiles
}

type rankedSpotCoin struct {
	base   string
	volume decimal.Decimal
	change decimal.Decimal
}

func (m *Model) rankedSpotMarketTiles() ([]rankedSpotCoin, int) {
	if m.product != marketdata.Spot {
		return nil, 0
	}
	quote := strings.ToUpper(m.spotQuote)
	if quote == "" {
		quote = "USDT"
	}
	metadata := make(map[string]models.SymbolInfo, len(m.symbols))
	for _, symbol := range m.symbols {
		metadata[symbol.Symbol] = symbol
	}
	coins := make([]rankedSpotCoin, 0, len(m.tickers))
	for _, ticker := range m.tickers {
		info, hasMetadata := metadata[ticker.Symbol]
		quoteAsset := ""
		baseAsset := ""
		if hasMetadata {
			if info.Status != "" && info.Status != "TRADING" {
				continue
			}
			quoteAsset, baseAsset = info.QuoteAsset, info.BaseAsset
		} else if strings.HasSuffix(ticker.Symbol, quote) {
			quoteAsset = quote
			baseAsset = strings.TrimSuffix(ticker.Symbol, quote)
		}
		if quoteAsset != quote || baseAsset == "" {
			continue
		}
		coins = append(coins, rankedSpotCoin{base: baseAsset, volume: ticker.QuoteVolume, change: ticker.PriceChangePercent})
	}
	sort.Slice(coins, func(i, j int) bool {
		if coins[i].volume.Equal(coins[j].volume) {
			return coins[i].base < coins[j].base
		}
		return coins[i].volume.GreaterThan(coins[j].volume)
	})
	return coins, len(coins)
}

func (m *Model) spotQuoteOptions() []string {
	quotes := make(map[string]bool)
	metadata := make(map[string]models.SymbolInfo, len(m.symbols))
	for _, symbol := range m.symbols {
		metadata[symbol.Symbol] = symbol
	}
	for symbol := range m.tickers {
		info, ok := metadata[symbol]
		if !ok || info.Status == "" || info.Status == "TRADING" {
			if ok && info.QuoteAsset != "" {
				quotes[info.QuoteAsset] = true
				continue
			}
			for _, candidate := range []string{"USDT", "USDC", "BTC", "FDUSD", "BNB", "ETH", "TUSD", "DAI", "TRY", "EUR", "BRL", "GBP", "AUD", "JPY", "BIDR", "IDRT", "UAH", "RUB", "ZAR", "PLN", "RON", "ARS", "MXN", "NGN", "VAI", "UST", "USDP"} {
				if strings.HasSuffix(symbol, candidate) && len(symbol) > len(candidate) {
					quotes[candidate] = true
					break
				}
			}
		}
	}
	result := make([]string, 0, len(quotes))
	for quote := range quotes {
		result = append(result, quote)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i] == "USDT" {
			return true
		}
		if result[j] == "USDT" {
			return false
		}
		return result[i] < result[j]
	})
	return result
}

func (m *Model) cycleSpotQuote() {
	quotes := m.spotQuoteOptions()
	if len(quotes) == 0 {
		return
	}
	current := -1
	for index, quote := range quotes {
		if quote == m.spotQuote {
			current = index
			break
		}
	}
	m.spotQuote = quotes[(current+1)%len(quotes)]
	m.spotPage, m.spotGridTop = 0, 0
}

func (m *Model) bestPrices(ticker models.Ticker24h) (string, string) {
	if quote, ok := m.bookTickers[ticker.Symbol]; ok && (!quote.BidPrice.IsZero() || !quote.AskPrice.IsZero()) {
		return displayPrice(quote.BidPrice), displayPrice(quote.AskPrice)
	}
	if !ticker.BidPrice.IsZero() || !ticker.AskPrice.IsZero() {
		return displayPrice(ticker.BidPrice), displayPrice(ticker.AskPrice)
	}
	return "—", "—"
}

func (m *Model) updateLabel() string {
	if m.lastUpdate.IsZero() {
		return "loading"
	}
	return m.lastUpdate.Format("15:04:05")
}

func dashboardSymbols(product marketdata.Product) []string {
	switch product {
	case marketdata.COINM:
		return []string{"BTCUSD_PERP", "ETHUSD_PERP", "BNBUSD_PERP", "SOLUSD_PERP", "XRPUSD_PERP", "ADAUSD_PERP", "DOGEUSD_PERP"}
	case marketdata.Options:
		return []string{}
	case marketdata.Spot, marketdata.USDM:
		return []string{"BTCUSDT", "ETHUSDT", "BNBUSDT", "SOLUSDT", "XRPUSDT", "DOGEUSDT", "ADAUSDT", "LINKUSDT", "AVAXUSDT"}
	default:
		return []string{}
	}
}

func roundedPanel(title, content string, width int, palette theme.Palette) string {
	return lipgloss.NewStyle().Width(max(8, width-2)).Border(lipgloss.RoundedBorder()).BorderForeground(palette.Accent).Padding(0, 1).
		Render(theme.Title(palette).Render(title) + "\n" + content)
}

func (m *Model) marketsView(height int) string {
	var out strings.Builder
	filter := quoteFilters[m.quoteIndex]
	_, _ = fmt.Fprintf(&out, "\n  MARKETS  %s  ·  quote %s  ·  sort %s  ·  %d symbols\n", strings.ToUpper(string(m.product)), filter, []string{"symbol", "24h change", "quote volume"}[m.sortBy], len(m.rows))
	wide := m.width >= 120
	if wide {
		_, _ = fmt.Fprintf(&out, "\n  %-3s %-12s %-13s %-8s %-12s %-12s %-12s %-12s %-10s %-10s\n", "★", "SYMBOL", "LAST", "CHANGE", "HIGH", "LOW", "BID", "ASK", "SPREAD", "QUOTE VOL")
	} else {
		_, _ = fmt.Fprintf(&out, "\n  %-3s %-12s %-13s %-8s %-12s %-12s\n", "★", "SYMBOL", "LAST", "CHANGE", "BID", "ASK")
	}
	start := m.page * m.pageSize()
	end := min(len(m.rows), start+m.pageSize())
	if start > end {
		start = end
	}
	for index := start; index < end && index-start < height-4; index++ {
		symbol := m.rows[index]
		ticker := m.tickers[symbol.Symbol]
		star := " "
		if m.favorites[symbol.Symbol] {
			star = "★"
		}
		bid, ask := m.bestPrices(ticker)
		line := fmt.Sprintf("  %-3s %-12s %-13s %-8s %-12s %-12s", star, symbol.Symbol, displayPrice(ticker.LastPrice), signedPercent(ticker.PriceChangePercent), bid, ask)
		if wide {
			spread := ticker.AskPrice.Sub(ticker.BidPrice)
			if quote, ok := m.bookTickers[ticker.Symbol]; ok {
				spread = quote.AskPrice.Sub(quote.BidPrice)
			}
			line = fmt.Sprintf("  %-3s %-12s %-13s %-8s %-12s %-12s %-12s %-12s %-10s %-10s", star, symbol.Symbol, displayPrice(ticker.LastPrice), signedPercent(ticker.PriceChangePercent), displayPrice(ticker.HighPrice), displayPrice(ticker.LowPrice), bid, ask, displayPrice(spread), compactDecimal(ticker.QuoteVolume))
		}
		if index == m.selected {
			line = lipgloss.NewStyle().Bold(true).Foreground(m.palette().Accent).Render(line)
		}
		out.WriteString(truncate(line, m.width))
		out.WriteByte('\n')
	}
	if m.loading && len(m.rows) == 0 {
		out.WriteString("\n  Loading exchange symbols and tickers…\n")
	}
	_, _ = fmt.Fprintf(&out, "\n  Page %d / %d  ·  visible %d-%d", m.page+1, max(1, (len(m.rows)+m.pageSize()-1)/m.pageSize()), start+1, end)
	return out.String()
}

func (m *Model) symbolView(height int) string {
	if m.symbol == "" {
		return "\n  Select a market symbol and press enter to open its detail view."
	}
	ticker := m.tickers[m.symbol]
	changeStyle := lipgloss.NewStyle().Foreground(m.palette().Green)
	if ticker.PriceChangePercent.IsNegative() {
		changeStyle = lipgloss.NewStyle().Foreground(m.palette().Red)
	}
	header := fmt.Sprintf("\n  %s   %s   %s   24h high %s  low %s  volume %s\n", m.symbol, displayPrice(ticker.LastPrice), changeStyle.Render(signedPercent(ticker.PriceChangePercent)), displayPrice(ticker.HighPrice), displayPrice(ticker.LowPrice), compactDecimal(ticker.Volume))
	chartHeight := max(4, min(10, height/2))
	chartWidth := max(8, m.width/2-5)
	visibleCandles := max(10, 80-m.zoom*12)
	chart := components.CandlesWithColors(m.klines, min(chartWidth, visibleCandles), chartHeight, m.palette().Green, m.palette().Red)
	if len(m.klines) == 0 {
		chart = "Loading candles…"
	}
	chartPanel := theme.Panel(m.palette()).Width(chartWidth).Render("CANDLES · " + intervals[m.interval] + "\n" + chart)
	bookPanel := m.renderBook(max(8, m.width-chartWidth-8), chartHeight)
	upper := lipgloss.JoinHorizontal(lipgloss.Top, chartPanel, " ", bookPanel)
	trades := m.renderTrades()
	footer := fmt.Sprintf("\n  interval %s  ·  zoom %d  ·  depth snapshot (REST)  ·  trades: %d", intervals[m.interval], m.zoom, len(m.trades))
	if m.indicators {
		footer += "\n" + m.indicatorSummary()
	}
	if m.detailErr != "" {
		footer += "\n  " + theme.Muted(m.palette()).Render(m.detailErr)
	}
	return lipgloss.JoinVertical(lipgloss.Left, header, upper, "\n  RECENT TRADES", trades, footer)
}

func (m *Model) indicatorSummary() string {
	closes := make([]decimal.Decimal, len(m.klines))
	for i, candle := range m.klines {
		closes[i] = candle.Close
	}
	if len(closes) < 20 {
		return "  Indicators: need at least 20 candles"
	}
	sma, _ := marketdata.SMA(closes, 20)
	ema, _ := marketdata.EMA(closes, 20)
	bands, _ := marketdata.Bollinger(closes, 20, decimal.NewFromInt(2))
	rsi, _ := marketdata.RSI(closes, 14)
	last := len(closes) - 1
	return fmt.Sprintf("  SMA20 %s  EMA20 %s  BB20 %s–%s  RSI14 %s", displayPrice(sma[last]), displayPrice(ema[last]), displayPrice(bands[last].Lower), displayPrice(bands[last].Upper), rsi[last].StringFixed(1))
}

func (m *Model) renderBook(width, height int) string {
	var out strings.Builder
	out.WriteString("ORDER BOOK\n")
	limit := min(height/2, min(len(m.book.Asks), len(m.book.Bids)))
	maxQty := decimal.Zero
	for index := 0; index < limit; index++ {
		maxQty = decimal.Max(maxQty, m.book.Asks[index].Quantity)
		maxQty = decimal.Max(maxQty, m.book.Bids[index].Quantity)
	}
	if maxQty.IsZero() {
		maxQty = decimal.NewFromInt(1)
	}
	for index := limit - 1; index >= 0; index-- {
		level := m.book.Asks[index]
		bar := barFor(level.Quantity, maxQty, max(1, width/3))
		_, _ = fmt.Fprintf(&out, "%-*s %-8s %s\n", max(6, width/2-7), displayPrice(level.Price), compactDecimal(level.Quantity), lipgloss.NewStyle().Foreground(m.palette().Red).Render(bar))
	}
	spread := "—"
	if len(m.book.Asks) > 0 && len(m.book.Bids) > 0 {
		spread = m.book.Asks[0].Price.Sub(m.book.Bids[0].Price).String()
	}
	_, _ = fmt.Fprintf(&out, "spread %s\n", spread)
	for index := 0; index < limit; index++ {
		level := m.book.Bids[index]
		bar := barFor(level.Quantity, maxQty, max(1, width/3))
		_, _ = fmt.Fprintf(&out, "%-*s %-8s %s\n", max(6, width/2-7), displayPrice(level.Price), compactDecimal(level.Quantity), lipgloss.NewStyle().Foreground(m.palette().Green).Render(bar))
	}
	return theme.Panel(m.palette()).Width(width).Render(out.String())
}

func (m *Model) renderTrades() string {
	var out strings.Builder
	count := min(5, len(m.trades))
	for index := len(m.trades) - count; index < len(m.trades); index++ {
		trade := m.trades[index]
		side, color := "BUY ", m.palette().Green
		if trade.IsBuyerMaker {
			side, color = "SELL", m.palette().Red
		}
		_, _ = fmt.Fprintf(&out, "  %s  %-16s %s  %s\n", lipgloss.NewStyle().Foreground(color).Render(side), displayPrice(trade.Price), compactDecimal(trade.Qty), time.UnixMilli(trade.Time).Format("15:04:05"))
	}
	if count == 0 {
		out.WriteString("  Loading recent trades…")
	}
	return out.String()
}

func (m *Model) placeholder(title, text string) string {
	return fmt.Sprintf("\n  %s\n\n  %s\n", theme.Title(m.palette()).Render(title), text)
}

func (m *Model) futuresView(height int) string {
	var out strings.Builder
	out.WriteString("\n  USDⓈ-M FUNDING RATE MONITOR  ·  LIVE REST SNAPSHOT\n\n")
	count := min(max(1, height-8), len(m.funding))
	if count == 0 {
		out.WriteString("  Loading funding rates…\n")
	} else {
		rows := append([]models.FundingRate(nil), m.funding...)
		sort.Slice(rows, func(i, j int) bool { return rows[i].FundingRate.GreaterThan(rows[j].FundingRate) })
		for i := 0; i < count; i++ {
			item := rows[i]
			style := lipgloss.NewStyle().Foreground(m.palette().Green)
			if item.FundingRate.IsNegative() {
				style = lipgloss.NewStyle().Foreground(m.palette().Red)
			}
			countdown := time.Until(time.UnixMilli(item.FundingTime)).Round(time.Minute)
			_, _ = fmt.Fprintf(&out, "  %-18s mark %-14s rate %s  next %s\n", item.Symbol, displayPrice(item.MarkPrice), style.Render(signedPercent(item.FundingRate.Mul(decimal.NewFromInt(100)))), countdown)
		}
	}
	out.WriteString("\n  BTCUSDT POSITIONING SNAPSHOT\n")
	if len(m.futures) == 0 {
		out.WriteString("  No positioning data available.\n")
	} else {
		item := m.futures[0]
		_, _ = fmt.Fprintf(&out, "  Global long/short ratio %s  ·  long %s%%  ·  short %s%%\n", item.LongShortRatio.StringFixed(3), item.LongAccount.Mul(decimal.NewFromInt(100)).StringFixed(1), item.ShortAccount.Mul(decimal.NewFromInt(100)).StringFixed(1))
	}
	if m.detailErr != "" {
		_, _ = fmt.Fprintf(&out, "\n  %s\n", theme.Muted(m.palette()).Render(m.detailErr))
	}
	return out.String()
}

func (m *Model) alertsView() string {
	var out strings.Builder
	out.WriteString("\n  LOCAL PRICE & FUNDING ALERTS\n\n")
	if len(m.alerts) == 0 {
		out.WriteString("  No alerts configured. Use: bnx alerts add BTCUSDT above 70000\n")
	} else {
		for _, alert := range m.alerts {
			_, _ = fmt.Fprintf(&out, "  #%-4d %-14s %-16s %s  cooldown %s\n", alert.ID, alert.Symbol, alert.Condition, alert.Target, alert.Cooldown)
		}
	}
	if m.toast != "" {
		_, _ = fmt.Fprintf(&out, "\n  %s\n", lipgloss.NewStyle().Bold(true).Foreground(m.palette().Accent).Render(m.toast))
	}
	return out.String()
}

func (m *Model) accountView(height int) string {
	if m.accountData == nil {
		return m.placeholder("Account", "Loading read-only account details…")
	}
	if m.accountData.err != nil {
		return m.placeholder("Account", "Connect a read-only key with `bnx auth login`, then revisit this view.\n\n"+m.accountData.err.Error())
	}
	data := m.accountData
	var out strings.Builder
	if data.snapshot.CanTrade || data.snapshot.CanWithdraw {
		out.WriteString("\n  ⚠ API KEY PERMISSIONS: trading or withdrawals are enabled. Use Enable Reading only and IP-restrict the key.\n")
	}
	out.WriteString("\n  ACCOUNT BALANCES\n\n")
	shown := 0
	for _, balance := range data.snapshot.Balances {
		total := balance.Free.Add(balance.Locked)
		if total.IsZero() {
			continue
		}
		_, _ = fmt.Fprintf(&out, "  %-12s free %-18s locked %s\n", balance.Asset, balance.Free.String(), balance.Locked.String())
		shown++
		if shown >= max(1, height/2) {
			break
		}
	}
	if shown == 0 {
		out.WriteString("  No non-zero balances.\n")
	}
	if len(data.snapshot.Positions) > 0 {
		out.WriteString("\n  FUTURES POSITIONS\n")
		for _, position := range data.snapshot.Positions {
			if position.PositionAmt.IsZero() {
				continue
			}
			_, _ = fmt.Fprintf(&out, "  %-18s qty %-12s entry %-12s mark %-12s liq %-12s uPnL %s\n", position.Symbol, position.PositionAmt, displayPrice(position.EntryPrice), displayPrice(position.MarkPrice), displayPrice(position.LiquidationPrice), position.UnrealizedProfit)
		}
	}
	out.WriteString("\n  OPEN ORDERS (DISPLAY ONLY)\n")
	if len(data.orders) == 0 {
		out.WriteString("  No open orders.\n")
	} else {
		for _, order := range data.orders {
			_, _ = fmt.Fprintf(&out, "  %-14s %-5s %-14s qty %-12s filled %s\n", order.Symbol, order.Side, order.Status, order.OrigQty, order.ExecutedQty)
		}
	}
	if m.detailErr != "" {
		_, _ = fmt.Fprintf(&out, "\n  %s\n", theme.Muted(m.palette()).Render(m.detailErr))
	}
	return out.String()
}

func (m *Model) loadAccount() tea.Cmd {
	reader, ok := m.service.(interface {
		Account(context.Context, marketdata.Product) (marketdata.AccountSnapshot, error)
		OpenOrders(context.Context, marketdata.Product, string) ([]models.AccountOrder, error)
	})
	if !ok {
		return nil
	}
	ctx, product := m.ctx, m.product
	return func() tea.Msg {
		snapshot, err := reader.Account(ctx, product)
		if err != nil {
			return accountDataMsg{err: err}
		}
		orders, err := reader.OpenOrders(ctx, product, "")
		return accountDataMsg{snapshot: snapshot, orders: orders, err: err}
	}
}

func (m *Model) loadLocalData() tea.Cmd {
	if m.local == nil {
		return nil
	}
	db, ctx, profile := m.local, m.ctx, m.profile
	return func() tea.Msg {
		watchlist, err := db.Watchlist(ctx, profile)
		if err != nil {
			return localDataMsg{err: err}
		}
		alerts, err := db.Alerts(ctx, profile)
		return localDataMsg{watchlist: watchlist, alerts: alerts, err: err}
	}
}

func (m *Model) saveFavorite(symbol string, enabled bool) tea.Cmd {
	db, ctx, profile := m.local, m.ctx, m.profile
	return func() tea.Msg {
		var err error
		if enabled {
			err = db.AddWatchlist(ctx, profile, symbol)
		} else {
			err = db.RemoveWatchlist(ctx, profile, symbol)
		}
		return favoriteSavedMsg(err)
	}
}

func (m *Model) evaluateAlerts() tea.Cmd {
	if m.local == nil || len(m.alerts) == 0 {
		return nil
	}
	db, ctx, profile := m.local, m.ctx, m.profile
	tickers := make(map[string]models.Ticker24h, len(m.tickers))
	for symbol, ticker := range m.tickers {
		tickers[symbol] = ticker
	}
	alerts := append([]store.Alert(nil), m.alerts...)
	return func() tea.Msg {
		triggered := make([]store.Alert, 0)
		seen := make(map[string]bool)
		for _, alert := range alerts {
			ticker, ok := tickers[alert.Symbol]
			if !ok {
				continue
			}
			condition := alert.Condition
			var value decimal.Decimal
			switch condition {
			case "above", "below":
				value = ticker.LastPrice
			case "change_above", "change_below":
				value = ticker.PriceChangePercent
			default:
				continue
			}
			key := alert.Symbol + "/" + condition
			if seen[key] {
				continue
			}
			seen[key] = true
			hits, err := db.Evaluate(ctx, profile, alert.Symbol, condition, value.String(), time.Now())
			if err == nil {
				triggered = append(triggered, hits...)
			}
		}
		return alertsTriggeredMsg{alerts: triggered}
	}
}

func (m *Model) loadFutures() tea.Cmd {
	reader, ok := m.service.(interface {
		Funding(context.Context, marketdata.Product, string, bool) ([]models.FundingRate, error)
		FuturesData(context.Context, marketdata.Product, string, string, string, int) ([]models.FuturesData, error)
	})
	if !ok {
		return nil
	}
	ctx, db, profile, configuredAlerts := m.ctx, m.local, m.profile, append([]store.Alert(nil), m.alerts...)
	return func() tea.Msg {
		funding, err := reader.Funding(ctx, marketdata.USDM, "", false)
		if err != nil {
			return futuresDataMsg{err: err}
		}
		ratios, ratioErr := reader.FuturesData(ctx, marketdata.USDM, "BTCUSDT", "global-account", "1h", 1)
		var triggered []store.Alert
		if db != nil {
			bySymbol := make(map[string]models.FundingRate, len(funding))
			for _, item := range funding {
				bySymbol[item.Symbol] = item
			}
			for _, alert := range configuredAlerts {
				if !strings.HasPrefix(alert.Condition, "funding_") {
					continue
				}
				item, ok := bySymbol[alert.Symbol]
				if !ok {
					continue
				}
				hits, err := db.Evaluate(ctx, profile, alert.Symbol, alert.Condition, item.FundingRate.String(), time.Now())
				if err == nil {
					triggered = append(triggered, hits...)
				}
			}
		}
		return futuresDataMsg{funding: funding, ratios: ratios, err: ratioErr, alerts: triggered}
	}
}

func clearToastAfter(duration time.Duration) tea.Cmd {
	return tea.Tick(duration, func(time.Time) tea.Msg { return toastClearMsg{} })
}

func waitUserDataEvent(ctx context.Context, events <-chan json.RawMessage) tea.Cmd {
	return func() tea.Msg {
		select {
		case <-ctx.Done():
			return nil
		case _, ok := <-events:
			return userDataStreamMsg{open: ok}
		}
	}
}

func (m *Model) helpView() string {
	bindings := []string{
		"1–9       Switch views",
		"9         Open Spot Market heatmap",
		"j/k ↑/↓   Move selection; g/G first/last row",
		"/         Search symbols",
		"s         Cycle sort: symbol, 24h change, quote volume",
		"f / F     Toggle selected favorite / show favorites only",
		"Q         Cycle quote filter; in Spot Market q or Q cycles quote tokens",
		"t         Cycle dark, light, high contrast, colorblind-safe themes",
		"enter     Open selected symbol",
		"[ / ]     Change candle interval or Spot Market 100-pair range; + / - zoom chart",
		"a         Toggle SMA, EMA, Bollinger, and RSI summaries",
		"m         Cycle market: Spot, USDⓈ-M, COIN-M, Options",
		"v         Cycle Spot heatmap layout; j/k scroll the compact grid",
		"Spot Market ranks pairs by 24h quote volume; tile area follows absolute 24h change.",
		"r         Refresh now; esc backs out",
		": / ctrl+p Search the action and symbol palette",
		"?         Open this help; q quits except in Spot Market; ctrl+c/q quits",
		"All account features are display-only. There are no order-entry controls.",
	}
	return "\n  KEYBOARD REFERENCE\n\n  " + strings.Join(bindings, "\n  ")
}

func (m *Model) loadMarket() tea.Cmd {
	ctx, service, product, view := m.ctx, m.service, m.product, m.view
	db := m.local
	cachedSymbols := append([]models.SymbolInfo(nil), m.symbols...)
	return func() tea.Msg {
		tickers, err := service.Ticker(ctx, product, "", "")
		if err != nil {
			return marketDataMsg{product: product, err: err}
		}
		bookTickers := []models.BookTicker(nil)
		var warning error
		if product != marketdata.Options && view != SpotMarket {
			bookTickers, warning = service.BookTickers(ctx, product)
		}
		symbols := cachedSymbols
		var optionSymbols []models.OptionSymbol
		if len(symbols) == 0 {
			info, infoErr := service.ExchangeInfo(ctx, product, "")
			if infoErr != nil {
				cached := false
				if db != nil {
					cached, _ = db.GetExchangeInfo(ctx, string(product), 24*time.Hour, time.Now(), &info)
				}
				if !cached {
					return marketDataMsg{product: product, err: infoErr}
				}
				warning = fmt.Errorf("using cached exchangeInfo: %w", infoErr)
			} else if db != nil {
				_ = db.PutExchangeInfo(ctx, string(product), info, time.Now())
			}
			symbols, optionSymbols = info.Symbols, info.OptionSymbols
		}
		return marketDataMsg{product: product, tickers: tickers, bookTickers: bookTickers, symbols: symbols, options: optionSymbols, warning: warning}
	}
}

func (m *Model) loadSparklines() tea.Cmd {
	if m.product == marketdata.Options {
		return nil
	}
	ctx, service, product := m.ctx, m.service, m.product
	return func() tea.Msg {
		result := make(map[string][]models.Kline, 3)
		symbols := dashboardSymbols(product)
		if len(symbols) > 3 {
			symbols = symbols[:3]
		}
		for _, symbol := range symbols {
			candles, err := service.Klines(ctx, product, symbol, "1h", 24, 0, 0)
			if err == nil {
				result[symbol] = candles
			}
		}
		return sparklineDataMsg{product: product, data: result}
	}
}

func (m *Model) loadDetail() tea.Cmd {
	if m.symbol == "" {
		return nil
	}
	ctx, service, product, db := m.ctx, m.service, m.product, m.local
	symbol, interval := m.symbol, intervals[m.interval]
	return func() tea.Msg {
		klines, klineErr := service.Klines(ctx, product, symbol, interval, 80, 0, 0)
		if klineErr != nil && db != nil {
			if found, cacheErr := db.GetKlines(ctx, string(product), symbol, interval, 24*time.Hour, time.Now(), &klines); found && cacheErr == nil {
				klineErr = nil
			}
		}
		if klineErr == nil && db != nil {
			_ = db.PutKlines(ctx, string(product), symbol, interval, klines, time.Now())
		}
		book, bookErr := service.Book(ctx, product, symbol, 20)
		trades, tradesErr := service.Trades(ctx, product, symbol, 20)
		var err error
		switch {
		case klineErr != nil:
			err = klineErr
		case bookErr != nil:
			err = bookErr
		case tradesErr != nil:
			err = tradesErr
		}
		return detailDataMsg{product: product, symbol: symbol, interval: interval, klines: klines, book: book, trades: trades, err: err}
	}
}

func refreshAfter(duration time.Duration) tea.Cmd {
	return tea.Tick(duration, func(time.Time) tea.Msg { return refreshMsg(time.Now()) })
}

func (m *Model) filterRows() {
	m.rows = m.rows[:0]
	for _, symbol := range m.symbols {
		if _, ok := m.tickers[symbol.Symbol]; !ok {
			continue
		}
		if filter := quoteFilters[m.quoteIndex]; filter != "ALL" && symbol.QuoteAsset != filter {
			continue
		}
		if m.showOnly && !m.favorites[symbol.Symbol] {
			continue
		}
		if m.search != "" && !strings.Contains(symbol.Symbol, m.search) && !strings.Contains(symbol.BaseAsset, m.search) && !strings.Contains(symbol.QuoteAsset, m.search) {
			continue
		}
		m.rows = append(m.rows, symbol)
	}
	sort.Slice(m.rows, func(i, j int) bool {
		a, b := m.tickers[m.rows[i].Symbol], m.tickers[m.rows[j].Symbol]
		switch m.sortBy {
		case 1:
			return a.PriceChangePercent.GreaterThan(b.PriceChangePercent)
		case 2:
			return a.QuoteVolume.GreaterThan(b.QuoteVolume)
		default:
			return m.rows[i].Symbol < m.rows[j].Symbol
		}
	})
	if m.selected >= len(m.rows) {
		m.selected = max(0, len(m.rows)-1)
	}
	m.page = m.selected / m.pageSize()
}

func (m *Model) pageSize() int          { return max(1, min(25, m.height-10)) }
func (m *Model) palette() theme.Palette { return theme.For(m.themeIndex) }
func (m *Model) moveSelection(delta int) {
	if len(m.rows) == 0 {
		return
	}
	m.selected = max(0, min(len(m.rows)-1, m.selected+delta))
	m.page = m.selected / m.pageSize()
}
func (m *Model) changePage(delta int) {
	pages := max(1, (len(m.rows)+m.pageSize()-1)/m.pageSize())
	m.page = max(0, min(pages-1, m.page+delta))
	m.selected = min(len(m.rows)-1, m.page*m.pageSize())
}
func displayPrice(value decimal.Decimal) string {
	abs := value.Abs()
	places := int32(2)
	if abs.LessThan(decimal.NewFromInt(1)) {
		places = 6
	}
	if abs.LessThan(decimal.NewFromInt(1).Div(decimal.NewFromInt(100))) && !abs.IsZero() {
		places = 8
	}
	formatted := strings.TrimRight(strings.TrimRight(value.Round(places).StringFixed(places), "0"), ".")
	if formatted == "" || formatted == "-" {
		formatted = "0"
	}
	parts := strings.SplitN(formatted, ".", 2)
	integer := parts[0]
	negative := strings.HasPrefix(integer, "-")
	if negative {
		integer = strings.TrimPrefix(integer, "-")
	}
	for index := len(integer) - 3; index > 0; index -= 3 {
		integer = integer[:index] + "," + integer[index:]
	}
	if negative {
		integer = "-" + integer
	}
	if len(parts) == 2 {
		return integer + "." + parts[1]
	}
	return integer
}

func compactDecimal(value decimal.Decimal) string {
	abs := value.Abs()
	for _, unit := range []struct {
		threshold decimal.Decimal
		suffix    string
	}{{decimal.NewFromInt(1_000_000_000_000), "T"}, {decimal.NewFromInt(1_000_000_000), "B"}, {decimal.NewFromInt(1_000_000), "M"}, {decimal.NewFromInt(1_000), "K"}} {
		if abs.GreaterThanOrEqual(unit.threshold) {
			scaled := value.Div(unit.threshold).Round(2).StringFixed(2)
			return strings.TrimRight(strings.TrimRight(scaled, "0"), ".") + unit.suffix
		}
	}
	return displayPrice(value)
}

func signedPercent(value decimal.Decimal) string {
	sign := ""
	if value.IsPositive() {
		sign = "+"
	}
	return sign + value.StringFixed(2) + "%"
}

func commaInt(value int) string {
	return strings.ReplaceAll(displayPrice(decimal.NewFromInt(int64(value))), ".00", "")
}

func volumeBar(value, maximum decimal.Decimal, width int) string {
	if maximum.IsZero() {
		return ""
	}
	length := int(value.Div(maximum).Mul(decimal.NewFromInt(int64(width))).IntPart())
	return strings.Repeat("━", max(1, min(width, length)))
}

func barFor(quantity, maximum decimal.Decimal, width int) string {
	length := int(quantity.Div(maximum).Mul(decimal.NewFromInt(int64(width))).IntPart())
	return strings.Repeat("▰", max(1, min(width, length)))
}

func truncate(value string, width int) string {
	if width < 1 {
		return ""
	}
	return ansi.Truncate(value, width, "…")
}
