package config

import (
	"fmt"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	DBURL     string
	RedisURL  string
	JWTSecret []byte
	SMTPHost  string
	SMTPPort  string
	SMTPUser  string
	SMTPPass  string
}

func LoadConfig() (*Config, error) {
	// Load .env file if it exists, but don't error out if it doesn't (we could be in Docker)
	_ = godotenv.Load()

	dbURL := os.Getenv("DB_URL")
	if dbURL == "" {
		// Fall back to individual vars — used by docker-compose and local dev.
		host := os.Getenv("DB_HOST")
		port := os.Getenv("DB_PORT")
		user := os.Getenv("DB_USER")
		password := os.Getenv("DB_PASSWORD")
		name := os.Getenv("DB_NAME")

		if host == "" || user == "" || password == "" || name == "" {
			return nil, fmt.Errorf("either DB_URL or all of DB_HOST/DB_PORT/DB_USER/DB_PASSWORD/DB_NAME must be set")
		}
		if port == "" {
			port = "5432"
		}
		dbURL = fmt.Sprintf("postgres://%s:%s@%s:%s/%s", user, password, host, port, name)
	}

	jwtSecretStr := os.Getenv("JWT_SECRET")
	if jwtSecretStr == "" {
		jwtSecretStr = "synthbull_fallback_secret_change_me"
	}

	redisURL := os.Getenv("REDIS_URL")
	if redisURL == "" {
		redisURL = "redis://localhost:6379/0"
	}

	return &Config{
		DBURL:     dbURL,
		RedisURL:  redisURL,
		JWTSecret: []byte(jwtSecretStr),
		SMTPHost:  os.Getenv("SMTP_HOST"),
		SMTPPort:  os.Getenv("SMTP_PORT"),
		SMTPUser:  os.Getenv("SMTP_USER"),
		SMTPPass:  os.Getenv("SMTP_PASS"),
	}, nil
}
