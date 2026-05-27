// Package auth verifies Firebase Phone Auth ID tokens and mints our own
// short-lived API tokens.
//
// Two-token flow:
//  1. Mobile signs in via Firebase Phone Auth → gets Firebase ID token.
//  2. Mobile POSTs the Firebase ID token to /v1/auth/exchange.
//  3. Server verifies via firebase-admin, looks up / creates the local user,
//     mints an "app token" (HMAC-signed compact JSON, short-lived).
//  4. Mobile uses the app token for all REST + WebSocket calls.
//
// Why: avoids verifying Firebase JWTs (which needs a key fetch) on every
// request, and lets us put our own claims (user_id) in the token.
package auth

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	firebase "firebase.google.com/go/v4"
	fbAuth "firebase.google.com/go/v4/auth"
	"google.golang.org/api/option"
)

// ----- Firebase verifier --------------------------------------------------

type Firebase struct {
	client *fbAuth.Client
}

func NewFirebase(ctx context.Context, credentialsPath string) (*Firebase, error) {
	app, err := firebase.NewApp(ctx, nil, option.WithCredentialsFile(credentialsPath))
	if err != nil {
		return nil, fmt.Errorf("firebase app: %w", err)
	}
	c, err := app.Auth(ctx)
	if err != nil {
		return nil, fmt.Errorf("firebase auth client: %w", err)
	}
	return &Firebase{client: c}, nil
}

// VerifyIDToken validates a Firebase Phone Auth ID token and returns the
// phone number embedded in it.
func (f *Firebase) VerifyIDToken(ctx context.Context, idToken string) (phone string, err error) {
	tok, err := f.client.VerifyIDToken(ctx, idToken)
	if err != nil {
		return "", fmt.Errorf("verify firebase token: %w", err)
	}
	if v, ok := tok.Firebase.Identities["phone"]; ok {
		if arr, ok := v.([]any); ok && len(arr) > 0 {
			if s, ok := arr[0].(string); ok {
				return s, nil
			}
		}
	}
	return "", errors.New("firebase token has no phone identity")
}

// ----- App tokens (HMAC signed) -------------------------------------------

// Claims is the body of an app token. Compact JSON, HMAC-SHA256 signed.
type Claims struct {
	UserID    string `json:"uid"`
	Phone     string `json:"ph"`
	ExpiresAt int64  `json:"exp"`
}

// Issue returns a signed token string: base64(claims).base64(hmacSHA256).
func Issue(secret string, c Claims) (string, error) {
	if c.ExpiresAt == 0 {
		return "", errors.New("auth: claims missing exp")
	}
	payload, err := json.Marshal(c)
	if err != nil {
		return "", err
	}
	body := base64.RawURLEncoding.EncodeToString(payload)
	sig := sign(secret, body)
	return body + "." + sig, nil
}

// Verify parses and validates a token. Returns ErrTokenExpired or ErrTokenInvalid.
func Verify(secret, token string) (Claims, error) {
	parts := strings.SplitN(token, ".", 2)
	if len(parts) != 2 {
		return Claims{}, ErrTokenInvalid
	}
	if !hmac.Equal([]byte(sign(secret, parts[0])), []byte(parts[1])) {
		return Claims{}, ErrTokenInvalid
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return Claims{}, ErrTokenInvalid
	}
	var c Claims
	if err := json.Unmarshal(raw, &c); err != nil {
		return Claims{}, ErrTokenInvalid
	}
	if time.Now().Unix() >= c.ExpiresAt {
		return Claims{}, ErrTokenExpired
	}
	return c, nil
}

func sign(secret, body string) string {
	h := hmac.New(sha256.New, []byte(secret))
	h.Write([]byte(body))
	return base64.RawURLEncoding.EncodeToString(h.Sum(nil))
}

var (
	ErrTokenInvalid = errors.New("auth: invalid token")
	ErrTokenExpired = errors.New("auth: token expired")
)
