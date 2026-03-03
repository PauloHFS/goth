package billing

import (
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/PauloHFS/goth/internal/db"
	httpErr "github.com/PauloHFS/goth/internal/platform/http"
	"github.com/PauloHFS/goth/internal/platform/httpclient"
)

type Handler struct {
	service *Service
	db      *sql.DB
	queries *db.Queries
}

func NewHandler(service *Service, dbConn *sql.DB, queries *db.Queries) *Handler {
	return &Handler{
		service: service,
		db:      dbConn,
		queries: queries,
	}
}

// Subscribe cria nova assinatura/pagamento
// @Summary Subscribe to plan
// @Description Creates a new payment/subscription
// @Tags Billing
// @Accept x-www-form-urlencoded
// @Produce json
// @Param plan_value formData string true "Plan value in cents"
// @Param billing_type formData string true "Billing type (PIX, BOLETO, CREDIT_CARD)"
// @Success 200 {object} PaymentResult
// @Failure 401 {object} map[string]string
// @Failure 400 {object} map[string]string
// @Router /checkout/subscribe [post]
func (h *Handler) Subscribe(w http.ResponseWriter, r *http.Request) error {
	// Timeout de 10 segundos para operações de database e API externa
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	userID := r.Context().Value("user_id")
	if userID == nil {
		httpErr.HandleError(w, r, httpErr.NewUnauthorizedError(""), "subscribe")
		return nil
	}

	// Limit request body size to prevent memory exhaustion (max 1MB for form data)
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20) // 1 MB

	planValue := r.FormValue("plan_value")
	billingType := r.FormValue("billing_type")

	if planValue == "" || billingType == "" {
		httpErr.HandleError(w, r, httpErr.NewValidationError("plan_value and billing_type are required", nil), "subscribe")
		return nil
	}

	amount, err := strconv.ParseFloat(planValue, 64)
	if err != nil {
		httpErr.HandleError(w, r, httpErr.NewValidationError("invalid plan_value", nil), "subscribe")
		return nil
	}

	user, err := h.queries.GetUserByID(ctx, userID.(int64))
	if err != nil {
		httpErr.HandleError(w, r, httpErr.NewNotFoundError("user"), "get_user")
		return nil
	}

	payment, err := h.service.CreatePayment(ctx, CreatePaymentInput{
		TenantID:    user.TenantID,
		UserID:      user.ID,
		Email:       user.Email,
		Amount:      amount,
		BillingType: billingType,
	})
	if err != nil {
		httpErr.HandleError(w, r, err, "create_payment")
		return nil
	}

	w.Header().Set("Content-Type", "application/json")
	return json.NewEncoder(w).Encode(payment)
}

// Webhook processa webhooks do Asaas
// @Summary Process Asaas webhook
// @Description Processes payment webhooks from Asaas
// @Tags Billing
// @Accept json
// @Produce json
// @Param asaas-hmac-sha256 header string false "Webhook signature"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]string
// @Router /webhook/asaas [post]
func (h *Handler) Webhook(w http.ResponseWriter, r *http.Request) error {
	// Timeout de 10 segundos para operações de database
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	payload, err := io.ReadAll(r.Body)
	if err != nil {
		httpErr.HandleError(w, r, err, "read_webhook_body")
		return nil
	}

	event, err := h.service.HandleWebhook(ctx, r.Header.Get("asaas-hmac-sha256"), payload)
	if err != nil {
		httpErr.HandleError(w, r, httpErr.NewValidationError("invalid webhook", nil), "process_webhook")
		return nil
	}

	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(event); err != nil {
		// Log error but don't fail - response already sent
		// This is acceptable since the webhook was already processed
		_ = err // explicitly ignore to avoid staticcheck
	}
	return nil
}

type CreatePaymentInput struct {
	TenantID    string
	UserID      int64
	Email       string
	Amount      float64
	BillingType string
}

