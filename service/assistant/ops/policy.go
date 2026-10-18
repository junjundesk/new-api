package ops

import (
	"strings"

	"github.com/QuantumNous/new-api/common"
)

// classifyRoute applies the static policy. It is intentionally centralized so
// every newly registered /api route must be reviewed by the catalog test.
func classifyRoute(method, path string) Operation {
	op := Operation{
		ID:          method + " " + path,
		Method:      method,
		Path:        path,
		Domain:      domainFor(path),
		Risk:        riskFor(method, path),
		Summary:     summaryFor(method, path),
		Description: descriptionFor(method, path),
		BodyAllowed: method != "GET" && method != "HEAD" && method != "DELETE",
		PathParams:  pathParams(path),
	}
	op.RedactFields = redactFieldsFor(method, path)

	if reason := exclusionFor(method, path); reason != "" {
		op.ExcludedReason = reason
		return op
	}

	op.BodyFields, op.BodyNotes = bodySpecFor(method, path)

	scope, minRole := scopeFor(method, path)
	op.Scope = scope
	op.MinRole = minRole
	op.Batchable = method == "POST" || method == "PUT" || method == "PATCH" || method == "DELETE"
	if strings.Contains(path, "/user/:id") {
		op.Batchable = true
		op.BatchTarget = "id"
	}
	if method == "POST" && path == "/api/user/:id/bonus/:grant_id/revoke" {
		op.Batchable = false
		op.BatchTarget = ""
	}
	if method == "POST" && path == "/api/user/manage" {
		op.Batchable = true
		op.BatchTarget = "body:id"
	}
	if method == "PUT" && path == "/api/user/" {
		op.Batchable = true
		op.BatchTarget = "body:id"
	}
	return op
}

func exclusionFor(method, path string) string {
	if strings.HasPrefix(path, "/api/assistant") {
		return "assistant-control-plane"
	}
	if strings.HasPrefix(path, "/api/relay") || strings.HasPrefix(path, "/v1") {
		return "data-plane"
	}
	if strings.HasPrefix(path, "/api/setup") || strings.HasPrefix(path, "/api/status/test") {
		return "setup-or-health"
	}
	if strings.HasPrefix(path, "/api/oauth") || strings.HasPrefix(path, "/api/user/oauth") ||
		strings.Contains(path, "/oauth/") || strings.Contains(path, "/passkey") ||
		strings.Contains(path, "/2fa") ||
		path == "/api/user/token" || path == "/api/user/reset" || path == "/api/verification" {
		return "authentication-or-security-flow"
	}
	if strings.HasPrefix(path, "/api/log/token") || strings.HasPrefix(path, "/api/usage") {
		return "token-auth-endpoint"
	}
	if strings.Contains(path, "/webhook") || strings.HasPrefix(path, "/api/stripe") ||
		strings.HasPrefix(path, "/api/creem") || strings.HasPrefix(path, "/api/waffo") ||
		strings.Contains(path, "/epay/notify") || strings.Contains(path, "/epay/return") ||
		strings.Contains(path, "/pay") || strings.Contains(path, "/amount") {
		return "payment-external-flow"
	}
	if path == "/api/token/:id/key" || path == "/api/channel/:id/key" || path == "/api/user/token" {
		return "secret-view"
	}
	if strings.Contains(path, "/ollama/pull/stream") || strings.Contains(path, "/performance/logs") ||
		strings.Contains(path, "/deployments/:id/logs") {
		return "stream-or-log-file"
	}
	if path == "/api/notice" || path == "/api/about" || path == "/api/status" ||
		path == "/api/uptime/status" || path == "/api/ratio_config" || path == "/api/home_page_content" ||
		path == "/api/user-agreement" || path == "/api/privacy-policy" {
		return "public-read"
	}
	if strings.HasPrefix(path, "/api/user/auth") || strings.HasPrefix(path, "/api/user/login") ||
		strings.HasPrefix(path, "/api/user/register") || strings.HasPrefix(path, "/api/user/epay") ||
		strings.HasPrefix(path, "/api/subscription/epay") {
		return "authentication-or-payment"
	}
	return ""
}

func scopeFor(method, path string) (string, int) {
	rootPrefixes := []string{
		"/api/option", "/api/performance", "/api/ratio_sync", "/api/custom-oauth-provider",
		"/api/system-task", "/api/system-info",
	}
	for _, prefix := range rootPrefixes {
		if strings.HasPrefix(path, prefix) {
			return ScopeRoot, common.RoleRootUser
		}
	}
	if isUserOperation(method, path) {
		return ScopeUser, common.RoleCommonUser
	}
	return ScopeAdmin, common.RoleAdminUser
}

func isUserOperation(method, path string) bool {
	if path == "/api/user/topup" {
		return method == "POST"
	}
	if path == "/api/user/topup/info" || path == "/api/user/topup/self" {
		return true
	}
	userPrefixes := []string{
		"/api/user/self", "/api/user/sessions", "/api/user/aff",
		"/api/user/referral", "/api/user/setting", "/api/user/checkin",
		"/api/user/groups", "/api/user/models", "/api/token",
		"/api/log/self", "/api/data/self", "/api/data/user-breakdown/self",
		"/api/subscription/plans", "/api/subscription/self",
		"/api/activity/offers", "/api/activity/grants", "/api/activity/verify",
		"/api/activity/redeem",
	}
	for _, prefix := range userPrefixes {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}
	if path == "/api/user/self/bonus" || path == "/api/user/self/channel_chains" {
		return true
	}
	if path == "/api/models" && method == "GET" {
		return true
	}
	return false
}

