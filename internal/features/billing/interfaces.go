package billing

import (
	"context"

	"github.com/PauloHFS/goth/internal/db"
)

type CustomerRepository interface {
	GetByEmail(ctx context.Context, tenantID, email string) (db.AsaasCustomer, error)
	Create(ctx context.Context, params CreateCustomerParams) (db.AsaasCustomer, error)
}

type PaymentRepository interface {
	Create(ctx context.Context, params CreatePaymentParams) (db.AsaasPayment, error)
	GetByID(ctx context.Context, id string) (db.AsaasPayment, error)
	ListByUser(ctx context.Context, userID int64) ([]db.AsaasPayment, error)
	UpdateStatus(ctx context.Context, id, status string) error
}

type SubscriptionRepository interface {
	GetByUser(ctx context.Context, userID int64) (db.AsaasSubscription, error)
}

type AsaasClient interface {
	CreateCustomer(customer *AsaasCustomerInput) (*AsaasCustomerOutput, error)
	CreatePayment(payment *AsaasPaymentInput) (*AsaasPaymentOutput, error)
}

type CreateCustomerParams struct {
	TenantID  string
	UserID    int64
	Email     string
	Name      string
	CpfCnpj   string
	AsaasData string
}

type CreatePaymentParams struct {
	TenantID    string
	CustomerID  string
	UserID      int64
	Amount      float64
	BillingType string
	DueDate     string
	ExternalRef string
}

type AsaasCustomerInput struct {
	Email string `json:"email"`
	Name  string `json:"name"`
}

type AsaasCustomerOutput struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type AsaasPaymentInput struct {
	Customer          string  `json:"customer"`
	BillingType       string  `json:"billingType"`
	Value             float64 `json:"value"`
	DueDate           string  `json:"dueDate"`
	ExternalReference string  `json:"externalReference"`
}

type AsaasPaymentOutput struct {
	ID           string `json:"id"`
	Status       string `json:"status"`
	DueDate      string `json:"dueDate"`
	InvoiceURL   string `json:"invoiceUrl"`
	InvoiceID    string `json:"invoiceId"`
	PixQRCode    string `json:"pixQrCode"`
	PixCopyPaste string `json:"pixCopyPaste"`
}
