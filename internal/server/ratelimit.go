package server

import (
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

type rateLimiter struct {
	mu       sync.Mutex
	attempts map[string][]time.Time
	limit    int
	window   time.Duration
}

func newRateLimiter(limit int, window time.Duration) *rateLimiter {
	return &rateLimiter{
		attempts: make(map[string][]time.Time),
		limit:    limit,
		window:   window,
	}
}

func (rl *rateLimiter) allow(key string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-rl.window)

	// Remove expired entries for the current key.
	valid := rl.attempts[key][:0]
	for _, t := range rl.attempts[key] {
		if t.After(cutoff) {
			valid = append(valid, t)
		}
	}
	rl.attempts[key] = valid

	// Evict stale keys to prevent unbounded map growth.
	for k, timestamps := range rl.attempts {
		if k == key {
			continue
		}
		latest := timestamps[len(timestamps)-1]
		if !latest.After(cutoff) {
			delete(rl.attempts, k)
		}
	}

	if len(valid) >= rl.limit {
		return false
	}
	rl.attempts[key] = append(rl.attempts[key], now)
	return true
}

func clientIP(r *http.Request) string {
	remote := remoteIPFromAddr(r.RemoteAddr)
	if remote == nil {
		return r.RemoteAddr
	}
	trusted := trustedProxyCIDRsFromEnv()
	if len(trusted) == 0 || !ipInTrustedRanges(remote, trusted) {
		return remote.String()
	}
	xff := strings.TrimSpace(r.Header.Get("X-Forwarded-For"))
	if xff == "" {
		return remote.String()
	}
	parts := strings.Split(xff, ",")
	for i := len(parts) - 1; i >= 0; i-- {
		part := strings.TrimSpace(parts[i])
		candidate := net.ParseIP(part)
		if candidate == nil {
			continue
		}
		if !ipInTrustedRanges(candidate, trusted) {
			return candidate.String()
		}
	}
	return remote.String()
}

func remoteIPFromAddr(remoteAddr string) net.IP {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		host = remoteAddr
	}
	return net.ParseIP(strings.TrimSpace(host))
}

func trustedProxyCIDRsFromEnv() []*net.IPNet {
	raw := strings.TrimSpace(os.Getenv("TICKET_TRUSTED_PROXY_CIDRS"))
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	ranges := make([]*net.IPNet, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed == "" {
			continue
		}
		_, cidr, err := net.ParseCIDR(trimmed)
		if err != nil {
			continue
		}
		ranges = append(ranges, cidr)
	}
	return ranges
}

func ipInTrustedRanges(ip net.IP, trusted []*net.IPNet) bool {
	for _, cidr := range trusted {
		if cidr.Contains(ip) {
			return true
		}
	}
	return false
}
