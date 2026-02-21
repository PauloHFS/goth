-- ===========================================
-- 004: Rate Limiting Infrastructure
-- ===========================================

-- Dead Letter Queue for failed jobs
CREATE TABLE IF NOT EXISTS dead_letter_jobs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    original_job_id INTEGER NOT NULL,
    tenant_id TEXT,
    type TEXT NOT NULL,
    payload JSON NOT NULL,
    attempt_count INTEGER NOT NULL,
    last_error TEXT,
    failed_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    archived_at DATETIME
);

CREATE INDEX IF NOT EXISTS idx_dlq_type ON dead_letter_jobs(type);
CREATE INDEX IF NOT EXISTS idx_dlq_failed_at ON dead_letter_jobs(failed_at);
CREATE INDEX IF NOT EXISTS idx_dlq_tenant ON dead_letter_jobs(tenant_id);

-- Email Provider Configuration
CREATE TABLE IF NOT EXISTS email_providers (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tenant_id TEXT NOT NULL REFERENCES tenants(id),
    provider TEXT NOT NULL CHECK(provider IN ('resend', 'aws_ses', 'sendgrid', 'postmark')),
    api_key_encrypted TEXT NOT NULL,
    config JSON DEFAULT '{}',
    is_active BOOLEAN DEFAULT TRUE,
    is_primary BOOLEAN DEFAULT FALSE,
    daily_limit INTEGER DEFAULT 10000,
    monthly_limit INTEGER DEFAULT 100000,
    emails_sent_today INTEGER DEFAULT 0,
    emails_sent_month INTEGER DEFAULT 0,
    last_reset_daily DATETIME DEFAULT CURRENT_TIMESTAMP,
    last_reset_monthly DATETIME DEFAULT CURRENT_TIMESTAMP,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(tenant_id, provider)
);

CREATE INDEX IF NOT EXISTS idx_email_providers_tenant ON email_providers(tenant_id);
CREATE INDEX IF NOT EXISTS idx_email_providers_active ON email_providers(is_active);

-- System Configuration (for rate limits, feature flags, etc.)
CREATE TABLE IF NOT EXISTS system_config (
    key TEXT PRIMARY KEY,
    value JSON NOT NULL,
    description TEXT,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Insert default rate limit config
INSERT OR IGNORE INTO system_config (key, value, description) VALUES
    ('rate_limits', '{"auth": {"rate": 5, "burst": 10}, "api": {"rate": 20, "burst": 40}, "webhook": {"rate": 100, "burst": 200}, "upload": {"rate": 2, "burst": 5}, "default": {"rate": 10, "burst": 20}}', 'HTTP rate limit configuration'),
    ('job_rate_limits', '{"send_email": {"concurrency": 5, "rate": 2, "burst": 5}, "send_verification_email": {"concurrency": 5, "rate": 2, "burst": 5}, "send_password_reset_email": {"concurrency": 5, "rate": 2, "burst": 5}, "process_ai": {"concurrency": 3, "rate": 1, "burst": 3}, "process_webhook": {"concurrency": 10, "rate": 5, "burst": 10}}', 'Worker job rate limits');

-- Admin Users (separate from regular users)
CREATE TABLE IF NOT EXISTS admin_users (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL REFERENCES users(id),
    role TEXT NOT NULL DEFAULT 'admin' CHECK(role IN ('super_admin', 'admin', 'viewer')),
    permissions JSON DEFAULT '[]',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(user_id)
);

CREATE INDEX IF NOT EXISTS idx_admin_users_user ON admin_users(user_id);
