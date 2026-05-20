package proxy

import (
	"net/http"
	"testing"
	"time"
)

func TestParseRetryAfterSeconds(t *testing.T) {
	now := time.Date(2026, 5, 20, 9, 0, 0, 0, time.UTC)
	until, ok := parseRetryAfter("15", now)
	if !ok {
		t.Fatalf("expected retry-after to parse")
	}
	if got := until.Sub(now); got != 15*time.Second {
		t.Fatalf("expected 15s cooldown, got %s", got)
	}
}

func TestParseRateLimitResetHeader(t *testing.T) {
	now := time.Date(2026, 5, 20, 9, 0, 0, 0, time.UTC)
	headers := http.Header{}
	headers.Set("x-ratelimit-reset", "1800000000")

	until, ok := parseRateLimitReset(headers, now)
	if !ok {
		t.Fatalf("expected x-ratelimit-reset to parse")
	}
	if !until.After(now) {
		t.Fatalf("expected future cooldown, got %s", until)
	}
}

func TestFallbackCooldownFor429(t *testing.T) {
	if got := getFallbackCooldown(429); got != 15*time.Second {
		t.Fatalf("expected 15s for 429, got %s", got)
	}
}
