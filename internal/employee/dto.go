package employee

type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

type EmployeeResponse struct {
	ID         string  `json:"id"`
	FullName   string  `json:"full_name"`
	Email      string  `json:"email"`
	Phone      *string `json:"phone"`
	Role       string  `json:"role"`
	OutletID   *string `json:"outlet_id"`
	IsActive   bool    `json:"is_active"`
	DeletedAt  *string `json:"deleted_at"`
	InviteSent bool    `json:"invite_sent,omitempty"`
	Message    string  `json:"message,omitempty"`
}

type CreateEmployeeRequest struct {
	FullName string  `json:"full_name" binding:"required"`
	Email    string  `json:"email" binding:"required,email"`
	Phone    string  `json:"phone"`
	Password string  `json:"password"`
	Role     string  `json:"role" binding:"required,oneof=super_admin outlet_admin washing_worker ironing_worker packing_worker driver"`
	OutletID *string `json:"outlet_id,omitempty"`
}

// AssignOutletRequest's OutletID is a pointer so JSON null decodes to a nil
// pointer, which the handler treats as "unassign." encoding/json cannot
// distinguish an omitted field from an explicit null (both decode to nil),
// but this endpoint has no "no change" state to represent — every call is
// either assign (non-null id) or unassign (nil), so that's not a gap here.
type AssignOutletRequest struct {
	OutletID *string `json:"outlet_id"`
}

type EmployeeListResponse struct {
	Data       []EmployeeResponse `json:"data"`
	TotalCount int64              `json:"total_count"`
}

type UpdateEmployeeRequest struct {
	FullName string `json:"full_name" binding:"required"`
	Phone    string `json:"phone"`
	Role     string `json:"role" binding:"required,oneof=super_admin outlet_admin washing_worker ironing_worker packing_worker driver"`
}

type ForgotPasswordRequest struct {
	Email string `json:"email" binding:"required,email"`
}

type ResetPasswordRequest struct {
	Token           string `json:"token" binding:"required"`
	NewPassword     string `json:"new_password" binding:"required,min=8"`
	ConfirmPassword string `json:"confirm_password" binding:"required,eqfield=NewPassword"`
}

type ChangePasswordRequest struct {
	CurrentPassword string `json:"current_password" binding:"required"`
	NewPassword     string `json:"new_password" binding:"required,min=8"`
	ConfirmPassword string `json:"confirm_password" binding:"required,eqfield=NewPassword"`
}
