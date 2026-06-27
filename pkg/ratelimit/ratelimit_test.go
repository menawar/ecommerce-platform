package ratelimit_test

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"

	"github.com/menawar/ecommerce-platform/pkg/ratelimit"
)

func newRedis(t *testing.T) *redis.Client {
	t.Helper()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	t.Cleanup(mr.Close)
	c := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = c.Close() })
	return c
}

// TestBurstThenDeny: a fresh bucket allows exactly `burst` requests, then denies
// with a positive Retry-After.
func TestBurstThenDeny(t *testing.T) {
	ctx := context.Background()
	l := ratelimit.New(newRedis(t), 1, 3) // 1 tok/s, burst 3

	for i := 0; i < 3; i++ {
		res, err := l.Allow(ctx, "user:1")
		if err != nil {
			t.Fatalf("allow %d: %v", i, err)
		}
		if !res.Allowed {
			t.Fatalf("request %d should be allowed within burst", i+1)
		}
	}

	res, err := l.Allow(ctx, "user:1")
	if err != nil {
		t.Fatalf("allow: %v", err)
	}
	if res.Allowed {
		t.Fatal("4th request should be denied (burst exhausted)")
	}
	if res.RetryAfter <= 0 {
		t.Errorf("denied result should carry a positive Retry-After, got %v", res.RetryAfter)
	}
}

// TestRefill: after the bucket empties, advancing the clock past the refill
// interval lets a request through again.
func TestRefill(t *testing.T) {
	ctx := context.Background()
	now := time.Unix(1000, 0)
	l := ratelimit.New(newRedis(t), 1, 1, ratelimit.WithClock(func() time.Time { return now }))

	if res, _ := l.Allow(ctx, "k"); !res.Allowed {
		t.Fatal("first request should pass")
	}
	if res, _ := l.Allow(ctx, "k"); res.Allowed {
		t.Fatal("second request should be denied (burst=1, no time passed)")
	}

	now = now.Add(1100 * time.Millisecond) // > 1s at 1 tok/s
	if res, _ := l.Allow(ctx, "k"); !res.Allowed {
		t.Error("after refill the request should pass again")
	}
}

// TestKeysAreIndependent: one key's exhaustion doesn't affect another.
func TestKeysAreIndependent(t *testing.T) {
	ctx := context.Background()
	l := ratelimit.New(newRedis(t), 1, 1)

	if res, _ := l.Allow(ctx, "ip:a"); !res.Allowed {
		t.Fatal("ip:a first request should pass")
	}
	if res, _ := l.Allow(ctx, "ip:a"); res.Allowed {
		t.Fatal("ip:a should now be limited")
	}
	if res, _ := l.Allow(ctx, "ip:b"); !res.Allowed {
		t.Error("ip:b has its own bucket and should pass")
	}
}
