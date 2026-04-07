package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestGinCORSMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		allowedOrigins []string
		requestOrigin  string
		expectedCORS   string
	}{
		{
			name:           "Allow all with *",
			allowedOrigins: []string{"*"},
			requestOrigin:  "http://example.com",
			expectedCORS:   "http://example.com",
		},
		{
			name:           "Allow specific origin",
			allowedOrigins: []string{"http://example.com"},
			requestOrigin:  "http://example.com",
			expectedCORS:   "http://example.com",
		},
		{
			name:           "Deny unknown origin",
			allowedOrigins: []string{"http://example.com"},
			requestOrigin:  "http://malicious.com",
			expectedCORS:   "",
		},
		{
			name:           "Allow multiple origins",
			allowedOrigins: []string{"http://example.com", "http://test.com"},
			requestOrigin:  "http://test.com",
			expectedCORS:   "http://test.com",
		},
		{
			name:           "No origin header",
			allowedOrigins: []string{"*"},
			requestOrigin:  "",
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

			req, _ := http.NewRequest(http.MethodGet, "/test", nil)
			if tt.requestOrigin != "" {
				req.Header.Set("Origin", tt.requestOrigin)
			}
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Errorf("expected status OK, got %v", w.Code)
			}
			gotCORS := w.Header().Get("Access-Control-Allow-Origin")
			if gotCORS != tt.expectedCORS {
				t.Errorf("expected CORS %q, got %q", tt.expectedCORS, gotCORS)
			}
		})
	}
}
