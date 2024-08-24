package main

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"github.com/golang-jwt/jwt/v5"
	"os"
	"time"
)

var jwtSecret = []byte(os.Getenv("JWT_SECRET"))

type Claims struct {
	Username string `json:"username"`
	jwt.RegisteredClaims
}

func GenerateJWT(username string) (string, string, error) {
	expirationTime := time.Now().Add(15 * time.Minute)
	claims := &Claims{
		Username: username,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expirationTime),
			Issuer:    "my-app",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(jwtSecret)
	if err != nil {
		return "", "", err
	}

	refreshToken, err := generateRefreshToken()
	if err != nil {
		return "", "", err
	}

	err = storeRefreshToken(username, refreshToken)
	if err != nil {
		return "", "", err
	}

	return tokenString, refreshToken, nil
}

func ValidateJWT(tokenString string) (*Claims, error) {
	claims := &Claims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		return jwtSecret, nil
	})

	if err != nil {
		if err == jwt.ErrSignatureInvalid {
			return nil, err
		}
		return nil, err
	}

	if !token.Valid {
		return nil, jwt.ErrSignatureInvalid
	}

	return claims, nil
}

func generateRefreshToken() (string, error) {
	b := make([]byte, 32)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}

func storeRefreshToken(username, refreshToken string) error {
	tx, err := DB.Begin()
	if err != nil {
		return err
	}

	// Revoke existing tokens
	_, err = tx.Exec(`
		UPDATE refresh_tokens SET revoked = TRUE 
		WHERE user_id = (SELECT id FROM users WHERE username = $1)
	`, username)
	if err != nil {
		tx.Rollback()
		return err
	}

	// Insert the new token
	_, err = tx.Exec(`
		INSERT INTO refresh_tokens (user_id, token, expires_at) 
		SELECT id, $1, $2 FROM users WHERE username = $3
	`, refreshToken, time.Now().Add(24*time.Hour), username)
	if err != nil {
		tx.Rollback()
		return err
	}

	return tx.Commit()
}

func validateRefreshToken(ctx context.Context, username, refreshToken string) (bool, error) {
	var token string
	err := DB.QueryRowContext(ctx, `
		SELECT token FROM refresh_tokens 
		WHERE token = $1 AND revoked = FALSE 
		AND expires_at > CURRENT_TIMESTAMP
	`, refreshToken).Scan(&token)
	if err != nil {
		if err == sql.ErrNoRows {
			return false, nil
		}
		return false, err
	}
	return token == refreshToken, nil
}
