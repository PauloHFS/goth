-- +goose Up
-- Feature Flags System
-- Allows dynamic enabling/disabling of features without deployment

CREATE TABLE IF NOT EXISTS feature_flags (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL UNIQUE,
    description TEXT,
    enabled BOOLEAN NOT NULL DEFAULT FALSE,
    metadata TEXT,  -- JSON metadata for additional configuration
    tenant_id TEXT NOT NULL DEFAULT 'global',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Index for fast lookups
CREATE INDEX IF NOT EXISTS idx_feature_flags_name_tenant ON feature_flags(name, tenant_id);
CREATE INDEX IF NOT EXISTS idx_feature_flags_enabled ON feature_flags(enabled);

-- Insert default feature flags
INSERT OR IGNORE INTO feature_flags (name, description, enabled, tenant_id) VALUES
    ('dark_mode', 'Enable dark mode UI theme', false, 'global'),
    ('new_dashboard', 'Enable new dashboard design', false, 'global'),
    ('beta_features', 'Enable beta features for testing', false, 'global'),
    ('maintenance_mode', 'Enable maintenance mode', false, 'global'),
    ('rate_limit_strict', 'Enable strict rate limiting', false, 'global'),
    ('oauth_google', 'Enable Google OAuth login', true, 'global'),
    ('email_verification', 'Require email verification', true, 'global'),
    ('billing_enabled', 'Enable billing features', true, 'global');

-- +goose Down
-- Drop feature flags table
DROP TABLE IF EXISTS feature_flags;
