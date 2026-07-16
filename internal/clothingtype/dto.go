package clothingtype

type ClothingTypeRequest struct {
	Name     string `json:"name" binding:"required,max=100"`
	IsActive bool   `json:"is_active"`
}

type ClothingTypeResponse struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	IsActive bool   `json:"is_active"`
	Message  string `json:"message,omitempty"`
}

type ClothingTypeListResponse struct {
	Data       []ClothingTypeResponse `json:"data"`
	TotalCount int64                  `json:"total_count"`
}
