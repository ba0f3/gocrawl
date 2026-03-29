package api

import (
	"context"
	"log"
	"net/http"
	"strings"
	"sync"

	"gocrawl/internal/config"
	"gocrawl/internal/db"
	"gocrawl/internal/user"

	"golang.org/x/time/rate"
)

// LoggingMiddleware logs HTTP requests
func LoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("%s %s %s", r.Method, r.URL.Path, r.RemoteAddr)
		next.ServeHTTP(w, r)
	})
}

// CORSMiddleware adds CORS headers
func CORSMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// AuthMiddleware validates API keys
func AuthMiddleware(database db.Store, enableAuth bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Skip authentication if disabled
			if !enableAuth {
				next.ServeHTTP(w, r)
				return
			}

			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				http.Error(w, "Authorization header required", http.StatusUnauthorized)
				return
			}

			// Extract API key from "Bearer <api_key>"
			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || parts[0] != "Bearer" {
				http.Error(w, "Invalid authorization header format", http.StatusUnauthorized)
				return
			}

			apiKey := parts[1]
			u, err := user.GetUserByAPIKey(database, apiKey)
			if err != nil {
				http.Error(w, "Invalid API key", http.StatusUnauthorized)
				return
			}

			// Add user to request context
			ctx := context.WithValue(r.Context(), "user", u)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RateLimitMiddleware limits requests per client (API key when present, else remote IP).
func RateLimitMiddleware(cfg config.RateLimitConfig) func(http.Handler) http.Handler {
	var mu sync.Mutex
	limiters := make(map[string]*rate.Limiter)
	var r rate.Limit
	if cfg.Window > 0 && cfg.Requests > 0 {
		r = rate.Limit(float64(cfg.Requests) / cfg.Window.Seconds())
	} else {
		r = rate.Inf
	}
	if r <= 0 {
		r = rate.Inf
	}
	burst := cfg.Requests
	if burst < 1 {
		burst = 1
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			key := "ip:" + req.RemoteAddr
			if ah := req.Header.Get("Authorization"); ah != "" {
				parts := strings.SplitN(ah, " ", 2)
				if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") && parts[1] != "" {
					key = "key:" + parts[1]
				}
			}
			mu.Lock()
			lim, ok := limiters[key]
			if !ok {
				lim = rate.NewLimiter(r, burst)
				limiters[key] = lim
			}
			mu.Unlock()
			if !lim.Allow() {
				http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
				return
			}
			next.ServeHTTP(w, req)
		})
	}
}
