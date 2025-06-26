package service

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestAcquire_HighConcurrency(t *testing.T) {
	s := &Service{
		userCalls: sync.Map{},
		limit:     1000,
		window:    time.Second,
	}

	var wg sync.WaitGroup
	concurrency := 10000
	var success int64
	var failed int64
	ctx := context.Background()
	input := map[string]interface{}{"key": "loadtest"}

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			res := s.Acquire(ctx, input)
			if allowed, ok := res["allowed"].(bool); ok && allowed {
				atomic.AddInt64(&success, 1)
			} else {
				atomic.AddInt64(&failed, 1)
			}
		}()
	}
	wg.Wait()
	t.Logf("Total allowed: %d out of %d and failed are %d", success, concurrency, failed)
}
