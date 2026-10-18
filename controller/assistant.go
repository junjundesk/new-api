package controller

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service/assistant"
	"github.com/QuantumNous/new-api/setting/assistant_setting"

	"github.com/gin-gonic/gin"
)

const (
	assistantMaxMessageChars     = 100000
	assistantHeartbeatInterval   = 15 * time.Second
	assistantMaxRunDuration      = 15 * time.Minute
	assistantChatBodyLimit       = 8 << 20
	assistantTestBodyLimit       = 16 << 10
	assistantMaxResponseTextSize = 500
)

type assistantChatRequest struct {
	Message string                  `json:"message"`
	History []assistant.ChatMessage `json:"history"`
}

type assistantTestRequest struct {
	Profile         string `json:"profile"`
	Source          string `json:"source"`
	Protocol        string `json:"protocol"`
	BaseURL         string `json:"base_url"`
	APIKey          string `json:"api_key"`
	Model           string `json:"model"`
	ChannelID       int    `json:"channel_id"`
	ChannelModel    string `json:"channel_model"`
	ContextWindow   int    `json:"context_window"`
	MaxOutputTokens int    `json:"max_output_tokens"`
	ReasoningEffort string `json:"reasoning_effort"`
}

type assistantChannelItem struct {
	ID       int      `json:"id"`
	Name     string   `json:"name"`
	Type     int      `json:"type"`
	TypeName string   `json:"type_name"`
	Status   int      `json:"status"`
	BaseURL  string   `json:"base_url"`
	Models   []string `json:"models"`
}

func validAssistantProfile(name string) bool {
	return name == assistant_setting.ProfileUser || name == assistant_setting.ProfileAdmin
}

// GetAssistantConfig returns the user-facing runtime switches for the user
// profile. It never exposes the upstream base URL, API key, channel, or prompt.
func GetAssistantConfig(c *gin.Context) {
	getAssistantRuntimeConfig(c, assistant_setting.ProfileUser)
}

// GetAdminAssistantConfig returns the runtime switches for the admin profile.
// It is root-only and still never returns the upstream key.
func GetAdminAssistantConfig(c *gin.Context) {
	getAssistantRuntimeConfig(c, assistant_setting.ProfileAdmin)
}

func getAssistantRuntimeConfig(c *gin.Context, profileName string) {
	contextWindow := assistant_setting.GetProfile(profileName).ContextWindow
	if contextWindow <= 0 {
		contextWindow = 1000000
	}
	common.ApiSuccess(c, gin.H{
		"enabled":           assistant_setting.IsAvailable(profileName),
		"model":             assistant_setting.ModelName(profileName),
		"can_create_tokens": true,
		"max_message_chars": assistantMaxMessageChars,
		"context_window":    contextWindow,
	})
}

// AssistantChat is the free user-facing assistant endpoint.
func AssistantChat(c *gin.Context) {
	assistantChatStream(c, assistant_setting.ProfileUser)
}

// AdminAssistantChat is the separate administrator assistant endpoint. It
// uses the admin profile (its own API/key/channel binding); root-only routes
// remain protected by RootAuth when tools invoke them.
func AdminAssistantChat(c *gin.Context) {
	assistantChatStream(c, assistant_setting.ProfileAdmin)
}

