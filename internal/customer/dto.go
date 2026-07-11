package customer

type RegisterRequest struct {
	FullName        string `json:"full_name" binding:"required"`
	Email           string `json:"email" binding:"required,email"`
	Password        string `json:"password" binding:"required,min=8"`
	ConfirmPassword string `json:"confirm_password" binding:"required,eqfield=Password"`
}

type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

type CustomerResponse struct {
	ID        string `json:"id"`
	FullName  string `json:"full_name"`
	Email     string `json:"email"`
	Phone     string `json:"phone,omitempty"`
	AvatarURL string `json:"avatar_url,omitempty"`
	Message   string `json:"message"`
}

type VerifyRequest struct {
	Token string `json:"token" binding:"required"`
}

type ResendVerificationRequest struct {
	Email string `json:"email" binding:"required,email"`
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

type UpdateProfileRequest struct {
	FullName string `json:"full_name" binding:"required"`
	Phone    string `json:"phone" binding:"required"`
}

type RequestEmailChangeRequest struct {
	NewEmail        string `json:"new_email" binding:"required,email"`
	CurrentPassword string `json:"current_password" binding:"required"`
}

type VerifyEmailChangeRequest struct {
	Token string `json:"token" binding:"required"`
}
