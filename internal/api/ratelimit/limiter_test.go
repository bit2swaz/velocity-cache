package ratelimit

import (
	"testing"
	"time"
)

func TestLimiterAllow(t *testing.T) {
	limiter := New(2, time.Second)
	id := "client"

	if ok, _ := limiter.Allow(id); !ok {
		t.Fatalf("expected first attempt to be allowed")
	}
	if ok, _ := limiter.Allow(id); !ok {
		t.Fatalf("expected second attempt to be allowed")
	}
	if ok, _ := limiter.Allow(id); ok {
		t.Fatalf("expected third attempt to be denied")
	}

	time.Sleep(time.Second + 10*time.Millisecond)

	if ok, _ := limiter.Allow(id); !ok {
		t.Fatalf("expected attempt after reset to be allowed")
	}
}
