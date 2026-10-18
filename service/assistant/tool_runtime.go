package assistant

import (
	"context"
	"fmt"

	assistantops "github.com/QuantumNous/new-api/service/assistant/ops"
)

// NewRuntimeToolExecutor builds the production executor for one chat run.
func NewRuntimeToolExecutor(cfg Config, emit EmitFunc) *DefaultToolExecutor {
	return &DefaultToolExecutor{
		adminScope:     cfg.AdminScope,
		actorID:        cfg.ActorID,
		actorUsername:  cfg.ActorUsername,
		actorRole:      cfg.ActorRole,
		profile:        cfg.Profile,
		actorSessionID: cfg.ActorSessionID,
		authVersion:    cfg.AuthVersion,
		sessionVersion: cfg.SessionVersion,
		actorIP:        cfg.ActorIP,
		actorUserAgent: cfg.ActorUserAgent,
		confirmWrites:  cfg.ConfirmWrites,
		adminDBTools:   cfg.AdminDBTools,
		emit:           emit,
		confirmation:   DefaultConfirmationBroker(),
	}
}

func (e *DefaultToolExecutor) actor() assistantops.Actor {
	profile := e.profile
	if profile == "" {
		if e.adminScope {
			profile = "admin"
		} else {
			profile = "user"
		}
	}
	return assistantops.Actor{
		UserID:         e.actorID,
		Username:       e.actorUsername,
		Role:           e.actorRole,
		Profile:        profile,
		SessionID:      e.actorSessionID,
		AuthVersion:    e.authVersion,
		SessionVersion: e.sessionVersion,
		IP:             e.actorIP,
		UserAgent:      e.actorUserAgent,
	}
}

// askConfirmation applies the write-confirmation policy to one operation.
func (e *DefaultToolExecutor) askConfirmation(ctx context.Context, request ConfirmationRequest) error {
	if !e.confirmWrites {
		return nil
	}
	if e.confirmation == nil || e.actorID <= 0 {
		return fmt.Errorf("confirmation is unavailable for this request")
	}
	approved, err := e.confirmation.Ask(ctx, e.actorID, e.actor().Profile, request, e.emit)
	if err != nil {
		return err
	}
	if !approved {
		return fmt.Errorf("operation denied by the user")
	}
	return nil
}
