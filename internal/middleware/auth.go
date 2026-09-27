package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"backend-challenge/internal/service"
)

type contextKey string

const userIDKey contextKey = "userID"

func NewAuthMiddlewate(secret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			header := r.Header.Get("Authorization")
			tokenString, ok := strings.CutPrefix(header, "Bearer ")
			if !ok || tokenString == "" {
				unauthorized(w, "missing or invalid authorization header")
				return
			}

			userID, err := service.ParseToken(secret, tokenString)
			if err != nil {
				unauthorized(w, "invalid or expired token")
				return
			}

			ctx := context.WithValue(r.Context(), userIDKey, userID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func unauthorized(w http.ResponseWriter, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": message})
}
