package laundryitem

type LaundryItemRequest struct {
	Name        string  `json:"name" binding:"required,max=100"`
	Description string  `json:"description"`
	Unit        string  `json:"unit"`
	BasePrice   float64 `json:"base_price"`
	IsActive    bool    `json:"is_active"`
}

type LaundryItemResponse struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Description string  `json:"description"`
	Unit        string  `json:"unit"`
	BasePrice   float64 `json:"base_price"`
	IsActive    bool    `json:"is_active"`
	Message     string  `json:"message,omitempty"`
}

type LaundryItemListResponse struct {
	Data       []LaundryItemResponse `json:"data"`
	TotalCount int64                 `json:"total_count"`
}

type PublicLaundryItemResponse struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Description string  `json:"description"`
	Unit        string  `json:"unit"`
	BasePrice   float64 `json:"base_price"`
}
