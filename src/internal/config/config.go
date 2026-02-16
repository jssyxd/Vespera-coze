package config

import (
	"fmt"
	"os"
	"strconv"
)

type DatabaseConfig struct {
	Host     string
	Port     int
	User     string
	Password string
	DBName   string
	SSLMode  string
}

type AIConfig struct {
	APIKey   string
	BaseURL  string
	Models   map[string]ModelConfig
}

type ModelConfig struct {
	Name    string
	Timeout int
}

type Config struct {
	Database DatabaseConfig
	AI       AIConfig
}

func Load() (*Config, error) {
	cfg := &Config{
		Database: DatabaseConfig{
			Host:     getEnv("SUPABASE_HOST", "localhost"),
			Port:     getEnvAsInt("SUPABASE_PORT", 5432),
			User:     getEnv("SUPABASE_USER", "postgres"),
			Password: getEnv("SUPABASE_PASSWORD", ""),
			DBName:   getEnv("SUPABASE_DB", "postgres"),
			SSLMode:  getEnv("SUPABASE_SSLMODE", "require"),
		},
		AI: AIConfig{
			APIKey:  getEnv("AI_API_KEY", ""),
			BaseURL: getEnv("AI_BASE_URL", "http://139.224.113.163:8317/v1"),
			Models: map[string]ModelConfig{
				"deepseek": {
					Name:    "deepseek-v3.2",
					Timeout: 300,
				},
				"glm": {
					Name:    "glm-4.7",
					Timeout: 300,
				},
				"minimax": {
					Name:    "minimax-m2.1",
					Timeout: 120,
				},
				"kimi": {
					Name:    "kimi-k2.5",
					Timeout: 300,
				},
			},
		},
	}

	if err := cfg.validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

func (c *Config) validate() error {
	if c.Database.Password == "" {
		return fmt.Errorf("SUPABASE_PASSWORD is required")
	}
	if c.AI.APIKey == "" {
		return fmt.Errorf("AI_API_KEY is required")
	}
	return nil
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvAsInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intVal, err := strconv.Atoi(value); err == nil {
			return intVal
		}
	}
	return defaultValue
}
