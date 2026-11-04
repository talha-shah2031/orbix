package models

import (
	"encoding/json"
	"fmt"

	"github.com/shopspring/decimal"
)

type Decimal = decimal.Decimal

type ServerTime struct {
	ServerTime int64 `json:"serverTime"`
}
type PriceTicker struct {
	Symbol string  `json:"symbol"`
	Price  Decimal `json:"price"`
	Pair   string  `json:"ps,omitempty"`
	Time   int64   `json:"time,omitempty"`
}
type BookTicker struct {
	Symbol   string  `json:"symbol"`
	BidPrice Decimal `json:"bidPrice"`
	BidQty   Decimal `json:"bidQty"`
	AskPrice Decimal `json:"askPrice"`
	AskQty   Decimal `json:"askQty"`
}
type AvgPrice struct {
	Mins      int     `json:"mins"`
	Price     Decimal `json:"price"`
	CloseTime int64   `json:"closeTime"`
}

type Ticker24h struct {
	Symbol             string  `json:"symbol"`
	PriceChange        Decimal `json:"priceChange"`
	PriceChangePercent Decimal `json:"priceChangePercent"`
	WeightedAvgPrice   Decimal `json:"weightedAvgPrice"`
	PrevClosePrice     Decimal `json:"prevClosePrice"`
	LastPrice          Decimal `json:"lastPrice"`
	LastQty            Decimal `json:"lastQty"`
	BidPrice           Decimal `json:"bidPrice"`
	BidQty             Decimal `json:"bidQty"`
	AskPrice           Decimal `json:"askPrice"`
	AskQty             Decimal `json:"askQty"`
	OpenPrice          Decimal `json:"openPrice"`
	HighPrice          Decimal `json:"highPrice"`
	LowPrice           Decimal `json:"lowPrice"`
	Volume             Decimal `json:"volume"`
	QuoteVolume        Decimal `json:"quoteVolume"`
	OpenTime           int64   `json:"openTime"`
	CloseTime          int64   `json:"closeTime"`
	FirstID            int64   `json:"firstId"`
	LastID             int64   `json:"lastId"`
	Count              int64   `json:"count"`
}

type Trade struct {
	ID           int64   `json:"id"`
	Price        Decimal `json:"price"`
	Qty          Decimal `json:"qty"`
	QuoteQty     Decimal `json:"quoteQty,omitempty"`
	Time         int64   `json:"time"`
	IsBuyerMaker bool    `json:"isBuyerMaker"`
	IsBestMatch  bool    `json:"isBestMatch,omitempty"`
}

type Depth struct {
	LastUpdateID    int64   `json:"lastUpdateId"`
	EventTime       int64   `json:"E,omitempty"`
	TransactionTime int64   `json:"T,omitempty"`
	Bids            []Level `json:"bids"`
	Asks            []Level `json:"asks"`
}

type Level struct {
	Price    Decimal
	Quantity Decimal
}

func (l Level) MarshalJSON() ([]byte, error) { return json.Marshal([2]Decimal{l.Price, l.Quantity}) }

func (l *Level) UnmarshalJSON(data []byte) error {
	var fields []json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return err
	}
	if len(fields) != 2 {
		return fmt.Errorf("order book level must contain exactly two values, got %d", len(fields))
	}
	if err := json.Unmarshal(fields[0], &l.Price); err != nil {
		return fmt.Errorf("parse level price: %w", err)
	}
	if err := json.Unmarshal(fields[1], &l.Quantity); err != nil {
		return fmt.Errorf("parse level quantity: %w", err)
	}
	return nil
}

type Kline struct {
	OpenTime      int64   `json:"openTime"`
	Open          Decimal `json:"open"`
	High          Decimal `json:"high"`
	Low           Decimal `json:"low"`
	Close         Decimal `json:"close"`
	Volume        Decimal `json:"volume"`
	CloseTime     int64   `json:"closeTime"`
	QuoteVolume   Decimal `json:"quoteVolume"`
	Trades        int64   `json:"trades"`
	TakerBuyBase  Decimal `json:"takerBuyBase"`
	TakerBuyQuote Decimal `json:"takerBuyQuote"`
}

func (k *Kline) UnmarshalJSON(data []byte) error {
	var fields []json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return err
	}
	if len(fields) < 11 {
		return fmt.Errorf("kline needs at least 11 fields, got %d", len(fields))
	}
	parse := func(index int, target any) error { return json.Unmarshal(fields[index], target) }
	for _, field := range []struct {
		index  int
		target any
	}{
		{0, &k.OpenTime}, {1, &k.Open}, {2, &k.High}, {3, &k.Low}, {4, &k.Close}, {5, &k.Volume}, {6, &k.CloseTime}, {7, &k.QuoteVolume}, {8, &k.Trades}, {9, &k.TakerBuyBase}, {10, &k.TakerBuyQuote},
	} {
		if err := parse(field.index, field.target); err != nil {
			return fmt.Errorf("parse kline field %d: %w", field.index, err)
		}
	}
	return nil
}

