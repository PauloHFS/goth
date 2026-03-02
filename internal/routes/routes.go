package routes

const (
	// Public pages
	Home    = "/"
	Pricing = "/pricing"
	About   = "/about"
	Contact = "/contact"
	Terms   = "/terms"
	Privacy = "/privacy"

	// Auth routes
	Login          = "/login"
	Logout         = "/logout"
	Register       = "/register"
	ForgotPassword = "/forgot-password"
	ResetPassword  = "/reset-password"
	VerifyEmail    = "/verify-email"

	// App routes (auth required)
	Dashboard     = "/dashboard"
	Profile       = "/profile"
	Settings      = "/settings"
	Notifications = "/notifications"

	// Admin routes
	Admin      = "/admin"
	AdminUsers = "/admin/users"

	// System routes
	Health   = "/health"
	Metrics  = "/metrics"
	Error404 = "/404"
	Error500 = "/500"

	// Payment routes
	CheckoutSubscribe = "/checkout/subscribe"
	CheckoutSuccess   = "/checkout/success"
	AsaasWebhook      = "/webhooks/asaas"

	// OAuth routes
	GoogleLogin    = "/auth/google"
	GoogleCallback = "/auth/google/callback"
)
