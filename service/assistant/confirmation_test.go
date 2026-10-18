package assistant

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfirmationBrokerApproveAndDeny(t *testing.T) {
	broker := NewConfirmationBroker()
	events := make([]Event, 0)
	done := make(chan bool, 1)
	go func() {
		approved, err := broker.Ask(context.Background(), 7, "user", ConfirmationRequest{
			OperationID: "POST /api/token", Risk: "write", Summary: "Create key",
		}, func(event Event) error {
			events = append(events, event)
			return nil
		})
		require.NoError(t, err)
		done <- approved
	}()

	var confirmationID string
	require.Eventually(t, func() bool {
		for _, event := range events {
			if event.Type == "confirmation_required" {
				confirmationID = event.ConfirmationID
				return true
			}
		}
		return false
	}, time.Second, 10*time.Millisecond)
	require.NotEmpty(t, confirmationID)

	assert.ErrorIs(t, broker.Resolve(8, "user", confirmationID, true), ErrConfirmationForbidden)
	assert.ErrorIs(t, broker.Resolve(7, "admin", confirmationID, true), ErrConfirmationForbidden)
	require.NoError(t, broker.Resolve(7, "user", confirmationID, true))
	assert.True(t, <-done)

	require.Eventually(t, func() bool {
		for _, event := range events {
			if event.Type == "confirmation_resolved" && event.Status == "approved" {
				return true
			}
		}
		return false
	}, time.Second, 10*time.Millisecond)
}
