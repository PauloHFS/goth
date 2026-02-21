package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	HttpRequestsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "http_requests_total",
		Help: "Total number of HTTP requests",
	}, []string{"path", "method", "status"})

	JobDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name: "job_processing_seconds",
		Help: "Time taken to process jobs",
	}, []string{"type", "status"})

	JobsProcessed = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "jobs_processed_total",
		Help: "Total number of processed jobs",
	}, []string{"type", "status"})

	JobRetries = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "job_retries_total",
		Help: "Total number of job retries",
	}, []string{"type"})

	JobsDeadLetter = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "jobs_dead_letter_total",
		Help: "Total number of jobs moved to dead letter queue",
	}, []string{"type"})
)
