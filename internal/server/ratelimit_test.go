package server

import (
	"net/http/httptest"
	"testing"
)

func TestClientIPWithoutTrustedProxyUsesRemoteAddr(t *testing.T) {
	t.Setenv("TICKET_TRUSTED_PROXY_CIDRS", "")
	req := httptest.NewRequest("POST", "/api/login", nil)
	req.RemoteAddr = "198.51.100.42:1234"
	if got := clientIP(req); got != "198.51.100.42" {
		t.Fatalf("clientIP() = %q, want %q", got, "198.51.100.42")
	}
}

func TestClientIPWithTrustedProxyUsesForwardedFor(t *testing.T) {
	t.Setenv("TICKET_TRUSTED_PROXY_CIDRS", "10.0.0.0/8,127.0.0.1/32")
	req := httptest.NewRequest("POST", "/api/login", nil)
	req.RemoteAddr = "10.1.2.3:1234"
	req.Header.Set("X-Forwarded-For", "203.0.113.9, 10.1.2.3")
	if got := clientIP(req); got != "203.0.113.9" {
		t.Fatalf("clientIP() = %q, want %q", got, "203.0.113.9")
	}
}

func TestClientIPIgnoresForwardedForFromUntrustedSource(t *testing.T) {
	t.Setenv("TICKET_TRUSTED_PROXY_CIDRS", "10.0.0.0/8")
	req := httptest.NewRequest("POST", "/api/login", nil)
	req.RemoteAddr = "198.51.100.42:1234"
	req.Header.Set("X-Forwarded-For", "203.0.113.9")
	if got := clientIP(req); got != "198.51.100.42" {
		t.Fatalf("clientIP() = %q, want %q", got, "198.51.100.42")
	}
}