// MarshalJSON preserves Binance's positional kline tuple and decimal strings.
func (k Kline) MarshalJSON() ([]byte, error) {
	return json.Marshal([]any{k.OpenTime, k.Open, k.High, k.Low, k.Close, k.Volume, k.CloseTime, k.QuoteVolume, k.Trades, k.TakerBuyBase, k.TakerBuyQuote, nil})
}

type ExchangeInfo struct {
	Timezone        string            `json:"timezone"`
	ServerTime      int64             `json:"serverTime"`
	RateLimits      []RateLimit       `json:"rateLimits"`
	ExchangeFilters []json.RawMessage `json:"exchangeFilters,omitempty"`
	Symbols         []SymbolInfo      `json:"symbols"`
	OptionSymbols   []OptionSymbol    `json:"optionSymbols,omitempty"`
}

type RateLimit struct {
	Type        string `json:"rateLimitType"`
	Interval    string `json:"interval"`
	IntervalNum int    `json:"intervalNum"`
	Limit       int    `json:"limit"`
}
type SymbolInfo struct {
	Symbol            string         `json:"symbol"`
	Status            string         `json:"status"`
	BaseAsset         string         `json:"baseAsset"`
	QuoteAsset        string         `json:"quoteAsset"`
	PricePrecision    int            `json:"pricePrecision"`
	QuantityPrecision int            `json:"quantityPrecision"`
	Filters           []SymbolFilter `json:"filters"`
}
type SymbolFilter struct {
	Type              string  `json:"filterType"`
	MinPrice          Decimal `json:"minPrice,omitempty"`
	MaxPrice          Decimal `json:"maxPrice,omitempty"`
	TickSize          Decimal `json:"tickSize,omitempty"`
	MinQty            Decimal `json:"minQty,omitempty"`
	MaxQty            Decimal `json:"maxQty,omitempty"`
	StepSize          Decimal `json:"stepSize,omitempty"`
	MinNotional       Decimal `json:"minNotional,omitempty"`
	Notional          Decimal `json:"notional,omitempty"`
	MultiplierUp      Decimal `json:"multiplierUp,omitempty"`
	MultiplierDown    Decimal `json:"multiplierDown,omitempty"`
	BidMultiplierUp   Decimal `json:"bidMultiplierUp,omitempty"`
	BidMultiplierDown Decimal `json:"bidMultiplierDown,omitempty"`
	AskMultiplierUp   Decimal `json:"askMultiplierUp,omitempty"`
	AskMultiplierDown Decimal `json:"askMultiplierDown,omitempty"`
	MaxNumOrders      int64   `json:"maxNumOrders,omitempty"`
	MaxNumAlgoOrders  int64   `json:"maxNumAlgoOrders,omitempty"`
	AvgPriceMins      int64   `json:"avgPriceMins,omitempty"`
	ApplyToMarket     bool    `json:"applyToMarket,omitempty"`
	raw               json.RawMessage
}

func (filter *SymbolFilter) UnmarshalJSON(data []byte) error {
	type alias SymbolFilter
	var decoded alias
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	decoded.raw = append(decoded.raw[:0], data...)
	*filter = SymbolFilter(decoded)
	return nil
}

func (filter SymbolFilter) MarshalJSON() ([]byte, error) {
	if len(filter.raw) > 0 {
		return append([]byte(nil), filter.raw...), nil
	}
	type alias SymbolFilter
	return json.Marshal(alias(filter))
}

type OptionSymbol struct {
	Symbol      string         `json:"symbol"`
	Side        string         `json:"side"`
	StrikePrice Decimal        `json:"strikePrice"`
	ExpiryDate  int64          `json:"expiryDate"`
	Status      string         `json:"status"`
	Filters     []SymbolFilter `json:"filters"`
}

type FundingRate struct {
	Symbol      string  `json:"symbol"`
	FundingRate Decimal `json:"fundingRate"`
	FundingTime int64   `json:"fundingTime"`
	MarkPrice   Decimal `json:"markPrice"`
}
type OpenInterest struct {
	Symbol       string  `json:"symbol"`
	OpenInterest Decimal `json:"openInterest"`
	Time         int64   `json:"time"`
}
type PremiumIndex struct {
	Symbol          string  `json:"symbol"`
	Pair            string  `json:"pair,omitempty"`
	MarkPrice       Decimal `json:"markPrice"`
	IndexPrice      Decimal `json:"indexPrice"`
	LastFundingRate Decimal `json:"lastFundingRate"`
	NextFundingTime int64   `json:"nextFundingTime"`
	Time            int64   `json:"time"`
}