func assistantChatStream(c *gin.Context, profileName string) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, assistantChatBodyLimit)
	request := assistantChatRequest{}
	if err := common.DecodeJson(c.Request.Body, &request); err != nil {
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}
	message := strings.TrimSpace(request.Message)
	if message == "" || utf8.RuneCountInString(message) > assistantMaxMessageChars {
		common.ApiErrorI18n(c, i18n.MsgAssistantMessageTooLong)
		return
	}
	history := make([]assistant.ChatMessage, 0, len(request.History))
	for _, entry := range request.History {
		role := strings.TrimSpace(entry.Role)
		if role != "user" && role != "assistant" {
			common.ApiErrorI18n(c, i18n.MsgInvalidParams)
			return
		}
		content := strings.TrimSpace(entry.Content)
		if content == "" {
			continue
		}
		history = append(history, assistant.ChatMessage{Role: role, Content: content})
	}

	c.Writer.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	c.Writer.Header().Set("Cache-Control", "no-cache, no-transform")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("X-Accel-Buffering", "no")
	c.Status(http.StatusOK)

	writer := newSSEWriter(c.Writer)
	stopHeartbeat := startSSEHeartbeat(writer, assistantHeartbeatInterval)
	defer stopHeartbeat()

	writeError := func(code, key string, args ...map[string]any) {
		_ = writer.WriteEvent(assistant.Event{
			Type:    "error",
			Code:    code,
			Message: common.TranslateMessage(c, key, args...),
		})
	}

	if !assistant_setting.IsAvailable(profileName) {
		writeError("assistant_disabled", i18n.MsgAssistantDisabled)
		return
	}
	profile := assistant_setting.GetProfile(profileName)
	cfg, err := assistant.ResolveConfig(profile)
	if err != nil {
		logger.LogError(c.Request.Context(), fmt.Sprintf("assistant profile resolution failed: profile=%s err=%v", profileName, err))
		if profile.Source == assistant_setting.SourceChannel {
			writeError("assistant_channel_unavailable", i18n.MsgAssistantChannelUnavailable)
		} else {
			writeError("assistant_not_configured", i18n.MsgAssistantNotConfigured)
		}
		return
	}
	userID := c.GetInt("id")
	if userID <= 0 {
		writeError("assistant_disabled", i18n.MsgAssistantDisabled)
		return
	}
	cfg.ActorID = userID
	cfg.ActorUsername = c.GetString("username")
	cfg.ActorRole = c.GetInt("role")
	cfg.Profile = profileName
	cfg.ActorSessionID = c.GetString("session_id")
	cfg.AuthVersion = c.GetInt64("auth_version")
	cfg.SessionVersion = c.GetInt64("session_version")
	cfg.ActorIP = c.ClientIP()
	cfg.ActorUserAgent = c.Request.UserAgent()

	if profileName == assistant_setting.ProfileUser {
		limit := assistant_setting.Get().UserDailyTokenLimit
		exceeded, retryAfter, quotaErr := assistant.CheckDailyTokenQuota(userID, limit)
		if quotaErr != nil {
			logger.LogError(c.Request.Context(), "assistant daily quota check failed: "+quotaErr.Error())
			writeError("assistant_quota_error", i18n.MsgAssistantUpstreamError)
			return
		}
		if exceeded {
			_ = writer.WriteEvent(assistant.Event{
				Type:       "error",
				Code:       "assistant_daily_token_limit",
				Message:    common.TranslateMessage(c, i18n.MsgAssistantDailyTokenLimit, map[string]any{"Limit": limit}),
				RetryAfter: retryAfter,
			})
			return
		}
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), assistantMaxRunDuration)
	defer cancel()

	if !assistant.AcquireUser(userID) {
		writeError("assistant_busy", i18n.MsgAssistantBusy)
		return
	}
	defer assistant.ReleaseUser(userID)
	if !assistant.AcquireGlobal(ctx) {
		writeError("assistant_busy", i18n.MsgAssistantBusy)
		return
	}
	defer assistant.ReleaseGlobal()

	client := assistant.NewHTTPClient(cfg)
	executor := assistant.NewRuntimeToolExecutor(cfg, writer.WriteEvent)
	audit := func(action string, params map[string]any) {
		if profileName == assistant_setting.ProfileAdmin {
			recordManageAudit(c, action, params)
			return
		}
		recordUserSecurityAudit(c, userID, action, params)
	}
	orchestrator := assistant.NewOrchestrator(cfg, client, executor, writer.WriteEvent, audit)
	runUsage, err := orchestrator.Run(ctx, userID, history, message)
	if profileName == assistant_setting.ProfileUser {
		if recordErr := assistant.RecordTokenUsage(userID, runUsage); recordErr != nil {
			logger.LogError(c.Request.Context(), "assistant usage record failed: "+recordErr.Error())
		}
	}
	if err != nil {
		handleAssistantRunError(c, writer, err)
		return
	}
	doneEvent := assistant.Event{
		Type:          "done",
		Usage:         &runUsage,
		ContextTokens: orchestrator.ContextTokens(),
		ContextWindow: cfg.ContextWindow,
	}
	if err := writer.WriteEvent(doneEvent); err != nil {
		logger.LogWarn(c.Request.Context(), "assistant done event write failed: "+err.Error())
		return
	}
	logger.LogInfo(c.Request.Context(), fmt.Sprintf("assistant chat completed profile=%s user_id=%d model=%s", profileName, userID, cfg.Model))
}

