//go:build fts5
// +build fts5

package benchmarks

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/PauloHFS/goth/internal/contextkeys"
	"github.com/PauloHFS/goth/internal/db"
	"github.com/PauloHFS/goth/internal/features/sse"
	httpMiddleware "github.com/PauloHFS/goth/internal/platform/http/middleware"
	"github.com/PauloHFS/goth/internal/policies"
	"github.com/PauloHFS/goth/internal/validator"
	"github.com/PauloHFS/goth/internal/view"
	"github.com/PauloHFS/goth/internal/view/pages"
	"github.com/alexedwards/scs/v2"
	_ "github.com/mattn/go-sqlite3"
	"golang.org/x/crypto/bcrypt"
)

// TestMain runs all benchmarks and cleans up shared resources
func TestMain(m *testing.M) {
	code := m.Run()
	cleanupTestDB()
	os.Exit(code)
}

// testDB holds a shared database instance for benchmarks
var (
	testDB      *sql.DB
	testQueries *db.Queries
	dbOnce      sync.Once
	dbFile      string
)

// init initializes the shared test database for all benchmarks
func init() {
	dbOnce.Do(func() {
		dbFile = fmt.Sprintf("test_perf_shared_%d.db", time.Now().UnixNano())
		var err error
		testDB, err = sql.Open("sqlite3", dbFile+"?_journal_mode=WAL")
		if err != nil {
			panic(err)
		}

		// Create all necessary tables
		schemas := []string{
			`CREATE TABLE IF NOT EXISTS users (
				id INTEGER PRIMARY KEY,
				tenant_id TEXT,
				email TEXT UNIQUE,
				password_hash TEXT,
				role_id TEXT,
				is_verified BOOLEAN,
				avatar_url TEXT,
				created_at DATETIME
			)`,
			`CREATE TABLE IF NOT EXISTS webhooks (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				source TEXT NOT NULL,
				external_id TEXT,
				payload JSON NOT NULL,
				headers JSON NOT NULL,
				status TEXT NOT NULL DEFAULT 'pending',
				created_at DATETIME DEFAULT CURRENT_TIMESTAMP
			)`,
			`CREATE TABLE IF NOT EXISTS jobs (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				tenant_id TEXT,
				type TEXT NOT NULL,
				payload JSON NOT NULL,
				status TEXT NOT NULL DEFAULT 'pending',
				idempotency_key TEXT,
				attempt_count INTEGER DEFAULT 0,
				max_attempts INTEGER DEFAULT 3,
				last_error TEXT,
				run_at DATETIME DEFAULT CURRENT_TIMESTAMP,
				next_retry_at DATETIME,
				worker_id TEXT,
				started_at DATETIME,
				completed_at DATETIME,
				timeout_seconds INTEGER DEFAULT 300,
				priority INTEGER DEFAULT 5,
				created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
				updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
			)`,
			`CREATE TABLE IF NOT EXISTS posts (
				id INTEGER PRIMARY KEY,
				tenant_id TEXT,
				user_id INTEGER,
				title TEXT,
				content TEXT
			)`,
			`CREATE VIRTUAL TABLE IF NOT EXISTS posts_idx USING fts5(
				title,
				content,
				content='posts',
				content_rowid='id'
			)`,
		}

		for _, schema := range schemas {
			if _, err := testDB.Exec(schema); err != nil {
				panic(err)
			}
		}

		testQueries = db.New(testDB)

		// Seed users
		for i := 0; i < 100; i++ {
			_, _ = testQueries.CreateUser(context.Background(), db.CreateUserParams{
				TenantID:     "default",
				Email:        fmt.Sprintf("user%d@test.com", i),
				PasswordHash: "hash",
				RoleID:       "user",
			})
		}

		// Seed posts for FTS5
		for i := 0; i < 1000; i++ {
			_, _ = testDB.Exec(`INSERT OR IGNORE INTO posts (id, tenant_id, user_id, title, content) VALUES (?, ?, ?, ?, ?)`,
				i+1, "default", 1,
				fmt.Sprintf("Post Title %d", i),
				fmt.Sprintf("This is the content of post %d. It contains some keywords like GOTH and SQLite.", i))
			_, _ = testDB.Exec(`INSERT OR IGNORE INTO posts_idx (rowid, title, content) VALUES (?, ?, ?)`,
				i+1, fmt.Sprintf("Post Title %d", i),
				fmt.Sprintf("This is the content of post %d. It contains some keywords like GOTH and SQLite.", i))
		}
	})
}

