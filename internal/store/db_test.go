package store

import (
	"testing"
	"time"
)

func TestShouldOpenCircuit(t *testing.T) {
	if shouldOpenCircuit(503, 2) {
		t.Fatalf("circuit must stay closed before threshold")
	}
	if !shouldOpenCircuit(503, 3) {
		t.Fatalf("circuit should open on repeated 5xx")
	}
	if !shouldOpenCircuit(0, 3) {
		t.Fatalf("circuit should open on repeated transport errors")
	}
	if shouldOpenCircuit(429, 5) {
		t.Fatalf("429 should use cooldown, not circuit open")
	}
}

func TestGetCircuitBreakDuration(t *testing.T) {
	if got := getCircuitBreakDuration(3); got != 20*time.Second {
		t.Fatalf("expected 20s, got %s", got)
	}
	if got := getCircuitBreakDuration(4); got != 45*time.Second {
		t.Fatalf("expected 45s, got %s", got)
	}
	if got := getCircuitBreakDuration(6); got != 90*time.Second {
		t.Fatalf("expected 90s, got %s", got)
	}
}

func TestEstimateCost(t *testing.T) {
	pricing := map[string]interface{}{
		"gpt-test": map[string]interface{}{
			"promptCostPer1k":     0.002,
			"completionCostPer1k": 0.004,
		},
	}
	got := estimateCost(pricing, "openai/gpt-test", 500, 250)
	want := 0.002
	if got != want {
		t.Fatalf("expected cost %v, got %v", want, got)
	}
}
