package bot

import (
	"log"
	"sync"
	pb "synthbull/pkg/gen/strategy"
)

// CustomPythonStrategy is an adapter that implements the Strategy interface
// for a custom Python script running in a Docker container.
type CustomPythonStrategy struct {
	runner *PythonStrategyRunner
	name   string

	// mu protects position and pnl which are accessed from multiple goroutines
	// (the strategy event loop and the status broadcast ticker).
	mu       sync.Mutex
	position float64
	pnl      float64
}

// NewCustomPythonStrategy creates a new strategy instance for a Python script.
func NewCustomPythonStrategy(cfg BotConfig) (Strategy, error) {
	runner, err := NewPythonStrategyRunner(&cfg)
	if err != nil {
		return nil, err
	}
	return &CustomPythonStrategy{
		runner: runner,
		name:   "custom_python_" + cfg.CustomScriptID,
	}, nil
}

// Name returns the name of the strategy.
func (s *CustomPythonStrategy) Name() string {
	return s.name
}

// GetPNL returns the internally tracked profit and loss.
func (s *CustomPythonStrategy) GetPNL() (float64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.pnl, nil
}

// OnMarketData is called when a market data event is received.
// It passes the data to the Python script and returns the script's decision.
func (s *CustomPythonStrategy) OnMarketData(msg IncomingMessage) []OutgoingOrder {
	// TODO: This is a placeholder implementation.
	// We need to convert the IncomingMessage to a pb.MarketTick and back.
	log.Printf("CustomPythonStrategy received market data: %+v", msg)

	// Placeholder: create a pb.MarketTick from IncomingMessage
	tick := pb.MarketTick{
		Symbol:      msg.Symbol,
		Bid:         msg.BestBid,
		Ask:         msg.BestAsk,
		LastPrice:   msg.LastTradePrice,
		TimestampNs: msg.Timestamp * 1e6,
	}

	decision, err := s.runner.OnMarketData(tick)
	if err != nil {
		log.Printf("Python strategy %s failed to process market data: %v", s.name, err)
		return nil // Return no orders on error
	}

	// Placeholder: convert pb.TradeDecision to []OutgoingOrder
	if decision.Action == pb.TradeDecision_BUY {
		return []OutgoingOrder{
			{
				Symbol:   msg.Symbol,
				Type:     "limit",
				Side:     "buy",
				Quantity: decision.Quantity,
				Price:    decision.LimitPrice,
			},
		}
	} else if decision.Action == pb.TradeDecision_SELL {
		return []OutgoingOrder{
			{
				Symbol:   msg.Symbol,
				Type:     "limit",
				Side:     "sell",
				Quantity: decision.Quantity,
				Price:    decision.LimitPrice,
			},
		}
	}

	return nil
}

// OnFill is called when an order is filled.
func (s *CustomPythonStrategy) OnFill(msg IncomingMessage) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if msg.Side == "buy" {
		s.position += msg.Quantity
		s.pnl -= msg.Price * msg.Quantity
	} else if msg.Side == "sell" {
		s.position -= msg.Quantity
		s.pnl += msg.Price * msg.Quantity
	}

	log.Printf("CustomPythonStrategy received fill: %+v. Current internal PnL: %f", msg, s.pnl)
}

// OnAck is called when an order is acknowledged.
func (s *CustomPythonStrategy) OnAck(msg IncomingMessage) {
	log.Printf("CustomPythonStrategy received ack for order on symbol: %s", msg.Symbol)
}

// Stop stops the strategy and its runner.
func (s *CustomPythonStrategy) Stop() {
	s.runner.Stop()
}
