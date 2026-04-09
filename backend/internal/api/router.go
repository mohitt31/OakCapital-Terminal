package api

import (
	"net/url"
	"strings"
	"synthbull/internal/api/ws"
	"synthbull/internal/auth"
	"synthbull/internal/bot"
	"synthbull/internal/db"
	"synthbull/internal/eventbus"
	"synthbull/internal/market"
	"synthbull/internal/portfolio"
	"synthbull/internal/simbot"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

// SetupRouter configures and returns the main Gin engine with all routes registered.
func SetupRouter(
	mktMgr *market.Manager,
	portfolioMgr *portfolio.Manager,
	botMgr *bot.BotManager,
	authSvc *auth.Service,
	wsHub *ws.Hub,
	candleRepo *db.CandleRepo,
	simbotMgr *simbot.BotManager,
	bookManager *BookManager,
	busPub *eventbus.Publisher,
	watchlistRepo *db.WatchlistRepo,
	limitTracker *LimitOrderTracker,
	userBotPnL *db.UserBotPnLRepo,
	leaderboardRepo *db.LeaderboardRepo,
) *gin.Engine {
	r := gin.Default()

	// Global CORS middleware (explicit to avoid frontend preflight failures)
	r.Use(cors.New(cors.Config{
		AllowOriginFunc: func(origin string) bool {
			// Accept localhost on any port in dev (covers Vite 5173, docker 3976, etc.)
			u, err := url.Parse(origin)
			if err != nil || u.Scheme == "" || u.Host == "" {
				return false
			}
			host := strings.ToLower(u.Hostname())
			return host == "localhost" || host == "127.0.0.1" || host == "::1"
		},
		AllowMethods: []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders: []string{
			"Origin", "Content-Type", "Content-Length", "Accept", "Authorization",
			"X-Requested-With", "X-User-Id",
		},
		ExposeHeaders:    []string{"Content-Length", "Content-Type"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	authHandler := auth.NewHandler(authSvc)
	wsHandler := ws.NewHandler(wsHub, authSvc)
	orderHandler := NewOrderHandler(bookManager, mktMgr, portfolioMgr, authSvc, busPub, limitTracker)

	// -------------------------------------------------------------------------
	// API v1 Routes
	// -------------------------------------------------------------------------
	v1 := r.Group("/api/v1")
	{
		// Public Auth
		v1.POST("/auth/register", authHandler.Register)
		v1.POST("/auth/login", authHandler.Login)
		v1.POST("/auth/forgot-password", authHandler.ForgotPassword)
		v1.POST("/auth/reset-password", authHandler.ResetPassword)

		// Public Data
		v1.GET("/book/info", orderHandler.GetBookInfo)
		v1.GET("/book/depth", orderHandler.GetBookDepth)
		v1.GET("/book/list", orderHandler.ListBooks)
		if candleRepo != nil {
			candleHandler := NewCandleHandler(candleRepo)
			v1.GET("/candles", candleHandler.GetCandles)
		}

		// Protected Routes
		protected := v1.Group("/")
		protected.Use(auth.JWTMiddleware(authSvc))
		{
			// Account Management
			protected.POST("/auth/change-password", authHandler.ChangePassword)
			protected.POST("/auth/request-delete", authHandler.RequestDeleteAccount)
			protected.DELETE("/auth/account", authHandler.DeleteAccount)

			// Orders
			protected.POST("/order/limit/add", orderHandler.PlaceLimitOrder)
			protected.POST("/order/limit/cancel", orderHandler.CancelLimit)
			protected.POST("/order/limit/modify", orderHandler.ModifyLimit)
			protected.POST("/order/market", orderHandler.PlaceMarketOrder)
			protected.POST("/order/stop/add", orderHandler.AddStop)
			protected.POST("/order/stop/cancel", orderHandler.CancelStop)
			protected.POST("/order/stop/modify", orderHandler.ModifyStop)
			protected.POST("/order/stop-limit/add", orderHandler.AddStopLimit)
			protected.POST("/order/stop-limit/cancel", orderHandler.CancelStopLimit)
			protected.POST("/order/stop-limit/modify", orderHandler.ModifyStopLimit)
			protected.GET("/orders", orderHandler.ListOpenOrders)

			// Portfolio
			portfolioHandler := NewPortfolioHandler(portfolioMgr, userBotPnL)
			protected.GET("/portfolio", portfolioHandler.GetPortfolio)
			protected.GET("/portfolio/bot-pnl", portfolioHandler.GetBotPnLBreakdown)

			leaderboardHandler := NewLeaderboardHandler(leaderboardRepo)
			protected.GET("/leaderboard", leaderboardHandler.Get)
			protected.POST("/leaderboard/publish", leaderboardHandler.Publish)

			// Watchlist
			if watchlistRepo != nil {
				watchlistHandler := NewWatchlistHandler(watchlistRepo)
				protected.GET("/watchlist", watchlistHandler.List)
				protected.POST("/watchlist", watchlistHandler.Add)
				protected.DELETE("/watchlist", watchlistHandler.Remove)
			}

			// Bots
			botHandler := NewBotHandler(botMgr)
			protected.POST("/bot/start", botHandler.CreateBot)
			protected.POST("/bot/stop/:id", botHandler.StopBot)
			protected.POST("/bot/stop", botHandler.StopBotByBody)
			protected.GET("/bots", botHandler.ListBots)
			protected.POST("/bots/strategy", botHandler.SaveStrategy)
			protected.GET("/bots/strategies", botHandler.ListStrategies)
			protected.GET("/bots/strategies/:name", botHandler.GetStrategy)

			// Script Uploads
			botScriptHandler := NewBotScriptHandler()
			protected.POST("/bots/scripts/upload", botScriptHandler.UploadCustomScript)

			// Simulation Bots
			if simbotMgr != nil {
				simHandler := NewSimHandler(simbotMgr, userBotPnL)
				protected.POST("/simbot/start", simHandler.StartBot)
				protected.POST("/simbot/stop", simHandler.StopBot)
				protected.GET("/simbot/status", simHandler.GetBotStatus)
				protected.GET("/simbot/list", simHandler.ListBots)
				protected.POST("/flagship/start", simHandler.StartFlagshipBot)
				// Backward-compatible aliases
				protected.GET("/bot/status", simHandler.GetBotStatus)
				protected.GET("/bot/list", simHandler.ListBots)
			}

			// Market Control
			marketHandler := NewMarketHandler(mktMgr)
			protected.GET("/market/symbols", marketHandler.ListSymbols)
			protected.GET("/market/symbols/:symbol", marketHandler.GetSymbolDetails)
			protected.POST("/market/symbols/:symbol/start", marketHandler.StartSymbol)
			protected.POST("/market/symbols/:symbol/stop", marketHandler.StopSymbol)
		}
	}

	// Health check (unprotected)
	r.GET("/health", orderHandler.Health)
	r.POST("/internal/simbot/order/market", orderHandler.PlaceInternalMarketOrder)

	// WebSocket (top-level as it has its own auth logic)
	r.GET("/ws", wsHandler.HandleConnection)
	r.GET("/ws/internal", wsHandler.HandleInternalConnection)

	return r
}