type assistantConfirmRequest struct {
	ConfirmationID string `json:"confirmation_id"`
	Approve        bool   `json:"approve"`
}

// ConfirmAssistantOperation resolves one pending user assistant write.
func ConfirmAssistantOperation(c *gin.Context) {
	confirmAssistantOperation(c, assistant_setting.ProfileUser)
}

// ConfirmAdminAssistantOperation resolves one pending admin assistant write.
func ConfirmAdminAssistantOperation(c *gin.Context) {
	confirmAssistantOperation(c, assistant_setting.ProfileAdmin)
}

func confirmAssistantOperation(c *gin.Context, profileName string) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, assistantTestBodyLimit)
	request := assistantConfirmRequest{}
	if err := common.DecodeJson(c.Request.Body, &request); err != nil {
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}
	id := strings.TrimSpace(request.ConfirmationID)
	if id == "" {
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}
	if err := assistant.DefaultConfirmationBroker().Resolve(c.GetInt("id"), profileName, id, request.Approve); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, nil)
}

func handleAssistantRunError(c *gin.Context, writer *sseWriter, err error) {
	if errors.Is(err, context.Canceled) {
		return
	}
	var upstreamError *assistant.UpstreamError
	if errors.As(err, &upstreamError) {
		logger.LogError(c.Request.Context(), fmt.Sprintf("assistant upstream error: %v", upstreamError))
		_ = writer.WriteEvent(assistant.Event{Type: "error", Code: "assistant_upstream_error", Message: common.TranslateMessage(c, i18n.MsgAssistantUpstreamError)})
		return
	}
	var contextError *assistant.ContextExceededError
	if errors.As(err, &contextError) {
		_ = writer.WriteEvent(assistant.Event{
			Type:    "error",
			Code:    "assistant_context_exceeded",
			Message: common.TranslateMessage(c, i18n.MsgAssistantContextExceeded),
		})
		return
	}
	var incompleteError *assistant.IncompleteError
	if errors.As(err, &incompleteError) {
		reason := incompleteError.Reason
		if reason == "" {
			reason = "incomplete"
		}
		_ = writer.WriteEvent(assistant.Event{
			Type:    "error",
			Code:    "assistant_incomplete",
			Message: common.TranslateMessage(c, i18n.MsgAssistantIncomplete, map[string]any{"Reason": reason}),
		})
		return
	}
	if errors.Is(err, context.DeadlineExceeded) {
		_ = writer.WriteEvent(assistant.Event{Type: "error", Code: "assistant_upstream_timeout", Message: common.TranslateMessage(c, i18n.MsgAssistantUpstreamTimeout)})
		return
	}
	logger.LogError(c.Request.Context(), "assistant chat failed: "+err.Error())
	_ = writer.WriteEvent(assistant.Event{Type: "error", Code: "assistant_upstream_error", Message: common.TranslateMessage(c, i18n.MsgAssistantUpstreamError)})
}

