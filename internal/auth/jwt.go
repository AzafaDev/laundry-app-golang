package auth

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type CustomerClaims struct {
	CustomerID string `json:"customer_id"`
	jwt.RegisteredClaims
}

func GenerateAccessToken(customerID, secret string) (string, error) {
	claims := CustomerClaims{
		CustomerID: customerID,
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
