package metrics

import (
	"database/sql"
	"time"

	"github.com/PauloHFS/goth/internal/platform/httpclient"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// HTTP metrics
	HttpRequestsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "http_requests_total",
		Help: "Total number of HTTP requests",
	}, []string{"path", "method", "status"})

	HTTPRequestDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "http_request_duration_seconds",
		Help:    "HTTP request duration in seconds",
		Buckets: prometheus.DefBuckets,
	}, []string{"path", "method"})

	// Job metrics
	JobDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "job_processing_seconds",
		Help:    "Time taken to process jobs",
		Buckets: prometheus.ExponentialBuckets(0.1, 2, 10),
	}, []string{"type", "status"})

	JobPending = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "worker_pending_jobs",
		Help: "Number of pending jobs in queue",
	})

	JobProcessing = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "worker_processing_jobs",
		Help: "Number of jobs currently processing",
	})

	JobProcessedTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "worker_jobs_processed_total",
		Help: "Total number of jobs processed",
	}, []string{"status"})

	JobFailedTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "worker_jobs_failed_total",
		Help: "Total number of jobs failed",
	}, []string{"type"})

	JobDeadLetterTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "worker_jobs_dead_letter_total",
		Help: "Total number of jobs moved to dead letter queue",
	})

	// Business metrics - Auth
	AuthRegistrationsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "auth_registrations_total",
		Help: "Total number of user registrations",
	}, []string{"status"}) // success, failure

	AuthLoginsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "auth_logins_total",
		Help: "Total number of login attempts",
	}, []string{"status"}) // success, failure

	AuthPasswordResetsTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "auth_password_resets_total",
		Help: "Total number of password reset requests",
	})

	AuthOAuthLoginsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "auth_oauth_logins_total",
		Help: "Total number of OAuth logins by provider",
	}, []string{"provider", "status"}) // provider: google, github, etc.

	AuthEmailVerificationsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "auth_email_verifications_total",
		Help: "Total number of email verifications",
	}, []string{"status"}) // success, failure

	AuthActiveSessions = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "auth_active_sessions",
		Help: "Number of active user sessions",
	})

	// Business metrics - Billing
	BillingPaymentsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "billing_payments_total",
		Help: "Total number of payment operations",
	}, []string{"type", "status"}) // type: payment, subscription; status: created, completed, failed

	BillingRevenueTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "billing_revenue_total_cents",
		Help: "Total revenue in cents",
	})

	BillingSubscriptionsActive = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "billing_subscriptions_active",
		Help: "Number of active subscriptions",
	})

	BillingSubscriptionsCreated = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "billing_subscriptions_created_total",
		Help: "Total number of new subscriptions created",
	}, []string{"plan"})

	BillingSubscriptionsCancelled = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "billing_subscriptions_cancelled_total",
		Help: "Total number of subscriptions cancelled",
	}, []string{"reason"})

	BillingMRR = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "billing_mrr_total_cents",
		Help: "Monthly Recurring Revenue in cents",
	})

	BillingChurnRate = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "billing_churn_rate",
		Help: "Customer churn rate (0.0-1.0)",
	})

	// Business metrics - Users
	ActiveUsers = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "active_users",
		Help: "Number of active users (logged in last 24h)",
	})

	UserRegistrationsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "user_registrations_total",
		Help: "Total number of user registrations",
	}, []string{"method"}) // method: email, oauth

	UserActionsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "user_actions_total",
		Help: "Total number of user actions by type",
	}, []string{"action"})

	UserSessionDuration = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "user_session_duration_seconds",
		Help:    "User session duration in seconds",
		Buckets: prometheus.ExponentialBuckets(60, 2, 10), // 1min to ~17h
	})

	// Business metrics - Feature Flags
	FeatureFlagsTotal = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "feature_flags_total",
		Help: "Total number of feature flags",
	})

	FeatureFlagsEnabled = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "feature_flags_enabled",
		Help: "Number of enabled feature flags",
	})

	FeatureFlagEvaluationsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "feature_flag_evaluations_total",
		Help: "Total number of feature flag evaluations",
	}, []string{"flag", "result"}) // result: enabled, disabled

	// System metrics - Database Pool
	DBConnectionsOpen = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "db_connections_open",
		Help: "Number of open database connections",
	})

	DBConnectionsInUse = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "db_connections_in_use",
		Help: "Number of database connections currently in use",
	})

	DBConnectionsIdle = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "db_connections_idle",
		Help: "Number of idle database connections",
	})

	DBConnectionsWaitTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "db_connections_wait_total",
		Help: "Total number of times connections had to wait",
	})

	DBConnectionsWaitDuration = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "db_connections_wait_duration_seconds",
		Help:    "Time spent waiting for a connection",
		Buckets: prometheus.DefBuckets,
	})

	DBMaxOpenConnections = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "db_max_open_connections",
		Help: "Maximum number of open connections allowed",
	})

	// Database stats
	DBStatsGauge = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "db_stats",
		Help: "Database statistics",
	}, []string{"stat"})

	// Cache metrics
	CacheHitsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "cache_hits_total",
		Help: "Total cache hits by type",
	}, []string{"cache"})

	CacheMissesTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "cache_misses_total",
		Help: "Total cache misses by type",
	}, []string{"cache"})

	// Circuit Breaker metrics
	CircuitBreakerState = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "circuit_breaker_state",
		Help: "Circuit breaker state (0=closed, 1=open, 2=half-open)",
	}, []string{"name", "state"})

	CircuitBreakerFailures = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "circuit_breaker_failures_total",
		Help: "Total circuit breaker failures",
	}, []string{"name"})

	CircuitBreakerSuccesses = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "circuit_breaker_successes_total",
		Help: "Total circuit breaker successes",
	}, []string{"name"})

	CircuitBreakerHalfOpenReqs = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "circuit_breaker_half_open_requests",
		Help: "Current number of half-open requests",
	}, []string{"name"})
)

