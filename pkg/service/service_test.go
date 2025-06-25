package service

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestInMemoryAcquire_Allowed(t *testing.T) {
	s := &Service{
		userCalls: sync.Map{},
		limit:     2,
		window:    time.Second,
	}

	ctx := context.Background()
	input := map[string]interface{}{"key": "user1"}

	// First call should be allowed
	res := s.Acquire(ctx, input)
	if allowed, ok := res["allowed"].(bool); !ok || !allowed {
		t.Errorf("expected allowed=true, got %v", res)
	}

	// Second call should be allowed
	res = s.Acquire(ctx, input)
	if allowed, ok := res["allowed"].(bool); !ok || !allowed {
		t.Errorf("expected allowed=true, got %v", res)
	}
}

func TestInMemoryAcquire_RateLimited(t *testing.T) {
	s := &Service{
		userCalls: sync.Map{},
		limit:     1,
		window:    time.Second,
	}

	ctx := context.Background()
	input := map[string]interface{}{"key": "user2"}

	// First call should be allowed
	res := s.Acquire(ctx, input)
	if allowed, ok := res["allowed"].(bool); !ok || !allowed {
		t.Errorf("expected allowed=true, got %v", res)
	}

	// Second call should be rate limited
	res = s.Acquire(ctx, input)
	if allowed, ok := res["allowed"].(bool); !ok || allowed {
		t.Errorf("expected allowed=false, got %v", res)
	}
	if errMsg, ok := res["error"].(string); !ok || errMsg != "rate limit exceeded" {
		t.Errorf("expected rate limit error, got %v", res)
	}
}

func TestStatus_TokensLeft(t *testing.T) {
	s := &Service{
		userCalls: sync.Map{},
		limit:     3,
		window:    time.Second,
	}

	ctx := context.Background()
	key := "user3"
	input := map[string]interface{}{"key": key}

	// Use up 2 tokens
	s.Acquire(ctx, input)
	s.Acquire(ctx, input)

	status := s.Status(ctx, key)
	if tokens, ok := status["tokens_left"].(int); !ok || tokens != 1 {
		t.Errorf("expected tokens_left=1, got %v", status)
	}
}

func TestAcquire_MissingKey(t *testing.T) {
	s := &Service{
		userCalls: sync.Map{},
		limit:     1,
		window:    time.Second,
	}

	ctx := context.Background()
	input := map[string]interface{}{}

	res := s.Acquire(ctx, input)
	if allowed, ok := res["allowed"].(bool); !ok || allowed {
		t.Errorf("expected allowed=false for missing key, got %v", res)
	}
	if errMsg, ok := res["error"].(string); !ok || errMsg != "missing or invalid key" {
		t.Errorf("expected missing key error, got %v", res)
	}
}
