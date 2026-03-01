package worker

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"math/rand"
	"runtime"
	"runtime/debug"
	"sync"
	"sync/atomic"
	"time"

	"github.com/PauloHFS/goth/internal/db"
	"github.com/PauloHFS/goth/internal/features/billing"
	"github.com/PauloHFS/goth/internal/features/sse"
	"github.com/PauloHFS/goth/internal/platform/config"
	"github.com/PauloHFS/goth/internal/platform/logging"
	"github.com/PauloHFS/goth/internal/platform/mailer"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// JobTypeConfig configurações por tipo de job
type JobTypeConfig struct {
	TimeoutSeconds int
	MaxAttempts    int
	BackoffBaseSec int
	BackoffMaxSec  int
	Enabled        bool
}

// DefaultJobConfigs configurações default por tipo
var DefaultJobConfigs = map[string]JobTypeConfig{
	"send_email":                {TimeoutSeconds: 60, MaxAttempts: 5, BackoffBaseSec: 30, BackoffMaxSec: 600, Enabled: true},
	"send_password_reset_email": {TimeoutSeconds: 60, MaxAttempts: 5, BackoffBaseSec: 30, BackoffMaxSec: 600, Enabled: true},
	"send_verification_email":   {TimeoutSeconds: 60, MaxAttempts: 5, BackoffBaseSec: 30, BackoffMaxSec: 600, Enabled: true},
	"process_ai":                {TimeoutSeconds: 120, MaxAttempts: 3, BackoffBaseSec: 60, BackoffMaxSec: 300, Enabled: true},
	"process_webhook":           {TimeoutSeconds: 300, MaxAttempts: 5, BackoffBaseSec: 30, BackoffMaxSec: 600, Enabled: true},
	"process_asaas_webhook":     {TimeoutSeconds: 300, MaxAttempts: 5, BackoffBaseSec: 30, BackoffMaxSec: 600, Enabled: true},
}

// Metrics mantém todas as métricas do worker
type Metrics struct {
	JobsProcessed     *prometheus.CounterVec
	JobsSucceeded     *prometheus.CounterVec
	JobsFailed        *prometheus.CounterVec
	JobsDLQ           prometheus.Counter
	JobDuration       *prometheus.HistogramVec
	ActiveJobs        prometheus.GaugeFunc
	JobsPending       prometheus.GaugeFunc
	JobsProcessing    prometheus.GaugeFunc
	ThroughputPerHour *prometheus.GaugeVec
	// Novas métricas de observabilidade
	QueueDepth          *prometheus.GaugeVec
	JobWaitTime         *prometheus.HistogramVec
	CircuitBreakerState *prometheus.GaugeVec
	DBWriterQueueLen    prometheus.Gauge
	WorkerCount         prometheus.GaugeFunc
	ScalingEvents       *prometheus.CounterVec
}

