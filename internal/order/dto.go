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

type ProcessOrderItemRequest struct {
	LaundryItemID string  `json:"laundry_item_id" binding:"required"`
	Quantity      float64 `json:"quantity" binding:"required"`
}

type ProcessOrderBreakdownRequest struct {
	ClothingTypeID string `json:"clothing_type_id" binding:"required"`
	Quantity       int32  `json:"quantity"`
}

type ProcessOrderRequest struct {
	TotalWeightKG float64                        `json:"total_weight_kg"`
	Items         []ProcessOrderItemRequest      `json:"items" binding:"required,min=1"`
	Breakdown     []ProcessOrderBreakdownRequest `json:"breakdown"`
}

type NormalizedItem struct {
	ItemType string `json:"item_type"` // "clothing_type" or "laundry_item"
	ItemID   string `json:"item_id"`
	Name     string `json:"name"`
	Quantity int32  `json:"quantity"`
}

type Discrepancy struct {
	ItemType string `json:"item_type"`
	ItemID   string `json:"item_id"`
	Name     string `json:"name"`
	Expected int32  `json:"expected"`
	Actual   int32  `json:"actual"`
}

type SubmitItemsRequest struct {
	ActualItems       []ActualBreakdownItem `json:"actual_items"`
	ActualSatuanItems []ActualSatuanItem    `json:"actual_satuan_items"`
}

type ActualBreakdownItem struct {
	ClothingTypeID string `json:"clothing_type_id" binding:"required"`
	ActualQuantity int32  `json:"actual_quantity"`
}

type ActualSatuanItem struct {
	LaundryItemID  string `json:"laundry_item_id" binding:"required"`
	ActualQuantity int32  `json:"actual_quantity"`
}

type SubmitItemsResponse struct {
	Success        bool           `json:"success"`
	RequiresBypass bool           `json:"requires_bypass,omitempty"`
	Discrepancies  []Discrepancy  `json:"discrepancies,omitempty"`
	Data           *OrderResponse `json:"data,omitempty"`
}

type CreateBypassRequest struct {
	OrderID                string                `json:"order_id" binding:"required"`
	DiscrepancyDescription string                `json:"discrepancy_description" binding:"required"`
	ActualItems            []ActualBreakdownItem `json:"actual_items"`
	ActualSatuanItems      []ActualSatuanItem    `json:"actual_satuan_items"`
	PhotoEvidence          []string              `json:"photo_evidence"`
}

type BypassResponse struct {
	ID                     string           `json:"id"`
	OrderID                string           `json:"order_id"`
	Station                string           `json:"station"`
	RequestedBy            string           `json:"requested_by"`
	ExpectedItems          []NormalizedItem `json:"expected_items"`
	ActualItems            []NormalizedItem `json:"actual_items"`
	DiscrepancyDescription string           `json:"discrepancy_description"`
	PhotoEvidence          []string         `json:"photo_evidence"`
	AttemptNumber          int32            `json:"attempt_number"`
	Status                 string           `json:"status"`
	ReviewedBy             string           `json:"reviewed_by,omitempty"`
	AdminNotes             string           `json:"admin_notes,omitempty"`
	Message                string           `json:"message,omitempty"`
}

type BypassListResponse struct {
	Data       []BypassResponse `json:"data"`
	TotalCount int64            `json:"total_count"`
}

type ReviewBypassRequestBody struct {
	Approve    bool   `json:"approve"`
	AdminNotes string `json:"admin_notes"`
}

type DriverTaskResponse struct {
	ID            string  `json:"id"`
	OrderID       string  `json:"order_id"`
	DriverID      string  `json:"driver_id,omitempty"`
	TaskType      string  `json:"task_type"`
	Status        string  `json:"status"`
	InvoiceNumber string  `json:"invoice_number,omitempty"`
	DistanceKM    float64 `json:"distance_km,omitempty"`
	TakenAt       string  `json:"taken_at,omitempty"`
	CompletedAt   string  `json:"completed_at,omitempty"`
	Message       string  `json:"message,omitempty"`
}

type DriverTaskListResponse struct {
	Data       []DriverTaskResponse `json:"data"`
	TotalCount int64                `json:"total_count"`
}
