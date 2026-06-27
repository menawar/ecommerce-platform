// Package ratelimit is a distributed token-bucket rate limiter backed by Redis.
//
// Token bucket: each key owns a bucket of at most `burst` tokens that refills at
// `rate` tokens/sec. Each request spends one token; an empty bucket is rejected.
// This permits short bursts (up to `burst`) while bounding the *sustained* rate —
// the behaviour you want for an API edge, unlike a fixed window (which allows 2×
// the limit across a boundary) or a strict leaky bucket (no burst headroom).
//
// The check-refill-decrement happens inside a single Lua script so it is ATOMIC
// on the Redis server: concurrent requests for the same key (across many gateway
// instances) can never both read a stale token count and overspend.
package ratelimit

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

// bucketScript implements the token bucket. State is a hash {tokens, ts} per key.
// KEYS[1]=bucket key; ARGV: rate (tok/s), burst, now (ms). Returns {allowed, retry_ms}.
const bucketScript = `
local rate  = tonumber(ARGV[1])
local burst = tonumber(ARGV[2])
local now   = tonumber(ARGV[3])

local b = redis.call('HMGET', KEYS[1], 'tokens', 'ts')
local tokens = tonumber(b[1])
local ts     = tonumber(b[2])
if tokens == nil then
  tokens = burst
  ts = now
end

-- Refill for the time elapsed since we last touched the bucket, capped at burst.
local elapsed = math.max(0, now - ts) / 1000.0
tokens = math.min(burst, tokens + elapsed * rate)

local allowed = 0
local retry_ms = 0
if tokens >= 1 then
  tokens = tokens - 1
  allowed = 1
else
  retry_ms = math.ceil((1 - tokens) / rate * 1000)
end

redis.call('HSET', KEYS[1], 'tokens', tokens, 'ts', now)
-- Drop idle buckets after they'd have fully refilled, so keys don't accumulate.
redis.call('PEXPIRE', KEYS[1], math.ceil(burst / rate * 1000) + 1000)
return {allowed, retry_ms}
`

// Result is the outcome of an Allow check.
type Result struct {
	Allowed    bool          // false = over the limit
	RetryAfter time.Duration // when !Allowed, how long until a token is available
}

// Limiter consumes tokens from per-key buckets in Redis.
type Limiter struct {
	rdb    redis.Scripter
	script *redis.Script
	rate   float64
	burst  int
	nowMS  func() int64
}

// Option configures a Limiter.
type Option func(*Limiter)

// WithClock overrides the time source (tests inject a controllable clock so they
// can exercise refill without sleeping).
func WithClock(now func() time.Time) Option {
	return func(l *Limiter) { l.nowMS = func() int64 { return now().UnixMilli() } }
}

// New builds a limiter allowing `burst` tokens with a steady refill of `rate`
// tokens per second.
func New(rdb redis.Scripter, rate float64, burst int, opts ...Option) *Limiter {
	l := &Limiter{
		rdb:    rdb,
		script: redis.NewScript(bucketScript),
		rate:   rate,
		burst:  burst,
		nowMS:  func() int64 { return time.Now().UnixMilli() },
	}
	for _, o := range opts {
		o(l)
	}
	return l
}

// Allow spends one token for key. The caller decides what to do with a !Allowed
// result (typically respond 429 with Retry-After). A Redis error is returned so
// the caller can choose its failure mode (the gateway fails open).
func (l *Limiter) Allow(ctx context.Context, key string) (Result, error) {
	vals, err := l.script.Run(ctx, l.rdb, []string{key}, l.rate, l.burst, l.nowMS()).Slice()
	if err != nil {
		return Result{}, err
	}
	allowed, _ := vals[0].(int64)
	retryMS, _ := vals[1].(int64)
	return Result{
		Allowed:    allowed == 1,
		RetryAfter: time.Duration(retryMS) * time.Millisecond,
	}, nil
}
