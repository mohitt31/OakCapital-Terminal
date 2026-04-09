package bot

// Strategy defines the contract all bot strategies must implement.
// Built-in strategies (market maker, alpha, simulation) and any future
// custom bots all satisfy this interface.
type Strategy interface {
	// Name returns the strategy identifier (e.g. "market_maker", "alpha", "simulation").
	Name() string

	// GetPNL returns the current profit and loss for the strategy.
	GetPNL() (float64, error)

	// OnMarketData is called for each market_data tick.
	// Returns a slice of orders to send (can be empty).
	OnMarketData(msg IncomingMessage) []OutgoingOrder

	// OnFill is called when one of the bot's orders is filled.
	OnFill(msg IncomingMessage)

	// OnAck is called when the engine acknowledges or rejects an order.
	OnAck(msg IncomingMessage)
}

// StrategyFactory creates a new Strategy instance with the given config.
type StrategyFactory func(cfg BotConfig) (Strategy, error)
