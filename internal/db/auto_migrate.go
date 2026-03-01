package db

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/PauloHFS/goth/migrations"
	"github.com/pressly/goose/v3"
)

// RunMigrations executa todas as migrations embutidas usando goose.
func RunMigrations(ctx context.Context, db *sql.DB) error {
	// Configurar dialect SQLite
	if err := goose.SetDialect("sqlite3"); err != nil {
		return fmt.Errorf("falha ao configurar dialect SQLite: %w", err)
	}

	// Configura o goose para usar o embed.FS
	goose.SetBaseFS(migrations.FS)

	// Executa todas as migrations na ordem
	if err := goose.UpContext(ctx, db, "."); err != nil {
		return fmt.Errorf("falha ao executar migrations: %w", err)
	}

	return nil
}

// RollbackMigrations reverte a última migration aplicada.
func RollbackMigrations(ctx context.Context, db *sql.DB) error {
	if err := goose.SetDialect("sqlite3"); err != nil {
		return fmt.Errorf("falha ao configurar dialect SQLite: %w", err)
	}
	goose.SetBaseFS(migrations.FS)

	if err := goose.DownContext(ctx, db, "."); err != nil {
		return fmt.Errorf("falha ao reverter migration: %w", err)
	}

	return nil
}

// RollbackMigrationsTo reverte migrations até uma versão específica.
// Use version=0 para reverter todas.
func RollbackMigrationsTo(ctx context.Context, db *sql.DB, version int64) error {
	if err := goose.SetDialect("sqlite3"); err != nil {
		return fmt.Errorf("falha ao configurar dialect SQLite: %w", err)
	}
	goose.SetBaseFS(migrations.FS)

	if err := goose.DownToContext(ctx, db, ".", version); err != nil {
		return fmt.Errorf("falha ao reverter migrations até versão %d: %w", version, err)
	}

	return nil
}

// GetMigrationStatus retorna a versão atual do schema.
func GetMigrationStatus(db *sql.DB) (int64, error) {
	return goose.GetDBVersion(db)
}