type PaymentResult struct {
	InvoiceURL   string `json:"invoiceUrl"`
	PaymentID    string `json:"paymentId"`
	PixQRCode    string `json:"pixQrCode"`
	PixCopyPaste string `json:"pixCopyPaste"`
}

type Service struct {
	deps ServiceDeps
}

type ServiceDeps struct {
	CustomerRepo CustomerRepository
	PaymentRepo  PaymentRepository
	AsaasClient  AsaasClient
}

func NewService(deps ServiceDeps) *Service {
	return &Service{deps: deps}
}

func (s *Service) CreatePayment(ctx context.Context, input CreatePaymentInput) (PaymentResult, error) {
	customer, err := s.deps.CustomerRepo.GetByEmail(ctx, input.TenantID, input.Email)
	if err != nil {
		newCustomer, err := s.deps.AsaasClient.CreateCustomer(&AsaasCustomerInput{
			Email: input.Email,
			Name:  input.Email,
		})
		if err != nil {
			return PaymentResult{}, err
		}

		customer, err = s.deps.CustomerRepo.Create(ctx, CreateCustomerParams{
			TenantID:  input.TenantID,
			UserID:    input.UserID,
			Email:     input.Email,
			Name:      newCustomer.Name,
			AsaasData: "",
		})
		if err != nil {
			return PaymentResult{}, err
		}
	}

	asaasPayment, err := s.deps.AsaasClient.CreatePayment(&AsaasPaymentInput{
		Customer:          customer.ID,
		BillingType:       input.BillingType,
		Value:             input.Amount,
		DueDate:           time.Now().Add(24 * time.Hour).Format("2006-01-02"),
		ExternalReference: "user_" + strconv.FormatInt(input.UserID, 10),
	})
	if err != nil {
		return PaymentResult{}, err
	}

	payment, err := s.deps.PaymentRepo.Create(ctx, CreatePaymentParams{
		TenantID:    input.TenantID,
		CustomerID:  customer.ID,
		UserID:      input.UserID,
		Amount:      input.Amount,
		BillingType: input.BillingType,
		DueDate:     asaasPayment.DueDate,
		ExternalRef: asaasPayment.ID,
	})
	if err != nil {
		return PaymentResult{}, err
	}

	return PaymentResult{
		InvoiceURL:   payment.InvoiceUrl.String,
		PaymentID:    payment.ID,
		PixQRCode:    payment.PixQrCode.String,
		PixCopyPaste: payment.PixCopyPaste.String,
	}, nil
}

func (s *Service) HandleWebhook(ctx context.Context, signature string, payload []byte) (map[string]interface{}, error) {
	var event map[string]interface{}
	if err := json.Unmarshal(payload, &event); err != nil {
		return nil, err
	}

	eventType, ok := event["event"].(string)
	if !ok {
		return nil, nil
	}

	switch eventType {
	case "PAYMENT_RECEIVED":
		payment := event["payment"].(map[string]interface{})
		externalID := payment["id"].(string)
		_ = s.deps.PaymentRepo.UpdateStatus(ctx, externalID, "RECEIVED")
	}

	return event, nil
}

type customerRepository struct {
	db *sql.DB
	q  *db.Queries
}

func NewCustomerRepository(dbConn *sql.DB) CustomerRepository {
	return &customerRepository{db: dbConn, q: db.New(dbConn)}
}

func (r *customerRepository) GetByEmail(ctx context.Context, tenantID, email string) (db.AsaasCustomer, error) {
	return r.q.GetAsaasCustomerByEmail(ctx, db.GetAsaasCustomerByEmailParams{
		TenantID: tenantID,
		Email:    email,
	})
}

func (r *customerRepository) Create(ctx context.Context, params CreateCustomerParams) (db.AsaasCustomer, error) {
	data, _ := json.Marshal(params.AsaasData)
	return r.q.CreateAsaasCustomer(ctx, db.CreateAsaasCustomerParams{
		ID:        params.TenantID + "_" + strconv.FormatInt(params.UserID, 10),
		TenantID:  params.TenantID,
		UserID:    sql.NullInt64{Int64: params.UserID, Valid: true},
		Email:     params.Email,
		Name:      sql.NullString{String: params.Name, Valid: params.Name != ""},
		AsaasData: data,
	})
}

