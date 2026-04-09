package auth

import (
	"context"
	"errors"
	"fmt"
	"time"

	jwt "github.com/golang-jwt/jwt/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

type User struct {
	ID           string
	Username     string
	Email        string
	PasswordHash string
	CreatedAt    time.Time
}

type Service struct {
	db        *pgxpool.Pool
	jwtSecret []byte
	emailSvc  *EmailService
}

func NewService(db *pgxpool.Pool, secret []byte, es *EmailService) *Service {
	return &Service{
		db:        db,
		jwtSecret: secret,
		emailSvc:  es,
	}
}

func (s *Service) GetJWTSecret() []byte {
	return s.jwtSecret
}

type Claims struct {
	UserID   string `json:"user_id"`
	Username string `json:"username"`
	Role     string `json:"role"`
	jwt.RegisteredClaims
}

func (s *Service) HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(bytes), err
}

func (s *Service) CheckPasswordHash(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

func (s *Service) GenerateToken(userID string, username, role string) (string, error) {
	expirationTime := time.Now().Add(24 * time.Hour)
	claims := &Claims{
		UserID:   userID,
		Username: username,
		Role:     role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expirationTime),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(s.jwtSecret)
}

func (s *Service) ValidateToken(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return s.jwtSecret, nil
	})
	if err != nil {
		return nil, err
	}
	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		return claims, nil
	}
	return nil, errors.New("invalid token")
}

func (s *Service) UserExists(ctx context.Context, userID string) (bool, error) {
	if s.db == nil {
		return false, errors.New("database connection not initialized")
	}
	var exists bool
	err := s.db.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM users WHERE id::text = $1)", userID).Scan(&exists)
	if err != nil {
		return false, err
	}
	return exists, nil
}

func (s *Service) Register(username, password, email string) (string, error) {
	if s.db == nil {
		return "", errors.New("database connection not initialized")
	}

	hash, err := s.HashPassword(password)
	if err != nil {
		return "", errors.New("failed to hash password")
	}

	// 1. START TRANSACTION
	tx, err := s.db.Begin(context.Background())
	if err != nil {
		return "", errors.New("failed to start database transaction")
	}
	// Defer a rollback in case anything panics or returns an error before committing
	defer tx.Rollback(context.Background())

	var userID string
	err = tx.QueryRow(context.Background(),
		`INSERT INTO users (username, password_hash, email)
		 VALUES ($1, $2, $3) RETURNING id`,
		username, hash, email,
	).Scan(&userID)

	if err != nil {
		// e.g. unique constraint violation
		return "", errors.New("username already exists or database conflict occurred")
	}

	// Create initial portfolio for user with the $100k
	_, err = tx.Exec(context.Background(),
		`INSERT INTO portfolios (user_id, name, total_cash, available_cash, blocked_cash, margin_locked)
		 VALUES ($1, $2, 100000.0, 100000.0, 0, 0)`,
		userID, "Default",
	)
	if err != nil {
		return "", fmt.Errorf("failed to initialize portfolio: %w", err)
	}

	// 2. COMMIT TRANSACTION (saves both the user and portfolio safely)
	if err = tx.Commit(context.Background()); err != nil {
		return "", errors.New("failed to commit transaction")
	}

	return s.GenerateToken(userID, username, "HUMAN")
}

func (s *Service) Login(username, password string) (string, error) {
	if s.db == nil {
		return "", errors.New("database connection not initialized")
	}

	var userID string
	var hash string

	err := s.db.QueryRow(context.Background(),
		`SELECT id, password_hash FROM users WHERE username = $1`,
		username,
	).Scan(&userID, &hash)

	if err != nil {
		if err == pgx.ErrNoRows {
			return "", errors.New("invalid credentials")
		}
		return "", errors.New("database error during login")
	}

	if !s.CheckPasswordHash(password, hash) {
		return "", errors.New("invalid credentials")
	}

	// Ensure portfolio row exists for legacy users created before portfolio bootstrap.
	_, err = s.db.Exec(context.Background(), `
		INSERT INTO portfolios (user_id, name, total_cash, available_cash, blocked_cash, margin_locked)
		SELECT $1::uuid, 'Default', 100000.0, 100000.0, 0, 0
		WHERE NOT EXISTS (
			SELECT 1 FROM portfolios WHERE user_id = $1::uuid
		)
	`, userID)
	if err != nil {
		return "", errors.New("failed to initialize portfolio")
	}

	return s.GenerateToken(userID, username, "HUMAN")
}

