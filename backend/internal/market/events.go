package market

// EventSource identifies who produced a book-style update when multiple producers
// fan into the same event-bus channel (e.g. GBM vs market-maker).
const (
	// SourceGBMUntracked is synthetic BBO derived from the latent GBM path each tick.
	SourceGBMUntracked = "gbm_untracked"
	// SourceMMTracked is reserved for when your market maker publishes tracked LOB updates.
	SourceMMTracked = "mm_tracked"
)

// BasePriceSample is the GBM latent level S_t after each tick (before order placement).
// Intended for a candle-builder or analytics channel (JSON-ready for your event bus).
type BasePriceSample struct {
	Symbol    string  `json:"symbol"`
	Timestamp int64   `json:"timestamp_ms"`
	Price     float64 `json:"price"` // same units as Config.InitialPrice (e.g. INR)
}

// GBMReferenceBook is a tight synthetic bid/ask around the GBM mid using MinSpread/2
// on each side (same geometry as submitOrders’ centering on basePrice). It is not the
// full matching-engine order book (which mixes all participants).
type GBMReferenceBook struct {
	Symbol       string `json:"symbol"`
	Timestamp    int64  `json:"timestamp_ms"`
	Source       string `json:"source"` // SourceGBMUntracked
	BestBidCents int    `json:"best_bid_cents"`
	BestAskCents int    `json:"best_ask_cents"`
	MidCents     int    `json:"mid_cents"`
}
