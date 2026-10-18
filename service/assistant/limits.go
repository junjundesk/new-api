package assistant

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
)

const maxConcurrentAssistantRequests = 8

var (
	dailyLimitMutex sync.Mutex
	dailyLimitStore = make(map[int]dailyLimitEntry)
	activeUsers     sync.Map
	globalSlots     = make(chan struct{}, maxConcurrentAssistantRequests)
)

type dailyLimitEntry struct {
	Day   string
	Count int
}

// AcquireDailyLimit increments the current user's daily message counter.
// A limit <= 0 disables the check. Returns false with retryAfter seconds when
// the budget is exhausted.
func AcquireDailyLimit(userID int, limit int) (bool, int64) {
	if limit <= 0 {
		return true, 0
	}
	now := time.Now()
	day := now.Format("2006-01-02")
	next := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, now.Location())
	retryAfter := int64(next.Sub(now).Seconds())
	if retryAfter < 1 {
		retryAfter = 1
	}

	if common.RedisEnabled && common.RDB != nil {
		key := fmt.Sprintf("assistant:daily:%d:%s", userID, day)
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		count, err := common.RDB.Incr(ctx, key).Result()
		if err == nil {
			if count == 1 {
				_ = common.RDB.Expire(ctx, key, next.Sub(now)).Err()
			}
			if int(count) > limit {
				return false, retryAfter
			}
			return true, 0
		}
		common.SysError("assistant daily limit redis error: " + err.Error())
	}

	dailyLimitMutex.Lock()
	defer dailyLimitMutex.Unlock()
	entry := dailyLimitStore[userID]
	if entry.Day != day {
		entry = dailyLimitEntry{Day: day}
	}
	entry.Count++
	dailyLimitStore[userID] = entry
	if entry.Count > limit {
		return false, retryAfter
	}
	return true, 0
}

// AcquireUser rejects a second concurrent assistant request for the same user.
func AcquireUser(userID int) bool {
	_, loaded := activeUsers.LoadOrStore(userID, struct{}{})
	return !loaded
}

func ReleaseUser(userID int) {
	activeUsers.Delete(userID)
}

// AcquireGlobal takes one of the process-wide upstream concurrency slots.
func AcquireGlobal(ctx context.Context) bool {
	select {
	case globalSlots <- struct{}{}:
		return true
	case <-ctx.Done():
		return false
	default:
		return false
	}
}

func ReleaseGlobal() {
	select {
	case <-globalSlots:
	default:
	}
}
