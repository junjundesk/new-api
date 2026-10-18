package ops

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInvokeRunsRouteInProcessWithPrivateActor(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.GET("/api/user/self", func(c *gin.Context) {
		actor, ok := ActorFromContext(c.Request.Context())
		require.True(t, ok)
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"data": gin.H{
				"id":   actor.UserID,
				"role": actor.Role,
			},
		})
	})
	Initialize(engine)
	op, ok := Find("GET /api/user/self")
	require.True(t, ok)

	var externalCalled bool
	engine.GET("/api/external", func(c *gin.Context) {
		externalCalled = true
		c.JSON(http.StatusOK, gin.H{"success": true})
	})
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/external", nil))
	require.True(t, externalCalled)

	result := Invoke(context.Background(), Actor{UserID: 7, Role: 1}, op, nil, nil, nil)
	require.True(t, result.Success, result.Message)
	data, ok := result.Data.(map[string]any)
	require.True(t, ok)
	assert.EqualValues(t, 7, data["id"])
	assert.EqualValues(t, 1, data["role"])
}

func TestInvokeRejectsInvalidPathParameter(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.GET("/api/token/:id", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"success": true})
	})
	Initialize(engine)
	op, ok := Find("GET /api/token/:id")
	require.True(t, ok)
	result := Invoke(context.Background(), Actor{UserID: 1, Role: 1}, op, map[string]any{"id": "../admin"}, nil, nil)
	require.False(t, result.Success)
	assert.Contains(t, result.Message, "invalid characters")
}
