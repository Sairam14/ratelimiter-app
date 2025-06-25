package service

import (
	"context"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

func setupRedis(t *testing.T) *Service {
	addr := os.Getenv("REDIS_ADDR")
	if addr == "" {
		addr = "localhost:6379"
	}
	rdb := redis.NewClient(&redis.Options{
		Addr: addr,
	})
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := rdb.Ping(ctx).Err(); err != nil {
		t.Skip("Redis not available for integration test:", err)
	}
	// Clean up keys before test
	rdb.FlushDB(ctx)
	return &Service{
		userCalls:   sync.Map{},
		limit:       2,
		window:      time.Second,
		redisClient: rdb,
		useRedis:    true,
		limits:      make(map[string]int),
		algorithm:   TokenBucket, // or LeakyBucket if you want to test that
	}
}

func TestRedisAcquire_AllowedAndRateLimited(t *testing.T) {
	s := setupRedis(t)
	ctx := context.Background()
	input := map[string]interface{}{"key": "redisuser1"}

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

	// Third call should be rate limited
	res = s.Acquire(ctx, input)
	if allowed, ok := res["allowed"].(bool); !ok || allowed {
		t.Errorf("expected allowed=false, got %v", res)
	}
	if errMsg, ok := res["error"].(string); !ok || errMsg != "rate limit exceeded" {
		t.Errorf("expected rate limit error, got %v", res)
	}
}

func TestRedisStatus_TokensLeft(t *testing.T) {
	s := setupRedis(t)
	ctx := context.Background()
	key := "redisuser2"
	input := map[string]interface{}{"key": key}

	// Use up 1 token
	s.Acquire(ctx, input)

	status := s.Status(ctx, key)
	tokens, ok := status["tokens_left"].(int)
	if !ok {
		// Prometheus metrics may encode as float64, so try that
		if f, ok := status["tokens_left"].(float64); ok {
			tokens = int(f)
		} else {
			t.Fatalf("tokens_left not found or wrong type: %v", status)
		}
	}
	if tokens != 1 {
		t.Errorf("expected tokens_left=1, got %v", tokens)
	}
}
