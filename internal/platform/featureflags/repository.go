package featureflags

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

// FeatureFlag representa uma feature flag no sistema
type FeatureFlag struct {
	ID          int64     `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Enabled     bool      `json:"enabled"`
	Metadata    string    `json:"metadata,omitempty"` // JSON adicional para config
	TenantID    string    `json:"tenant_id"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// FeatureFlagInput representa input para criar/atualizar feature flag
type FeatureFlagInput struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Enabled     bool   `json:"enabled"`
	Metadata    string `json:"metadata,omitempty"`
	TenantID    string `json:"tenant_id"`
}

// Repository para persistência de feature flags
type Repository struct {
	db *sql.DB
}

// NewRepository cria nova instância do repositório
func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

// GetAll retorna todas as feature flags
func (r *Repository) GetAll(ctx context.Context, tenantID string) ([]FeatureFlag, error) {
	query := `
		SELECT id, name, description, enabled, metadata, tenant_id, created_at, updated_at
		FROM feature_flags
		WHERE tenant_id = ? OR tenant_id = 'global'
		ORDER BY name ASC
	`

	rows, err := r.db.QueryContext(ctx, query, tenantID)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = rows.Close()
	}()

	var flags []FeatureFlag
	for rows.Next() {
		var f FeatureFlag
		err := rows.Scan(&f.ID, &f.Name, &f.Description, &f.Enabled, &f.Metadata, &f.TenantID, &f.CreatedAt, &f.UpdatedAt)
		if err != nil {
			return nil, err
		}
		flags = append(flags, f)
	}

	return flags, rows.Err()
}

// GetByName retorna uma feature flag específica
func (r *Repository) GetByName(ctx context.Context, name, tenantID string) (*FeatureFlag, error) {
	query := `
		SELECT id, name, description, enabled, metadata, tenant_id, created_at, updated_at
		FROM feature_flags
		WHERE name = ? AND (tenant_id = ? OR tenant_id = 'global')
		LIMIT 1
	`

	var f FeatureFlag
	err := r.db.QueryRowContext(ctx, query, name, tenantID).Scan(
		&f.ID, &f.Name, &f.Description, &f.Enabled, &f.Metadata, &f.TenantID, &f.CreatedAt, &f.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	return &f, nil
}

// IsEnabled verifica se uma feature flag está habilitada
func (r *Repository) IsEnabled(ctx context.Context, name, tenantID string) (bool, error) {
	flag, err := r.GetByName(ctx, name, tenantID)
	if err != nil {
		return false, err
	}
	if flag == nil {
		return false, nil
	}
	return flag.Enabled, nil
}

// Create cria uma nova feature flag
func (r *Repository) Create(ctx context.Context, input FeatureFlagInput) (*FeatureFlag, error) {
	query := `
		INSERT INTO feature_flags (name, description, enabled, metadata, tenant_id, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`

	now := time.Now()
	result, err := r.db.ExecContext(ctx, query, input.Name, input.Description, input.Enabled, input.Metadata, input.TenantID, now, now)
	if err != nil {
		return nil, err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, err
	}

	return &FeatureFlag{
		ID:          id,
		Name:        input.Name,
		Description: input.Description,
		Enabled:     input.Enabled,
		Metadata:    input.Metadata,
		TenantID:    input.TenantID,
		CreatedAt:   now,
		UpdatedAt:   now,
	}, nil
}

// Update atualiza uma feature flag existente
func (r *Repository) Update(ctx context.Context, id int64, input FeatureFlagInput) (*FeatureFlag, error) {
	query := `
		UPDATE feature_flags
		SET name = ?, description = ?, enabled = ?, metadata = ?, updated_at = ?
		WHERE id = ?
	`

	now := time.Now()
	_, err := r.db.ExecContext(ctx, query, input.Name, input.Description, input.Enabled, input.Metadata, now, id)
	if err != nil {
		return nil, err
	}

	return r.GetByID(ctx, id)
}

// Toggle alterna o estado de uma feature flag
func (r *Repository) Toggle(ctx context.Context, id int64) (*FeatureFlag, error) {
	// Primeiro pega o estado atual
	flag, err := r.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	query := `
		UPDATE feature_flags
		SET enabled = ?, updated_at = ?
		WHERE id = ?
	`

	now := time.Now()
	_, err = r.db.ExecContext(ctx, query, !flag.Enabled, now, id)
	if err != nil {
		return nil, err
	}

	return r.GetByID(ctx, id)
}

// Delete remove uma feature flag
func (r *Repository) Delete(ctx context.Context, id int64) error {
	query := `DELETE FROM feature_flags WHERE id = ?`
	_, err := r.db.ExecContext(ctx, query, id)
	return err
}

// GetByID retorna feature flag por ID
func (r *Repository) GetByID(ctx context.Context, id int64) (*FeatureFlag, error) {
	query := `
		SELECT id, name, description, enabled, metadata, tenant_id, created_at, updated_at
		FROM feature_flags
		WHERE id = ?
	`

	var f FeatureFlag
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&f.ID, &f.Name, &f.Description, &f.Enabled, &f.Metadata, &f.TenantID, &f.CreatedAt, &f.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	return &f, nil
}
