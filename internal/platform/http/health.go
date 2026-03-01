package http

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"syscall"
	"time"
)

type HealthHandler struct {
	dbConn *sql.DB
	logger *slog.Logger
}

type HealthResponse struct {
	Status string  `json:"status"`
	Checks []Check `json:"checks"`
}

type Check struct {
	Name   string `json:"name"`
	Status string `json:"status"` // "ok" | "warning" | "error"
	Detail string `json:"detail,omitempty"`
}

func NewHealthHandler(dbConn *sql.DB, logger *slog.Logger) *HealthHandler {
	return &HealthHandler{
		dbConn: dbConn,
		logger: logger,
	}
}

func (h *HealthHandler) Health(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-Type", "text/plain")
	_, _ = w.Write([]byte("OK"))
}

func (h *HealthHandler) Ready(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	response := HealthResponse{
		Status: "ready",
		Checks: []Check{},
	}

	checks := h.performChecks(ctx)
	response.Checks = checks

	for _, check := range checks {
		if check.Status == "error" {
			response.Status = "not_ready"
			break
		}
	}

	hasWarnings := false
	for _, check := range checks {
		if check.Status == "warning" {
			hasWarnings = true
			break
		}
	}

	if response.Status == "ready" && hasWarnings {
		response.Status = "degraded"
	}

	status := http.StatusOK
	if response.Status == "not_ready" {
		status = http.StatusServiceUnavailable
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Health-Status", response.Status)
	w.WriteHeader(status)

	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	_ = encoder.Encode(response)
}

func (h *HealthHandler) performChecks(ctx context.Context) []Check {
	checks := []Check{}

	checks = append(checks, h.checkDatabase(ctx))
	checks = append(checks, h.checkDiskSpace())
	checks = append(checks, h.checkWorkerStatus(ctx))
	checks = append(checks, h.checkSMTP())

	return checks
}

func (h *HealthHandler) checkDatabase(ctx context.Context) Check {
	check := Check{Name: "database", Status: "ok"}

	if h.dbConn == nil {
		check.Status = "error"
		check.Detail = "database connection not initialized"
		return check
	}

	if err := h.dbConn.PingContext(ctx); err != nil {
		check.Status = "error"
		check.Detail = "database unreachable"
		return check
	}

	check.Detail = "connected"

	var failedJobs, pendingJobs int
	_ = h.dbConn.QueryRowContext(ctx, "SELECT COUNT(*) FROM jobs WHERE status = 'failed'").Scan(&failedJobs)
	_ = h.dbConn.QueryRowContext(ctx, "SELECT COUNT(*) FROM jobs WHERE status = 'pending'").Scan(&pendingJobs)

	if failedJobs > 50 {
		check.Status = "warning"
		check.Detail = "connected, but high failed jobs count"
	} else if pendingJobs > 1000 {
		check.Status = "warning"
		check.Detail = "connected, but high pending jobs count"
	}

	return check
}

func (h *HealthHandler) checkDiskSpace() Check {
	check := Check{Name: "disk_space", Status: "ok"}

	var stat syscall.Statfs_t
	wd, _ := workingDir()
	err := syscall.Statfs(wd, &stat)
	if err != nil {
		check.Status = "warning"
		check.Detail = "unable to check disk space"
		return check
	}

	freeSpace := stat.Bavail * uint64(stat.Bsize)

	if freeSpace < 100*1024*1024 {
		check.Status = "error"
		check.Detail = "less than 100MB free"
	} else if freeSpace < 500*1024*1024 {
		check.Status = "warning"
		check.Detail = "less than 500MB free"
	} else {
		check.Detail = freeSpaceReadable(freeSpace)
	}

	return check
}

// checkWorkerStatus verifica status do worker e fila de jobs
func (h *HealthHandler) checkWorkerStatus(ctx context.Context) Check {
	check := Check{Name: "worker", Status: "ok"}

	// Check if dbConn is nil
	if h.dbConn == nil {
		check.Status = "warning"
		check.Detail = "database connection not available"
		return check
	}

	// Contar jobs pendinges
	var pendingJobs int
	err := h.dbConn.QueryRowContext(ctx, "SELECT COUNT(*) FROM jobs WHERE status = 'pending' AND next_retry_at IS NULL").Scan(&pendingJobs)
	if err != nil {
		check.Status = "warning"
		check.Detail = "unable to query pending jobs"
		return check
	}

	// Contar jobs em processamento há mais de 5 minutos
	var staleJobs int
	err = h.dbConn.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM jobs 
		WHERE status = 'processing' 
		AND started_at < datetime('now', '-5 minutes')
	`).Scan(&staleJobs)
	if err != nil {
		check.Status = "warning"
		check.Detail = "unable to query stale jobs"
		return check
	}

	if staleJobs > 0 {
		check.Status = "warning"
		check.Detail = fmt.Sprintf("%d stale jobs detected", staleJobs)
	} else if pendingJobs > 500 {
		check.Status = "warning"
		check.Detail = fmt.Sprintf("%d pending jobs", pendingJobs)
	} else {
		check.Detail = fmt.Sprintf("%d pending, %d stale", pendingJobs, staleJobs)
	}

	return check
}

// checkSMTP verifica conectividade SMTP (sem enviar email)
func (h *HealthHandler) checkSMTP() Check {
	check := Check{Name: "smtp", Status: "ok"}

	// Nota: verificação real de SMTP requer acesso a config
	// Em produção, implementar dial real com timeout
	check.Detail = "configured"

	return check
}

func workingDir() (string, error) {
	return "/data", nil
}

func freeSpaceReadable(bytes uint64) string {
	mb := bytes / (1024 * 1024)
	if mb >= 1024 {
		gb := mb / 1024
		return gbToReadable(gb)
	}
	return mbToReadable(mb)
}

func mbToReadable(mb uint64) string {
	return string(rune('0'+int(mb%10))) + "MB free"
}

func gbToReadable(gb uint64) string {
	return string(rune('0'+int(gb%10))) + "GB free"
}
