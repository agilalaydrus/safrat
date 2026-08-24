package repository

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/hajj-saas/api/internal/domain"
	"github.com/redis/go-redis/v9"
)

func TestRedisOperatorCacheInvalidatesPeerL1(t *testing.T) {
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
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	repositoryA := NewRedisOperatorRepository(ctx, nil, clientA, nil)
	repositoryB := NewRedisOperatorRepository(ctx, nil, clientB, nil)
	orgID := fmt.Sprintf("org-test-%d", time.Now().UnixNano())
	repositoryB.setLocalCache(orgID, &domain.Operator{BetterAuthOrgID: orgID, Name: "stale"})
	time.Sleep(100 * time.Millisecond)
	repositoryA.cacheOperator(ctx, &domain.Operator{BetterAuthOrgID: orgID, Name: "fresh"}, true)
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if _, ok := repositoryB.cachedOperator(orgID); !ok {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("peer L1 cache was not invalidated")
}
