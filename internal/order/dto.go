package order

type CreateOrderRequest struct {
	PickupAddressID string `json:"pickup_address_id" binding:"required"`
	PickupDate      string `json:"pickup_date" binding:"required"` // YYYY-MM-DD
}

type OrderResponse struct {
	ID              string  `json:"id"`
	InvoiceNumber   string  `json:"invoice_number"`
	OutletID        string  `json:"outlet_id"`
	PickupAddressID string  `json:"pickup_address_id"`
	Status          string  `json:"status"`
	PickupDate      string  `json:"pickup_date"`
	DeliveryFee     float64 `json:"delivery_fee"`
	TotalPrice      float64 `json:"total_price"`
	CreatedAt       string  `json:"created_at"`
	Message         string  `json:"message,omitempty"`
}

type OrderListResponse struct {
	Data       []OrderResponse `json:"data"`
	TotalCount int64           `json:"total_count"`
}

type CreateComplaintRequest struct {
	ComplaintType string   `json:"complaint_type" binding:"required"`
	Description   string   `json:"description" binding:"required"`
	PhotoURLs     []string `json:"photo_urls"`
}

type ComplaintResponse struct {
	ID            string   `json:"id"`
	OrderID       string   `json:"order_id"`
	ComplaintType string   `json:"complaint_type"`
	Description   string   `json:"description"`
	PhotoURLs     []string `json:"photo_urls"`
	Status        string   `json:"status"`
	Message       string   `json:"message,omitempty"`
}
