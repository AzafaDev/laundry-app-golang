package customer

type RegisterRequest struct {
	FullName        string `json:"full_name" binding:"required"`
	Email           string `json:"email" binding:"required,email"`
	Phone           string `json:"phone" binding:"required"`
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

type AddressRequest struct {
	Label      string  `json:"label" binding:"required"`
	Address    string  `json:"address" binding:"required"`
	ProvinceID int32   `json:"province_id" binding:"required"`
	CityID     int32   `json:"city_id" binding:"required"`
	DistrictID int32   `json:"district_id" binding:"required"`
	PostalCode string  `json:"postal_code"`
	Latitude   float64 `json:"latitude" binding:"required"`
	Longitude  float64 `json:"longitude" binding:"required"`
	IsPrimary  bool    `json:"is_primary"`
}

type AddressResponse struct {
	ID         string  `json:"id"`
	Label      string  `json:"label"`
	Address    string  `json:"address"`
	ProvinceID int32   `json:"province_id"`
	CityID     int32   `json:"city_id"`
	DistrictID int32   `json:"district_id"`
	Province   string  `json:"province"`
	City       string  `json:"city"`
	District   string  `json:"district"`
	PostalCode string  `json:"postal_code,omitempty"`
	Latitude   float64 `json:"latitude"`
	Longitude  float64 `json:"longitude"`
	IsPrimary  bool    `json:"is_primary"`
	Message    string  `json:"message,omitempty"`
}
