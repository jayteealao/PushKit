package auth

import (
	"crypto/hmac"
	"net/http"

	"github.com/pushkit/backend/internal/config"
	"github.com/pushkit/backend/internal/models"
)

type contextKey string

const UserIDKey contextKey = "user_id"

// Middleware returns an HTTP middleware that validates the X-API-Key header
// and injects the user_id into the request context.
func Middleware(cfg *config.Config) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			apiKey := r.Header.Get("X-API-Key")
			if apiKey == "" {
				writeJSON(w, http.StatusUnauthorized, models.ErrorResponse{Error: "missing X-API-Key header"})
				return
			}

			userID, ok := lookupKey(cfg.APIKeyMap, apiKey)
			if !ok {
				writeJSON(w, http.StatusUnauthorized, models.ErrorResponse{Error: "invalid API key"})
				return
			}

			ctx := r.Context()
			ctx = setUserID(ctx, userID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// lookupKey uses constant-time comparison to prevent timing attacks.
func lookupKey(keyMap map[string]string, provided string) (string, bool) {
	providedBytes := []byte(provided)
	for key, userID := range keyMap {
		if hmac.Equal([]byte(key), providedBytes) {
			return userID, true
		}
	}
	return "", false
}
