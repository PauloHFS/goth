package benchmarks

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"os"
	"runtime"
	"testing"

	"github.com/PauloHFS/goth/internal/contextkeys"
	"github.com/PauloHFS/goth/internal/db"
	"github.com/PauloHFS/goth/internal/vector"
	"github.com/PauloHFS/goth/internal/view"
	"github.com/PauloHFS/goth/internal/view/pages"
	sqlitevec "github.com/asg017/sqlite-vec-go-bindings/cgo"
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

// ============================================
// Vector Search Benchmarks (sqlite-vec)
// ============================================

func setupVectorService(b *testing.B, dbConn *sql.DB, dimension int) *vector.Service {
	config := vector.Config{
		Enabled:            true,
		EmbeddingDimension: dimension,
		TableName:          "vectors_test",
	}

	store := vector.NewStore(dbConn, config)

	ctx := context.Background()
	if err := store.EnsureTable(ctx); err != nil {
		b.Fatal(err)
	}

	return vector.NewService(store)
}

func generateRandomVector(dimension int) []float64 {
	vec := make([]float64, dimension)
	for i := range vec {
		vec[i] = float64(i%100) / 100.0
	}
	return vec
}

func BenchmarkVector_Insert(b *testing.B) {
	dbConn, _ := setupTestDB(b, "single")
	service := setupVectorService(b, dbConn, 128)

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		embedding := vector.Embedding{
			ContentType: "document",
			ContentID:   int64(i),
			Vector:      generateRandomVector(128),
			Metadata: map[string]any{
				"title": fmt.Sprintf("Doc %d", i),
			},
		}

		_, err := service.Store(ctx, embedding)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkVector_Search_Cosine(b *testing.B) {
	dbConn, _ := setupTestDB(b, "single")
	service := setupVectorService(b, dbConn, 128)

	ctx := context.Background()

	// Inserir 1000 vetores para busca
	for i := 0; i < 1000; i++ {
		embedding := vector.Embedding{
			ContentType: "document",
			ContentID:   int64(i),
			Vector:      generateRandomVector(128),
			Metadata:    map[string]any{"title": fmt.Sprintf("Doc %d", i)},
		}
		_, _ = service.Store(ctx, embedding)
	}

	queryVector := generateRandomVector(128)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		results, err := service.Search(ctx, "document", queryVector, 10, vector.DistanceCosine)
		if err != nil {
			b.Fatal(err)
		}
		if len(results) == 0 {
			b.Error("expected results")
		}
	}
}

func BenchmarkVector_Search_L2(b *testing.B) {
	dbConn, _ := setupTestDB(b, "single")
	service := setupVectorService(b, dbConn, 128)

	ctx := context.Background()

	for i := 0; i < 1000; i++ {
		embedding := vector.Embedding{
			ContentType: "document",
			ContentID:   int64(i),
			Vector:      generateRandomVector(128),
			Metadata:    map[string]any{"title": fmt.Sprintf("Doc %d", i)},
		}
		_, _ = service.Store(ctx, embedding)
	}

	queryVector := generateRandomVector(128)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		results, err := service.Search(ctx, "document", queryVector, 10, vector.DistanceL2)
		if err != nil {
			b.Fatal(err)
		}
		if len(results) == 0 {
			b.Error("expected results")
		}
	}
}

func BenchmarkVector_Search_Global(b *testing.B) {
	dbConn, _ := setupTestDB(b, "single")
	service := setupVectorService(b, dbConn, 128)

	ctx := context.Background()

	for i := 0; i < 1000; i++ {
		embedding := vector.Embedding{
			ContentType: "document",
			ContentID:   int64(i),
			Vector:      generateRandomVector(128),
			Metadata:    map[string]any{"title": fmt.Sprintf("Doc %d", i)},
		}
		_, _ = service.Store(ctx, embedding)
	}

	queryVector := generateRandomVector(128)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		results, err := service.SearchGlobal(ctx, queryVector, 10, vector.DistanceCosine)
		if err != nil {
			b.Fatal(err)
		}
		if len(results) == 0 {
			b.Error("expected results")
		}
	}
}

func BenchmarkVector_BatchInsert(b *testing.B) {
	dbConn, _ := setupTestDB(b, "single")
	_ = setupVectorService(b, dbConn, 128)

	ctx := context.Background()
	batchSize := 100

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tx, err := dbConn.BeginTx(ctx, nil)
		if err != nil {
			b.Fatal(err)
		}

		for j := 0; j < batchSize; j++ {
			idx := i*batchSize + j
			vectorData := generateRandomVector(128)
			metadata := map[string]any{"title": fmt.Sprintf("Doc %d", idx)}

			// Converter para float32 e serializar
			vector32 := make([]float32, len(vectorData))
			for k, v := range vectorData {
				vector32[k] = float32(v)
			}
			vectorBin, err := sqlitevec.SerializeFloat32(vector32)
			if err != nil {
				tx.Rollback()
				b.Fatal(err)
			}

			metadataJSON, _ := json.Marshal(metadata)

			_, err = tx.ExecContext(ctx,
				`INSERT INTO vectors_test (content_type, content_id, embedding, metadata) VALUES (?, ?, ?, ?)`,
				"document",
				int64(idx),
				vectorBin,
				string(metadataJSON),
			)
			if err != nil {
				tx.Rollback()
				b.Fatal(err)
			}
		}

		if err := tx.Commit(); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkVector_ConcurrentSearch(b *testing.B) {
	dbConn, _ := setupTestDB(b, "dual")
	service := setupVectorService(b, dbConn, 128)

	ctx := context.Background()

	for i := 0; i < 1000; i++ {
		embedding := vector.Embedding{
			ContentType: "document",
			ContentID:   int64(i),
			Vector:      generateRandomVector(128),
			Metadata:    map[string]any{"title": fmt.Sprintf("Doc %d", i)},
		}
		_, _ = service.Store(ctx, embedding)
	}

	queryVector := generateRandomVector(128)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			results, err := service.Search(ctx, "document", queryVector, 10, vector.DistanceCosine)
			if err != nil {
				b.Fatal(err)
			}
			if len(results) == 0 {
				b.Error("expected results")
			}
		}
	})
}

func BenchmarkVector_Dimension_Scale(b *testing.B) {
	dimensions := []int{64, 128, 256}

	for _, dim := range dimensions {
		b.Run(fmt.Sprintf("Dim%d", dim), func(b *testing.B) {
			dbConn, _ := setupTestDB(b, "single")
			service := setupVectorService(b, dbConn, dim)

			ctx := context.Background()

			for i := 0; i < 200; i++ {
				embedding := vector.Embedding{
					ContentType: "document",
					ContentID:   int64(i),
					Vector:      generateRandomVector(dim),
					Metadata:    map[string]any{"title": fmt.Sprintf("Doc %d", i)},
				}
				_, _ = service.Store(ctx, embedding)
			}

			queryVector := generateRandomVector(dim)

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				results, err := service.Search(ctx, "document", queryVector, 10, vector.DistanceCosine)
				if err != nil {
					b.Fatal(err)
				}
				if len(results) == 0 {
					b.Error("expected results")
				}
			}
		})
	}
}
