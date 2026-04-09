package api

import (
	"encoding/json"
	"net/http"
	"synthbull/internal/bot"
	"synthbull/internal/bot/builtin"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// BotHandler handles API requests for managing trading bots.
type BotHandler struct {
	botManager *bot.BotManager
}

// NewBotHandler creates a new handler for bot operations.
func NewBotHandler(manager *bot.BotManager) *BotHandler {
	// Register built-in strategies
	manager.RegisterStrategy("market_maker", builtin.NewMarketMakerStrategy)
	return &BotHandler{botManager: manager}
}

// CreateBotRequest defines the structure for a new bot request.
type CreateBotRequest struct {
	Name           string        `json:"name"`                                                       // User-defined name for the bot
	StrategyType   string        `json:"strategy_type" binding:"required,oneof=market_maker custom"` // Strategy type
	CustomScriptID string        `json:"custom_script_id"`                                           // Required if strategy_type is 'custom'
	Symbol         string        `json:"symbol" binding:"required"`
	Params         bot.BotConfig `json:"params"`
}

// CreateBot handles the creation and starting of a new bot.
func (h *BotHandler) CreateBot(c *gin.Context) {
	var req CreateBotRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request: " + err.Error()})
		return
	}

	// Validate request for custom strategies
	if req.StrategyType == "custom" {
		if req.CustomScriptID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "custom_script_id is required for 'custom' strategy type"})
			return
		}
	}

	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User ID not found in token"})
		return
	}

	botID := uuid.New().String()
	cfg := req.Params
	cfg.ClientID = botID // Use the unique bot ID as the client ID for the exchange
	cfg.StrategyType = req.StrategyType
	cfg.CustomScriptID = req.CustomScriptID

	// Resolve script path server-side from script_id — never trust client-provided paths.
	if req.StrategyType == "custom" {
		scriptPath, err := ResolveScriptPath(req.CustomScriptID)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid or missing script: " + err.Error()})
			return
		}
		cfg.ScriptPath = scriptPath
	}

	inst, err := h.botManager.Create(userID, botID, req.StrategyType, cfg)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create bot: " + err.Error()})
		return
	}

	if err := h.botManager.Start(inst.ID); err != nil {
		// If starting fails, remove the created bot instance
		h.botManager.Remove(inst.ID)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to start bot: " + err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"id":           inst.ID,
		"bot_user_id":  inst.OwnerID,
		"bot__user_id": inst.OwnerID, // backward compatibility for existing clients
		"status":       "running",
	})
}

// StopBot handles stopping a running bot.
func (h *BotHandler) StopBot(c *gin.Context) {
	botID := c.Param("id")
	h.stopBotByID(c, botID)
}

// StopBotByBody supports POST /api/v1/bot/stop with {"bot_id":"..."} payload.
func (h *BotHandler) StopBotByBody(c *gin.Context) {
	var req struct {
		BotID string `json:"bot_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request: " + err.Error()})
		return
	}
	if req.BotID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "bot_id is required"})
		return
	}
	h.stopBotByID(c, req.BotID)
}

func (h *BotHandler) stopBotByID(c *gin.Context, botID string) {
	userID := c.GetString("user_id")

	// Verify the user owns the bot before stopping
	botInfo, err := h.botManager.Get(botID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Bot not found"})
		return
	}
	if botInfo.OwnerID != userID {
		c.JSON(http.StatusForbidden, gin.H{"error": "You do not have permission to stop this bot"})
		return
	}

	if err := h.botManager.Stop(botID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to stop bot: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "stopped"})
}

// ListBots lists all bots for the current user.
func (h *BotHandler) ListBots(c *gin.Context) {
	userID := c.GetString("user_id")
	allBots := h.botManager.List()

	userBots := make([]bot.BotInfo, 0)
	for _, b := range allBots {
		if b.OwnerID == userID {
			userBots = append(userBots, b)
		}
	}

	c.JSON(http.StatusOK, userBots)
}

// SaveStrategyRequest defines the structure for saving a bot strategy.
type SaveStrategyRequest struct {
	Name     string          `json:"name" binding:"required"`
	Strategy json.RawMessage `json:"strategy" binding:"required"`
}

// SaveStrategy saves a custom bot strategy for the user.
func (h *BotHandler) SaveStrategy(c *gin.Context) {
	var req SaveStrategyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request: " + err.Error()})
		return
	}

	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User ID not found in token"})
		return
	}

	// Call the actual manager method to save to disk
	err := h.botManager.SaveUserStrategy(userID, req.Name, req.Strategy)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save strategy: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Strategy saved successfully",
		"data": gin.H{
			"name": req.Name,
		},
	})
}

// ListStrategies returns saved strategy names for the current user.
func (h *BotHandler) ListStrategies(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User ID not found in token"})
		return
	}
	items, err := h.botManager.ListUserStrategies(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list strategies: " + err.Error()})
		return
	}
	type item struct {
		Name      string `json:"name"`
		UpdatedAt string `json:"updated_at"`
	}
	resp := make([]item, 0, len(items))
	for _, s := range items {
		resp = append(resp, item{Name: s.Name, UpdatedAt: s.UpdatedAt.Format("2006-01-02T15:04:05Z07:00")})
	}
	c.JSON(http.StatusOK, gin.H{"data": resp})
}

// GetStrategy returns one saved strategy by name for the current user.
func (h *BotHandler) GetStrategy(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User ID not found in token"})
		return
	}
	name := c.Param("name")
	if name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "strategy name is required"})
		return
	}
	strategyJSON, err := h.botManager.LoadUserStrategy(userID, name)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Strategy not found"})
		return
	}
	var strategy interface{}
	if err := json.Unmarshal(strategyJSON, &strategy); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Stored strategy is invalid JSON"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"data": gin.H{
			"name":     name,
			"strategy": strategy,
		},
	})
}
