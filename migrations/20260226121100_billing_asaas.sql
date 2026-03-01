-- +goose Up
PRAGMA foreign_keys = ON;

CREATE TABLE IF NOT EXISTS asaas_customers (
    id TEXT PRIMARY KEY,
    tenant_id TEXT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    user_id INTEGER REFERENCES users(id) ON DELETE SET NULL,
    email TEXT NOT NULL,
    cpf_cnpj TEXT,
    name TEXT,
    asaas_data JSON,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS asaas_payments (
    id TEXT PRIMARY KEY,
    tenant_id TEXT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    customer_id TEXT NOT NULL REFERENCES asaas_customers(id) ON DELETE CASCADE,
    user_id INTEGER REFERENCES users(id) ON DELETE SET NULL,
    amount REAL NOT NULL CHECK(amount > 0),
    billing_type TEXT NOT NULL CHECK(billing_type IN ('CREDIT_CARD', 'BOLETO', 'PIX', 'UNDEFINED')),
    status TEXT NOT NULL DEFAULT 'PENDING' CHECK(status IN ('PENDING', 'RECEIVED', 'CONFIRMED', 'OVERDUE', 'REFUNDED', 'CANCELLED')),
    due_date DATE NOT NULL,
    payment_date DATE,
    invoice_url TEXT,
    invoice_id TEXT,
    pix_qr_code TEXT,
    pix_copy_paste TEXT,
    asaas_response JSON,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS asaas_subscriptions (
    id TEXT PRIMARY KEY,
    tenant_id TEXT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    customer_id TEXT NOT NULL REFERENCES asaas_customers(id) ON DELETE CASCADE,
    user_id INTEGER REFERENCES users(id) ON DELETE SET NULL,
    plan_value REAL NOT NULL CHECK(plan_value > 0),
    billing_type TEXT NOT NULL CHECK(billing_type IN ('CREDIT_CARD', 'BOLETO', 'PIX', 'UNDEFINED')),
    cycle_type TEXT NOT NULL CHECK(cycle_type IN ('WEEKLY', 'MONTHLY', 'BIMONTHLY', 'QUARTERLY', 'SEMIANNUALLY', 'YEARLY')),
    status TEXT NOT NULL DEFAULT 'ACTIVE' CHECK(status IN ('ACTIVE', 'INACTIVE', 'CANCELLED', 'PENDING')),
    next_billing_date DATE,
    asaas_response JSON,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Indexes
CREATE INDEX IF NOT EXISTS idx_asaas_customers_tenant ON asaas_customers(tenant_id);
CREATE INDEX IF NOT EXISTS idx_asaas_customers_email ON asaas_customers(email);
CREATE UNIQUE INDEX IF NOT EXISTS udx_asaas_customers_tenant_email ON asaas_customers(tenant_id, email);
CREATE INDEX IF NOT EXISTS idx_asaas_payments_status ON asaas_payments(status);
CREATE INDEX IF NOT EXISTS idx_asaas_payments_tenant ON asaas_payments(tenant_id);
CREATE INDEX IF NOT EXISTS idx_asaas_payments_customer ON asaas_payments(customer_id);
CREATE INDEX IF NOT EXISTS idx_asaas_payments_due_date ON asaas_payments(due_date);
CREATE INDEX IF NOT EXISTS idx_asaas_subscriptions_status ON asaas_subscriptions(status);
CREATE INDEX IF NOT EXISTS idx_asaas_subscriptions_tenant ON asaas_subscriptions(tenant_id);
CREATE INDEX IF NOT EXISTS idx_asaas_subscriptions_customer ON asaas_subscriptions(customer_id);

-- +goose Down
DROP INDEX IF EXISTS idx_asaas_subscriptions_customer;
DROP INDEX IF EXISTS idx_asaas_subscriptions_tenant;
DROP INDEX IF EXISTS idx_asaas_subscriptions_status;
DROP INDEX IF EXISTS idx_asaas_payments_due_date;
DROP INDEX IF EXISTS idx_asaas_payments_customer;
DROP INDEX IF EXISTS idx_asaas_payments_tenant;
DROP INDEX IF EXISTS idx_asaas_payments_status;
DROP INDEX IF EXISTS udx_asaas_customers_tenant_email;
DROP INDEX IF EXISTS idx_asaas_customers_email;
DROP INDEX IF EXISTS idx_asaas_customers_tenant;
DROP TABLE IF EXISTS asaas_subscriptions;
DROP TABLE IF EXISTS asaas_payments;
DROP TABLE IF EXISTS asaas_customers;
