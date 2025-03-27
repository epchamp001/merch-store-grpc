package jwt

import (
	"fmt"
	"github.com/golang-jwt/jwt/v5"
	"time"
)

type TokenService interface {
	GenerateToken(userID int) (string, error)
	ParseJWTToken(tokenString string) (int, error)
}

type TokenServiceImpl struct {
	secretKey           string
	tokenExpirationTime int
}

func NewTokenService(secretKey string, tokenExpTime int) TokenService {
	return &TokenServiceImpl{secretKey: secretKey, tokenExpirationTime: tokenExpTime}
}

func (t *TokenServiceImpl) GenerateToken(userID int) (string, error) {
	now := time.Now()

	expiration := now.Add(time.Duration(t.tokenExpirationTime) * time.Second)

	claims := jwt.MapClaims{
		"user_id": userID,
		"exp":     expiration.Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	return token.SignedString([]byte(t.secretKey))
}

func (t *TokenServiceImpl) ParseJWTToken(tokenString string) (int, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		return []byte(t.secretKey), nil
	})

	if err != nil {
		return 0, err
	}

	if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
		userID := int(claims["user_id"].(float64))
		return userID, nil
	}

	return 0, fmt.Errorf("invalid token")
}
