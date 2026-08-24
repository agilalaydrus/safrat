package middleware

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"golang.org/x/time/rate"
)

func TestRedisRateLimiterSharesBurstAcrossReplicas(t *testing.T) {
	url := os.Getenv("REDIS_TEST_URL")
	if url == "" {
		t.Skip("REDIS_TEST_URL not set")
	}
	options, err := redis.ParseURL(url)
	if err != nil {
		t.Fatalf("parse redis URL: %v", err)
	}
	clientA := redis.NewClient(options)
	clientB := redis.NewClient(options)
	defer func() { _ = clientA.Close(); _ = clientB.Close() }()
	limiterA := &redisRateLimiter{client: clientA}
	limiterB := &redisRateLimiter{client: clientB}
	ip := fmt.Sprintf("test-%d", time.Now().UnixNano())
	limit := rate.Every(time.Hour)
	for request := 0; request < rateLimitBurst; request++ {
		limiter := limiterA
		if request%2 == 1 {
			limiter = limiterB
		}
		allowed, err := limiter.allow(context.Background(), "/test.Service/Method", ip, limit)
		if err != nil || !allowed {
			t.Fatalf("request %d: allowed=%v err=%v", request+1, allowed, err)
		}
	}
	allowed, err := limiterB.allow(context.Background(), "/test.Service/Method", ip, limit)
	if err != nil {
		t.Fatalf("sixth request: %v", err)
	}
	if allowed {
		t.Fatal("sixth request should be rejected by shared burst")
	}
}
