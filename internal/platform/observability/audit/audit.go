package audit

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"
)

// ActionType define tipos de ações auditáveis
type ActionType string

const (
	ActionLogin                ActionType = "auth.login"
	ActionLogout               ActionType = "auth.logout"
	ActionRegister             ActionType = "auth.register"
	ActionPasswordReset        ActionType = "auth.password_reset"
	ActionPasswordChange       ActionType = "auth.password_change"
	ActionEmailVerification    ActionType = "auth.email_verification"
	ActionProfileUpdate        ActionType = "user.profile_update"
	ActionAvatarUpload         ActionType = "user.avatar_upload"
	ActionPaymentCreated       ActionType = "billing.payment_created"
	ActionPaymentCompleted     ActionType = "billing.payment_completed"
	ActionPaymentFailed        ActionType = "billing.payment_failed"
	ActionSubscriptionCreated  ActionType = "billing.subscription_created"
	ActionSubscriptionCanceled ActionType = "billing.subscription_canceled"
	ActionAdminAction          ActionType = "admin.action"
)

// Event representa um evento de audit log
type Event struct {
	ID        int64          `json:"id"`
	UserID    sql.NullInt64  `json:"user_id,omitempty"`
	Action    ActionType     `json:"action"`
	Resource  string         `json:"resource,omitempty"`
	Details   map[string]any `json:"details,omitempty"`
	IPAddress string         `json:"ip_address,omitempty"`
	UserAgent string         `json:"user_agent,omitempty"`
	TenantID  string         `json:"tenant_id,omitempty"`
	RequestID string         `json:"request_id,omitempty"`
	CreatedAt time.Time      `json:"created_at"`
}

// Repository define interface para audit log
type Repository interface {
	Log(ctx context.Context, event Event) error
	GetByUserID(ctx context.Context, userID int64, limit int) ([]Event, error)
	GetByAction(ctx context.Context, action ActionType, limit int) ([]Event, error)
}

// repository implementa Repository
type repository struct {
	db *sql.DB
}

// NewRepository cria novo repositório de audit log
func NewRepository(db *sql.DB) Repository {
	return &repository{db: db}
}

// Log registra um evento de audit
func (r *repository) Log(ctx context.Context, event Event) error {
	detailsJSON := "{}"
	if len(event.Details) > 0 {
		data, err := json.Marshal(event.Details)
		if err == nil {
			detailsJSON = string(data)
		}
	}

	query := `
		INSERT INTO audit_log (
			user_id, action, resource, details, 
			ip_address, user_agent, tenant_id, request_id, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	_, err := r.db.ExecContext(ctx, query,
		event.UserID,
		event.Action,
		event.Resource,
		detailsJSON,
		event.IPAddress,
		event.UserAgent,
		event.TenantID,
		event.RequestID,
		event.CreatedAt,
	)

	return err
}

// GetByUserID retorna eventos de um usuário
func (r *repository) GetByUserID(ctx context.Context, userID int64, limit int) ([]Event, error) {
	if limit <= 0 {
		limit = 100
	}

	query := `
		SELECT id, user_id, action, resource, details, ip_address, user_agent, tenant_id, request_id, created_at
		FROM audit_log
		WHERE user_id = ?
		ORDER BY created_at DESC
		LIMIT ?
	`

	rows, err := r.db.QueryContext(ctx, query, userID, limit)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = rows.Close()
	}()

	return scanEvents(rows)
}

// GetByAction retorna eventos por tipo de ação
func (r *repository) GetByAction(ctx context.Context, action ActionType, limit int) ([]Event, error) {
	if limit <= 0 {
		limit = 100
	}

	query := `
		SELECT id, user_id, action, resource, details, ip_address, user_agent, tenant_id, request_id, created_at
		FROM audit_log
		WHERE action = ?
		ORDER BY created_at DESC
		LIMIT ?
	`

	rows, err := r.db.QueryContext(ctx, query, action, limit)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = rows.Close()
	}()

	return scanEvents(rows)
}

func scanEvents(rows *sql.Rows) ([]Event, error) {
	var events []Event

	for rows.Next() {
		var e Event
		var detailsJSON string

		err := rows.Scan(
			&e.ID,
			&e.UserID,
			&e.Action,
			&e.Resource,
			&detailsJSON,
			&e.IPAddress,
			&e.UserAgent,
			&e.TenantID,
			&e.RequestID,
			&e.CreatedAt,
		)
		if err != nil {
			return nil, err
		}

		if detailsJSON != "" && detailsJSON != "{}" {
			_ = json.Unmarshal([]byte(detailsJSON), &e.Details)
		}

		events = append(events, e)
	}

	return events, rows.Err()
}

// Logger helper para logging de audit
type Logger struct {
	repo Repository
}

// NewLogger cria logger de audit
func NewLogger(repo Repository) *Logger {
	return &Logger{repo: repo}
}

// LogAction registra ação de forma simplificada
func (l *Logger) LogAction(ctx context.Context, action ActionType, userID int64, details map[string]any) {
	event := Event{
		UserID:    sql.NullInt64{Int64: userID, Valid: userID > 0},
		Action:    action,
		Details:   details,
		CreatedAt: time.Now(),
	}
	_ = l.repo.Log(ctx, event)
}

// LogActionWithRequest registra ação com dados de request HTTP
func (l *Logger) LogActionWithRequest(ctx context.Context, action ActionType, userID int64,
	ipAddress, userAgent, tenantID, requestID string, details map[string]any) {

	event := Event{
		UserID:    sql.NullInt64{Int64: userID, Valid: userID > 0},
		Action:    action,
		Details:   details,
		IPAddress: ipAddress,
		UserAgent: userAgent,
		TenantID:  tenantID,
		RequestID: requestID,
		CreatedAt: time.Now(),
	}
	_ = l.repo.Log(ctx, event)
}

// AuthAuditLogger implementa a interface AuditLogger para auth
type AuthAuditLogger struct {
	logger *Logger
}

// NewAuthAuditLogger cria logger para ações de auth
func NewAuthAuditLogger(repo Repository) *AuthAuditLogger {
	return &AuthAuditLogger{
		logger: NewLogger(repo),
	}
}

func (a *AuthAuditLogger) LogLogin(ctx context.Context, userID int64, success bool, ipAddress, userAgent string) {
	details := map[string]any{
		"success": success,
	}
	action := ActionLogin
	a.logger.LogActionWithRequest(ctx, action, userID, ipAddress, userAgent, "default", "", details)
}

func (a *AuthAuditLogger) LogLogout(ctx context.Context, userID int64, ipAddress, userAgent string) {
	details := map[string]any{}
	a.logger.LogActionWithRequest(ctx, ActionLogout, userID, ipAddress, userAgent, "default", "", details)
}

func (a *AuthAuditLogger) LogRegister(ctx context.Context, email string, ipAddress, userAgent string) {
	details := map[string]any{
		"email": email,
	}
	a.logger.LogActionWithRequest(ctx, ActionRegister, 0, ipAddress, userAgent, "default", "", details)
}

func (a *AuthAuditLogger) LogPasswordReset(ctx context.Context, email string, ipAddress, userAgent string) {
	details := map[string]any{
		"email": email,
	}
	a.logger.LogActionWithRequest(ctx, ActionPasswordReset, 0, ipAddress, userAgent, "default", "", details)
}

func (a *AuthAuditLogger) LogPasswordChange(ctx context.Context, userID int64, ipAddress, userAgent string) {
	details := map[string]any{}
	a.logger.LogActionWithRequest(ctx, ActionPasswordChange, userID, ipAddress, userAgent, "default", "", details)
}
