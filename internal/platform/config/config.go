package config

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/joho/godotenv"
	"gopkg.in/yaml.v3"
)

type Config struct {
	Port               string
	DatabaseURL        string
	SMTPHost           string
	SMTPPort           string
	SMTPUser           string
	SMTPPass           string
	SMTPFrom           string
	SessionSecret      string
	Env                string
	AsaasAPIKey        string
	AsaasEnvironment   string
	AsaasWebhookToken  string
	AsaasHmacSecret    string
	GoogleClientID     string
	GoogleClientSecret string
	AppURL             string
	ReadTimeout        int
	WriteTimeout       int
	// HTTP Server timeouts
	ReadHeaderTimeout int
	IdleTimeout       int
	LogLevel          string
	// Security
	PasswordPepper string
	// OpenTelemetry
	OtelEndpoint    string
	OtelProtocol    string
	OtelServiceName string
	// Database connection pool settings
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime int
	// Worker pool settings
	WorkerCount   int
	CPUMultiplier float64
	MinWorkers    int
	MaxWorkers    int
}

type YAMLConfig struct {
	Environments map[string]EnvironmentConfig `yaml:"environments"`
	Defaults     DefaultsConfig               `yaml:"defaults"`
}

type EnvironmentConfig struct {
	Server   ServerConfig   `yaml:"server"`
	Database DatabaseConfig `yaml:"database"`
	Logging  LoggingConfig  `yaml:"logging"`
	Session  SessionConfig  `yaml:"session"`
	SMTP     SMTPConfig     `yaml:"smtp"`
	Features FeaturesConfig `yaml:"features"`
	Workers  WorkersConfig  `yaml:"workers"`
}

type ServerConfig struct {
	Port              string `yaml:"port"`
	ReadTimeout       int    `yaml:"read_timeout"`
	WriteTimeout      int    `yaml:"write_timeout"`
	ReadHeaderTimeout int    `yaml:"read_header_timeout"`
	IdleTimeout       int    `yaml:"idle_timeout"`
}

type DatabaseConfig struct {
	URL             string `yaml:"url"`
	MaxOpenConns    int    `yaml:"max_open_conns"`
	MaxIdleConns    int    `yaml:"max_idle_conns"`
	ConnMaxLifetime int    `yaml:"conn_max_lifetime"`
}

type LoggingConfig struct {
	Level  string `yaml:"level"`
	Format string `yaml:"format"`
}

type SessionConfig struct {
	Secret        string `yaml:"secret"`
	LifetimeHours int    `yaml:"lifetime_hours"`
}

type SMTPConfig struct {
	Host string `yaml:"host"`
	Port string `yaml:"port"`
	From string `yaml:"from"`
}

type WorkersConfig struct {
	Count         int     `yaml:"count"`
	CPUMultiplier float64 `yaml:"cpu_multiplier"`
	MinWorkers    int     `yaml:"min_workers"`
	MaxWorkers    int     `yaml:"max_workers"`
}

type FeaturesConfig struct {
	Swagger   bool `yaml:"swagger"`
	Metrics   bool `yaml:"metrics"`
	Profiling bool `yaml:"profiling"`
}

type DefaultsConfig struct {
	AppURL     string           `yaml:"app_url"`
	Asaas      AsaasDefaults    `yaml:"asaas"`
	OAuth      OAuthDefaults    `yaml:"oauth"`
	Monitoring MonitoringConfig `yaml:"monitoring"`
}

type AsaasDefaults struct {
	Environment  string `yaml:"environment"`
	WebhookToken string `yaml:"webhook_token"`
}

type OAuthDefaults struct {
	Google GoogleOAuthDefaults `yaml:"google"`
}

type GoogleOAuthDefaults struct {
	ClientID     string `yaml:"client_id"`
	ClientSecret string `yaml:"client_secret"`
}

type MonitoringConfig struct {
	HealthCheckInterval int `yaml:"health_check_interval"`
	MetricsInterval     int `yaml:"metrics_interval"`
}