// NewMetrics cria métricas do worker (usando métricas globais quando possível)
func NewMetrics(p *Processor) *Metrics {
	return &Metrics{
		// Usar métricas globais de platform/metrics para evitar duplicação
		// JobsProcessed: metrics.JobProcessedTotal (já registrado)
		// JobsFailed: metrics.JobFailedTotal (já registrado)
		// JobsDLQ: metrics.JobDeadLetterTotal (já registrado)
		// JobDuration: metrics.JobDuration (já registrado)
		// JobsPending: metrics.JobPending (já registrado)
		// JobsProcessing: metrics.JobProcessing (já registrado)

		// Métricas específicas do worker (não duplicadas)
		JobsProcessed: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "worker_jobs_processed_by_type_total",
			Help: "Total number of jobs processed by type",
		}, []string{"job_type", "status"}),
		JobsSucceeded: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "worker_jobs_succeeded_total",
			Help: "Total number of jobs succeeded",
		}, []string{"job_type"}),
		JobsFailed: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "worker_jobs_failed_by_type_total",
			Help: "Total number of jobs failed by type",
		}, []string{"job_type"}),
		JobsDLQ: promauto.NewCounter(prometheus.CounterOpts{
			Name: "worker_jobs_dead_letter_by_type_total",
			Help: "Total number of jobs sent to dead letter queue by type",
		}),
		JobDuration: promauto.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "worker_job_duration_by_type_seconds",
			Help:    "Duration of jobs by type in seconds",
			Buckets: prometheus.ExponentialBuckets(0.1, 2, 10),
		}, []string{"job_type", "status"}),
		ActiveJobs: promauto.NewGaugeFunc(prometheus.GaugeOpts{
			Name: "worker_active_jobs",
			Help: "Number of jobs currently being processed",
		}, func() float64 {
			return float64(p.ActiveJobs())
		}),
		// Usar métricas globais de platform/metrics (já registradas):
		// JobsPending: metrics.JobPending (worker_pending_jobs)
		// JobsProcessing: metrics.JobProcessing (worker_processing_jobs)
		JobsPending: promauto.NewGaugeFunc(prometheus.GaugeOpts{
			Name: "worker_pending_jobs_by_type",
			Help: "Number of jobs pending in queue by type",
		}, func() float64 {
			return p.PendingJobs()
		}),
		JobsProcessing: promauto.NewGaugeFunc(prometheus.GaugeOpts{
			Name: "worker_processing_jobs_by_type",
			Help: "Number of jobs currently processing by type",
		}, func() float64 {
			return p.ProcessingJobs()
		}),
		ThroughputPerHour: promauto.NewGaugeVec(prometheus.GaugeOpts{
			Name: "worker_throughput_per_hour",
			Help: "Jobs processed per hour by type",
		}, []string{"job_type", "status"}),
		// Novas métricas
		QueueDepth: promauto.NewGaugeVec(prometheus.GaugeOpts{
			Name: "worker_queue_depth",
			Help: "Number of pending jobs by priority",
		}, []string{"priority"}),
		JobWaitTime: promauto.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "worker_job_wait_time_seconds",
			Help:    "Time jobs spend waiting in queue",
			Buckets: prometheus.ExponentialBuckets(1, 2, 10),
		}, []string{"job_type"}),
		CircuitBreakerState: promauto.NewGaugeVec(prometheus.GaugeOpts{
			Name: "worker_circuit_breaker_state",
			Help: "Circuit breaker state by job type (0=closed, 1=open, 2=half-open)",
		}, []string{"job_type"}),
		DBWriterQueueLen: promauto.NewGauge(prometheus.GaugeOpts{
			Name: "worker_db_writer_queue_length",
			Help: "Current length of DB writer queue",
		}),
		WorkerCount: promauto.NewGaugeFunc(prometheus.GaugeOpts{
			Name: "worker_count",
			Help: "Current number of active workers",
		}, func() float64 {
			return float64(p.GetWorkerCount())
		}),
		ScalingEvents: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "worker_scaling_events_total",
			Help: "Total number of scaling events",
		}, []string{"direction"}), // "up" or "down"
	}
}

type Processor struct {
	db              *sql.DB
	queries         *db.Queries
	logger          *slog.Logger
	mailer          *mailer.Mailer
	billingWebhook  *billing.WebhookHandler
	sse             sse.Broker
	metrics         *Metrics
	configs         map[string]JobTypeConfig
	configsMu       sync.RWMutex
	workerID        string
	wg              sync.WaitGroup
	activeJobs      atomic.Int64
	processingJobs  atomic.Int64
	stopOnce        sync.Once
	stopChan        chan struct{}
	newJobNotify    chan struct{} // Notificação para otimizar polling
	jobPollInterval time.Duration

	// Novos componentes
	dbWriter       *DBWriter
	circuitBreaker *CircuitBreaker
	apiThrottler   *APIThrottler
	batchProcessor *BatchProcessor

	// Dynamic scaling
	workerCount      atomic.Int32
	minWorkers       int32
	maxWorkers       int32
	scaleChan        chan struct{}
	autoScaleEnabled bool
	spawned          chan struct{} // Signal when initial workers are spawned
}

func New(cfg *config.Config, dbConn *sql.DB, q *db.Queries, l *slog.Logger, broker sse.Broker) *Processor {
	p := &Processor{
		db:              dbConn,
		queries:         q,
		logger:          l,
		mailer:          mailer.New(cfg),
		billingWebhook:  billing.NewWebhookHandler(q, cfg.AsaasWebhookToken, cfg.AsaasHmacSecret, l),
		sse:             broker,
		configs:         make(map[string]JobTypeConfig),
		workerID:        fmt.Sprintf("worker-%d", time.Now().UnixNano()),
		stopChan:        make(chan struct{}),
		spawned:         make(chan struct{}),
		newJobNotify:    make(chan struct{}, 100), // Buffer para múltiplas notificações
		jobPollInterval: 100 * time.Millisecond,   // Polling mais rápido com notificação
	}

	// Calcula workers baseados em vCPUs
	cpuCount := runtime.NumCPU()
	p.minWorkers = int32(cfg.MinWorkers)
	if p.minWorkers == 0 {
		p.minWorkers = 1
	}

	// Max workers baseado em vCPUs * multiplier
	if cfg.MaxWorkers > 0 {
		p.maxWorkers = int32(cfg.MaxWorkers)
	} else {
		// Auto: CPU * multiplier para I/O bound
		p.maxWorkers = int32(float64(cpuCount) * cfg.CPUMultiplier)
		if p.maxWorkers < 8 {
			p.maxWorkers = 8 // Mínimo 8 workers para I/O bound
		}
	}

	p.scaleChan = make(chan struct{}, 1)
	p.autoScaleEnabled = true

	// Worker count inicial
	initialWorkers := cfg.WorkerCount
	if initialWorkers == 0 {
		// Usa base workers: CPU * multiplier / 2 (ponto médio)
		initialWorkers = int(float64(cpuCount) * cfg.CPUMultiplier * 0.5)
		if initialWorkers < 2 {
			initialWorkers = 2
		}
	}
	p.workerCount.Store(int32(initialWorkers))

	// Inicializa componentes
	if q != nil {
		p.dbWriter = NewDBWriter(dbConn, q, l)
		p.circuitBreaker = NewCircuitBreaker(q, DefaultCircuitBreakerConfig)
		p.apiThrottler = NewAPIThrottler()
		p.batchProcessor = NewBatchProcessor(dbConn, p.mailer, q, l)
	}

	// Inicializa configurações
	p.loadJobConfigs()

	// Inicializa métricas apenas se queries estiver disponível
	if q != nil {
		p.metrics = NewMetrics(p)
	}

	return p
}

