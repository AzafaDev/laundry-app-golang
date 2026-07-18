package order

import "laundry-app-with-golang/internal/payment"

type CreateOrderRequest struct {
	PickupAddressID string `json:"pickup_address_id" binding:"required"`
	PickupDate      string `json:"pickup_date" binding:"required"` // YYYY-MM-DD
}

type OrderResponse struct {
	ID              string  `json:"id"`
	InvoiceNumber   string  `json:"invoice_number"`
	OutletID        string  `json:"outlet_id"`
	OutletName      string  `json:"outlet_name,omitempty"`
	OutletAddress   string  `json:"outlet_address,omitempty"`
	PickupAddressID string  `json:"pickup_address_id"`
	Status          string  `json:"status"`
	PickupDate      string  `json:"pickup_date"`
	DeliveryFee     float64 `json:"delivery_fee"`
	TotalWeightKG   float64 `json:"total_weight_kg,omitempty"`
	TotalPrice      float64 `json:"total_price"`
	CreatedAt       string  `json:"created_at"`
	BypassStatus    string  `json:"bypass_status,omitempty"` // "pending" | "rejected" (kosong = belum pernah/gak relevan)
	Message         string  `json:"message,omitempty"`
	CustomerName    string  `json:"customer_name,omitempty"`
	CustomerPhone   string  `json:"customer_phone,omitempty"`
}

type OrderListResponse struct {
	Data       []OrderResponse `json:"data"`
	TotalCount int64           `json:"total_count"`
}

type OrderItemResponse struct {
	ID            string  `json:"id"`
	LaundryItemID string  `json:"laundry_item_id"`
	Quantity      float64 `json:"quantity"`
	PriceAtOrder  float64 `json:"price_at_order"`
}

type BreakdownResponse struct {
	ID             string `json:"id"`
	ClothingTypeID string `json:"clothing_type_id"`
	Quantity       int32  `json:"quantity"`
}

type StatusHistoryResponse struct {
	ID            string `json:"id"`
	OldStatus     string `json:"old_status,omitempty"`
	NewStatus     string `json:"new_status"`
	ChangedByType string `json:"changed_by_type"`
	ChangedByID   string `json:"changed_by_id"`
	Note          string `json:"note,omitempty"`
	CreatedAt     string `json:"created_at"`
}

type ComplaintSummary struct {
	ID            string `json:"id"`
	ComplaintType string `json:"complaint_type"`
	Description   string `json:"description"`
	Status        string `json:"status"`
	CreatedAt     string `json:"created_at"`
}

type OrderDetailResponse struct {
	OrderResponse
	Items         []OrderItemResponse      `json:"items"`
	Breakdown     []BreakdownResponse      `json:"breakdown"`
	StatusHistory []StatusHistoryResponse  `json:"status_history"`
	Payment       *payment.PaymentResponse `json:"payment,omitempty"`
	Complaints    []ComplaintSummary       `json:"complaints"`
}

type CreateComplaintRequest struct {
	ComplaintType string   `json:"complaint_type" binding:"required"`
	Description   string   `json:"description" binding:"required"`
	PhotoURLs     []string `json:"photo_urls"`
}

const maxComplaintPhotos = 5

const maxBypassPhotos = 5

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

type StationHistoryEntry struct {
	ID            string `json:"id"`
	OrderID       string `json:"order_id"`
	InvoiceNumber string `json:"invoice_number"`
	FromStatus    string `json:"from_status"`
	ToStatus      string `json:"to_status"`
	ProcessedAt   string `json:"processed_at"`
}

type StationHistoryResponse struct {
	Data       []StationHistoryEntry `json:"data"`
	TotalCount int64                 `json:"total_count"`
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
	InvoiceNumber          string           `json:"invoice_number,omitempty"`
	Station                string           `json:"station"`
	RequestedBy            string           `json:"requested_by"`
	RequestedByName        string           `json:"requested_by_name,omitempty"`
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
	AdminNotes string `json:"admin_notes" binding:"required"`
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

type ComplaintListResponse struct {
	Data       []ComplaintResponse `json:"data"`
	TotalCount int64               `json:"total_count"`
}

type ComplaintStatsResponse struct {
	Open       int64 `json:"open"`
	InProgress int64 `json:"in_progress"`
	Resolved   int64 `json:"resolved"`
	Rejected   int64 `json:"rejected"`
}

type UpdateComplaintStatusRequest struct {
	Status                 string `json:"status" binding:"required"`
	ResolutionNotes        string `json:"resolution_notes"`
	ExpectedResolutionDate string `json:"expected_resolution_date"` // "YYYY-MM-DD"
}
