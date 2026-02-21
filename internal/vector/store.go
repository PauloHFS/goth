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
			content_type TEXT NOT NULL,
			content_id INTEGER NOT NULL,
			embedding float[%d],
			metadata TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);
		
		CREATE INDEX IF NOT EXISTS idx_%s_content ON %s(content_type, content_id);
		CREATE INDEX IF NOT EXISTS idx_%s_content_type ON %s(content_type);
	`,
		s.config.TableName,
		s.config.EmbeddingDimension,
		s.config.TableName, s.config.TableName,
		s.config.TableName, s.config.TableName,
	)

	_, err := s.db.ExecContext(ctx, query)
	return err
}

func (s *Store) Version(ctx context.Context) (string, error) {
	var version string
	err := s.db.QueryRowContext(ctx, "SELECT vec_version()").Scan(&version)
	return version, err
}
