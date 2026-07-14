package employee

type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

type EmployeeResponse struct {
	ID         string `json:"id"`
	FullName   string `json:"full_name"`
	Email      string `json:"email"`
	Role       string `json:"role"`
	InviteSent bool   `json:"invite_sent,omitempty"`
	Message    string `json:"message,omitempty"`
}

type CreateEmployeeRequest struct {
	FullName string `json:"full_name" binding:"required"`
	Email    string `json:"email" binding:"required,email"`
	Phone    string `json:"phone"`
	Password string `json:"password"`
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