// CircuitBreakerStats interface para obter estatísticas do circuit breaker
type CircuitBreakerStats interface {
	State() httpclient.CircuitState
	Stats() map[string]interface{}
	Name() string
}

// StartDBStatsCollector inicia coleta periódica de métricas do database pool
func StartDBStatsCollector(db *sql.DB, interval time.Duration, stopCh <-chan struct{}) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-stopCh:
			return
		case <-ticker.C:
			collectDBStats(db)
		}
	}
}

// collectDBStats coleta e atualiza métricas do database pool
func collectDBStats(db *sql.DB) {
	if db == nil {
		return
	}

	stats := db.Stats()

	DBConnectionsOpen.Set(float64(stats.OpenConnections))
	DBConnectionsInUse.Set(float64(stats.InUse))
	DBConnectionsIdle.Set(float64(stats.Idle))
	DBMaxOpenConnections.Set(float64(stats.MaxOpenConnections))

	// Counter só pode ser incrementado, não setado
	// Para wait count, precisamos trackear a diferença
	currentWaitCount := float64(stats.WaitCount)
	DBStatsGauge.WithLabelValues("wait_count").Set(currentWaitCount)

	if stats.WaitDuration > 0 {
		DBConnectionsWaitDuration.Observe(stats.WaitDuration.Seconds())
	}

	// Coletar estatísticas adicionais como gauges individuais
	DBStatsGauge.WithLabelValues("open_connections").Set(float64(stats.OpenConnections))
	DBStatsGauge.WithLabelValues("in_use").Set(float64(stats.InUse))
	DBStatsGauge.WithLabelValues("idle").Set(float64(stats.Idle))
	DBStatsGauge.WithLabelValues("max_open_connections").Set(float64(stats.MaxOpenConnections))
}

// StartCircuitBreakerMetricsCollector inicia coleta periódica de métricas do circuit breaker
func StartCircuitBreakerMetricsCollector(cb CircuitBreakerStats, interval time.Duration, stopCh <-chan struct{}) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-stopCh:
			return
		case <-ticker.C:
			collectCircuitBreakerStats(cb)
		}
	}
}

// collectCircuitBreakerStats coleta e atualiza métricas do circuit breaker
func collectCircuitBreakerStats(cb CircuitBreakerStats) {
	if cb == nil {
		return
	}

	name := cb.Name()
	state := cb.State()
	stats := cb.Stats()

	// Reset all states for this name first
	for _, s := range []string{"closed", "open", "half-open"} {
		CircuitBreakerState.WithLabelValues(name, s).Set(0)
	}

	// Set current state to 1
	CircuitBreakerState.WithLabelValues(name, state.String()).Set(1)

	// Update failures (as gauge for current count)
	if failures, ok := stats["failures"].(int); ok {
		CircuitBreakerHalfOpenReqs.WithLabelValues(name).Set(float64(failures))
	}

	// Update half-open requests
	if halfOpenReqs, ok := stats["half_open_reqs"].(int); ok {
		CircuitBreakerHalfOpenReqs.WithLabelValues(name).Set(float64(halfOpenReqs))
	}
}
