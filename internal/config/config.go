package config

import (
	"fmt"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	Port          string
	DatabaseURL   string
	BaseURL       string
	SMTPHost      string
	SMTPPort      string
	SMTPUser      string
	SMTPPass      string
	SMTPFrom      string
	SessionSecret string
	Env           string // "dev" or "prod"
	Vector        VectorConfig
}

type VectorConfig struct {
	Enabled            bool
	EmbeddingDimension int
	TableName          string
}

func Load() (*Config, error) {
	_ = godotenv.Load()

	cfg := &Config{
		Port:          getEnv("PORT", "8080"),
		DatabaseURL:   getEnv("DATABASE_URL", "./goth.db"),
		BaseURL:       getEnv("BASE_URL", "http://localhost:8080"),
		SMTPHost:      getEnv("SMTP_HOST", "localhost"),
		SMTPPort:      getEnv("SMTP_PORT", "1025"),
		SMTPUser:      os.Getenv("SMTP_USER"),
		SMTPPass:      os.Getenv("SMTP_PASS"),
		SMTPFrom:      getEnv("SMTP_FROM", "noreply@goth.com"),
		SessionSecret: os.Getenv("SESSION_SECRET"),
		Env:           getEnv("APP_ENV", "dev"),
		Vector: VectorConfig{
			Enabled:            getEnvAsBool("VECTOR_ENABLED", false),
			EmbeddingDimension: getEnvAsInt("VECTOR_EMBEDDING_DIMENSION", 1536),
			TableName:          getEnv("VECTOR_TABLE_NAME", "vectors"),
		},
	}

	// Validação Estrita para Produção
	if cfg.Env == "prod" {
		if cfg.SMTPPass == "" {
			return nil, fmt.Errorf("produção: SMTP_PASS é obrigatório")
		}
		if cfg.SMTPUser == "" {
			return nil, fmt.Errorf("produção: SMTP_USER é obrigatório")
		}
		if cfg.SessionSecret == "" {
			return nil, fmt.Errorf("produção: SESSION_SECRET é obrigatório")
		}
	} else {
		// No dev, se não houver secret, usamos um valor fraco apenas para não quebrar o boot
		if cfg.SessionSecret == "" {
			cfg.SessionSecret = "dev-secret-keep-it-simple-but-not-safe"
		}
	}

	return cfg, nil
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

func getEnvAsBool(key string, fallback bool) bool {
	if value, ok := os.LookupEnv(key); ok {
		return value == "true" || value == "1" || value == "yes"
	}
	return fallback
}

func getEnvAsInt(key string, fallback int) int {
	if value, ok := os.LookupEnv(key); ok {
		var result int
		fmt.Sscanf(value, "%d", &result)
		return result
	}
	return fallback
}
