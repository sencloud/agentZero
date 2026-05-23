package auth

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/MicahParks/keyfunc/v3"
	"github.com/golang-jwt/jwt/v5"
)

const appleIssuer = "https://appleid.apple.com"
const appleJWKSURL = "https://appleid.apple.com/auth/keys"

type AppleClaims struct {
	Email          string `json:"email"`
	EmailVerified  any    `json:"email_verified"`
	IsPrivateEmail any    `json:"is_private_email"`
	jwt.RegisteredClaims
}

type AppleVerifier struct {
	bundleID string
	jwks     keyfunc.Keyfunc
}

func NewAppleVerifier(bundleID string) *AppleVerifier {
	jwks, err := keyfunc.NewDefaultCtx(context.Background(), []string{appleJWKSURL})
	if err != nil {
		jwks = nil
	}
	return &AppleVerifier{bundleID: bundleID, jwks: jwks}
}

func (v *AppleVerifier) Verify(idToken string) (*AppleClaims, error) {
	if v.jwks == nil {
		jwks, err := keyfunc.NewDefaultCtx(context.Background(), []string{appleJWKSURL})
		if err != nil {
			return nil, fmt.Errorf("fetch apple jwks: %w", err)
		}
		v.jwks = jwks
	}
	claims := &AppleClaims{}
	token, err := jwt.ParseWithClaims(idToken, claims, v.jwks.Keyfunc,
		jwt.WithValidMethods([]string{"RS256"}),
		jwt.WithIssuer(appleIssuer),
		jwt.WithAudience(v.bundleID),
		jwt.WithExpirationRequired(),
		jwt.WithLeeway(30*time.Second),
	)
	if err != nil {
		return nil, fmt.Errorf("parse apple id token: %w", err)
	}
	if !token.Valid {
		return nil, errors.New("apple id token invalid")
	}
	if claims.Subject == "" {
		return nil, errors.New("apple id token missing sub")
	}
	return claims, nil
}
