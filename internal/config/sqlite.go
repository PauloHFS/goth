package config

import (
	"database/sql"
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
)

type SQLiteConfig struct {
	CacheSizeKB int    // negativo = KB, positivo = páginas
	TempStore   string // "MEMORY" ou "FILE"
	WALMode     bool   // Write-Ahead Logging
	SyncLevel   string // "OFF", "NORMAL", "FULL", "EXTRA"
}

func GetSQLiteConfig() SQLiteConfig {
	cfg := SQLiteConfig{
		CacheSizeKB: -16000,
		TempStore:   "MEMORY",
		WALMode:     true,
		SyncLevel:   "NORMAL",
	}

	if v, ok := os.LookupEnv("SQLITE_CACHE_SIZE"); ok {
		if i, err := strconv.Atoi(v); err == nil {
			cfg.CacheSizeKB = i
		}
	}

	if v, ok := os.LookupEnv("SQLITE_TEMP_STORE"); ok {
		v = strings.ToUpper(v)
		if v == "MEMORY" || v == "FILE" {
			cfg.TempStore = v
		}
	}

	if v, ok := os.LookupEnv("SQLITE_WAL_MODE"); ok {
		cfg.WALMode = strings.ToLower(v) == "true" || v == "1"
	}

	if v, ok := os.LookupEnv("SQLITE_SYNC_LEVEL"); ok {
		v = strings.ToUpper(v)
		if v == "OFF" || v == "NORMAL" || v == "FULL" || v == "EXTRA" {
			cfg.SyncLevel = v
		}
	}

	if _, ok := os.LookupEnv("SQLITE_CACHE_SIZE"); !ok {
		if ramMB := detectRAM(); ramMB > 0 {
			cfg.CacheSizeKB = calculateCacheSize(ramMB)
		}
	}

	return cfg
}

func calculateCacheSize(ramMB int) int {
	cacheMB := int(math.Floor(float64(ramMB) * 0.02))
	cacheMB = max(cacheMB, 8)
	cacheMB = min(cacheMB, 256)
	return -cacheMB * 1024
}

func detectRAM() int {
	if v, ok := os.LookupEnv("SYSTEM_RAM_MB"); ok {
		if mb, err := strconv.Atoi(v); err == nil && mb > 0 {
			return mb
		}
	}

	data, err := os.ReadFile("/proc/meminfo")
	if err == nil {
		lines := string(data)
		for line := range strings.SplitSeq(lines, "\n") {
			if strings.HasPrefix(line, "MemTotal:") {
				fields := strings.Fields(line)
				if len(fields) >= 2 {
					if kb, err := strconv.ParseInt(fields[1], 10, 64); err == nil {
						return int(kb / 1024)
					}
				}
			}
		}
	}

	return 0
}

func (c SQLiteConfig) ApplyPragmas(db *sql.DB) error {
	pragmas := []struct {
		name  string
		value string
	}{
		{"temp_store", c.TempStore},
		{"cache_size", fmt.Sprintf("%d", c.CacheSizeKB)},
		{"journal_mode", "WAL"},
		{"wal_autocheckpoint", "1000"},
		{"synchronous", c.SyncLevel},
		{"mmap_size", "268435456"},
		{"page_size", "4096"},
	}

	for _, p := range pragmas {
		pragma := fmt.Sprintf("PRAGMA %s = %s", p.name, p.value)
		if _, err := db.Exec(pragma); err != nil {
			return fmt.Errorf("failed to set PRAGMA %s: %w", p.name, err)
		}
	}

	return nil
}
