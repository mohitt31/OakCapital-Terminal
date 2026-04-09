package models

import "time"

// User represents a trading platform user.
type User struct {
	ID           string    `json:"id" db:"id"` // UUID
	Username     string    `json:"username" db:"username"`
	Email        string    `json:"email" db:"email"`
	PasswordHash string    `json:"-" db:"password_hash"` // Never serialize to JSON
	CreatedAt    time.Time `json:"created_at" db:"created_at"`
}
