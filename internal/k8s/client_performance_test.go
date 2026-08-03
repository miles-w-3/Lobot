package k8s

import (
	"testing"

	"k8s.io/client-go/rest"
)

func TestConfigureClientPerformanceSetsUnsetRateLimits(t *testing.T) {
	config := &rest.Config{}
	configureClientPerformance(config)
	if config.QPS != startupClientQPS || config.Burst != startupClientBurst {
		t.Fatalf("rate limits = %v/%d, want %v/%d", config.QPS, config.Burst, startupClientQPS, startupClientBurst)
	}
}

func TestConfigureClientPerformancePreservesExplicitRateLimits(t *testing.T) {
	config := &rest.Config{QPS: 7, Burst: 11}
	configureClientPerformance(config)
	if config.QPS != 7 || config.Burst != 11 {
		t.Fatalf("explicit rate limits changed to %v/%d", config.QPS, config.Burst)
	}
}
