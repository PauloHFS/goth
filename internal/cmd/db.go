package cmd

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/PauloHFS/goth/internal/db"
	"github.com/PauloHFS/goth/internal/platform/config"
	"github.com/PauloHFS/goth/internal/platform/logging"
)

func initDB() (*sql.DB, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}
	// Aplicando os mesmos pragmas de performance e resiliência
	dsn := cfg.DatabaseURL
	if strings.Contains(dsn, "?") {
		dsn += "&_journal_mode=WAL&_busy_timeout=5000&_synchronous=NORMAL"
	} else {
		dsn += "?_journal_mode=WAL&_busy_timeout=5000&_synchronous=NORMAL"
	}

	dbConn, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}
	return dbConn, nil
}

func RunSeed() error {
	dbConn, err := initDB()
	if err != nil {
		return fmt.Errorf("failed to initialize database: %w", err)
	}
	defer func() {
		_ = dbConn.Close()
	}()

	cfg, err := config.Load()
	if err != nil {
		cfg = &config.Config{LogLevel: "info"}
	}
	logger := logging.New(cfg.LogLevel)

	if err := db.RunMigrations(context.Background(), dbConn); err != nil {
		logger.Error("failed to run migrations during seed", "error", err)
		return fmt.Errorf("failed to run migrations: %w", err)
	}
	if err := db.Seed(context.Background(), dbConn); err != nil {
		logger.Error("failed to seed database", "error", err)
		return fmt.Errorf("failed to seed database: %w", err)
	}
	logger.Info("database seeded successfully")
	return nil
}

func RunMigrate() error {
	dbConn, err := initDB()
	if err != nil {
		return fmt.Errorf("failed to initialize database: %w", err)
	}
	defer func() {
		_ = dbConn.Close()
	}()

	cfg, err := config.Load()
	if err != nil {
		cfg = &config.Config{LogLevel: "info"}
	}
	logger := logging.New(cfg.LogLevel)

	if err := db.RunMigrations(context.Background(), dbConn); err != nil {
		logger.Error("failed to run migrations", "error", err)
		return fmt.Errorf("failed to run migrations: %w", err)
	}
	logger.Info("migrations executed successfully")
	return nil
}