// loadJobConfigs carrega configurações do banco ou usa defaults
func (p *Processor) loadJobConfigs() {
	p.configsMu.Lock()
	defer p.configsMu.Unlock()

	// Inicializa com defaults
	p.configs = make(map[string]JobTypeConfig)
	for jobType, cfg := range DefaultJobConfigs {
		p.configs[jobType] = cfg
	}

	// Se queries não estiver disponível (testes), usa apenas defaults
	if p.queries == nil {
		return
	}

	// Tenta carregar do DB (pode falhar em testes ou se DB não estiver disponível)
	ctx := context.Background()
	configs, err := p.queries.ListJobTypeConfigs(ctx)
	if err != nil {
		p.logger.Debug("using default job configs (DB may not be available)", "error", err)
		return
	}

	// Sobrescreve com configs do DB
	for _, c := range configs {
		p.configs[c.JobType] = JobTypeConfig{
			TimeoutSeconds: int(c.TimeoutSeconds),
			MaxAttempts:    int(c.MaxAttempts),
			BackoffBaseSec: int(c.BackoffBaseSeconds),
			BackoffMaxSec:  int(c.BackoffMaxSeconds),
			Enabled:        c.Enabled,
		}
	}
}

// GetJobConfig retorna configuração para um tipo de job
func (p *Processor) getJobConfig(jobType string) JobTypeConfig {
	p.configsMu.RLock()
	defer p.configsMu.RUnlock()

	if cfg, ok := p.configs[jobType]; ok {
		return cfg
	}
	return DefaultJobConfigs[jobType]
}

// NotifyNewJob notifica os workers que há um novo job (otimiza polling)
func (p *Processor) NotifyNewJob() {
	select {
	case p.newJobNotify <- struct{}{}:
		// Notificação enviada
	default:
		// Buffer cheio, workers vão detectar no próximo polling
	}
}

// PendingJobs retorna número de jobs pending
func (p *Processor) PendingJobs() float64 {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	count, err := p.queries.GetPendingJobCount(ctx)
	if err != nil {
		return 0
	}
	return float64(count)
}

// ProcessingJobs retorna número de jobs em processamento
func (p *Processor) ProcessingJobs() float64 {
	return float64(p.processingJobs.Load())
}

// Start inicia o worker pool com notificação otimizada e auto-scaling
func (p *Processor) Start(ctx context.Context, workerCount int) {
	// Usa worker count calculado no New() se workerCount for 0
	if workerCount <= 0 {
		workerCount = int(p.workerCount.Load())
	} else {
		p.workerCount.Store(int32(workerCount))
	}

	p.logger.Info("worker started with hybrid auto-scaling",
		"initial_workers", workerCount,
		"worker_id", p.workerID,
		"poll_interval_ms", p.jobPollInterval.Milliseconds(),
		"auto_scale_enabled", p.autoScaleEnabled,
		"min_workers", p.minWorkers,
		"max_workers", p.maxWorkers,
		"vcpus", runtime.NumCPU(),
		"cpu_multiplier", p.getCPUMultiplier(),
	)

	workerCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Inicia goroutine de notificação de novos jobs
	go p.notifyWorkers(workerCtx)

	// Inicia goroutine de timeout checker
	go p.checkTimeouts(workerCtx)

	// Inicia goroutine de métricas horárias
	go p.collectHourlyMetrics(workerCtx)

	// Inicia goroutine de auto-scaling
	if p.autoScaleEnabled {
		go p.autoScale(workerCtx)
	}

	// Spawn worker pool inicial
	for i := 0; i < workerCount; i++ {
		p.spawnWorker(workerCtx, i)
	}

	// Signal that initial workers are spawned
	close(p.spawned)

	<-workerCtx.Done()
	p.logger.Info("worker signal received: waiting for active jobs to finish")

	// Shutdown dos componentes
	p.Shutdown()
}

