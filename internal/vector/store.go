package vector

import (
	"context"
	"database/sql"
	"fmt"

	sqlitevec "github.com/asg017/sqlite-vec-go-bindings/cgo"
)

func init() {
	sqlitevec.Auto()
}

type Store struct {
	db     *sql.DB
	config Config
}

func NewStore(db *sql.DB, config Config) *Store {
	return &Store{
		db:     db,
		config: config,
	}
}

func (s *Store) DB() *sql.DB {
	return s.db
}

func (s *Store) Config() Config {
	return s.config
}

func (s *Store) EnsureTable(ctx context.Context) error {
	query := fmt.Sprintf(`
		CREATE VIRTUAL TABLE IF NOT EXISTS %s USING vec0(
			id INTEGER PRIMARY KEY,
			content_type TEXT,
			content_id INTEGER,
			embedding float[%d],
			metadata TEXT
		)
	`,
		s.config.TableName,
		s.config.EmbeddingDimension,
	)

	_, err := s.db.ExecContext(ctx, query)
	return err
}

func (s *Store) Version(ctx context.Context) (string, error) {
	var version string
	err := s.db.QueryRowContext(ctx, "SELECT vec_version()").Scan(&version)
	return version, err
}
