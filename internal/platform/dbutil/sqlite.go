package dbutil

import (
	"context"
	"database/sql"
	"log/slog"
	"runtime"
	"time"
)

// WALConfig configura o modo WAL do SQLite
type WALConfig struct {
	// BusyTimeout timeout para locks (ms)
	BusyTimeout int
	// AutoCheckpoint número de páginas antes de checkpoint automático
	AutoCheckpoint int
	// Synchronous modo de sincronização (0=OFF, 1=NORMAL, 2=FULL)
	Synchronous int
}

// DefaultWALConfig retorna configuração padrão para produção
func DefaultWALConfig() WALConfig {
	return WALConfig{
		BusyTimeout:    5000, // 5 segundos
		AutoCheckpoint: 1000, // 1000 páginas (~4MB)
		Synchronous:    1,    // NORMAL
	}
}

// ConfigureWAL configura SQLite para produção com WAL mode
func ConfigureWAL(db *sql.DB, config WALConfig) error {
	// WAL mode
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		return err
	}

	// Busy timeout
	if _, err := db.Exec("PRAGMA busy_timeout=?", config.BusyTimeout); err != nil {
		return err
	}

	// Auto checkpoint
	if _, err := db.Exec("PRAGMA wal_autocheckpoint=?", config.AutoCheckpoint); err != nil {
		return err
	}

	// Synchronous mode
	if _, err := db.Exec("PRAGMA synchronous=?", config.Synchronous); err != nil {
		return err
	}

	// Cache size (páginas de 4KB)
	if _, err := db.Exec("PRAGMA cache_size=-2000"); err != nil { // -2000 = 2MB
		return err
	}

	// Temp store em memória
	if _, err := db.Exec("PRAGMA temp_store=MEMORY"); err != nil {
		return err
	}

	return nil
}

// StartWALCheckpoint goroutine para checkpoint periódico
func StartWALCheckpoint(ctx context.Context, db *sql.DB, logger *slog.Logger, interval time.Duration) {
	ticker := time.NewTicker(interval)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				// Passive checkpoint (não bloqueia)
				var busy int
				err := db.QueryRow("PRAGMA wal_checkpoint(PASSIVE)").Scan(&busy, &busy, &busy)
				if err != nil {
					logger.Warn("WAL checkpoint failed", "error", err)
					continue
				}
				if busy == 1 {
					logger.Debug("WAL checkpoint busy, skipping")
				}
			}
		}
	}()
}

// GetWALStats retorna estatísticas do WAL
func GetWALStats(db *sql.DB) (map[string]interface{}, error) {
	stats := make(map[string]interface{})

	// WAL size
	var walSize int
	err := db.QueryRow("PRAGMA wal_checkpoint").Scan(&walSize, &walSize, &walSize)
	if err != nil {
		return nil, err
	}
	stats["checkpoint_pages"] = walSize

	// Page size
	var pageSize int
	err = db.QueryRow("PRAGMA page_size").Scan(&pageSize)
	if err != nil {
		return nil, err
	}
	stats["page_size"] = pageSize

	// Page count
	var pageCount int
	err = db.QueryRow("PRAGMA page_count").Scan(&pageCount)
	if err != nil {
		return nil, err
	}
	stats["page_count"] = pageCount

	// Free page count
	var freeCount int
	err = db.QueryRow("PRAGMA freelist_count").Scan(&freeCount)
	if err != nil {
		return nil, err
	}
	stats["freelist_count"] = freeCount

	return stats, nil
}

// Optimize executa VACUUM e ANALYZE
func Optimize(ctx context.Context, db *sql.DB) error {
	// ANALYZE para atualizar estatísticas
	if _, err := db.ExecContext(ctx, "ANALYZE"); err != nil {
		return err
	}

	// VACUUM apenas se necessário (pode ser lento)
	// Só executar offline ou em janela de manutenção
	return nil
}

// GetConnectionStats retorna estatísticas de conexão
func GetConnectionStats(db *sql.DB) map[string]interface{} {
	stats := db.Stats()
	return map[string]interface{}{
		"max_open_connections": stats.MaxOpenConnections,
		"open_connections":     stats.OpenConnections,
		"in_use":               stats.InUse,
		"idle":                 stats.Idle,
		"wait_count":           stats.WaitCount,
		"wait_duration_ms":     stats.WaitDuration.Milliseconds(),
		"max_idle_closed":      stats.MaxIdleClosed,
		"max_lifetime_closed":  stats.MaxLifetimeClosed,
		"max_idle_time_closed": stats.MaxIdleTimeClosed,
	}
}

// HealthCheck verifica saúde do DB
func HealthCheck(ctx context.Context, db *sql.DB, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	var result int
	return db.QueryRowContext(ctx, "SELECT 1").Scan(&result)
}

// GetDiskUsage retorna uso de disco do database
func GetDiskUsage(dbPath string) (map[string]interface{}, error) {
	// Implementação depende do OS
	// Em produção, usar syscall ou biblioteca como github.com/shirou/gopsutil
	return map[string]interface{}{
		"note": "Use gopsutil for disk stats",
	}, nil
}

// GetMemoryUsage retorna uso de memória do processo
func GetMemoryUsage() map[string]interface{} {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	return map[string]interface{}{
		"alloc_mb":       m.Alloc / 1024 / 1024,
		"total_alloc_mb": m.TotalAlloc / 1024 / 1024,
		"sys_mb":         m.Sys / 1024 / 1024,
		"num_gc":         m.NumGC,
		"pause_total_ns": m.PauseTotalNs,
	}
}