func (s *Service) ChangePassword(userID string, oldPassword, newPassword string) error {
	var hash string
	err := s.db.QueryRow(context.Background(), "SELECT password_hash FROM users WHERE id = $1", userID).Scan(&hash)
	if err != nil {
		return errors.New("user not found")
	}

	if !s.CheckPasswordHash(oldPassword, hash) {
		return errors.New("invalid old password")
	}

	newHash, err := s.HashPassword(newPassword)
	if err != nil {
		return errors.New("failed to hash new password")
	}

	_, err = s.db.Exec(context.Background(), "UPDATE users SET password_hash = $1 WHERE id = $2", newHash, userID)
	return err
}

func (s *Service) ForgotPassword(username, emailAddr string) (string, error) {
	var userID string
	err := s.db.QueryRow(context.Background(), "SELECT id FROM users WHERE username = $1 AND email = $2", username, emailAddr).Scan(&userID)
	if err != nil {
		if err == pgx.ErrNoRows {
			return "", errors.New("user not found or email mismatch")
		}
		return "", err
	}

	// Generate a special "reset JWT" that only works for resetting password.
	expirationTime := time.Now().Add(15 * time.Minute)
	claims := &Claims{
		UserID: userID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expirationTime),
			Subject:   "password_reset",
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	resetToken, err := token.SignedString(s.jwtSecret)
	if err != nil {
		return "", errors.New("failed to generate reset token")
	}

	// Send email with the reset token
	if s.emailSvc != nil {
		err = s.emailSvc.SendPasswordResetEmail(emailAddr, resetToken)
		if err != nil {
			return "", errors.New("failed to send reset email")
		}
	} else {
		return "", errors.New("email service not configured")
	}

	return resetToken, nil
}

func (s *Service) ResetPassword(token, newPassword string) error {
	claims, err := s.ValidateToken(token)
	if err != nil {
		return errors.New("invalid or expired reset token")
	}

	// Verify this is actually a reset token
	if claims.RegisteredClaims.Subject != "password_reset" {
		return errors.New("invalid token type")
	}

	newHash, err := s.HashPassword(newPassword)
	if err != nil {
		return errors.New("failed to hash new password")
	}

	_, err = s.db.Exec(context.Background(), "UPDATE users SET password_hash = $1 WHERE id = $2", newHash, claims.UserID)
	return err
}

func (s *Service) ResetPasswordByEmail(email, newPassword string) error {
	var userID string
	err := s.db.QueryRow(context.Background(), "SELECT id FROM users WHERE email = $1", email).Scan(&userID)
	if err != nil {
		if err == pgx.ErrNoRows {
			return errors.New("user with this email not found")
		}
		return err
	}

	newHash, err := s.HashPassword(newPassword)
	if err != nil {
		return errors.New("failed to hash new password")
	}

	_, err = s.db.Exec(context.Background(), "UPDATE users SET password_hash = $1 WHERE id = $2", newHash, userID)
	return err
}

func (s *Service) RequestDeleteAccount(userID string) (string, error) {
	var emailAddr string
	err := s.db.QueryRow(context.Background(), "SELECT email FROM users WHERE id = $1", userID).Scan(&emailAddr)
	if err != nil {
		return "", errors.New("user not found")
	}

	expirationTime := time.Now().Add(15 * time.Minute)
	claims := &Claims{
		UserID: userID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expirationTime),
			Subject:   "delete_account",
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	deleteToken, err := token.SignedString(s.jwtSecret)
	if err != nil {
		return "", errors.New("failed to generate confirmation token")
	}

	if s.emailSvc != nil {
		err = s.emailSvc.SendDeleteAccountEmail(emailAddr, deleteToken)
		if err != nil {
			return "", errors.New("failed to send confirmation email")
		}
	} else {
		return "", errors.New("email service not configured")
	}

	return deleteToken, nil
}

func (s *Service) DeleteAccount(userID, token string) error {
	claims, err := s.ValidateToken(token)
	if err != nil {
		return errors.New("invalid or expired confirmation token")
	}
	if claims.RegisteredClaims.Subject != "delete_account" {
		return errors.New("invalid token type")
	}
	if claims.UserID != userID {
		return errors.New("token does not belong to this user")
	}

	// Let the database's ON DELETE CASCADE handle associated records like portfolios.
	result, err := s.db.Exec(context.Background(), "DELETE FROM users WHERE id = $1", userID)
	if err != nil {
		return err
	}

	rowsAffected := result.RowsAffected()
	if rowsAffected == 0 {
		return errors.New("user not found")
	}

	return nil
}