// FuturesData contains the shared fields returned by futures sentiment and OI history routes.
type FuturesData struct {
	Symbol               string  `json:"symbol,omitempty"`
	Pair                 string  `json:"pair,omitempty"`
	ContractType         string  `json:"contractType,omitempty"`
	LongShortRatio       Decimal `json:"longShortRatio,omitempty"`
	LongAccount          Decimal `json:"longAccount,omitempty"`
	ShortAccount         Decimal `json:"shortAccount,omitempty"`
	LongPosition         Decimal `json:"longPosition,omitempty"`
	ShortPosition        Decimal `json:"shortPosition,omitempty"`
	BuySellRatio         Decimal `json:"buySellRatio,omitempty"`
	TakerBuyVol          Decimal `json:"takerBuyVol,omitempty"`
	TakerSellVol         Decimal `json:"takerSellVol,omitempty"`
	SumOpenInterest      Decimal `json:"sumOpenInterest,omitempty"`
	SumOpenInterestValue Decimal `json:"sumOpenInterestValue,omitempty"`
	Timestamp            int64   `json:"timestamp,omitempty"`
}

type AccountBalance struct {
	Asset            string  `json:"asset"`
	Free             Decimal `json:"free"`
	Locked           Decimal `json:"locked"`
	WalletBalance    Decimal `json:"walletBalance,omitempty"`
	AvailableBalance Decimal `json:"availableBalance,omitempty"`
	MarginBalance    Decimal `json:"marginBalance,omitempty"`
}

type SpotAccount struct {
	CanTrade    bool             `json:"canTrade"`
	CanWithdraw bool             `json:"canWithdraw"`
	CanDeposit  bool             `json:"canDeposit"`
	UpdateTime  int64            `json:"updateTime"`
	Balances    []AccountBalance `json:"balances"`
}

type FuturesAccount struct {
	CanTrade              bool              `json:"canTrade"`
	CanWithdraw           bool              `json:"canWithdraw"`
	CanDeposit            bool              `json:"canDeposit"`
	TotalWalletBalance    Decimal           `json:"totalWalletBalance"`
	TotalUnrealizedProfit Decimal           `json:"totalUnrealizedProfit"`
	AvailableBalance      Decimal           `json:"availableBalance"`
	Assets                []AccountBalance  `json:"assets"`
	Positions             []FuturesPosition `json:"positions"`
}

type FuturesPosition struct {
	Symbol           string  `json:"symbol"`
	PositionSide     string  `json:"positionSide"`
	PositionAmt      Decimal `json:"positionAmt"`
	EntryPrice       Decimal `json:"entryPrice"`
	BreakEvenPrice   Decimal `json:"breakEvenPrice"`
	MarkPrice        Decimal `json:"markPrice"`
	LiquidationPrice Decimal `json:"liquidationPrice"`
	UnrealizedProfit Decimal `json:"unRealizedProfit"`
	Leverage         int     `json:"leverage"`
	MarginType       string  `json:"marginType"`
	Isolated         bool    `json:"isolated"`
}

type AccountOrder struct {
	Symbol      string  `json:"symbol"`
	OrderID     int64   `json:"orderId"`
	Side        string  `json:"side"`
	Type        string  `json:"type"`
	Status      string  `json:"status"`
	Price       Decimal `json:"price"`
	OrigQty     Decimal `json:"origQty"`
	ExecutedQty Decimal `json:"executedQty"`
	Time        int64   `json:"time"`
	UpdateTime  int64   `json:"updateTime"`
}

type AccountTrade struct {
	Symbol          string  `json:"symbol"`
	ID              int64   `json:"id"`
	OrderID         int64   `json:"orderId"`
	Price           Decimal `json:"price"`
	Qty             Decimal `json:"qty"`
	QuoteQty        Decimal `json:"quoteQty"`
	Commission      Decimal `json:"commission"`
	CommissionAsset string  `json:"commissionAsset"`
	Time            int64   `json:"time"`
	IsBuyer         bool    `json:"isBuyer"`
	IsMaker         bool    `json:"isMaker"`
}

type IncomeRecord struct {
	Symbol     string  `json:"symbol"`
	IncomeType string  `json:"incomeType"`
	Income     Decimal `json:"income"`
	Asset      string  `json:"asset"`
	Info       string  `json:"info"`
	Time       int64   `json:"time"`
	TranID     int64   `json:"tranId"`
}

type APIErrorBody struct {
	Code    int    `json:"code"`
	Message string `json:"msg"`
}