func Load() (*Config, error) {
	_ = godotenv.Load()

	env := getEnv("APP_ENV", "dev")

	yamlCfg, defaults, err := loadYAMLConfig(env)
	if err != nil {
		return nil, fmt.Errorf("falha ao carregar config.yaml: %w", err)
	}

	defaultsCfg := defaults
	if defaultsCfg == nil {
		defaultsCfg = getDefaults()
	}

	cfg := &Config{
		Port:               getEnv("PORT", yamlCfg.Server.Port),
		DatabaseURL:        getEnv("DATABASE_URL", yamlCfg.Database.URL),
		SMTPHost:           resolveEnvVar(yamlCfg.SMTP.Host, "localhost"),
		SMTPPort:           resolveEnvVar(yamlCfg.SMTP.Port, "1025"),
		SMTPUser:           os.Getenv("SMTP_USER"),
		SMTPPass:           os.Getenv("SMTP_PASS"),
		SMTPFrom:           getEnv("SMTP_FROM", yamlCfg.SMTP.From),
		SessionSecret:      os.Getenv("SESSION_SECRET"),
		PasswordPepper:     os.Getenv("PASSWORD_PEPPER"),
		Env:                env,
		AsaasAPIKey:        os.Getenv("ASAAS_API_KEY"),
		AsaasEnvironment:   getEnv("ASAAS_ENVIRONMENT", defaultsCfg.AppURL),
		AsaasWebhookToken:  os.Getenv("ASAAS_WEBHOOK_TOKEN"),
		AsaasHmacSecret:    os.Getenv("ASAAS_HMAC_SECRET"),
		GoogleClientID:     os.Getenv("GOOGLE_CLIENT_ID"),
		GoogleClientSecret: os.Getenv("GOOGLE_CLIENT_SECRET"),
		AppURL:             getEnv("APP_URL", defaultsCfg.AppURL),
		ReadTimeout:        yamlCfg.Server.ReadTimeout,
		WriteTimeout:       yamlCfg.Server.WriteTimeout,
		ReadHeaderTimeout:  yamlCfg.Server.ReadHeaderTimeout,
		IdleTimeout:        yamlCfg.Server.IdleTimeout,
		LogLevel:           yamlCfg.Logging.Level,
		// OpenTelemetry
		OtelEndpoint:    os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"),
		OtelProtocol:    os.Getenv("OTEL_EXPORTER_OTLP_PROTOCOL"),
		OtelServiceName: os.Getenv("OTEL_SERVICE_NAME"),
		// Database pool settings
		MaxOpenConns:    yamlCfg.Database.MaxOpenConns,
		MaxIdleConns:    yamlCfg.Database.MaxIdleConns,
		ConnMaxLifetime: yamlCfg.Database.ConnMaxLifetime,
		WorkerCount:     yamlCfg.Workers.Count,
		CPUMultiplier:   yamlCfg.Workers.CPUMultiplier,
		MinWorkers:      yamlCfg.Workers.MinWorkers,
		MaxWorkers:      yamlCfg.Workers.MaxWorkers,
	}

	if cfg.ReadTimeout == 0 {
		cfg.ReadTimeout = 30
	}
	if cfg.WriteTimeout == 0 {
		cfg.WriteTimeout = 30
	}
	// HTTP Server timeouts defaults (proteção contra Slowloris e resource exhaustion)
	if cfg.ReadHeaderTimeout == 0 {
		cfg.ReadHeaderTimeout = 10 // 10 segundos para ler headers
	}
	if cfg.IdleTimeout == 0 {
		cfg.IdleTimeout = 120 // 2 minutos para keep-alive
	}
	// Database pool defaults
	if cfg.MaxOpenConns == 0 {
		cfg.MaxOpenConns = 50 // 50 para I/O bound com múltiplos workers
	}
	if cfg.MaxIdleConns == 0 {
		cfg.MaxIdleConns = 10
	}
	if cfg.ConnMaxLifetime == 0 {
		cfg.ConnMaxLifetime = 300
	}
	// Worker pool defaults
	if cfg.WorkerCount == 0 {
		cfg.WorkerCount = 3
	}
	// CPU-based scaling defaults
	if cfg.CPUMultiplier == 0 {
		cfg.CPUMultiplier = 4.0 // 4x vCPUs para I/O bound (emails, APIs, webhooks)
	}
	if cfg.MinWorkers == 0 {
		cfg.MinWorkers = 2 // Mínimo 2 workers para I/O bound
	}
	if cfg.MaxWorkers == 0 {
		// Será calculado no runtime baseado em vCPUs * multiplier
		cfg.MaxWorkers = -1 // -1 = auto
	}

	if cfg.Env == "prod" || cfg.Env == "staging" {
		if cfg.SMTPPass == "" {
			return nil, fmt.Errorf("produção: SMTP_PASS é obrigatório")
		}
		if cfg.SMTPUser == "" {
			return nil, fmt.Errorf("produção: SMTP_USER é obrigatório")
		}
		if cfg.SessionSecret == "" {
			return nil, fmt.Errorf("produção: SESSION_SECRET é obrigatório")
		}
		if cfg.PasswordPepper == "" {
			return nil, fmt.Errorf("produção: PASSWORD_PEPPER é obrigatório")
		}
		if cfg.AsaasWebhookToken == "" {
			return nil, fmt.Errorf("produção: ASAAS_WEBHOOK_TOKEN é obrigatório")
		}
	} else {
		// Dev: gerar secret aleatório se não definido
		if cfg.SessionSecret == "" {
			cfg.SessionSecret = generateSecureSecret()
			slog.Warn("SESSION_SECRET não definido, usando secret gerado automaticamente (apenas para desenvolvimento)",
				"env", cfg.Env)
		}
		// Dev: pepper padrão (não usar em produção!)
		if cfg.PasswordPepper == "" {
			cfg.PasswordPepper = "dev-pepper-change-in-production"
			slog.Warn("PASSWORD_PEPPER não definido, usando pepper padrão (apenas para desenvolvimento)",
				"env", cfg.Env)
		}
	}

	return cfg, nil
}