// cleanupTestDB cleans up the shared test database
// Call this in TestMain or when all benchmarks are done
func cleanupTestDB() {
	if testDB != nil {
		testDB.Close()
	}
	os.Remove(dbFile)
	os.Remove(dbFile + "-shm")
	os.Remove(dbFile + "-wal")
}

// setupSharedTestDB returns the shared test database instance
func setupSharedTestDB() (*sql.DB, *db.Queries) {
	return testDB, testQueries
}

// setupIsolatedTestDB creates a fresh database for benchmarks that need complete isolation
// This prevents state leakage between benchmark runs
func setupIsolatedTestDB(b *testing.B) (*sql.DB, *db.Queries) {
	b.Helper()
	dbFile := fmt.Sprintf("test_perf_isolated_%d_%d.db", os.Getpid(), time.Now().UnixNano())
	dbConn, err := sql.Open("sqlite3", dbFile+"?_journal_mode=WAL&_busy_timeout=5000")
	if err != nil {
		b.Fatal(err)
	}

	// Create minimal schema
	schemas := []string{
		`CREATE TABLE IF NOT EXISTS users (
			id INTEGER PRIMARY KEY,
			tenant_id TEXT,
			email TEXT UNIQUE,
			password_hash TEXT,
			role_id TEXT,
			is_verified BOOLEAN,
			avatar_url TEXT,
			created_at DATETIME
		)`,
		`CREATE TABLE IF NOT EXISTS jobs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			tenant_id TEXT,
			type TEXT NOT NULL,
			payload JSON NOT NULL,
			status TEXT NOT NULL DEFAULT 'pending',
			idempotency_key TEXT,
			attempt_count INTEGER DEFAULT 0,
			max_attempts INTEGER DEFAULT 3,
			last_error TEXT,
			run_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			next_retry_at DATETIME,
			worker_id TEXT,
			started_at DATETIME,
			completed_at DATETIME,
			timeout_seconds INTEGER DEFAULT 300,
			priority INTEGER DEFAULT 5,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS webhooks (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			source TEXT NOT NULL,
			external_id TEXT,
			payload JSON NOT NULL,
			headers JSON NOT NULL,
			status TEXT NOT NULL DEFAULT 'pending',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
	}

	for _, schema := range schemas {
		if _, err := dbConn.Exec(schema); err != nil {
			b.Fatal(err)
		}
	}

	queries := db.New(dbConn)

	b.Cleanup(func() {
		dbConn.Close()
		os.Remove(dbFile)
		os.Remove(dbFile + "-shm")
		os.Remove(dbFile + "-wal")
	})

	return dbConn, queries
}

// clearJobs removes all jobs from the database to ensure clean state
func clearJobs(b *testing.B, db *sql.DB) {
	b.Helper()
	ctx := context.Background()
	_, _ = db.ExecContext(ctx, "DELETE FROM jobs WHERE 1=1")
}

func BenchmarkDashboardRendering(b *testing.B) {
	_, queries := setupSharedTestDB()

	user := db.User{ID: 1, Email: "admin@test.com"}
	ctx := context.WithValue(context.Background(), contextkeys.UserContextKey, user)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Simular lógica do handler
		page := 1
		perPage := 10
		offset := (page - 1) * perPage

		users, _ := queries.ListUsersPaginated(ctx, db.ListUsersPaginatedParams{
			TenantID: "default",
			Limit:    int64(perPage),
			Offset:   int64(offset),
		})

		totalUsers, _ := queries.CountUsers(ctx, "default")
		pag := view.NewPagination(page, int(totalUsers), perPage)

		// Renderizar Templ para o Recorder (mede CPU e Memória de renderização)
		w := httptest.NewRecorder()
		component := pages.Dashboard(user, users, pag)
		_ = component.Render(ctx, w)
	}
}

func BenchmarkConcurrentWebhookIngestion(b *testing.B) {
	_, queries := setupSharedTestDB()

	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			ctx := context.Background()
			// Simular o GOTH Ingestion flow: Persistir Webhook + Criar Job
			_, err := queries.CreateWebhook(ctx, db.CreateWebhookParams{
				Source:     "stripe",
				ExternalID: sql.NullString{String: "evt_test", Valid: true},
				Payload:    []byte(`{"id": "evt_test"}`),
				Headers:    []byte(`{}`),
			})
			if err != nil {
				b.Error(err)
			}

			_, err = queries.CreateJob(ctx, db.CreateJobParams{
				TenantID: sql.NullString{String: "1", Valid: true},
				Type:     "process_webhook",
				Payload:  []byte(`{"webhook_id": 1}`),
			})
			if err != nil {
				b.Error(err)
			}
		}
	})
}

func BenchmarkFTS5Search(b *testing.B) {
	_, _ = setupSharedTestDB()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rows, err := testDB.Query(`SELECT title FROM posts_idx WHERE posts_idx MATCH 'GOTH AND SQLite' LIMIT 20`)
		if err != nil {
			b.Fatal(err)
		}
		rows.Close()
	}
}

func BenchmarkPasswordHashing(b *testing.B) {
	password := "super-secret-password-123"

	b.Run("Bcrypt-Cost-10", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_, _ = bcrypt.GenerateFromPassword([]byte(password), 10)
		}
	})

	b.Run("Bcrypt-Cost-12", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_, _ = bcrypt.GenerateFromPassword([]byte(password), 12)
		}
	})

	b.Run("Bcrypt-Cost-14", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_, _ = bcrypt.GenerateFromPassword([]byte(password), 14)
		}
	})
}

func BenchmarkSQLiteReadWriteStress(b *testing.B) {
	_, queries := setupSharedTestDB()

	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		ctx := context.Background()

		for pb.Next() {
			// Mix de 80% leitura e 20% escrita
			if b.N%5 == 0 {
				// Escrita
				_, _ = queries.CreateWebhook(ctx, db.CreateWebhookParams{
					Source:  "stress-test",
					Payload: []byte(`{}`),
					Headers: []byte(`{}`),
				})
			} else {
				// Leitura (Dashboard)
				_, _ = queries.ListUsersPaginated(ctx, db.ListUsersPaginatedParams{
					TenantID: "default",
					Limit:    10,
					Offset:   0,
				})
			}
		}
	})
}

// =============================================================================
// Middleware Benchmarks
// =============================================================================

func BenchmarkRequireAuthMiddleware(b *testing.B) {
	_, queries := setupSharedTestDB()

	// Create session manager
	sessionManager := scs.New()

	// Create test user
	user := db.User{ID: 1, Email: "test@test.com", TenantID: "default"}

	// Create handler to wrap
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Create request with session context
		req := httptest.NewRequest("GET", "/protected", nil)
		w := httptest.NewRecorder()

		// Load session and set user_id
		ctx, _ := sessionManager.Load(req.Context(), "")
		sessionManager.Put(ctx, "user_id", user.ID)
		ctx = context.WithValue(ctx, contextkeys.UserContextKey, user)
		req = req.WithContext(ctx)

		// Wrap with middleware
		wrapped := httpMiddleware.RequireAuth(sessionManager, queries, nextHandler)
		wrapped.ServeHTTP(w, req)
	}
}

func BenchmarkSessionLookup(b *testing.B) {
	sessionManager := scs.New()

	b.Run("ValidSession", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			req := httptest.NewRequest("GET", "/test", nil)
			w := httptest.NewRecorder()

			// Load session and simulate session operations
			ctx, _ := sessionManager.Load(req.Context(), "")
			sessionManager.Put(ctx, "user_id", int64(1))
			_ = sessionManager.GetInt64(ctx, "user_id")

			_ = w
		}
	})
}

// =============================================================================
// SSE Broker Benchmarks
// =============================================================================

func BenchmarkBroadcastScalability(b *testing.B) {
	ctx := context.Background()

	benchCases := []struct {
		name       string
		numClients int
		numUsers   int
	}{
		{"10Clients-1User", 10, 1},
		{"100Clients-10Users", 100, 10},
		{"1000Clients-100Users", 1000, 100},
	}

	for _, bc := range benchCases {
		b.Run(bc.name, func(b *testing.B) {
			broker := sse.NewBroker(ctx)
			b.Cleanup(func() { broker.Shutdown() })

			// Register clients
			clients := make([]chan string, bc.numClients)
			for i := 0; i < bc.numClients; i++ {
				userID := int64(i%bc.numUsers + 1)
				ch := make(chan string, 100)
				clients[i] = ch
				sse.RegisterClient(broker, userID, ch)
			}

			b.Cleanup(func() {
				for i := 0; i < bc.numClients; i++ {
					userID := int64(i%bc.numUsers + 1)
					sse.UnregisterClient(broker, userID, clients[i])
				}
			})

			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				broker.Broadcast("test_event", fmt.Sprintf("data_%d", i))
			}
			b.StopTimer()
		})
	}
}

func BenchmarkBroadcastToUser(b *testing.B) {
	ctx := context.Background()
	broker := sse.NewBroker(ctx)
	b.Cleanup(func() { broker.Shutdown() })

	// Register multiple users
	numUsers := 100
	clients := make([]chan string, numUsers)
	for i := 0; i < numUsers; i++ {
		ch := make(chan string, 100)
		clients[i] = ch
		sse.RegisterClient(broker, int64(i+1), ch)
	}

	b.Cleanup(func() {
		for i := 0; i < numUsers; i++ {
			sse.UnregisterClient(broker, int64(i+1), clients[i])
		}
	})

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		targetUser := int64(i%numUsers + 1)
		broker.BroadcastToUser(targetUser, "test_event", fmt.Sprintf("data_%d", i))
	}
}

func BenchmarkSSEClientRegistration(b *testing.B) {
	ctx := context.Background()
	broker := sse.NewBroker(ctx)
	b.Cleanup(func() { broker.Shutdown() })

	b.Run("RegisterClient", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			ch := make(chan string, 10)
			sse.RegisterClient(broker, int64(i%100+1), ch)
			b.StopTimer()
			sse.UnregisterClient(broker, int64(i%100+1), ch)
			b.StartTimer()
		}
	})

	b.Run("UnregisterClient", func(b *testing.B) {
		// Pre-register clients
		clients := make([]chan string, 1000)
		for i := 0; i < 1000; i++ {
			ch := make(chan string, 10)
			clients[i] = ch
			sse.RegisterClient(broker, int64(i%100+1), ch)
		}

		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			idx := i % 1000
			sse.UnregisterClient(broker, int64(idx%100+1), clients[idx])
			b.StopTimer()
			sse.RegisterClient(broker, int64(idx%100+1), clients[idx])
			b.StartTimer()
		}
	})
}

// =============================================================================
// Validator & Policies Benchmarks
// =============================================================================

func BenchmarkInputValidation(b *testing.B) {
	val := validator.New()

	type TestStruct struct {
		Email    string `validate:"required,email"`
		Password string `validate:"required,min=8,max=50"`
		Name     string `validate:"required,min=2,max=100"`
		Age      int    `validate:"required,min=18,max=120"`
	}

	validData := TestStruct{
		Email:    "test@example.com",
		Password: "securepassword123",
		Name:     "Test User",
		Age:      25,
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = val.Validate(validData)
	}
}

func BenchmarkPostPolicyChecks(b *testing.B) {
	adminUser := db.User{ID: 1, RoleID: "admin"}
	regularUser := db.User{ID: 2, RoleID: "user"}
	otherUser := db.User{ID: 3, RoleID: "user"}

	post := db.Post{ID: 1, UserID: 2}

	b.Run("Admin-CacheHit", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = policies.CanEditPost(adminUser, post)
		}
	})

	b.Run("Owner-CacheHit", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = policies.CanEditPost(regularUser, post)
		}
	})

	b.Run("NonOwner-CacheMiss", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = policies.CanEditPost(otherUser, post)
		}
	})
}

// =============================================================================
// Database Query Benchmarks
// =============================================================================

func BenchmarkUserAuthentication(b *testing.B) {
	_, queries := setupSharedTestDB()

	// Create user with real password hash
	hash, _ := bcrypt.GenerateFromPassword([]byte("password123"), bcrypt.DefaultCost)
	_, _ = queries.CreateUser(context.Background(), db.CreateUserParams{
		TenantID:     "default",
		Email:        "auth@test.com",
		PasswordHash: string(hash),
		RoleID:       "user",
	})

	b.ResetTimer()
	b.Run("GetUserByEmail", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_, _ = queries.GetUserByEmail(context.Background(), db.GetUserByEmailParams{
				TenantID: "default",
				Email:    "auth@test.com",
			})
		}
	})

	b.Run("GetUserByID", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_, _ = queries.GetUserByID(context.Background(), 1)
		}
	})
}

func BenchmarkTenantIsolation(b *testing.B) {
	_, queries := setupSharedTestDB()

	// Create users in different tenants
	tenants := []string{"tenant-a", "tenant-b", "tenant-c"}
	for _, tenant := range tenants {
		for i := 0; i < 50; i++ {
			_, _ = queries.CreateUser(context.Background(), db.CreateUserParams{
				TenantID:     tenant,
				Email:        fmt.Sprintf("user-%d@%s.com", i, tenant),
				PasswordHash: "hash",
				RoleID:       "user",
			})
		}
	}

	b.ResetTimer()
	for _, tenant := range tenants {
		b.Run(fmt.Sprintf("Tenant-%s", tenant), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_, _ = queries.ListUsersPaginated(context.Background(), db.ListUsersPaginatedParams{
					TenantID: tenant,
					Limit:    10,
					Offset:   0,
				})
			}
		})
	}
}

func BenchmarkPaginationLargeDataset(b *testing.B) {
	dbFile := fmt.Sprintf("test_pagination_%d.db", time.Now().UnixNano())
	dbConn, err := sql.Open("sqlite3", dbFile+"?_journal_mode=WAL")
	if err != nil {
		panic(err)
	}
	defer func() {
		dbConn.Close()
		os.Remove(dbFile)
		os.Remove(dbFile + "-shm")
		os.Remove(dbFile + "-wal")
	}()

	_, _ = dbConn.Exec("CREATE TABLE users (id INTEGER PRIMARY KEY, tenant_id TEXT, email TEXT, password_hash TEXT, role_id TEXT, created_at DATETIME)")
	queries := db.New(dbConn)

	// Insert 10K users
	for i := 0; i < 10000; i++ {
		_, _ = queries.CreateUser(context.Background(), db.CreateUserParams{
			TenantID:     "default",
			Email:        fmt.Sprintf("user%d@test.com", i),
			PasswordHash: "hash",
			RoleID:       "user",
		})
	}

	benchSizes := []struct {
		name  string
		page  int
		limit int
	}{
		{"Page1-10", 1, 10},
		{"Page100-10", 100, 10},
		{"Page1000-10", 1000, 10},
		{"Page1-100", 1, 100},
	}

	b.ResetTimer()
	for _, bs := range benchSizes {
		b.Run(bs.name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_, _ = queries.ListUsersPaginated(context.Background(), db.ListUsersPaginatedParams{
					TenantID: "default",
					Limit:    int64(bs.limit),
					Offset:   int64((bs.page - 1) * bs.limit),
				})
			}
		})
	}
}

// =============================================================================
// Template Rendering Benchmarks
// =============================================================================

func BenchmarkComponentRendering(b *testing.B) {
	_, queries := setupSharedTestDB()

	user := db.User{ID: 1, Email: "admin@test.com", RoleID: "admin"}
	ctx := context.WithValue(context.Background(), contextkeys.UserContextKey, user)

	users, _ := queries.ListUsersPaginated(ctx, db.ListUsersPaginatedParams{
		TenantID: "default",
		Limit:    10,
		Offset:   0,
	})
	totalUsers, _ := queries.CountUsers(ctx, "default")
	pag := view.NewPagination(1, int(totalUsers), 10)

	b.Run("Dashboard", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			w := httptest.NewRecorder()
			component := pages.Dashboard(user, users, pag)
			_ = component.Render(ctx, w)
		}
	})
}

func BenchmarkTemplateWithLoops(b *testing.B) {
	_, queries := setupSharedTestDB()

	user := db.User{ID: 1, Email: "admin@test.com"}
	ctx := context.WithValue(context.Background(), contextkeys.UserContextKey, user)

	benchSizes := []struct {
		name    string
		perPage int
	}{
		{"10Items", 10},
		{"50Items", 50},
		{"100Items", 100},
	}

	for _, bs := range benchSizes {
		b.Run(bs.name, func(b *testing.B) {
			users, _ := queries.ListUsersPaginated(ctx, db.ListUsersPaginatedParams{
				TenantID: "default",
				Limit:    int64(bs.perPage),
				Offset:   0,
			})
			totalUsers, _ := queries.CountUsers(ctx, "default")
			pag := view.NewPagination(1, int(totalUsers), bs.perPage)

			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				w := httptest.NewRecorder()
				component := pages.Dashboard(user, users, pag)
				_ = component.Render(ctx, w)
			}
		})
	}
}

// =============================================================================
// Advanced Concurrency Benchmarks
// =============================================================================

func BenchmarkReadWriteContention(b *testing.B) {
	// Use isolated DB with busy timeout for better concurrency handling
	dbConn, queries := setupIsolatedTestDB(b)

	// Use mutex to serialize writes and prevent SQLite lock contention
	var writeMu sync.Mutex
	var writeCounter int64

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		ctx := context.Background()
		for pb.Next() {
			// 90% reads, 10% writes using atomic counter
			if atomic.AddInt64(&writeCounter, 1)%10 == 0 {
				// Serialize writes to prevent SQLite lock contention
				writeMu.Lock()
				_, _ = queries.CreateWebhook(ctx, db.CreateWebhookParams{
					Source:  "contention-test",
					Payload: []byte(`{}`),
					Headers: []byte(`{}`),
				})
				writeMu.Unlock()
			} else {
				// Reads can be concurrent
				_, _ = queries.GetUserByID(ctx, 1)
			}
		}
	})

	// Prevent compiler optimization
	_ = dbConn
}

func BenchmarkIdempotencyChecks(b *testing.B) {
	_, queries := setupSharedTestDB()

	ctx := context.Background()

	// Create job with idempotency key
	_, _ = queries.CreateJob(ctx, db.CreateJobParams{
		TenantID:       sql.NullString{String: "default", Valid: true},
		Type:           "test_job",
		Payload:        []byte(`{}`),
		IdempotencyKey: sql.NullString{String: "unique-key-123", Valid: true},
	})

	b.ResetTimer()
	b.Run("CheckIdempotency", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_, _ = queries.IsJobProcessed(ctx, 1)
		}
	})
}

// =============================================================================
// Comparative Benchmarks
// =============================================================================

func BenchmarkBcryptCosts(b *testing.B) {
	password := []byte("benchmark-password-123!")

	b.Run("Cost-10", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_, _ = bcrypt.GenerateFromPassword(password, 10)
		}
	})

	b.Run("Cost-12", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_, _ = bcrypt.GenerateFromPassword(password, 12)
		}
	})

	b.Run("Cost-14", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_, _ = bcrypt.GenerateFromPassword(password, 14)
		}
	})

	b.Run("Verify-Cost-10", func(b *testing.B) {
		hash, _ := bcrypt.GenerateFromPassword(password, 10)
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = bcrypt.CompareHashAndPassword(hash, password)
		}
	})

	b.Run("Verify-Cost-12", func(b *testing.B) {
		hash, _ := bcrypt.GenerateFromPassword(password, 12)
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = bcrypt.CompareHashAndPassword(hash, password)
		}
	})

	b.Run("Verify-Cost-14", func(b *testing.B) {
		hash, _ := bcrypt.GenerateFromPassword(password, 14)
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = bcrypt.CompareHashAndPassword(hash, password)
		}
	})
}

// =============================================================================
// Worker / Job Queue Benchmarks
// =============================================================================

func BenchmarkJobQueueOperations(b *testing.B) {
	// Use isolated DB to prevent state leakage
	dbConn, queries := setupIsolatedTestDB(b)

	ctx := context.Background()

	b.Run("CreateJob", func(b *testing.B) {
		// Clear state before benchmark
		clearJobs(b, dbConn)

		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = queries.CreateJob(ctx, db.CreateJobParams{
				TenantID: sql.NullString{String: "default", Valid: true},
				Type:     "benchmark_job",
				Payload:  []byte(`{"iteration": i}`),
			})
		}
	})

	b.Run("PickNextJob", func(b *testing.B) {
		// Clear and pre-create jobs for each benchmark iteration
		clearJobs(b, dbConn)
		for i := 0; i < 100; i++ {
			_, _ = queries.CreateJob(ctx, db.CreateJobParams{
				TenantID: sql.NullString{String: "default", Valid: true},
				Type:     "benchmark_job",
				Payload:  []byte(`{}`),
			})
		}

		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			// Clear jobs after each iteration to prevent accumulation
			if i%10 == 0 {
				clearJobs(b, dbConn)
				for j := 0; j < 100; j++ {
					_, _ = queries.CreateJob(ctx, db.CreateJobParams{
						TenantID: sql.NullString{String: "default", Valid: true},
						Type:     "benchmark_job",
						Payload:  []byte(`{}`),
					})
				}
			}
			_, _ = queries.PickNextJob(ctx)
		}
	})

	b.Run("CompleteJob", func(b *testing.B) {
		// Clear and pre-create jobs
		clearJobs(b, dbConn)
		jobIDs := make([]int64, 100)
		for i := 0; i < 100; i++ {
			job, _ := queries.CreateJob(ctx, db.CreateJobParams{
				TenantID: sql.NullString{String: "default", Valid: true},
				Type:     "benchmark_job",
				Payload:  []byte(`{}`),
			})
			jobIDs[i] = job.ID
		}

		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			idx := i % 100
			_ = queries.CompleteJob(ctx, jobIDs[idx])
			// Recreate job for next iteration
			queries.CreateJob(ctx, db.CreateJobParams{
				TenantID: sql.NullString{String: "default", Valid: true},
				Type:     "benchmark_job",
				Payload:  []byte(`{}`),
			})
		}
	})
}

func BenchmarkConcurrentJobProcessing(b *testing.B) {
	_, queries := setupSharedTestDB()

	ctx := context.Background()

	// Pre-create jobs for workers to pick up
	for i := 0; i < 1000; i++ {
		_, _ = queries.CreateJob(ctx, db.CreateJobParams{
			TenantID: sql.NullString{String: "default", Valid: true},
			Type:     "concurrent_job",
			Payload:  []byte(`{}`),
		})
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			// Simulate worker: pick -> process -> complete
			job, err := queries.PickNextJob(ctx)
			if err != nil {
				continue // No jobs available
			}

			// Simulate processing
			_ = queries.CompleteJob(ctx, job.ID)
		}
	})
}

func BenchmarkJobIdempotency(b *testing.B) {
	_, queries := setupSharedTestDB()

	ctx := context.Background()

	b.Run("CreateJobWithIdempotencyKey", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = queries.CreateJob(ctx, db.CreateJobParams{
				TenantID:       sql.NullString{String: "default", Valid: true},
				Type:           "idempotent_job",
				Payload:        []byte(`{}`),
				IdempotencyKey: sql.NullString{String: fmt.Sprintf("key-%d", i%100), Valid: true},
			})
		}
	})

	b.Run("CheckProcessedJob", func(b *testing.B) {
		// Pre-create and process a job
		job, _ := queries.CreateJob(ctx, db.CreateJobParams{
			TenantID: sql.NullString{String: "default", Valid: true},
			Type:     "idempotent_job",
			Payload:  []byte(`{}`),
		})
		_ = queries.CompleteJob(ctx, job.ID)

		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = queries.IsJobProcessed(ctx, job.ID)
		}
	})
}

func BenchmarkZombieJobRecovery(b *testing.B) {
	_, queries := setupSharedTestDB()

	ctx := context.Background()

	// Create jobs and mark them as processing (simulating zombies)
	for i := 0; i < 100; i++ {
		_, _ = queries.CreateJob(ctx, db.CreateJobParams{
			TenantID: sql.NullString{String: "default", Valid: true},
			Type:     "zombie_job",
			Payload:  []byte(`{}`),
		})
		// Simulate worker picking the job
		_, _ = queries.PickNextJob(ctx)
	}

	b.ResetTimer()
	b.Run("GetStaleProcessingJobs", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_, _ = queries.GetStaleProcessingJobs(ctx, "300") // 5 minutes
		}
	})

	b.Run("RecoverZombieJobs", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			staleJobs, _ := queries.GetStaleProcessingJobs(ctx, "300")
			for _, job := range staleJobs {
				_ = queries.FailJob(ctx, db.FailJobParams{
					LastError: sql.NullString{String: "Zombie recovery", Valid: true},
					ID:        job.ID,
				})
			}
			// Recreate zombie jobs for next iteration
			if i%10 == 0 {
				for j := 0; j < 10; j++ {
					_, _ = queries.CreateJob(ctx, db.CreateJobParams{
						TenantID: sql.NullString{String: "default", Valid: true},
						Type:     "zombie_job",
						Payload:  []byte(`{}`),
					})
					_, _ = queries.PickNextJob(ctx)
				}
			}
		}
	})
}

func BenchmarkJobPriorityScheduling(b *testing.B) {
	_, queries := setupSharedTestDB()

	ctx := context.Background()

	// Create jobs for scheduling
	for i := 0; i < 100; i++ {
		_, _ = queries.CreateJob(ctx, db.CreateJobParams{
			TenantID: sql.NullString{String: "default", Valid: true},
			Type:     "priority_job",
			Payload:  []byte(`{}`),
		})
	}

	b.ResetTimer()
	b.Run("PickNextJob-FIFO", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_, _ = queries.PickNextJob(ctx)
		}
	})
}
