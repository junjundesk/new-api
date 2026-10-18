package controller

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/service/assistant"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfirmAssistantOperationResolvesPendingApproval(t *testing.T) {
	events := make(chan assistant.Event, 4)
	done := make(chan bool, 1)
	go func() {
		approved, err := assistant.DefaultConfirmationBroker().Ask(context.Background(), 7, "user", assistant.ConfirmationRequest{
			OperationID: "POST /api/token/probe", Risk: "write", Summary: "Create key",
		}, func(event assistant.Event) error {
			events <- event
			return nil
		})
		require.NoError(t, err)
		done <- approved
	}()

	var confirmationID string
	require.Eventually(t, func() bool {
		select {
		case event := <-events:
			if event.Type == "confirmation_required" {
				confirmationID = event.ConfirmationID
				return true
			}
			return false
		default:
			return false
		}
	}, time.Second, 10*time.Millisecond)
	require.NotEmpty(t, confirmationID)

	body, err := common.Marshal(map[string]any{"confirmation_id": confirmationID, "approve": true})
	require.NoError(t, err)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/assistant/confirm", strings.NewReader(string(body)))
	ctx.Request.Header.Set("Content-Type", "application/json")
	ctx.Set("id", 7)
	ConfirmAssistantOperation(ctx)
	require.Equal(t, http.StatusOK, recorder.Code)
	payload := map[string]any{}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &payload))
	assert.Equal(t, true, payload["success"])
	assert.True(t, <-done)
}
