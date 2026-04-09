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
