package config

import (
	"errors"
	"os"
	"strconv"
	"time"
)

// Config holds all configuration for the synthetic-worlds service.
type Config struct {
	Env    string
	Server ServerConfig
	DB     DatabaseConfig
	Redis  RedisConfig
	LLM    LLMConfig
	Auth   AuthConfig
	World  WorldConfig
}

type ServerConfig struct {
	Port            int
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	ShutdownTimeout time.Duration
}

type DatabaseConfig struct {
	PostgresURL   string
	ClickHouseURL string // Optional: if set, call logs go to ClickHouse
}

type RedisConfig struct {
	URL string
}

type LLMConfig struct {
	AnthropicKey string
	OpenAIKey    string
	XAIKey       string
	DefaultModel string
}

type AuthConfig struct {
	Mode   string // "static" or "service"
	APIKey string // Static API key for standalone mode
}

type WorldConfig struct {
	TTLSeconds   int
	MaxGenTokens int
}

// Load loads configuration from environment variables.
func Load() (*Config, error) {
	cfg := &Config{
		Env: envOrDefault("ENV", "development"),
		Server: ServerConfig{
			Port:            envIntOrDefault("PORT", 7878),
			ReadTimeout:     30 * time.Second,
			WriteTimeout:    120 * time.Second,
			ShutdownTimeout: 30 * time.Second,
		},
		DB: DatabaseConfig{
			PostgresURL:   envOrDefault("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/synthetic_worlds?sslmode=disable"),
			ClickHouseURL: os.Getenv("CLICKHOUSE_URL"),
		},
		Redis: RedisConfig{
			URL: envOrDefault("REDIS_URL", "redis://localhost:6379/0"),
		},
		LLM: LLMConfig{
			AnthropicKey: os.Getenv("ANTHROPIC_API_KEY"),
			OpenAIKey:    os.Getenv("OPENAI_API_KEY"),
			XAIKey:       os.Getenv("XAI_API_KEY"),
			DefaultModel: envOrDefault("DEFAULT_MODEL", "claude-sonnet-4-6"),
		},
		Auth: AuthConfig{
			Mode:   envOrDefault("AUTH_MODE", "static"),
			APIKey: os.Getenv("SYNTH_API_KEY"),
		},
		World: WorldConfig{
			TTLSeconds:   envIntOrDefault("WORLD_TTL_SECONDS", 3600),
			MaxGenTokens: envIntOrDefault("MAX_GEN_TOKENS", 2048),
		},
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

// Validate checks required configuration.
func (c *Config) Validate() error {
	var errs []error

	if c.DB.PostgresURL == "" {
		errs = append(errs, errors.New("DATABASE_URL is required"))
	}
	if c.Redis.URL == "" {
		errs = append(errs, errors.New("REDIS_URL is required"))
	}
	if c.Auth.Mode == "static" && c.Auth.APIKey == "" {
		errs = append(errs, errors.New("SYNTH_API_KEY is required when AUTH_MODE=static"))
	}
	if c.LLM.AnthropicKey == "" && c.LLM.OpenAIKey == "" && c.LLM.XAIKey == "" {
		errs = append(errs, errors.New("at least one LLM API key is required (ANTHROPIC_API_KEY, OPENAI_API_KEY, or XAI_API_KEY)"))
	}
	if c.Server.Port < 1 || c.Server.Port > 65535 {
		errs = append(errs, errors.New("PORT must be between 1 and 65535"))
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}

func envOrDefault(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}

func envIntOrDefault(key string, defaultVal int) int {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return defaultVal
}
