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

	scope, minRole := scopeFor(method, path)
	op.Scope = scope
	op.MinRole = minRole
	op.Batchable = method == "POST" || method == "PUT" || method == "PATCH" || method == "DELETE"
	if strings.Contains(path, "/user/:id") {
		op.Batchable = true
		op.BatchTarget = "id"
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
	return "Dashboard API operation " + method + " " + path + ". Parameters are validated by the existing endpoint."
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