func riskFor(method, path string) Risk {
	if method == "GET" || method == "HEAD" {
		return RiskRead
	}
	lower := strings.ToLower(path)
	for _, token := range []string{"delete", "batch", "reset", "revoke", "invalidate", "clear", "cleanup", "disable", "remove"} {
		if strings.Contains(lower, token) {
			return RiskDestructive
		}
	}
	return RiskWrite
}

func domainFor(path string) string {
	trimmed := strings.TrimPrefix(path, "/api/")
	parts := strings.Split(trimmed, "/")
	if len(parts) == 0 || parts[0] == "" {
		return "system"
	}
	return parts[0]
}

func summaryFor(method, path string) string {
	switch method + " " + path {
	case "GET /api/user/:id/bonus":
		return "List user bonus grants"
	case "POST /api/user/:id/bonus":
		return "Grant bonus to one user"
	case "POST /api/user/bonus/grant_all":
		return "Grant bonus to all users"
	case "POST /api/user/:id/bonus/:grant_id/revoke":
		return "Revoke a user bonus grant"
	}
	parts := strings.Split(strings.Trim(path, "/"), "/")
	resource := parts[len(parts)-1]
	if strings.HasPrefix(resource, ":") && len(parts) >= 2 {
		resource = parts[len(parts)-2]
	}
	action := map[string]string{
		"GET": "Read", "POST": "Create/Operate", "PUT": "Update", "PATCH": "Update", "DELETE": "Delete",
	}[method]
	if action == "" {
		action = method
	}
	return action + " " + resource
}

func descriptionFor(method, path string) string {
	switch method + " " + path {
	case "GET /api/user/:id/bonus":
		return "List every bonus grant for one user (active, used, expired, revoked) plus the active balance summary."
	case "POST /api/user/:id/bonus":
		return "Grant bonus quota to one user. amount is quota units, not currency; expire_input controls the expiry."
	case "POST /api/user/bonus/grant_all":
		return "Grant bonus quota to every non-deleted user. The same expiry applies to each grant."
	case "POST /api/user/:id/bonus/:grant_id/revoke":
		return "Revoke one active bonus grant. The unconsumed amount leaves the user's bonus balance; amounts already consumed are not refunded."
	}
	return "Dashboard API operation " + method + " " + path + ". Parameters are validated by the existing endpoint."
}

// bodySpecFor documents request body fields for operations whose JSON body is
// not self-explanatory. Field names match the controller request structs.
func bodySpecFor(method, path string) ([]BodyField, string) {
	switch method + " " + path {
	case "POST /api/user/:id/bonus":
		return []BodyField{
			{Name: "amount", Type: "integer", Required: true, Description: "Bonus quota to grant, in internal quota units (not currency). Must be greater than 0.", Example: 500000},
			{Name: "expire_input", Type: "string", Required: false, Description: "Expiry duration. A bare number means hours (\"1\" = 1 hour); units may be combined (\"1天\" = 24 hours, \"1天2小时\" = 26 hours). Empty or omitted means the grant never expires.", Example: "24"},
			{Name: "remark", Type: "string", Required: false, Description: "Optional admin note stored on the grant.", Example: "活动赠金"},
			{Name: "send_email", Type: "boolean", Required: false, Description: "Send the arrival notification email. Defaults to true.", Example: true},
		}, "The target user id comes from the path, not the body. amount is quota units, not dollars. expire_input empty means permanent."
	case "POST /api/user/bonus/grant_all":
		return []BodyField{
			{Name: "amount", Type: "integer", Required: true, Description: "Bonus quota granted to each user, in internal quota units (not currency). Must be greater than 0.", Example: 500000},
			{Name: "expire_input", Type: "string", Required: false, Description: "Expiry duration applied to every grant. A bare number means hours (\"1\" = 1 hour); units may be combined. Empty or omitted means the grants never expire.", Example: "24"},
			{Name: "remark", Type: "string", Required: false, Description: "Optional admin note stored on each grant.", Example: "活动赠金"},
			{Name: "send_email", Type: "boolean", Required: false, Description: "Send the arrival notification email. Defaults to true.", Example: true},
		}, "This endpoint always targets every non-deleted user; there is no user_id field."
	case "POST /api/user/:id/bonus/:grant_id/revoke":
		return []BodyField{
			{Name: "remark", Type: "string", Required: false, Description: "Optional reason stored on the revoked grant.", Example: "expiry was entered incorrectly"},
		}, "Only an active grant can be revoked. The unconsumed amount is removed from the user's bonus balance; already consumed amounts are not refunded. Use GET /api/user/:id/bonus to find grant_id."
	}
	return nil, ""
}

func pathParams(path string) []string {
	parts := strings.Split(path, "/")
	params := make([]string, 0)
	for _, part := range parts {
		if strings.HasPrefix(part, ":") {
			params = append(params, strings.TrimPrefix(part, ":"))
		}
	}
	return params
}

func redactFieldsFor(method, path string) []string {
	fields := []string{"password", "original_password", "access_token", "refresh_token", "api_key", "secret", "private_key", "key"}
	if strings.Contains(path, "/token") {
		fields = append(fields, "key")
	}
	return fields
}