// spawnWorker cria um novo worker goroutine
func (p *Processor) spawnWorker(ctx context.Context, id int) {
	p.wg.Add(1)
	go func(id int) {
		defer p.wg.Done()
		defer func() {
			if r := recover(); r != nil {
				p.logger.Error("worker panic recovered",
					"worker_id", id,
					"panic", r,
					"stack", string(debug.Stack()))
			}
		}()
		p.runWorker(ctx, id)
	}(id)
}

// Shutdown para todos os componentes gracefulmente
func (p *Processor) Shutdown() {
	p.logger.Info("shutting down worker components")

	if p.dbWriter != nil {
		p.dbWriter.Stop()
	}
	if p.apiThrottler != nil {
		p.apiThrottler.Stop()
	}

	p.logger.Info("all worker components stopped")
}

// autoScale gerencia scaling dinâmico de workers
func (p *Processor) autoScale(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			p.evaluateScaling()
		case <-p.scaleChan:
			p.evaluateScaling()
		}
	}
}

// evaluateScaling avalia se precisa escalar up ou down baseado em vCPUs + fila
func (p *Processor) evaluateScaling() {
	currentWorkers := int(p.workerCount.Load())
	activeJobs := int(p.activeJobs.Load())
	pendingJobs := int(p.PendingJobs())

	// Calcula limites baseados em vCPUs para I/O bound
	cpuCount := runtime.NumCPU()
	cpuBasedMax := int32(float64(cpuCount) * 4.0) // 4x vCPUs para I/O bound

	// Usa o menor entre maxWorkers configurado e CPU-based max
	effectiveMax := p.maxWorkers
	if cpuBasedMax < effectiveMax {
		effectiveMax = cpuBasedMax
	}

	// Scale up: se há fila E workers < limite de vCPUs
	if pendingJobs > 50 && currentWorkers < int(effectiveMax) {
		// Adiciona 1 worker por vez
		newCount := currentWorkers + 1
		p.workerCount.Store(int32(newCount))
		p.logger.Info("scaling up workers",
			"from", currentWorkers,
			"to", newCount,
			"pending_jobs", pendingJobs,
			"active_jobs", activeJobs,
			"vcpus", cpuCount,
			"effective_max", effectiveMax,
		)
		// Spawn novo worker
		go p.spawnWorker(context.Background(), newCount-1)

		// Métrica de scaling
		if p.metrics != nil {
			p.metrics.ScalingEvents.WithLabelValues("up").Inc()
		}
		return
	}

	// Scale down: se fila vazia E workers > minWorkers
	if pendingJobs == 0 && activeJobs == 0 && currentWorkers > int(p.minWorkers) {
		// Remove 1 worker por vez (não mata workers ativos)
		newCount := currentWorkers - 1
		p.workerCount.Store(int32(newCount))
		p.logger.Info("scaling down workers",
			"from", currentWorkers,
			"to", newCount,
			"reason", "idle workers with empty queue",
			"vcpus", cpuCount,
		)

		// Métrica de scaling
		if p.metrics != nil {
			p.metrics.ScalingEvents.WithLabelValues("down").Inc()
		}
		return
	}

	// Log status (debug)
	p.logger.Debug("worker scaling evaluation",
		"current_workers", currentWorkers,
		"active_jobs", activeJobs,
		"pending_jobs", pendingJobs,
		"vcpus", cpuCount,
		"effective_max", effectiveMax,
		"min_workers", p.minWorkers,
	)
}

// ScaleWorkers força um número específico de workers (para testes ou admin)
func (p *Processor) ScaleWorkers(count int) {
	if count < int(p.minWorkers) {
		count = int(p.minWorkers)
	}
	if count > int(p.maxWorkers) {
		count = int(p.maxWorkers)
	}
	p.workerCount.Store(int32(count))
	select {
	case p.scaleChan <- struct{}{}:
	default:
	}
}

// GetWorkerCount retorna número atual de workers
func (p *Processor) GetWorkerCount() int {
	return int(p.workerCount.Load())
}

// SetAutoScaleEnabled habilita/desabilita auto-scaling
func (p *Processor) SetAutoScaleEnabled(enabled bool) {
	p.autoScaleEnabled = enabled
}

// getCPUMultiplier retorna o multiplier de CPUs configurado
func (p *Processor) getCPUMultiplier() float64 {
	return float64(p.maxWorkers) / float64(runtime.NumCPU())
}

// getCPUBasedMaxWorkers calcula max workers baseado em vCPUs para I/O bound
// func (p *Processor) getCPUBasedMaxWorkers() int32 {
// 	cpuCount := runtime.NumCPU()
// 	return int32(float64(cpuCount) * 4.0) // 4x vCPUs para I/O bound
// }

// notifyWorkers escuta notificações de novos jobs
func (p *Processor) notifyWorkers(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-p.newJobNotify:
			p.logger.Debug("new job notification received")
			// Workers vão detectar no próximo tick
		}
	}
}

