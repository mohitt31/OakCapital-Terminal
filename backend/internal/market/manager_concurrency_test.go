package market

import (
	"fmt"
	"math/rand"
	"sync"
	"testing"
	"time"

	"synthbull/internal/engine"
)

func TestEngineConcurrency(t *testing.T) {
	mgr := NewManager()
	defer mgr.CloseAll()

	const numSymbols = 10
	symbols := make([]*SymbolInfo, numSymbols)
	for k := 0; k < numSymbols; k++ {
		sym := fmt.Sprintf("CONCUR_%d", k)
		mgr.AddSymbol(sym, Stock, DefaultConfig())
		info := mgr.GetSymbol(sym)
		if info == nil {
			t.Fatalf("Symbol %s not found", sym)
		}
		symbols[k] = info
	}

	const numBots = 50
	const ordersPerBot = 20

	var startWg sync.WaitGroup
	var doneWg sync.WaitGroup
	startWg.Add(1)

	for i := 0; i < numBots; i++ {
		doneWg.Add(1)
		botID := i
		info := symbols[botID%numSymbols]

		go func() {
			defer doneWg.Done()
			startWg.Wait()

			r := rand.New(rand.NewSource(int64(botID)))
			for j := 0; j < ordersPerBot; j++ {
				side := engine.SideBuy
				if r.Float32() > 0.5 {
					side = engine.SideSell
				}

				price := 10000 + int(r.Float32()*2000-1000)
				qty := int(r.Float32()*10) + 1
				orderID := botID*1000 + j + 1

				info.Mu.Lock()
				if r.Float32() > 0.8 {
					info.Book.Market(orderID, side, qty)
				} else {
					info.Book.AddLimit(orderID, side, qty, price)
				}
				info.Mu.Unlock()

				time.Sleep(time.Microsecond * 10)
			}
		}()
	}

	startWg.Done()
	doneWg.Wait()

	firstBook := symbols[0]
	firstBook.Mu.Lock()
	depth := firstBook.Book.GetDepth()
	firstBook.Mu.Unlock()

	t.Logf("Concurrency test across %d distinct symbols finished successfully.", numSymbols)
	t.Logf("First Book -> Bids: %d, Asks: %d", len(depth.Bids), len(depth.Asks))
}
