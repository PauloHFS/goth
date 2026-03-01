package contextkeys

type contextKey string

const UserContextKey contextKey = "user"
const TenantContextKey contextKey = "tenant_id"
const LocaleKey contextKey = "locale"
const CSRFTokenKey contextKey = "csrf_token"
const LoggerKey contextKey = "logger"
const RequestIDKey contextKey = "request_id"
const ValidatableKey contextKey = "validatable"
const CSPNonceKey contextKey = "csp_nonce"
