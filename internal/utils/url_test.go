package utils

import (
	"net/url"
	"testing"
)

func TestResolveHref(t *testing.T) {
	baseURL, _ := url.Parse("https://example.com/some/path/page.html")

	tests := []struct {
		href     string
		expected string
	}{
		{"", ""},
		{"https://other.com/link", "https://other.com/link"},
		{"http://insecure.com", "http://insecure.com"},
		{"/root/path", "https://example.com/root/path"},
		{"/", "https://example.com/"},
		{"relative/link", "https://example.com/some/path/relative/link"},
		{"../up/link", "https://example.com/some/up/link"},
		{"?query=1", "https://example.com/some/path/page.html?query=1"},
		{"#frag", "https://example.com/some/path/page.html#frag"},
		{"//schemeless.com/path", "https://schemeless.com/path"},
		{"http://:bad", ""}, // malformed absolute URL should fail parse and return empty
		{"/some/%xx", ""},   // malformed root relative URL
	}

	for _, tt := range tests {
		t.Run(tt.href, func(t *testing.T) {
			got := ResolveHref(baseURL, tt.href)
			if got != tt.expected {
				t.Errorf("ResolveHref(%q) = %v, want %v", tt.href, got, tt.expected)
			}
		})
	}
}

func TestResolveHref_NilBase(t *testing.T) {
	got := ResolveHref(nil, "/some/path")
	if got != "" {
		t.Errorf("ResolveHref(nil, ...) = %v, want empty string", got)
	}
}

func TestResolveHref_EmptyBaseHost(t *testing.T) {
	baseURL, _ := url.Parse("/just/a/path")
	got := ResolveHref(baseURL, "/some/path")
	expected := "/some/path"
	if got != expected {
		t.Errorf("ResolveHref with empty base host = %v, want %v", got, expected)
	}
}

func BenchmarkResolveHref_Absolute(b *testing.B) {
	baseURL, _ := url.Parse("https://example.com/some/path/page.html")
	href := "https://other.com/link"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ResolveHref(baseURL, href)
	}
}

func BenchmarkResolveHref_RootRelative(b *testing.B) {
	baseURL, _ := url.Parse("https://example.com/some/path/page.html")
	href := "/root/path"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ResolveHref(baseURL, href)
	}
}

func BenchmarkResolveHref_Relative(b *testing.B) {
	baseURL, _ := url.Parse("https://example.com/some/path/page.html")
	href := "relative/link"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ResolveHref(baseURL, href)
	}
}
