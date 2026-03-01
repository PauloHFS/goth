package user

import (
	"context"
	"database/sql"

	"github.com/PauloHFS/goth/internal/db"
)

type UserRepository interface {
	GetByID(ctx context.Context, id int64) (db.User, error)
	UpdateAvatar(ctx context.Context, id int64, url string) error
	ListPaginated(ctx context.Context, params ListParams) ([]db.User, int64, error)
}

type ListParams struct {
	TenantID string
	Search   string
	Page     int
	PerPage  int
}

type repository struct {
	db *sql.DB
	q  *db.Queries
}

func NewRepository(dbConn *sql.DB) UserRepository {
	return &repository{
		db: dbConn,
		q:  db.New(dbConn),
	}
}

func (r *repository) GetByID(ctx context.Context, id int64) (db.User, error) {
	return r.q.GetUserByID(ctx, id)
}

func (r *repository) UpdateAvatar(ctx context.Context, id int64, url string) error {
	return r.q.UpdateUserAvatar(ctx, db.UpdateUserAvatarParams{
		AvatarUrl: sql.NullString{String: url, Valid: true},
		ID:        id,
	})
}

func (r *repository) ListPaginated(ctx context.Context, params ListParams) ([]db.User, int64, error) {
	users, err := r.q.ListUsersPaginated(ctx, db.ListUsersPaginatedParams{
		TenantID: params.TenantID,
		Column2:  sql.NullString{String: params.Search, Valid: params.Search != ""},
		Column3:  sql.NullString{String: params.Search, Valid: params.Search != ""},
		Limit:    int64(params.PerPage),
		Offset:   int64((params.Page - 1) * params.PerPage),
	})
	if err != nil {
		return nil, 0, err
	}

	total, err := r.q.CountUsers(ctx, params.TenantID)
	if err != nil {
		return nil, 0, err
	}

	return users, total, nil
}