type paymentRepository struct {
	db *sql.DB
	q  *db.Queries
}

func NewPaymentRepository(dbConn *sql.DB) PaymentRepository {
	return &paymentRepository{db: dbConn, q: db.New(dbConn)}
}

func (r *paymentRepository) Create(ctx context.Context, params CreatePaymentParams) (db.AsaasPayment, error) {
	dueDate, _ := time.Parse("2006-01-02", params.DueDate)
	resp, _ := json.Marshal(params)
	return r.q.CreateAsaasPayment(ctx, db.CreateAsaasPaymentParams{
		ID:            params.ExternalRef,
		TenantID:      params.TenantID,
		CustomerID:    params.CustomerID,
		UserID:        sql.NullInt64{Int64: params.UserID, Valid: true},
		Amount:        params.Amount,
		BillingType:   params.BillingType,
		Status:        "PENDING",
		DueDate:       dueDate,
		AsaasResponse: resp,
	})
}

func (r *paymentRepository) GetByID(ctx context.Context, id string) (db.AsaasPayment, error) {
	return r.q.GetAsaasPaymentByID(ctx, id)
}

func (r *paymentRepository) ListByUser(ctx context.Context, userID int64) ([]db.AsaasPayment, error) {
	return r.q.ListAsaasPaymentsByUser(ctx, sql.NullInt64{Int64: userID, Valid: true})
}

func (r *paymentRepository) UpdateStatus(ctx context.Context, id, status string) error {
	_, err := r.q.UpdateAsaasPaymentStatus(ctx, db.UpdateAsaasPaymentStatusParams{
		Status: status,
		ID:     id,
	})
	return err
}

type AsaasClientImpl struct {
	apiKey      string
	baseURL     string
	environment string
	client      *httpclient.Client
}

func NewAsaasClientImpl(apiKey, environment string) AsaasClient {
	baseURL := "https://www.asaas.com/api/v3"
	if environment == "sandbox" {
		baseURL = "https://sandbox.asaas.com/api/v3"
	}

	// Configurar client com circuit breaker e retry
	clientConfig := httpclient.DefaultClientConfig("asaas")
	clientConfig.Timeout = 30 * time.Second
	clientConfig.CircuitBreaker.MaxFailures = 5
	clientConfig.CircuitBreaker.Timeout = 30 * time.Second
	clientConfig.Retry.MaxRetries = 3
	clientConfig.Retry.InitialBackoff = 100 * time.Millisecond

	return &AsaasClientImpl{
		apiKey:      apiKey,
		baseURL:     baseURL,
		environment: environment,
		client:      httpclient.NewClient(clientConfig),
	}
}

func (c *AsaasClientImpl) CreateCustomer(customer *AsaasCustomerInput) (*AsaasCustomerOutput, error) {
	// TODO: Implementar chamada HTTP real com circuit breaker
	// Por enquanto, manter mock para desenvolvimento
	return &AsaasCustomerOutput{
		ID:   "cus_" + customer.Email,
		Name: customer.Name,
	}, nil
}

func (c *AsaasClientImpl) CreatePayment(payment *AsaasPaymentInput) (*AsaasPaymentOutput, error) {
	// TODO: Implementar chamada HTTP real com circuit breaker
	// Por enquanto, manter mock para desenvolvimento
	return &AsaasPaymentOutput{
		ID:           "pay_" + payment.ExternalReference,
		Status:       "PENDING",
		DueDate:      payment.DueDate,
		InvoiceURL:   "https://asaas.com/i/" + payment.ExternalReference,
		InvoiceID:    "inv_" + payment.ExternalReference,
		PixQRCode:    "00000000000000000000000000000000000000000000000",
		PixCopyPaste: "00000000000000000000000000000000000000000000000",
	}, nil
}