// checkTimeouts verifica jobs com timeout
func (p *Processor) checkTimeouts(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			p.checkStaleJobs(ctx)
		}
	}
}

// getMetrics retorna metrics se disponível, ou nil se não
func (p *Processor) getMetrics() *Metrics {
	if p.metrics == nil {
		return nil
	}
	return p.metrics
}

// checkStaleJobs verifica e faz timeout de jobs travados
func (p *Processor) checkStaleJobs(ctx context.Context) {
	// Busca jobs processando há mais que o timeout máximo (600 segundos = 10 minutos)
	staleJobs, err := p.queries.GetStaleProcessingJobs(ctx, "600")
	if err != nil {
		p.logger.Warn("failed to get stale jobs", "error", err)
		return
	}

	for _, job := range staleJobs {
		cfg := p.getJobConfig(job.Type)
		timeoutSec := cfg.TimeoutSeconds
		if timeoutSec <= 0 {
			timeoutSec = 300 // Default 5 minutos
		}

		p.logger.Warn("job timed out",
			"job_id", job.ID,
			"job_type", job.Type,
			"timeout_seconds", timeoutSec,
		)

		// Move para dead letter queue
		p.moveToDeadLetterQueue(ctx, job, fmt.Sprintf("Job timed out after %d seconds", timeoutSec))

		// Marca como failed
		_ = p.queries.TimeoutJob(ctx, job.ID)

		if m := p.getMetrics(); m != nil {
			m.JobsFailed.WithLabelValues(job.Type).Inc()
		}
	}
}

// collectHourlyMetrics coleta métricas horárias
func (p *Processor) collectHourlyMetrics(ctx context.Context) {
	if p.metrics == nil {
		return
	}

	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// Coleta métricas da última hora
			hour := time.Now().Format("2006-01-02T15")
			metrics, err := p.queries.GetJobMetricsHourly(ctx, db.GetJobMetricsHourlyParams{
				Hour:  hour,
				Limit: 24,
			})
			if err != nil {
				continue
			}

			for _, m := range metrics {
				p.metrics.ThroughputPerHour.WithLabelValues(m.JobType, "succeeded").Set(float64(m.TotalSucceeded.Int64))
				p.metrics.ThroughputPerHour.WithLabelValues(m.JobType, "failed").Set(float64(m.TotalFailed.Int64))
			}
		}
	}
}

// RescueZombies recupera jobs que ficaram travados em processing
func (p *Processor) RescueZombies(ctx context.Context) error {
	// Busca jobs em processing há mais de 5 minutos (300 segundos)
	staleJobs, err := p.queries.GetStaleProcessingJobs(ctx, "300")
	if err != nil {
		return err
	}

	rescued := 0
	for _, job := range staleJobs {
		p.logger.Info("rescuing zombie job",
			"job_id", job.ID,
			"job_type", job.Type,
			"started_at", job.StartedAt,
		)

		// Reset para pending com retry
		if err := p.queries.UpdateJobNextRetry(ctx, db.UpdateJobNextRetryParams{
			NextRetryAt: sql.NullTime{Time: time.Now(), Valid: true},
			ID:          job.ID,
		}); err != nil {
			p.logger.Warn("failed to update zombie job next_retry", "error", err)
			continue
		}

		// Atualiza status para pending
		if err := p.queries.FailJob(ctx, db.FailJobParams{
			LastError: sql.NullString{String: "Job rescued from zombie state", Valid: true},
			ID:        job.ID,
		}); err != nil {
			p.logger.Warn("failed to reset zombie job status", "error", err)
			continue
		}

		rescued++
	}

	if rescued > 0 {
		p.logger.Info("zombie jobs rescued", "count", rescued)
	}

	return nil
}

func (p *Processor) runWorker(ctx context.Context, workerID int) {
	ticker := time.NewTicker(p.jobPollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			p.processNext(ctx, workerID)
		case <-p.newJobNotify:
			// Processa imediatamente quando notificado
			p.processNext(ctx, workerID)
		}
	}
}

// Wait blocks until all active jobs are finished
func (p *Processor) Wait() {
	p.stopOnce.Do(func() {
		close(p.stopChan)
	})
	// Wait for initial workers to be spawned before waiting for them to finish
	// This prevents race between wg.Add(1) in spawnWorker and wg.Wait() here
	<-p.spawned
	p.wg.Wait()
}

// ActiveJobs returns the number of currently processing jobs
func (p *Processor) ActiveJobs() int64 {
	return p.activeJobs.Load()
}

