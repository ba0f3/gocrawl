package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestGinCORSMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		allowedOrigins []string
		requestOrigin  string
		requestMethod  string
		expectedCORS   string
	}{
		{
			name:           "Allow all with *",
			allowedOrigins: []string{"*"},
			requestOrigin:  "http://example.com",
			requestMethod:  http.MethodGet,
			expectedCORS:   "http://example.com",
		},
		{
			name:           "Allow specific origin",
			allowedOrigins: []string{"http://example.com"},
			requestOrigin:  "http://example.com",
			requestMethod:  http.MethodGet,
			expectedCORS:   "http://example.com",
		},
		{
			name:           "Deny unknown origin",
			allowedOrigins: []string{"http://example.com"},
			requestOrigin:  "http://malicious.com",
			requestMethod:  http.MethodGet,
			expectedCORS:   "",
		},
		{
			name:           "Allow multiple origins",
			allowedOrigins: []string{"http://example.com", "http://test.com"},
			requestOrigin:  "http://test.com",
			requestMethod:  http.MethodGet,
			expectedCORS:   "http://test.com",
		},
		{
			name:           "No origin header",
			allowedOrigins: []string{"*"},
			requestOrigin:  "",
			requestMethod:  http.MethodGet,
			expectedCORS:   "",
		},
		{
			name:           "OPTIONS preflight allowed",
			allowedOrigins: []string{"http://example.com"},
			requestOrigin:  "http://example.com",
			requestMethod:  http.MethodOptions,
			expectedCORS:   "http://example.com",
		},
		{
			name:           "OPTIONS preflight denied",
			allowedOrigins: []string{"http://example.com"},
			requestOrigin:  "http://malicious.com",
			requestMethod:  http.MethodOptions,
			expectedCORS:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := gin.New()
			r.Use(GinCORSMiddleware(tt.allowedOrigins))
			r.GET("/test", func(c *gin.Context) {
				c.Status(http.StatusOK)
			})

			req, _ := http.NewRequest(tt.requestMethod, "/test", nil)
			if tt.requestOrigin != "" {
				req.Header.Set("Origin", tt.requestOrigin)
			}
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			if tt.requestMethod == http.MethodOptions {
				assert.Equal(t, http.StatusOK, w.Code)
			} else {
				assert.Equal(t, http.StatusOK, w.Code)
			}
			assert.Equal(t, tt.expectedCORS, w.Header().Get("Access-Control-Allow-Origin"))
			if tt.requestOrigin != "" {
				assert.Equal(t, "Origin", w.Header().Get("Vary"))
			}
		})
	}
}
