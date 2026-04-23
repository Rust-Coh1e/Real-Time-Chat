package jwt

import (
	"fmt"
	"real-time-chat/internal/model"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/google/uuid"
)

func GenerateToken(userID uuid.UUID, username string, secret string) (string, error) {
	claims := model.Claims{
		UserID:   userID.String(),
		Username: username,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(1 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	secretB := []byte(secret)

	tokenString, err := token.SignedString(secretB)

	if err != nil {
		return "", err
	}

	return tokenString, nil
}

func ParseToken(tokenString string, secret string) (*model.Claims, error) {
	secretB := []byte(secret)

	token, err := jwt.ParseWithClaims(tokenString, &model.Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("Unexpected sighing method")
		}

		return secretB, nil
	})

	if err != nil {
		return nil, err
	}

	claims, ok := token.Claims.(*model.Claims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token")
	}

	return claims, nil

}