func (p *Processor) processNext(ctx context.Context, workerID int) {
	// Try to pick a job with worker_id tracking
	job, err := p.queries.PickNextJobWithWorker(ctx, sql.NullString{String: p.workerID, Valid: true})
	if err != nil {
		return // Fila vazia
	}

	// Track active job
	p.activeJobs.Add(1)
	p.processingJobs.Add(1)
	defer p.activeJobs.Add(-1)
	defer p.processingJobs.Add(-1)

	start := time.Now()

	ctx, event := logging.NewEventContext(ctx)
	event.Add(
		slog.Int64("job_id", job.ID),
		slog.String("job_type", job.Type),
		slog.String("worker_id", p.workerID),
	)

	// Check if job is enabled
	cfg := p.getJobConfig(job.Type)
	if !cfg.Enabled {
		p.logger.WarnContext(ctx, "job type is disabled, skipping", event.Attrs()...)
		_ = p.queries.CompleteJob(ctx, job.ID)
		return
	}

	// Circuit Breaker Check - Fail fast se circuit estiver open
	if p.circuitBreaker != nil && !p.circuitBreaker.Allow(job.Type) {
		p.logger.WarnContext(ctx, "circuit breaker is open, requeuing job",
			append(event.Attrs(), slog.String("circuit_state", string(p.circuitBreaker.GetState(job.Type))))...)
		// Re-enfileira com delay
		_ = p.queries.UpdateJobNextRetry(ctx, db.UpdateJobNextRetryParams{
			NextRetryAt: sql.NullTime{Time: time.Now().Add(30 * time.Second), Valid: true},
			ID:          job.ID,
		})
		return
	}

	// Rate Limiter Check - Espera se necessário
	if p.apiThrottler != nil {
		if !p.apiThrottler.Allow(job.Type) {
			// Re-enfileira com delay baseado no rate limit
			delay := p.apiThrottler.Delay(job.Type)
			p.logger.DebugContext(ctx, "rate limit exceeded, requeuing",
				append(event.Attrs(), slog.Duration("delay", delay))...)
			_ = p.queries.UpdateJobNextRetry(ctx, db.UpdateJobNextRetryParams{
				NextRetryAt: sql.NullTime{Time: time.Now().Add(delay), Valid: true},
				ID:          job.ID,
			})
			return
		}
	}

	// Idempotency Check
	processed, err := p.queries.IsJobProcessed(ctx, job.ID)
	if err == nil && processed == 1 {
		p.logger.InfoContext(ctx, "job already processed, skipping", event.Attrs()...)
		_ = p.queries.CompleteJob(ctx, job.ID)
		if m := p.getMetrics(); m != nil {
			m.JobsProcessed.WithLabelValues(job.Type, "skipped").Inc()
		}
		return
	}

	// Create context with timeout
	jobCtx, cancel := context.WithTimeout(ctx, time.Duration(cfg.TimeoutSeconds)*time.Second)
	defer cancel()

	var errProcessing error
	switch job.Type {
	case "send_email":
		errProcessing = p.handleSendEmail(jobCtx, job.Payload)
	case "send_password_reset_email":
		errProcessing = p.handleSendPasswordResetEmail(jobCtx, job.Payload)
	case "send_verification_email":
		errProcessing = p.handleSendVerificationEmail(jobCtx, job.Payload)
	case "process_ai":
		errProcessing = p.handleProcessAI(jobCtx, job.Payload)
	case "process_webhook":
		errProcessing = p.handleProcessWebhook(jobCtx, job.Payload)
	default:
		p.logger.WarnContext(ctx, "unknown job type", "type", job.Type)
		errProcessing = fmt.Errorf("unknown job type: %s", job.Type)
	}

	duration := time.Since(start)

	if errProcessing != nil {
		currentAttempts := int(job.AttemptCount.Int64)
		if !job.AttemptCount.Valid {
			currentAttempts = 0
		}
		newAttemptCount := currentAttempts + 1

		// Backoff exponencial com jitter
		delay := p.calculateBackoffWithJitter(newAttemptCount, cfg)
		nextRetry := time.Now().Add(delay)

		if newAttemptCount >= cfg.MaxAttempts {
			// Max attempts reached, move to dead letter queue
			p.moveToDeadLetterQueue(ctx, job, errProcessing.Error())

			if err := p.queries.FailJob(ctx, db.FailJobParams{
				LastError: sql.NullString{String: errProcessing.Error(), Valid: true},
				ID:        job.ID,
			}); err != nil {
				p.logger.ErrorContext(ctx, "failed to record job failure", "error", err)
			}

			if m := p.getMetrics(); m != nil {
				m.JobsFailed.WithLabelValues(job.Type).Inc()
				m.JobsDLQ.Inc()
				m.JobDuration.WithLabelValues(job.Type, "failed").Observe(duration.Seconds())
			}

			// Record failure no circuit breaker
			if p.circuitBreaker != nil {
				p.circuitBreaker.RecordFailure(ctx, job.Type)
			}

			p.logger.ErrorContext(ctx, "job failed permanently, moved to DLQ",
				append(event.Attrs(),
					slog.String("error", errProcessing.Error()),
					slog.Int("attempts", newAttemptCount),
				)...)
		} else {
			// Schedule retry with jitter
			if err := p.queries.UpdateJobNextRetry(ctx, db.UpdateJobNextRetryParams{
				NextRetryAt: sql.NullTime{Time: nextRetry, Valid: true},
				ID:          job.ID,
			}); err != nil {
				p.logger.ErrorContext(ctx, "failed to update job next_retry_at", "error", err)
			}

			// Usa dbWriter para fail
			if p.dbWriter != nil {
				_ = p.dbWriter.FailJob(ctx, db.FailJobParams{
					LastError: sql.NullString{String: errProcessing.Error(), Valid: true},
					ID:        job.ID,
				})
			} else {
				_ = p.queries.FailJob(ctx, db.FailJobParams{
					LastError: sql.NullString{String: errProcessing.Error(), Valid: true},
					ID:        job.ID,
				})
			}

			if m := p.getMetrics(); m != nil {
				m.JobsFailed.WithLabelValues(job.Type).Inc()
				m.JobDuration.WithLabelValues(job.Type, "retry_scheduled").Observe(duration.Seconds())
			}

			// Record failure no circuit breaker
			if p.circuitBreaker != nil {
				p.circuitBreaker.RecordFailure(ctx, job.Type)
			}

			p.logger.WarnContext(ctx, "job failed, scheduling retry with jitter",
				append(event.Attrs(),
					slog.String("error", errProcessing.Error()),
					slog.Int("attempt", newAttemptCount),
					slog.Int("max_attempts", cfg.MaxAttempts),
					slog.Time("next_retry_at", nextRetry),
					slog.Duration("backoff_delay", delay),
				)...)
		}
		return
	}

	// Success: Usa dbWriter para writes serializados
	if p.dbWriter != nil {
		// Record processed
		if err := p.dbWriter.RecordJobProcessed(ctx, job.ID); err != nil {
			p.logger.ErrorContext(ctx, "failed to record job processed", "error", err)
			return
		}

		// Complete job
		if err := p.dbWriter.CompleteJob(ctx, job.ID); err != nil {
			p.logger.ErrorContext(ctx, "failed to complete job", "error", err)
			return
		}
	} else {
		// Fallback para transação direta se dbWriter não estiver disponível
		dbCtx, cancelDB := context.WithTimeout(ctx, 10*time.Second)
		defer cancelDB()

		tx, err := p.db.BeginTx(dbCtx, nil)
		if err != nil {
			p.logger.ErrorContext(ctx, "failed to start transaction", "error", err)
			return
		}
		defer func() { _ = tx.Rollback() }()

		qtx := p.queries.WithTx(tx)

		if err := qtx.RecordJobProcessed(dbCtx, job.ID); err != nil {
			p.logger.ErrorContext(ctx, "failed to record job processed", "error", err)
			return
		}

		if err := qtx.CompleteJob(dbCtx, job.ID); err != nil {
			p.logger.ErrorContext(ctx, "failed to complete job", "error", err)
			return
		}

		if err := tx.Commit(); err != nil {
			p.logger.ErrorContext(ctx, "failed to commit transaction", "error", err)
			return
		}
	}

	// Update hourly metrics
	hour := time.Now().Format("2006-01-02T15")
	_ = p.queries.UpsertJobMetricsHourly(ctx, db.UpsertJobMetricsHourlyParams{
		Hour:           hour,
		JobType:        job.Type,
		TotalProcessed: sql.NullInt64{Int64: 1, Valid: true},
		TotalSucceeded: sql.NullInt64{Int64: 1, Valid: true},
		TotalFailed:    sql.NullInt64{Int64: 0, Valid: true},
		AvgDurationMs:  sql.NullFloat64{Float64: duration.Seconds() * 1000, Valid: true},
		MinDurationMs:  sql.NullFloat64{Float64: duration.Seconds() * 1000, Valid: true},
		MaxDurationMs:  sql.NullFloat64{Float64: duration.Seconds() * 1000, Valid: true},
	})

	if m := p.getMetrics(); m != nil {
		m.JobsProcessed.WithLabelValues(job.Type, "success").Inc()
		m.JobsSucceeded.WithLabelValues(job.Type).Inc()
		m.JobDuration.WithLabelValues(job.Type, "success").Observe(duration.Seconds())
	}

	// Record success no circuit breaker
	if p.circuitBreaker != nil {
		p.circuitBreaker.RecordSuccess(ctx, job.Type)
	}

	event.Add(slog.Float64("duration_ms", float64(duration.Nanoseconds())/1e6))
	p.logger.InfoContext(ctx, "job completed", event.Attrs()...)

	// SSE notification
	if job.TenantID.Valid {
		var userID int64
		if _, err := fmt.Sscanf(job.TenantID.String, "%d", &userID); err != nil {
			p.logger.DebugContext(ctx, "tenant_id is not numeric, skipping broadcast", "tenant_id", job.TenantID.String)
		} else if userID > 0 {
			p.sse.BroadcastToUser(userID, "job_completed", job.Type)
			return
		}
	}

	p.sse.Broadcast("job_completed", job.Type)
}

