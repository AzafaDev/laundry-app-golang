package auth

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type CustomerClaims struct {
	CustomerID   string `json:"customer_id"`
	TokenVersion int32  `json:"token_version"`
	jwt.RegisteredClaims
}

func GenerateAccessToken(customerID string, tokenVersion int32, secret string) (string, error) {
	claims := CustomerClaims{
		CustomerID:   customerID,
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

func VerifyAccessToken(tokenString, secret string) (*CustomerClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &CustomerClaims{}, func(t *jwt.Token) (any, error) {
		return []byte(secret), nil
	})

	if err != nil {
		return nil, fmt.Errorf("error in verifying access token: %w", err)
	}

	claims, ok := token.Claims.(*CustomerClaims)
	if !ok {
		return nil, fmt.Errorf("invalid token claims")
	}

	return claims, nil
}
