package routes

const (
	Home           = "/"
	Login          = "/login"
	Logout         = "/logout"
	Register       = "/register"
	ForgotPassword = "/forgot-password"
	ResetPassword  = "/reset-password"
	VerifyEmail    = "/verify-email"
	Dashboard      = "/dashboard"
	Health         = "/health"
	Metrics        = "/metrics"

	// Payment routes
	CheckoutSubscribe = "/checkout/subscribe"
	CheckoutSuccess   = "/checkout/success"
	AsaasWebhook      = "/webhooks/asaas"

	// OAuth routes
	GoogleLogin    = "/auth/google"
	GoogleCallback = "/auth/google/callback"
)
