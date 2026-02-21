package db

import (
	"database/sql"
	"fmt"
	"runtime"
	"time"

	"github.com/PauloHFS/goth/internal/config"
)

type DualPool struct {
	Read  *sql.DB
	Write *sql.DB
}

type PoolConfig struct {
	ReadMaxOpen  int
	ReadMaxIdle  int
	WriteMaxOpen int
	WriteMaxIdle int
}

var defaultPoolConfig = PoolConfig{
	ReadMaxOpen:  runtime.NumCPU() * 2,
	ReadMaxIdle:  runtime.NumCPU(),
	WriteMaxOpen: 1,
	WriteMaxIdle: 1,
}

func NewDualPool(driver, dsn string, opts ...func(*PoolConfig)) (*DualPool, error) {
	cfg := defaultPoolConfig
	for _, opt := range opts {
		opt(&cfg)
	}

	readDB, err := sql.Open(driver, dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open read pool: %w", err)
	}
	readDB.SetMaxOpenConns(cfg.ReadMaxOpen)
	readDB.SetMaxIdleConns(cfg.ReadMaxIdle)
	readDB.SetConnMaxIdleTime(5 * time.Minute)
	readDB.SetConnMaxLifetime(time.Hour)

	writeDB, err := sql.Open(driver, dsn)
	if err != nil {
		readDB.Close()
		return nil, fmt.Errorf("failed to open write pool: %w", err)
	}
	writeDB.SetMaxOpenConns(cfg.WriteMaxOpen)
	writeDB.SetMaxIdleConns(cfg.WriteMaxIdle)
	writeDB.SetConnMaxIdleTime(5 * time.Minute)
	writeDB.SetConnMaxLifetime(time.Hour)

	pool := &DualPool{
		Read:  readDB,
		Write: writeDB,
	}

	sqliteCfg := config.GetSQLiteConfig()
	if err := sqliteCfg.ApplyPragmas(readDB); err != nil {
		pool.Close()
		return nil, fmt.Errorf("failed to apply read pragmas: %w", err)
	}
	if err := sqliteCfg.ApplyPragmas(writeDB); err != nil {
		pool.Close()
		return nil, fmt.Errorf("failed to apply write pragmas: %w", err)
	}

	return pool, nil
}

func WithReadPoolSize(maxOpen, maxIdle int) func(*PoolConfig) {
	return func(cfg *PoolConfig) {
		cfg.ReadMaxOpen = maxOpen
		cfg.ReadMaxIdle = maxIdle
	}
}

func WithWritePoolSize(maxOpen, maxIdle int) func(*PoolConfig) {
	return func(cfg *PoolConfig) {
		cfg.WriteMaxOpen = maxOpen
		cfg.WriteMaxIdle = maxIdle
	}
}

func (p *DualPool) Close() error {
	var errs []error
	if p.Read != nil {
		if err := p.Read.Close(); err != nil {
			errs = append(errs, fmt.Errorf("read pool close: %w", err))
		}
	}
	if p.Write != nil {
		if err := p.Write.Close(); err != nil {
			errs = append(errs, fmt.Errorf("write pool close: %w", err))
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("errors closing pools: %v", errs)
	}
	return nil
}

func (p *DualPool) Queries() *Queries {
	return New(p.Read)
}

func (p *DualPool) QueriesWrite() *Queries {
	return New(p.Write)
}
