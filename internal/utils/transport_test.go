package utils

import (
	"net/http"
	"testing"
)

func TestSafeTransport(t *testing.T) {
	rt := http.DefaultTransport
	safeRt := SafeTransport(rt)

	if safeRt == nil {
		t.Fatal("expected non-nil transport")
	}

	tc, ok := safeRt.(*http.Transport)
	if !ok {
		t.Fatal("expected *http.Transport")
	}

	if tc.DialContext == nil {
		t.Fatal("expected non-nil DialContext")
	}
}

func TestSafeControl(t *testing.T) {
	tests := []struct {
		address string
		wantErr bool
	}{
		// Blocked addresses
		{"127.0.0.1:80", true},
		{"[::1]:80", true},
		{"10.0.0.1:80", true},
		{"192.168.1.1:80", true},
		{"172.16.0.1:80", true},
		{"169.254.169.254:80", true},
		{"0.0.0.0:80", true},
		{"[::]:80", true},

		// Allowed addresses
		{"8.8.8.8:80", false},
		{"1.1.1.1:80", false},
		{"example.com:80", false},
	}

	for _, tc := range tests {
		t.Run(tc.address, func(t *testing.T) {
			err := SafeControl("tcp", tc.address, nil)
			if (err != nil) != tc.wantErr {
				t.Errorf("SafeControl(%q) error = %v, wantErr %v", tc.address, err, tc.wantErr)
			}
		})
	}
}
