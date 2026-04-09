package db

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
)

type BotStrategy struct {
	ID           uuid.UUID       `json:"id"`
	UserID       uuid.UUID       `json:"user_id"`
	Name         string          `json:"name"`
	StrategyData json.RawMessage `json:"strategy_data"`
	CreatedAt    string          `json:"created_at"`
	UpdatedAt    string          `json:"updated_at"`
}

func SaveBotStrategy(ctx context.Context, userID uuid.UUID, name string, strategyData json.RawMessage) (*BotStrategy, error) {
	query := `
		INSERT INTO bot_strategies (user_id, name, strategy_data)
		VALUES ($1, $2, $3)
		ON CONFLICT (user_id, name) DO UPDATE SET
			strategy_data = EXCLUDED.strategy_data,
			updated_at = NOW()
		RETURNING id, user_id, name, strategy_data, created_at, updated_at
	`

	var strategy BotStrategy
	err := Pool.QueryRow(ctx, query, userID, name, strategyData).Scan(
		&strategy.ID,
		&strategy.UserID,
		&strategy.Name,
		&strategy.StrategyData,
		&strategy.CreatedAt,
		&strategy.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	return &strategy, nil
}
