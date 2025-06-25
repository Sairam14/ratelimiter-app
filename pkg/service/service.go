package service

import (
	"context"
	"fmt"
	"log"
	"os"
	"runtime"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

type Storage interface {
	Acquire(ctx context.Context, key string) (bool, error)
	Status(ctx context.Context, key string) (tokensLeft int, err error)
}

type RateLimitAlgorithm int

const (
	TokenBucket RateLimitAlgorithm = iota
	LeakyBucket
)

type Service struct {
	mu          sync.Mutex
	userCalls   sync.Map // concurrent map for user calls
	limit       int
	window      time.Duration
	limits      map[string]int // key: user or API key, value: limit
	redisClient *redis.Client
	useRedis    bool

	// Metrics
	successfulAcquires int64
	failedAcquires     int64
	requestsLastSecond int64
	redisLatencyMicros int64

	algorithm RateLimitAlgorithm
}

func NewService(algorithm RateLimitAlgorithm) *Service {
	// Try to connect to Redis
	addr := os.Getenv("REDIS_ADDR")
	if addr == "" {
		addr = "localhost:6379"
	}
	rdb := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: "", // no password set
		DB:       0,  // use default DB
	})

	// Test the connection
	_, err := rdb.Ping(context.Background()).Result()
	if err != nil {
		log.Println("Redis not available, falling back to in-memory storage:", err)
		return &Service{
			userCalls: sync.Map{},
			limit:     5,
			window:    time.Minute,
			useRedis:  false,
			limits:    make(map[string]int),
			algorithm: algorithm,
		}
	}

	log.Println("Connected to Redis")
	return &Service{
		userCalls:   sync.Map{},
		limit:       5,
		window:      time.Minute,
		redisClient: rdb,
		useRedis:    true,
		limits:      make(map[string]int),
		algorithm:   algorithm,
	}
}

func (s *Service) Acquire(ctx context.Context, input map[string]interface{}) map[string]interface{} {
	key, ok := input["key"].(string)
	if !ok || key == "" {
		s.mu.Lock()
		s.failedAcquires++
		s.mu.Unlock()
		return map[string]interface{}{
			"allowed": false,
			"error":   "missing or invalid key",
		}
	}

	switch s.algorithm {
	case TokenBucket:
		return s.acquireTokenBucket(ctx, key)
	case LeakyBucket:
		return s.acquireLeakyBucket(ctx, key)
	default:
		return map[string]interface{}{
			"allowed": false,
			"error":   "unknown algorithm",
		}
	}
}

func (s *Service) acquireTokenBucket(ctx context.Context, key string) map[string]interface{} {
	if s.useRedis && s.redisClient != nil {
		redisKey := "ratelimit:" + key
		now := float64(time.Now().UnixNano()) / 1e9
		windowStart := now - s.window.Seconds()
		limit := s.getLimitForKey(key)

		pipe := s.redisClient.TxPipeline()
		pipe.ZRemRangeByScore(ctx, redisKey, "0", fmt.Sprintf("%f", windowStart))
		pipe.ZAdd(ctx, redisKey, redis.Z{Score: now, Member: fmt.Sprintf("%f", now)})
		zcard := pipe.ZCard(ctx, redisKey)
		pipe.Expire(ctx, redisKey, s.window)
		_, err := pipe.Exec(ctx)
		if err != nil {
			s.failedAcquires++
			return map[string]interface{}{
				"allowed": false,
				"error":   "redis error",
			}
		}
		count, err := zcard.Result()
		if err != nil {
			s.failedAcquires++
			return map[string]interface{}{
				"allowed": false,
				"error":   "redis error",
			}
		}
		if int(count) > limit { // <-- should be > limit, not >=
			s.failedAcquires++
			return map[string]interface{}{
				"allowed": false,
				"error":   "rate limit exceeded",
			}
		}
		s.successfulAcquires++
		return map[string]interface{}{
			"allowed": true,
		}
	}

	now := time.Now()
	limit := s.getLimitForKey(key)

	val, _ := s.userCalls.LoadOrStore(key, []time.Time{})
	calls, _ := val.([]time.Time)

	// Remove calls outside the window
	var recentCalls []time.Time
	for _, t := range calls {
		if now.Sub(t) < s.window {
			recentCalls = append(recentCalls, t)
		}
	}
	if len(recentCalls) >= limit {
		s.failedAcquires++
		return map[string]interface{}{
			"allowed": false,
			"error":   "rate limit exceeded",
		}
	}
	recentCalls = append(recentCalls, now)
	s.userCalls.Store(key, recentCalls)
	s.successfulAcquires++
	return map[string]interface{}{
		"allowed": true,
	}
}