// TestAssistantConnection verifies one profile before or after saving. Values
// from the request override the saved profile, so the admin can test unsaved
// form data. The API key is never returned.
func TestAssistantConnection(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, assistantTestBodyLimit)
	request := assistantTestRequest{}
	if c.Request.ContentLength != 0 {
		if err := common.DecodeJson(c.Request.Body, &request); err != nil {
			common.ApiErrorI18n(c, i18n.MsgInvalidParams)
			return
		}
	}
	profileName := strings.TrimSpace(request.Profile)
	if profileName == "" {
		profileName = assistant_setting.ProfileUser
	}
	if !validAssistantProfile(profileName) {
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}
	profile := assistant_setting.GetProfile(profileName)
	if source := strings.ToLower(strings.TrimSpace(request.Source)); source != "" {
		profile.Source = source
	}
	if request.BaseURL != "" {
		profile.BaseURL = strings.TrimSpace(request.BaseURL)
	}
	if request.APIKey != "" {
		profile.APIKey = strings.TrimSpace(request.APIKey)
	}
	if request.Model != "" {
		profile.Model = strings.TrimSpace(request.Model)
	}
	if request.ChannelID > 0 {
		profile.ChannelID = request.ChannelID
	}
	if request.ChannelModel != "" {
		profile.ChannelModel = strings.TrimSpace(request.ChannelModel)
	}
	if request.Protocol != "" {
		profile.Protocol = strings.ToLower(strings.TrimSpace(request.Protocol))
	}
	if request.ContextWindow > 0 {
		profile.ContextWindow = request.ContextWindow
	}
	if request.ReasoningEffort != "" {
		profile.ReasoningEffort = request.ReasoningEffort
	}
	if request.MaxOutputTokens > 0 {
		profile.MaxOutputTokens = request.MaxOutputTokens
	}
	cfg, err := assistant.ResolveConfig(profile)
	if err != nil {
		common.ApiErrorI18n(c, i18n.MsgAssistantConnectionFailed, map[string]any{"Reason": err.Error()})
		return
	}
	cfg.MaxOutputTokens = 16
	cfg.RequestTimeout = 15 * time.Second
	ctx, cancel := context.WithTimeout(c.Request.Context(), 15*time.Second)
	defer cancel()
	text, latency, err := assistant.TestConnection(ctx, cfg)
	if err != nil {
		message := strings.ReplaceAll(err.Error(), cfg.APIKey, "[REDACTED]")
		if len(message) > assistantMaxResponseTextSize {
			message = message[:assistantMaxResponseTextSize]
		}
		logger.LogError(c.Request.Context(), "assistant test connection failed: "+message)
		common.ApiErrorI18n(c, i18n.MsgAssistantConnectionFailed, map[string]any{"Reason": message})
		return
	}
	text = strings.ReplaceAll(text, cfg.APIKey, "[REDACTED]")
	if len(text) > assistantMaxResponseTextSize {
		text = text[:assistantMaxResponseTextSize]
	}
	common.ApiSuccess(c, gin.H{
		"latency_ms": latency.Milliseconds(),
		"model":      cfg.Model,
		"text":       text,
	})
}

// ListAssistantModels fetches the upstream model list for an unsaved or saved
// admin profile. It never returns the upstream key.
func ListAssistantModels(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, assistantTestBodyLimit)
	request := assistantTestRequest{}
	if c.Request.ContentLength != 0 {
		if err := common.DecodeJson(c.Request.Body, &request); err != nil {
			common.ApiErrorI18n(c, i18n.MsgInvalidParams)
			return
		}
	}
	profileName := strings.TrimSpace(request.Profile)
	if profileName == "" {
		profileName = assistant_setting.ProfileUser
	}
	if !validAssistantProfile(profileName) {
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}
	profile := assistant_setting.GetProfile(profileName)
	if source := strings.ToLower(strings.TrimSpace(request.Source)); source != "" {
		profile.Source = source
	}
	if request.BaseURL != "" {
		profile.BaseURL = strings.TrimSpace(request.BaseURL)
	}
	if request.APIKey != "" {
		profile.APIKey = strings.TrimSpace(request.APIKey)
	}
	if request.ChannelID > 0 {
		profile.ChannelID = request.ChannelID
	}
	if request.ChannelModel != "" {
		profile.ChannelModel = strings.TrimSpace(request.ChannelModel)
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 15*time.Second)
	defer cancel()
	models, err := assistant.ListModels(ctx, profile)
	if err != nil {
		message := err.Error()
		if len(message) > assistantMaxResponseTextSize {
			message = message[:assistantMaxResponseTextSize]
		}
		common.ApiErrorI18n(c, i18n.MsgAssistantConnectionFailed, map[string]any{"Reason": message})
		return
	}
	common.ApiSuccess(c, gin.H{"models": models})
}

