package payment

type PaymentResponse struct {
	ID                   string  `json:"id"`
	OrderID              string  `json:"order_id"`
	Amount               float64 `json:"amount"`
	PaymentMethod        string  `json:"payment_method"`
	GatewayName          string  `json:"gateway_name,omitempty"`
	GatewayTransactionID string  `json:"gateway_transaction_id,omitempty"`
	PaymentLink          string  `json:"payment_link,omitempty"`
	Status               string  `json:"status"`
	ExpiredAt            string  `json:"expired_at,omitempty"`
	PaidAt               string  `json:"paid_at,omitempty"`
	Message              string  `json:"message,omitempty"`
}

// webhookNotification is the payload Midtrans POSTs to the notification
// webhook. Only the fields needed for signature verification and status
// resolution are parsed — Midtrans sends more fields than this depending on
// payment method, the rest are ignored.
type webhookNotification struct {
	OrderID           string `json:"order_id"`
	StatusCode        string `json:"status_code"`
	GrossAmount       string `json:"gross_amount"`
	SignatureKey      string `json:"signature_key"`
	TransactionStatus string `json:"transaction_status"`
	FraudStatus       string `json:"fraud_status"`
}