func (s *Service) acquireLeakyBucket(ctx context.Context, key string) map[string]interface{} {
	now := time.Now()
	limit := s.getLimitForKey(key)
	interval := s.window / time.Duration(limit)

	val, _ := s.userCalls.LoadOrStore(key, []time.Time{})
	calls, _ := val.([]time.Time)

	// Remove calls outside the window
	var recentCalls []time.Time
	for _, t := range calls {
		if now.Sub(t) < s.window {
			recentCalls = append(recentCalls, t)
		}
	}
	// Allow if enough time has passed since the last allowed request
	if len(recentCalls) == 0 || now.Sub(recentCalls[len(recentCalls)-1]) >= interval {
		recentCalls = append(recentCalls, now)
		s.userCalls.Store(key, recentCalls)
		s.successfulAcquires++
		return map[string]interface{}{
			"allowed": true,
		}
	}
	s.failedAcquires++
	return map[string]interface{}{
		"allowed": false,
		"error":   "leaky bucket: rate limit exceeded",
	}
}

// Sliding window algorithm in Redis
// (acquireRedis is currently unused and retryRedis is not needed, so both can be removed to fix the compile error)

func (s *Service) CreateExampleData(inputData map[string]interface{}) map[string]interface{} {
	return map[string]interface{}{
		"status": "created",
		"input":  inputData,
	}
}

func (s *Service) GetExampleData() map[string]interface{} {
	return map[string]interface{}{
		"message": "Hello from GetExampleData",
	}
}

func (s *Service) Status(ctx context.Context, key string) map[string]interface{} {
	limit := s.getLimitForKey(key)
	if s.useRedis && s.redisClient != nil {
		tokensLeft, err := s.statusRedis(ctx, key)
		if err == nil {
			return map[string]interface{}{
				"key":         key,
				"tokens_left": tokensLeft,
				"limit":       limit,
				"window_sec":  int(s.window.Seconds()),
				"refill_rate": float64(limit) / s.window.Seconds(),
				"source":      "redis",
			}
		}
	}

	now := time.Now()
	val, _ := s.userCalls.Load(key)
	var calls []time.Time
	if val != nil {
		calls, _ = val.([]time.Time)
	}
	var recentCalls []time.Time
	for _, t := range calls {
		if now.Sub(t) < s.window {
			recentCalls = append(recentCalls, t)
		}
	}
	tokensLeft := limit - len(recentCalls)
	if tokensLeft < 0 {
		tokensLeft = 0
	}
	return map[string]interface{}{
		"key":         key,
		"tokens_left": tokensLeft,
		"limit":       limit,
		"window_sec":  int(s.window.Seconds()),
		"refill_rate": float64(limit) / s.window.Seconds(),
	}
}

func (s *Service) statusRedis(ctx context.Context, key string) (int, error) {
	redisKey := "ratelimit:" + key
	now := float64(time.Now().UnixNano()) / 1e9
	windowStart := now - s.window.Seconds()
	limit := s.getLimitForKey(key)

	pipe := s.redisClient.TxPipeline()
	pipe.ZRemRangeByScore(ctx, redisKey, "0", fmt.Sprintf("%f", windowStart))
	zcard := pipe.ZCard(ctx, redisKey)
	_, err := pipe.Exec(ctx)
	if err != nil {
		return 0, err
	}
	count, err := zcard.Result()
	if err != nil {
		return 0, err
	}
	tokensLeft := limit - int(count)
	if tokensLeft < 0 {
		tokensLeft = 0
	}
	return tokensLeft, nil
}

// ExampleMethod is a placeholder for a business logic method
func (s *Service) ExampleMethod(input string) string {
	// Implement the business logic here
	return "Processed: " + input
}

// Additional methods for the service can be added here as needed

// Add a method to get metrics in Prometheus format
func (s *Service) Metrics() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return `# HELP ratelimiter_successful_acquires Number of successful acquire attempts
# TYPE ratelimiter_successful_acquires counter
ratelimiter_successful_acquires ` + itoa(s.successfulAcquires) + `
# HELP ratelimiter_failed_acquires Number of failed acquire attempts
# TYPE ratelimiter_failed_acquires counter
ratelimiter_failed_acquires ` + itoa(s.failedAcquires) + `
# HELP ratelimiter_requests_last_second Requests in the last second
# TYPE ratelimiter_requests_last_second gauge
ratelimiter_requests_last_second ` + itoa(s.requestsLastSecond) + `
# HELP ratelimiter_redis_latency_microseconds Last Redis latency in microseconds
# TYPE ratelimiter_redis_latency_microseconds gauge
ratelimiter_redis_latency_microseconds ` + itoa(s.redisLatencyMicros) + `
# HELP ratelimiter_goroutines Number of goroutines
# TYPE ratelimiter_goroutines gauge
ratelimiter_goroutines ` + itoa(int64(runtime.NumGoroutine())) + `
`
}

func (s *Service) SetLimit(key string, limit int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.limits == nil {
		s.limits = make(map[string]int)
	}
	s.limits[key] = limit
}

func (s *Service) getLimitForKey(key string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	if l, ok := s.limits[key]; ok {
		return l
	}
	return s.limit // global default
}

// Helper to convert int64 to string (no strconv for simplicity)
func itoa(i int64) string {
	return fmt.Sprintf("%d", i)
}
