package ops

import (
	"context"
)

// Actor is the authenticated identity for one assistant tool call. It is
// attached to the private Go request context and cannot be set by external
// HTTP callers.
type Actor struct {
	UserID         int
	Username       string
	Role           int
	Profile        string
	SessionID      string
	AuthVersion    int64
	SessionVersion int64
	IP             string
	UserAgent      string
	RequestID      string
}

type actorContextKey struct{}

func WithActor(ctx context.Context, actor Actor) context.Context {
	return context.WithValue(ctx, actorContextKey{}, actor)
}

func ActorFromContext(ctx context.Context) (Actor, bool) {
	actor, ok := ctx.Value(actorContextKey{}).(Actor)
	return actor, ok
}

func IsInternal(ctx context.Context) bool {
	_, ok := ActorFromContext(ctx)
	return ok
}

type Risk string

const (
	RiskRead        Risk = "read"
	RiskWrite       Risk = "write"
	RiskDestructive Risk = "destructive"
)

const (
	ScopeUser  = "user"
	ScopeAdmin = "admin"
	ScopeRoot  = "root"
)

// Operation describes one callable dashboard API endpoint.
type Operation struct {
	ID             string   `json:"id"`
	Method         string   `json:"method"`
	Path           string   `json:"path"`
	Domain         string   `json:"domain"`
	Scope          string   `json:"scope"`
	MinRole        int      `json:"min_role"`
	Risk           Risk     `json:"risk"`
	Summary        string   `json:"summary"`
	Description    string   `json:"description,omitempty"`
	PathParams     []string `json:"path_params,omitempty"`
	BodyAllowed    bool     `json:"body_allowed"`
	Batchable      bool     `json:"batchable,omitempty"`
	BatchTarget    string   `json:"batch_target,omitempty"`
	RedactFields   []string `json:"-"`
	ExcludedReason string   `json:"-"`
}

// Result is the normalized outcome of an in-process route invocation.
type Result struct {
	StatusCode int    `json:"status_code"`
	Success    bool   `json:"success"`
	Message    string `json:"message,omitempty"`
	Data       any    `json:"data,omitempty"`
	Raw        string `json:"raw,omitempty"`
	Truncated  bool   `json:"truncated,omitempty"`
}

// Invocation is one concrete operation request.
type Invocation struct {
	Operation  Operation
	PathParams map[string]any
	Query      map[string]any
	Body       any
}
