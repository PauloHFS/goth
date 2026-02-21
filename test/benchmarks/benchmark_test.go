package benchmarks

import (
	"context"
	"database/sql"
	"fmt"
	"net/http/httptest"
	"os"
	"runtime"
	"testing"

	"github.com/PauloHFS/goth/internal/contextkeys"
	"github.com/PauloHFS/goth/internal/db"
	"github.com/PauloHFS/goth/internal/view"
	"github.com/PauloHFS/goth/internal/view/pages"
	_ "github.com/mattn/go-sqlite3"
	"golang.org/x/crypto/bcrypt"
)

const driverName = "sqlite3"

func setupTestDB(b *testing.B, poolMode string) (*sql.DB, *db.Queries) {
	dbFile := fmt.Sprintf("test_perf_%d.db", b.N)
	dsn := dbFile + "?_journal_mode=WAL&_busy_timeout=5000&_synchronous=NORMAL"

	dbConn, err := sql.Open(driverName, dsn)
	if err != nil {
		b.Fatal(err)
	}

	switch poolMode {
	case "single":
		dbConn.SetMaxOpenConns(1)
		dbConn.SetMaxIdleConns(1)
	case "dual":
		dbConn.SetMaxOpenConns(runtime.NumCPU() * 2)
		dbConn.SetMaxIdleConns(runtime.NumCPU())
	default:
	}

	_, _ = dbConn.Exec("PRAGMA temp_store = MEMORY")
	_, _ = dbConn.Exec("PRAGMA cache_size = -32000")

	_, _ = dbConn.Exec("CREATE TABLE users (id INTEGER PRIMARY KEY, tenant_id TEXT, email TEXT, password_hash TEXT, role_id TEXT, is_verified BOOLEAN, avatar_url TEXT, created_at DATETIME)")

	queries := db.New(dbConn)

	for i := range 100 {
		_, _ = queries.CreateUser(context.Background(), db.CreateUserParams{
			TenantID:     "default",
			Email:        fmt.Sprintf("user%d@test.com", i),
			PasswordHash: "hash",
			RoleID:       "user",
		})
	}

	b.Cleanup(func() {
		dbConn.Close()
		os.Remove(dbFile)
		os.Remove(dbFile + "-shm")
		os.Remove(dbFile + "-wal")
	})

	return dbConn, queries
}

func BenchmarkDashboardRendering_Single(b *testing.B) {
	_, queries := setupTestDB(b, "single")
	user := db.User{ID: 1, Email: "admin@test.com"}
	ctx := context.WithValue(context.Background(), contextkeys.UserContextKey, user)

	metrics := NewMetrics()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		start := testing.AllocsPerRun(1, func() {
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

			w := httptest.NewRecorder()
			component := pages.Dashboard(user, users, pag)
			_ = component.Render(ctx, w)
		})
		metrics.AllocsPerOp = int64(start)
	}

	b.ReportMetric(float64(metrics.P50().Nanoseconds()), "ns_p50")
	b.ReportMetric(float64(metrics.P99().Nanoseconds()), "ns_p99")
}

func BenchmarkDashboardRendering_Dual(b *testing.B) {
	_, queries := setupTestDB(b, "dual")
	user := db.User{ID: 1, Email: "admin@test.com"}
	ctx := context.WithValue(context.Background(), contextkeys.UserContextKey, user)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
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

		w := httptest.NewRecorder()
		component := pages.Dashboard(user, users, pag)
		_ = component.Render(ctx, w)
	}
}

func BenchmarkFTS5Search(b *testing.B) {
	dbConn, _ := setupTestDB(b, "single")

	_, _ = dbConn.Exec(`CREATE VIRTUAL TABLE users_fts USING fts5(email, content='users', content_rowid='id')`)

	for i := range 1000 {
		_, _ = dbConn.Exec(`INSERT INTO users_fts(rowid, email) VALUES (?, ?)`,
			i+1,
			fmt.Sprintf("user%d@domain.com", i))
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rows, err := dbConn.Query(`SELECT email FROM users_fts WHERE users_fts MATCH 'user*' LIMIT 20`)
		if err != nil {
			b.Fatal(err)
		}
		rows.Close()
	}
}

func BenchmarkConcurrentReads_Single(b *testing.B) {
	_, queries := setupTestDB(b, "single")

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		ctx := context.Background()
		for pb.Next() {
			_, _ = queries.ListUsersPaginated(ctx, db.ListUsersPaginatedParams{
				TenantID: "default",
				Limit:    10,
				Offset:   0,
			})
		}
	})
}

func BenchmarkConcurrentReads_Dual(b *testing.B) {
	_, queries := setupTestDB(b, "dual")

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		ctx := context.Background()
		for pb.Next() {
			_, _ = queries.ListUsersPaginated(ctx, db.ListUsersPaginatedParams{
				TenantID: "default",
				Limit:    10,
				Offset:   0,
			})
		}
	})
}

func BenchmarkPasswordHashing(b *testing.B) {
	password := "super-secret-password-123"

	b.Run("Bcrypt-Default", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_, _ = bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
		}
	})
}
