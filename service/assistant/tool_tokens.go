package assistant

import (
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
)

type createKeyArguments struct {
	Name          string `json:"name"`
	ExpiresInDays *int   `json:"expires_in_days"`
}

type setKeyStatusArguments struct {
	ID      int   `json:"id"`
	Enabled *bool `json:"enabled"`
}

func (e *DefaultToolExecutor) createAPIKey(targetUserID int, rawArguments string, audit AuditFunc) ToolResult {
	args := createKeyArguments{}
	if err := decodeArguments(rawArguments, &args); err != nil {
		return toolErrorf("invalid arguments: %v", err)
	}
	name, err := service.NormalizeUserTokenName(args.Name)
	if err != nil {
		return toolError(err)
	}
	expiredTime := int64(-1)
	if args.ExpiresInDays != nil {
		days := *args.ExpiresInDays
		if days < 1 || days > 3650 {
			return toolErrorf("expires_in_days must be between 1 and 3650")
		}
		expiredTime = time.Now().Add(time.Duration(days) * 24 * time.Hour).Unix()
	}
	token := &model.Token{
		Name:           name,
		UnlimitedQuota: true,
		ExpiredTime:    expiredTime,
		Group:          "",
	}
	if err := service.InsertUserToken(targetUserID, token); err != nil {
		return toolError(err)
	}
	username, _ := model.GetUsernameById(targetUserID, false)
	if audit != nil {
		audit("token.create", map[string]any{
			"id":              token.Id,
			"name":            token.Name,
			"target_user_id":  targetUserID,
			"target_username": username,
			"via":             "assistant",
		})
	}
	status := token.Status
	if status == 0 {
		status = common.TokenStatusEnabled
	}
	return ToolResult{OK: true, Data: map[string]any{
		"id":              token.Id,
		"user_id":         targetUserID,
		"username":        username,
		"name":            token.Name,
		"masked_key":      "sk-" + token.GetMaskedKey(),
		"status":          status,
		"status_text":     tokenStatusText(status),
		"unlimited_quota": true,
		"remain_quota":    0,
		"remain_display":  DisplayQuota(0),
		"expired_time":    token.ExpiredTime,
		"never_expires":   token.ExpiredTime == -1,
		"group":           token.Group,
		"hint":            "The full key is available from the key card in this chat or from the API Keys page.",
	}}
}

func (e *DefaultToolExecutor) setAPIKeyStatus(actorUserID int, rawArguments string, audit AuditFunc) ToolResult {
	args := setKeyStatusArguments{}
	if err := decodeArguments(rawArguments, &args); err != nil {
		return toolErrorf("invalid arguments: %v", err)
	}
	if args.ID <= 0 {
		return toolErrorf("id must be a positive integer")
	}
	if args.Enabled == nil {
		return toolErrorf("enabled is required")
	}
	var token *model.Token
	var err error
	if e.adminScope {
		token, err = model.GetTokenById(args.ID)
	} else {
		token, err = model.GetTokenByIds(args.ID, actorUserID)
	}
	if err != nil {
		return toolErrorf("API key not found")
	}
	nextStatus := common.TokenStatusDisabled
	if *args.Enabled {
		now := time.Now().Unix()
		if token.ExpiredTime > 0 && token.ExpiredTime <= now {
			return toolErrorf("cannot enable an expired API key")
		}
		if !token.UnlimitedQuota && token.RemainQuota <= 0 {
			return toolErrorf("cannot enable an exhausted API key")
		}
		nextStatus = common.TokenStatusEnabled
	}
	token.Status = nextStatus
	if err := token.SelectUpdate(); err != nil {
		return toolError(err)
	}
	action := "token.disable"
	if *args.Enabled {
		action = "token.enable"
	}
	username, _ := model.GetUsernameById(token.UserId, false)
	if audit != nil {
		audit(action, map[string]any{
			"id":              token.Id,
			"name":            token.Name,
			"target_user_id":  token.UserId,
			"target_username": username,
			"via":             "assistant",
		})
	}
	return ToolResult{OK: true, Data: map[string]any{
		"id":          token.Id,
		"user_id":     token.UserId,
		"username":    username,
		"name":        token.Name,
		"masked_key":  "sk-" + token.GetMaskedKey(),
		"status":      token.Status,
		"status_text": tokenStatusText(token.Status),
	}}
}