// generateSecureSecret gera um secret criptograficamente seguro de 32 bytes (64 chars hex)
func generateSecureSecret() string {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		// Fallback para secret fixo se geração falhar (extremamente raro)
		return "fallback-secret-change-in-production"
	}
	return hex.EncodeToString(bytes)
}

func loadYAMLConfig(env string) (*EnvironmentConfig, *DefaultsConfig, error) {
	cfgPath := filepath.Join(".", "config.yaml")

	data, err := os.ReadFile(cfgPath)
	if err != nil {
		return getDefaultConfig(env), getDefaults(), nil
	}

	var yamlCfg YAMLConfig
	if err := yaml.Unmarshal(data, &yamlCfg); err != nil {
		return nil, nil, fmt.Errorf("falha ao parsear config.yaml: %w", err)
	}

	envConfig, ok := yamlCfg.Environments[env]
	if !ok {
		return getDefaultConfig(env), getDefaults(), nil
	}

	return &envConfig, &yamlCfg.Defaults, nil
}

func getDefaultConfig(env string) *EnvironmentConfig {
	defaults := map[string]EnvironmentConfig{
		"dev": {
			Server:   ServerConfig{Port: "8080", ReadTimeout: 30, WriteTimeout: 30},
			Database: DatabaseConfig{URL: "./goth.db?_journal_mode=WAL&_busy_timeout=5000"},
			Logging:  LoggingConfig{Level: "debug", Format: "json"},
			Session:  SessionConfig{Secret: "", LifetimeHours: 24},
			SMTP:     SMTPConfig{Host: "localhost", Port: "1025", From: "noreply@localhost"},
			Workers:  WorkersConfig{Count: 3},
		},
		"staging": {
			Server:   ServerConfig{Port: "8080", ReadTimeout: 60, WriteTimeout: 60},
			Database: DatabaseConfig{URL: "/data/goth.db?_journal_mode=WAL&_busy_timeout=5000"},
			Logging:  LoggingConfig{Level: "info", Format: "json"},
			Session:  SessionConfig{Secret: "", LifetimeHours: 168},
			SMTP:     SMTPConfig{Host: "${SMTP_HOST}", Port: "${SMTP_PORT}", From: "noreply@staging.goth.com"},
			Workers:  WorkersConfig{Count: 2},
		},
		"prod": {
			Server:   ServerConfig{Port: "8080", ReadTimeout: 60, WriteTimeout: 60},
			Database: DatabaseConfig{URL: "/data/goth.db?_journal_mode=WAL&_busy_timeout=5000"},
			Logging:  LoggingConfig{Level: "warn", Format: "json"},
			Session:  SessionConfig{Secret: "", LifetimeHours: 720},
			SMTP:     SMTPConfig{Host: "${SMTP_HOST}", Port: "${SMTP_PORT}", From: "noreply@goth.com"},
			Workers:  WorkersConfig{Count: 5},
		},
	}

	cfg, ok := defaults[env]
	if ok {
		return &cfg
	}
	devCfg := defaults["dev"]
	return &devCfg
}

func getDefaults() *DefaultsConfig {
	return &DefaultsConfig{
		AppURL: "http://localhost:8080",
		Asaas: AsaasDefaults{
			Environment: "sandbox",
		},
	}
}

func resolveEnvVar(value, fallback string) string {
	if strings.HasPrefix(value, "${") && strings.HasSuffix(value, "}") {
		envVar := strings.TrimSuffix(strings.TrimPrefix(value, "${"), "}")
		if val := os.Getenv(envVar); val != "" {
			return val
		}
	}
	if value != "" {
		return value
	}
	return fallback
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}