// calculateBackoffWithJitter calcula backoff exponencial com jitter
func (p *Processor) calculateBackoffWithJitter(attempt int, cfg JobTypeConfig) time.Duration {
	baseDelay := time.Duration(cfg.BackoffBaseSec) * time.Second
	maxDelay := time.Duration(cfg.BackoffMaxSec) * time.Second

	// Backoff exponencial: baseDelay * 2^(attempt-1)
	delay := baseDelay * time.Duration(1<<(attempt-1))
	if delay > maxDelay {
		delay = maxDelay
	}

	// Adiciona jitter de até 20% para prevenir thundering herd
	jitter := time.Duration(rand.Int63n(int64(delay) / 5))
	delay += jitter

	return delay
}

// moveToDeadLetterQueue move um job para a dead letter queue
func (p *Processor) moveToDeadLetterQueue(ctx context.Context, job db.Job, errorMsg string) {
	metadata, _ := json.Marshal(map[string]interface{}{
		"worker_id":  p.workerID,
		"started_at": job.StartedAt,
		"last_error": errorMsg,
	})

	_, err := p.queries.CreateDeadLetterJob(ctx, db.CreateDeadLetterJobParams{
		JobID:           job.ID,
		OriginalType:    job.Type,
		OriginalPayload: job.Payload,
		ErrorMessage:    errorMsg,
		AttemptCount:    job.AttemptCount.Int64,
		TenantID:        job.TenantID,
		Metadata:        metadata,
	})
	if err != nil {
		p.logger.ErrorContext(ctx, "failed to create dead letter job", "error", err)
	}
}

