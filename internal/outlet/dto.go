package outlet

type OutletRequest struct {
	Name            string  `json:"name" binding:"required"`
	Address         string  `json:"address" binding:"required"`
	Latitude        float64 `json:"latitude" binding:"required"`
	Longitude       float64 `json:"longitude" binding:"required"`
	IsActive        bool    `json:"is_active"`
	ServiceRadiusKM float64 `json:"service_radius_km"`
}

type OutletResponse struct {
	ID              string  `json:"id"`
	Name            string  `json:"name"`
	Address         string  `json:"address"`
	Latitude        float64 `json:"latitude"`
	Longitude       float64 `json:"longitude"`
	IsActive        bool    `json:"is_active"`
	ServiceRadiusKM float64 `json:"service_radius_km"`
	Message         string  `json:"message,omitempty"`
}

type OutletListResponse struct {
	Data       []OutletResponse `json:"data"`
	TotalCount int64            `json:"total_count"`
}