// ClearAssistantKey removes one profile's stored API key override and disables
// that profile. Channel-backed profiles fall back to the channel key after the
// override is cleared, but the profile stays disabled until re-enabled.
func ClearAssistantKey(c *gin.Context) {
	profileName := strings.TrimSpace(c.Query("profile"))
	if profileName == "" {
		profileName = assistant_setting.ProfileUser
	}
	if !validAssistantProfile(profileName) {
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}
	apiKeyKey := assistant_setting.OptionKeyUserAPIKey
	enabledKey := assistant_setting.OptionKeyUserEnabled
	if profileName == assistant_setting.ProfileAdmin {
		apiKeyKey = assistant_setting.OptionKeyAdminAPIKey
		enabledKey = assistant_setting.OptionKeyAdminEnabled
	}
	if err := model.UpdateOptionsBulk(map[string]string{
		apiKeyKey:  "",
		enabledKey: "false",
	}); err != nil {
		common.ApiError(c, err)
		return
	}
	recordManageAudit(c, "assistant.key_clear", map[string]interface{}{"profile": profileName})
	common.ApiSuccess(c, nil)
}

// ListAssistantChannels returns every channel (including disabled channels)
// with its model list. Channel keys are never included.
func ListAssistantChannels(c *gin.Context) {
	channels, err := model.GetAllChannels(0, -1, false, false)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	items := make([]assistantChannelItem, 0, len(channels))
	for _, channel := range channels {
		if channel == nil {
			continue
		}
		models := make([]string, 0)
		for _, modelName := range channel.GetModels() {
			modelName = strings.TrimSpace(modelName)
			if modelName != "" {
				models = append(models, modelName)
			}
		}
		typeName := constant.ChannelTypeNames[channel.Type]
		if typeName == "" {
			typeName = "Unknown"
		}
		items = append(items, assistantChannelItem{
			ID:       channel.Id,
			Name:     channel.Name,
			Type:     channel.Type,
			TypeName: typeName,
			Status:   channel.Status,
			BaseURL:  channel.GetBaseURL(),
			Models:   models,
		})
	}
	common.ApiSuccess(c, items)
}

type sseWriter struct {
	mu     sync.Mutex
	writer gin.ResponseWriter
	closed bool
}

func newSSEWriter(writer gin.ResponseWriter) *sseWriter {
	return &sseWriter{writer: writer}
}

func (w *sseWriter) WriteEvent(event assistant.Event) error {
	payload, err := common.Marshal(event)
	if err != nil {
		return err
	}
	return w.write("data: " + string(payload) + "\n\n")
}

func (w *sseWriter) write(data string) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closed {
		return errors.New("assistant stream is closed")
	}
	if _, err := w.writer.Write([]byte(data)); err != nil {
		w.closed = true
		return err
	}
	w.writer.Flush()
	return nil
}

func (w *sseWriter) close() {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.closed = true
}

func startSSEHeartbeat(writer *sseWriter, interval time.Duration) func() {
	stop := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				if err := writer.write(": ping\n\n"); err != nil {
					return
				}
			case <-stop:
				return
			}
		}
	}()
	return func() {
		close(stop)
		<-done
		writer.close()
	}
}
