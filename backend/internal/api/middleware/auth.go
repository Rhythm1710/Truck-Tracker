// Package middleware holds HTTP middlewares used by the API router.
package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/rhythm/truck-tracker/internal/auth"
)

type ctxKey int

const claimsKey ctxKey = 1

// RequireAuth verifies the Bearer token and puts the claims on the request
// context. 401s if the token is missing/invalid/expired.
func RequireAuth(secret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authz := r.Header.Get("Authorization")
			if !strings.HasPrefix(authz, "Bearer ") {
				http.Error(w, "missing bearer token", http.StatusUnauthorized)
				return
			}
			tok := strings.TrimPrefix(authz, "Bearer ")
			c, err := auth.Verify(secret, tok)
			if err != nil {
				http.Error(w, "invalid token", http.StatusUnauthorized)
				return
			}
			ctx := context.WithValue(r.Context(), claimsKey, c)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// ClaimsFrom extracts the verified claims placed by RequireAuth.
func ClaimsFrom(ctx context.Context) (auth.Claims, bool) {
	c, ok := ctx.Value(claimsKey).(auth.Claims)
	return c, ok
}
