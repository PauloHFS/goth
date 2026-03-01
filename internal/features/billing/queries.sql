-- Billing Feature Queries (Asaas)

-- name: CreateAsaasCustomer :one
INSERT INTO asaas_customers (id, tenant_id, user_id, email, cpf_cnpj, name, asaas_data)
VALUES (?, ?, ?, ?, ?, ?, ?) RETURNING *;

-- name: GetAsaasCustomerByEmail :one
SELECT * FROM asaas_customers WHERE tenant_id = ? AND email = ? LIMIT 1;

-- name: GetAsaasCustomerByID :one
SELECT * FROM asaas_customers WHERE id = ? LIMIT 1;

-- name: CreateAsaasPayment :one
INSERT INTO asaas_payments (id, tenant_id, customer_id, user_id, amount, billing_type, status, due_date, invoice_url, invoice_id, pix_qr_code, pix_copy_paste, asaas_response)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?) RETURNING *;

-- name: GetAsaasPaymentByID :one
SELECT * FROM asaas_payments WHERE id = ? LIMIT 1;

-- name: UpdateAsaasPaymentStatus :one
UPDATE asaas_payments SET status = ?, payment_date = ?, updated_at = CURRENT_TIMESTAMP
WHERE id = ? RETURNING *;

-- name: ListAsaasPaymentsByCustomer :many
SELECT * FROM asaas_payments WHERE customer_id = ? ORDER BY created_at DESC;

-- name: ListAsaasPaymentsByUser :many
SELECT * FROM asaas_payments WHERE user_id = ? ORDER BY created_at DESC;

-- Subscriptions

-- name: CreateAsaasSubscription :one
INSERT INTO asaas_subscriptions (id, tenant_id, customer_id, user_id, plan_value, billing_type, cycle, status, next_billing_date, asaas_response)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?) RETURNING *;

-- name: GetAsaasSubscriptionByID :one
SELECT * FROM asaas_subscriptions WHERE id = ? LIMIT 1;

-- name: GetActiveSubscriptionByUser :one
SELECT * FROM asaas_subscriptions WHERE user_id = ? AND status = 'ACTIVE' LIMIT 1;

-- name: ListAsaasSubscriptionsByCustomer :many
SELECT * FROM asaas_subscriptions WHERE customer_id = ? ORDER BY created_at DESC;

-- name: ListAsaasSubscriptionsByUser :many
SELECT * FROM asaas_subscriptions WHERE user_id = ? ORDER BY created_at DESC;