func (p *Processor) handleSendEmail(ctx context.Context, payload json.RawMessage) error {
	var data struct {
		To      string `json:"to"`
		Subject string `json:"subject"`
		Body    string `json:"body"`
	}

	if err := json.Unmarshal(payload, &data); err != nil {
		return err
	}

	return p.mailer.Send(ctx, data.To, data.Subject, data.Body)
}

func (p *Processor) handleSendVerificationEmail(ctx context.Context, payload json.RawMessage) error {
	var data struct {
		Email string `json:"email"`
		Token string `json:"token"`
	}

	if err := json.Unmarshal(payload, &data); err != nil {
		return err
	}

	subject := "Verifique seu E-mail"
	body := "Olá,\n\nBem-vindo! Clique no link abaixo para verificar seu e-mail:\n\n" +
		"http://localhost:8080/verify-email?token=" + data.Token

	return p.mailer.Send(ctx, data.Email, subject, body)
}

func (p *Processor) handleSendPasswordResetEmail(ctx context.Context, payload json.RawMessage) error {
	var data struct {
		Email string `json:"email"`
		Token string `json:"token"`
	}

	if err := json.Unmarshal(payload, &data); err != nil {
		return err
	}

	subject := "Recuperação de Senha"
	body := "Olá,\n\nClique no link abaixo para redefinir sua senha:\n\n" +
		"http://localhost:8080/reset-password?token=" + data.Token + "\n\n" +
		"Este link expira em 1 hora."

	return p.mailer.Send(ctx, data.Email, subject, body)
}

func (p *Processor) handleProcessAI(ctx context.Context, payload json.RawMessage) error {
	var data struct {
		Prompt string `json:"prompt"`
	}

	if err := json.Unmarshal(payload, &data); err != nil {
		return err
	}

	p.logger.InfoContext(ctx, "AI processing started", slog.String("prompt", data.Prompt))
	time.Sleep(2 * time.Second)

	return nil
}

func (p *Processor) handleProcessWebhook(ctx context.Context, payload json.RawMessage) error {
	var data struct {
		WebhookID int64  `json:"webhook_id"`
		Source    string `json:"source"`
		EventID   string `json:"event_id"`
	}

	if err := json.Unmarshal(payload, &data); err != nil {
		return err
	}

	p.logger.InfoContext(ctx, "processing webhook event",
		slog.Int64("webhook_id", data.WebhookID),
		slog.String("source", data.Source),
		slog.String("event_id", data.EventID),
	)

	if data.Source == "asaas" {
		return p.billingWebhook.ProcessWebhookEvent(ctx, data.WebhookID, data.Source, data.EventID)
	}

	p.logger.DebugContext(ctx, "generic webhook processed",
		slog.Int64("webhook_id", data.WebhookID),
		slog.String("source", data.Source),
	)

	return nil
}
