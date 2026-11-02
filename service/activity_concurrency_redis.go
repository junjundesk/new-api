package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/go-redis/redis/v8"
)

// redisActivityLimitStore 用 Redis 承载多实例共享的活动限流状态：
// RPM 固定窗口用 INCR+EXPIRE 原子脚本，并发槽用 ZSET 租约（member=UUID 租约 ID，
// score=过期时间戳）配合三个原子 Lua 脚本。时间一律取服务端 TIME，避免多实例时钟漂移。
type redisActivityLimitStore struct {
	client *redis.Client
}

func newRedisActivityLimitStore(client *redis.Client) *redisActivityLimitStore {
	return &redisActivityLimitStore{client: client}
}

// activityRateWindowScript 原子完成固定窗口计数、过期与限流判定，
// 返回 {allowed, window ttl seconds}。
const activityRateWindowScript = `local count = redis.call("INCR", KEYS[1])
if count == 1 then
  redis.call("EXPIRE", KEYS[1], ARGV[2])
end
local ttl = redis.call("TTL", KEYS[1])
if ttl < 0 then
  redis.call("EXPIRE", KEYS[1], ARGV[2])
  ttl = redis.call("TTL", KEYS[1])
end
if count > tonumber(ARGV[1]) then
  return {0, ttl}
end
return {1, ttl}
`

// activityConcurrencyAcquireScript 先清理已过期租约，再做「同一 leaseID 重试幂等」
// 判断（剪枝后仍存在的成员必然未过期），最后按 limit 占用一个槽：
// ARGV = limit, lease 过期毫秒偏移, leaseID, key TTL 毫秒。
const activityConcurrencyAcquireScript = `local now = redis.call("TIME")
local nowMs = now[1] * 1000 + math.floor(now[2] / 1000)
redis.call("ZREMRANGEBYSCORE", KEYS[1], "-inf", nowMs)
if redis.call("ZSCORE", KEYS[1], ARGV[3]) then
  redis.call("PEXPIRE", KEYS[1], tonumber(ARGV[4]))
  return 1
end
if redis.call("ZCARD", KEYS[1]) >= tonumber(ARGV[1]) then
  return 0
end
redis.call("ZADD", KEYS[1], nowMs + tonumber(ARGV[2]), ARGV[3])
redis.call("PEXPIRE", KEYS[1], tonumber(ARGV[4]))
return 1
`

// activityConcurrencyRenewScript 只对仍存在且未过期的自己续租：
// 已被清理（过期/释放）的租约不会被复活。ARGV = leaseID, lease 过期毫秒偏移, key TTL 毫秒。
const activityConcurrencyRenewScript = `local now = redis.call("TIME")
local nowMs = now[1] * 1000 + math.floor(now[2] / 1000)
local score = redis.call("ZSCORE", KEYS[1], ARGV[1])
if score == false or tonumber(score) <= nowMs then
  return 0
end
redis.call("ZADD", KEYS[1], nowMs + tonumber(ARGV[2]), ARGV[1])
redis.call("PEXPIRE", KEYS[1], tonumber(ARGV[3]))
return 1
`

// activityConcurrencyReleaseScript 幂等归还租约，返回删除的成员数。
const activityConcurrencyReleaseScript = `return redis.call("ZREM", KEYS[1], ARGV[1])`

func (s *redisActivityLimitStore) RateTake(ctx context.Context, key string, limit int) (bool, int64, error) {
	if key == "" {
		return false, 0, errors.New("activity rate limit key is empty")
	}
	if limit <= 0 {
		return false, 0, errors.New("activity rate limit must be positive")
	}
	values, err := s.client.Eval(ctx, activityRateWindowScript, []string{key}, limit, activityRateLimitWindowSeconds).Slice()
	if err != nil {
		return false, 0, err
	}
	if len(values) != 2 {
		return false, 0, fmt.Errorf("unexpected activity rate limit reply length %d", len(values))
	}
	allowed, err := redisReplyInt64(values[0])
	if err != nil {
		return false, 0, err
	}
	ttl, err := redisReplyInt64(values[1])
	if err != nil {
		return false, 0, err
	}
	if ttl < 1 {
		ttl = 1
	}
	return allowed == 1, ttl, nil
}

func (s *redisActivityLimitStore) Acquire(ctx context.Context, key string, limit int, leaseID string) (bool, error) {
	if key == "" || leaseID == "" {
		return false, errors.New("activity concurrency key or lease is empty")
	}
	if limit <= 0 {
		return false, errors.New("activity concurrency limit must be positive")
	}
	result, err := s.client.Eval(ctx, activityConcurrencyAcquireScript, []string{key},
		limit, activityConcurrencyLeaseSeconds*1000, leaseID, activityConcurrencyKeyTTLSeconds*1000).Int()
	if err != nil {
		return false, err
	}
	return result == 1, nil
}

func (s *redisActivityLimitStore) Renew(ctx context.Context, key string, leaseID string) (bool, error) {
	if key == "" || leaseID == "" {
		return false, errors.New("activity concurrency key or lease is empty")
	}
	result, err := s.client.Eval(ctx, activityConcurrencyRenewScript, []string{key},
		leaseID, activityConcurrencyLeaseSeconds*1000, activityConcurrencyKeyTTLSeconds*1000).Int()
	if err != nil {
		return false, err
	}
	return result == 1, nil
}

func (s *redisActivityLimitStore) Release(ctx context.Context, key string, leaseID string) error {
	if key == "" || leaseID == "" {
		return errors.New("activity concurrency key or lease is empty")
	}
	return s.client.Eval(ctx, activityConcurrencyReleaseScript, []string{key}, leaseID).Err()
}

func redisReplyInt64(value interface{}) (int64, error) {
	switch typed := value.(type) {
	case int64:
		return typed, nil
	case string:
		var parsed int64
		_, err := fmt.Sscan(typed, &parsed)
		return parsed, err
	case []byte:
		var parsed int64
		_, err := fmt.Sscan(string(typed), &parsed)
		return parsed, err
	default:
		return 0, fmt.Errorf("unexpected Redis integer reply type %T", value)
	}
}
