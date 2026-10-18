package assistant

import (
	"context"
	"sync"
)

const maxConcurrentAssistantRequests = 8

var (
	activeUsers sync.Map
	globalSlots = make(chan struct{}, maxConcurrentAssistantRequests)
)

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
