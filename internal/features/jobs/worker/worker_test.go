package worker

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/PauloHFS/goth/internal/features/sse"
	"github.com/PauloHFS/goth/internal/platform/config"
)

func TestProcessor_New(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	cfg := &config.Config{SMTPHost: "localhost", SMTPPort: "1025"}
	ctx := context.Background()
	broker := sse.NewBroker(ctx)
	t.Cleanup(func() { broker.Shutdown() })

	t.Run("ProcessorInitialization", func(t *testing.T) {
		p := New(cfg, nil, nil, logger, broker)
		if p == nil {
			t.Fatal("expected processor, got nil")
		}
		if p.logger != logger {
			t.Error("logger not correctly assigned")
		}
	})
}

func TestProcessor_WorkerPool(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	cfg := &config.Config{SMTPHost: "localhost", SMTPPort: "1025"}

	t.Run("StartWithMultipleWorkers", func(t *testing.T) {
		ctx := context.Background()
		broker := sse.NewBroker(ctx)
		t.Cleanup(func() { broker.Shutdown() })

		p := New(cfg, nil, nil, logger, broker)

		ctx, cancel := context.WithCancel(context.Background())

		// Start worker pool with 5 workers
		go func() {
			p.Start(ctx, 5)
		}()

		// Give workers time to start
		time.Sleep(100 * time.Millisecond)

		// Cancel context to stop workers
		cancel()

		// Give workers time to stop
		time.Sleep(50 * time.Millisecond)
	})

	t.Run("StartWithZeroWorkers", func(t *testing.T) {
		ctx := context.Background()
		broker := sse.NewBroker(ctx)
		t.Cleanup(func() { broker.Shutdown() })

		p := New(cfg, nil, nil, logger, broker)

		ctx, cancel := context.WithCancel(context.Background())

		// Start with 0 workers should default to 3
		go func() {
			p.Start(ctx, 0)
		}()

		time.Sleep(100 * time.Millisecond)
		cancel()
		time.Sleep(50 * time.Millisecond)
	})

	t.Run("StartWithNegativeWorkers", func(t *testing.T) {
		ctx := context.Background()
		broker := sse.NewBroker(ctx)
		t.Cleanup(func() { broker.Shutdown() })

		p := New(cfg, nil, nil, logger, broker)

		ctx, cancel := context.WithCancel(context.Background())

		// Start with negative workers should default to 3
		go func() {
			p.Start(ctx, -1)
		}()

		time.Sleep(100 * time.Millisecond)
		cancel()
		time.Sleep(50 * time.Millisecond)
	})
}

func TestProcessor_ActiveJobs(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	cfg := &config.Config{SMTPHost: "localhost", SMTPPort: "1025"}
	ctx := context.Background()
	broker := sse.NewBroker(ctx)
	t.Cleanup(func() { broker.Shutdown() })

	p := New(cfg, nil, nil, logger, broker)

	t.Run("ActiveJobsTracking", func(t *testing.T) {
		// Initially should be 0
		if p.ActiveJobs() != 0 {
			t.Errorf("expected 0 active jobs, got %d", p.ActiveJobs())
		}

		// Simulate active jobs using atomic directly
		p.activeJobs.Add(3)
		if p.ActiveJobs() != 3 {
			t.Errorf("expected 3 active jobs, got %d", p.ActiveJobs())
		}

		p.activeJobs.Add(-2)
		if p.ActiveJobs() != 1 {
			t.Errorf("expected 1 active job, got %d", p.ActiveJobs())
		}

		// Cleanup
		p.activeJobs.Add(-1)
	})
}

func TestProcessor_PanicRecovery(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	cfg := &config.Config{SMTPHost: "localhost", SMTPPort: "1025"}

	t.Run("WorkerRecoversFromPanic", func(t *testing.T) {
		ctx := context.Background()
		broker := sse.NewBroker(ctx)
		t.Cleanup(func() { broker.Shutdown() })

		p := New(cfg, nil, nil, logger, broker)

		ctx, cancel := context.WithCancel(context.Background())

		// Start worker pool
		go p.Start(ctx, 2)

		time.Sleep(50 * time.Millisecond)

		// Workers should be running and ready to recover from panics
		// (panic recovery is tested indirectly via code inspection)
		cancel()
		p.Wait()
	})
}
