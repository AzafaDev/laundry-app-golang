package auth

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type EmployeeClaims struct {
	EmployeeID   string `json:"employee_id"`
	Role         string `json:"role"`
	TokenVersion int32  `json:"token_version"`
	jwt.RegisteredClaims
}

func GenerateEmployeeAccessToken(employeeID, role string, tokenVersion int32, secret string) (string, error) {
	claims := EmployeeClaims{
		EmployeeID:   employeeID,
		Role:         role,
		TokenVersion: tokenVersion,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(15 * time.Minute)),
		},
	}

	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(secret))
	if err != nil {
		return "", fmt.Errorf("error in generating token: %w", err)
	}

	return token, nil
}

func VerifyEmployeeAccessToken(tokenString, secret string) (*EmployeeClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &EmployeeClaims{}, func(t *jwt.Token) (any, error) {
		return []byte(secret), nil
	})

	if err != nil {
		return nil, fmt.Errorf("error in verifying access token: %w", err)
	}

	claims, ok := token.Claims.(*EmployeeClaims)
	if !ok {
		return nil, fmt.Errorf("invalid token claims")
	}

	return claims, nil
}
