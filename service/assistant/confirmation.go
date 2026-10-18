package assistant

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
)

const ConfirmationTTL = 120 * time.Second

var (
	ErrConfirmationNotFound  = errors.New("confirmation not found or expired")
	ErrConfirmationForbidden = errors.New("confirmation belongs to another user")
)

type ConfirmationRequest struct {
	OperationID string
	Method      string
	Path        string
	Risk        string
	Summary     string
	Params      map[string]any
}

type pendingConfirmation struct {
	actorID  int
	profile  string
	expires  time.Time
	decision chan bool
}

type redisPendingConfirmation struct {
	ActorID   int    `json:"actor_id"`
	Profile   string `json:"profile"`
	ExpiresAt int64  `json:"expires_at"`
}

// ConfirmationBroker keeps pending write approvals. The local map is used on
// single-node deployments; when Redis is enabled, the pending metadata and the
// user's decision are also written to Redis so the confirm endpoint may land on
// another node and the waiting stream polls the decision key.
type ConfirmationBroker struct {
	mu      sync.Mutex
	pending map[string]*pendingConfirmation
}

var defaultConfirmationBroker = NewConfirmationBroker()

func NewConfirmationBroker() *ConfirmationBroker {
	return &ConfirmationBroker{pending: make(map[string]*pendingConfirmation)}
}

func DefaultConfirmationBroker() *ConfirmationBroker {
	return defaultConfirmationBroker
}

func newConfirmationID() (string, error) {
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return hex.EncodeToString(raw), nil
}

func redisAvailable() bool {
	return common.RedisEnabled && common.RDB != nil
}

func confirmationPendingKey(id string) string {
	return "assistant:confirmation:pending:" + id
}

func confirmationDecisionKey(id string) string {
	return "assistant:confirmation:decision:" + id
}

// Ask sends a confirmation event and waits for the user's decision.
func (b *ConfirmationBroker) Ask(ctx context.Context, actorID int, profile string, request ConfirmationRequest, emit EmitFunc) (bool, error) {
	id, err := newConfirmationID()
	if err != nil {
		return false, err
	}
	expiresAt := time.Now().Add(ConfirmationTTL)
	pending := &pendingConfirmation{
		actorID:  actorID,
		profile:  profile,
		expires:  expiresAt,
		decision: make(chan bool, 1),
	}
	b.mu.Lock()
	b.pending[id] = pending
	b.mu.Unlock()
	if redisAvailable() {
		payload, marshalErr := common.Marshal(redisPendingConfirmation{ActorID: actorID, Profile: profile, ExpiresAt: expiresAt.Unix()})
		if marshalErr == nil {
			_ = common.RDB.Set(ctx, confirmationPendingKey(id), payload, ConfirmationTTL).Err()
		}
	}
	defer func() {
		b.mu.Lock()
		delete(b.pending, id)
		b.mu.Unlock()
		if redisAvailable() {
			_ = common.RDB.Del(context.Background(), confirmationPendingKey(id)).Err()
			_ = common.RDB.Del(context.Background(), confirmationDecisionKey(id)).Err()
		}
	}()

	if emit != nil {
		if err := emit(Event{
			Type:           "confirmation_required",
			ConfirmationID: id,
			Operation:      request.OperationID,
			Risk:           request.Risk,
			Summary:        request.Summary,
			Params:         request.Params,
			ExpiresAt:      expiresAt.Unix(),
		}); err != nil {
			return false, err
		}
	}

	timer := time.NewTimer(ConfirmationTTL)
	defer timer.Stop()
	var ticker *time.Ticker
	var tickerCh <-chan time.Time
	if redisAvailable() {
		ticker = time.NewTicker(250 * time.Millisecond)
		tickerCh = ticker.C
		defer ticker.Stop()
	}
	for {
		select {
		case approved := <-pending.decision:
			if emit != nil {
				_ = emit(Event{Type: "confirmation_resolved", ConfirmationID: id, Status: confirmationStatus(approved)})
			}
			return approved, nil
		case <-tickerCh:
			decision, ok := readRedisDecision(ctx, id)
			if !ok {
				continue
			}
			if emit != nil {
				_ = emit(Event{Type: "confirmation_resolved", ConfirmationID: id, Status: confirmationStatus(decision)})
			}
			return decision, nil
		case <-ctx.Done():
			_ = emit(Event{Type: "confirmation_resolved", ConfirmationID: id, Status: "expired"})
			return false, ctx.Err()
		case <-timer.C:
			_ = emit(Event{Type: "confirmation_resolved", ConfirmationID: id, Status: "expired"})
			return false, fmt.Errorf("confirmation expired")
		}
	}
}

func readRedisDecision(ctx context.Context, id string) (bool, bool) {
	value, err := common.RDB.Get(ctx, confirmationDecisionKey(id)).Result()
	if err != nil {
		return false, false
	}
	switch value {
	case "1":
		return true, true
	case "0":
		return false, true
	default:
		return false, false
	}
}

func (b *ConfirmationBroker) Resolve(actorID int, profile, id string, approved bool) error {
	b.mu.Lock()
	pending, ok := b.pending[id]
	if ok && time.Now().After(pending.expires) {
		b.mu.Unlock()
		return ErrConfirmationNotFound
	}
	if ok {
		if pending.actorID != actorID || pending.profile != profile {
			b.mu.Unlock()
			return ErrConfirmationForbidden
		}
		select {
		case pending.decision <- approved:
		default:
		}
	}
	b.mu.Unlock()

	if redisAvailable() {
		value, err := common.RDB.Get(context.Background(), confirmationPendingKey(id)).Result()
		if err == nil {
			var metadata redisPendingConfirmation
			if unmarshalErr := common.Unmarshal([]byte(value), &metadata); unmarshalErr == nil {
				if metadata.ActorID != actorID || metadata.Profile != profile {
					return ErrConfirmationForbidden
				}
				if metadata.ExpiresAt < time.Now().Unix() {
					return ErrConfirmationNotFound
				}
				decision := "0"
				if approved {
					decision = "1"
				}
				return common.RDB.Set(context.Background(), confirmationDecisionKey(id), decision, ConfirmationTTL).Err()
			}
		}
	}
	if ok {
		return nil
	}
	return ErrConfirmationNotFound
}

func confirmationStatus(approved bool) string {
	if approved {
		return "approved"
	}
	return "denied"
}
