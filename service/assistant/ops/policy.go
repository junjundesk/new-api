package ops

import (
	"strings"

	"github.com/QuantumNous/new-api/common"
)

// classifyRoute applies the static policy. It is intentionally centralized so
// every newly registered /api route must be reviewed by the catalog test.
func classifyRoute(method, path string) Operation {
	id := method + " " + path
	op := Operation{
		ID:          id,
		Method:      method,
		Path:        path,
		Domain:      domainFor(path),
		Risk:        riskFor(method, path),
		Summary:     defaultSummary(method, path),
		Description: "Dashboard API operation " + id + ". Parameters are validated by the existing endpoint.",
		BodyAllowed: method != "GET" && method != "HEAD" && method != "DELETE",
		PathParams:  pathParams(path),
	}

	if reason := exclusionFor(method, path); reason != "" {
		op.ExcludedReason = reason
		return op
	}

	if doc, ok := operationDocs[id]; ok {
		if doc.Summary != "" {
			op.Summary = doc.Summary
		}
		if doc.Description != "" {
			op.Description = doc.Description
		}
		op.BodyFields = doc.Body
		op.BodyNotes = doc.Notes
		op.QueryFields = doc.Query
	}

	op.Scope, op.MinRole = scopeFor(method, path)
	op.BatchTarget = batchTargetFor(method, path)
	op.Batchable = op.BatchTarget != ""
	return op
}

// batchTargetFor names where batch_operation writes each target user id: a
// path parameter name, or "body:<field>". Only operations that address exactly
// one user per call can be fanned out over users.
func batchTargetFor(method, path string) string {
	switch method + " " + path {
	case "POST /api/user/:id/bonus/:grant_id/revoke":
		// grant_id differs per user, so one shared parameter set cannot apply.
		return ""
	case "POST /api/user/manage", "PUT /api/user/":
		return "body:id"
	}
	if strings.Contains(path, "/user/:id") || strings.Contains(path, "/users/:id") {
		return "id"
	}
	return ""
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
		path == "/api/user/token" || path == "/api/user/reset" || path == "/api/verification" ||
		path == "/api/reset_password" || path == "/api/verify" {
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
	if path == "/api/token/:id/key" || path == "/api/token/batch/keys" || path == "/api/channel/:id/key" {
		return "secret-view"
	}
	if strings.Contains(path, "/ollama/pull/stream") || strings.Contains(path, "/performance/logs") ||
		strings.Contains(path, "/deployments/:id/logs") {
		return "stream-or-log-file"
	}
	if path == "/api/log/search" || path == "/api/log/self/search" {
		return "deprecated"
	}
	if method == "DELETE" && path == "/api/user/self" {
		// Deleting the signed-in account ends the session the assistant runs
		// in and cannot be undone from chat; it stays a deliberate UI action.
		return "account-deletion"
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

// isUserOperation lists the routes guarded by UserAuth (or public-with-login)
// that only ever act on the caller's own account.
func isUserOperation(method, path string) bool {
	switch path {
	case "/api/user/topup":
		return method == "POST"
	case "/api/models":
		return method == "GET"
	case "/api/user/topup/info", "/api/user/topup/self", "/api/user/aff_transfer",
		"/api/task/self", "/api/mj/self", "/api/data/flow/self",
		"/api/pricing", "/api/rankings":
		return true
	}
	userPrefixes := []string{
		"/api/user/self", "/api/user/sessions", "/api/user/aff",
		"/api/user/referral", "/api/user/setting", "/api/user/checkin",
		"/api/user/groups", "/api/user/models", "/api/user/activity/",
		"/api/token", "/api/log/self", "/api/data/self", "/api/data/user-breakdown/self",
		"/api/subscription/plans", "/api/subscription/self", "/api/perf-metrics",
	}
	for _, prefix := range userPrefixes {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}
	return false
}

// GET routes that are not reads: they call upstream providers (spending
// channel quota) or rewrite stored state.
var sideEffectGets = map[string]bool{
	"/api/channel/test":               true,
	"/api/channel/test/:id":           true,
	"/api/channel/update_balance":     true,
	"/api/channel/update_balance/:id": true,
}

// POST routes that only compute or preview and store nothing.
var readOnlyPosts = map[string]bool{
	"/api/deployments/price-estimation": true,
	"/api/user/activity/verify":         true,
}

// Non-DELETE routes whose effect is deletion, irreversible, or site-wide.
var destructiveWrites = map[string]bool{
	"POST /api/channel/batch":                  true, // deletes the listed channels
	"POST /api/token/batch":                    true, // deletes the listed API keys
	"POST /api/user/manage":                    true, // can delete, demote, or override quota
	"POST /api/user/bonus/grant_all":           true, // touches every user
	"PUT /api/option/":                         true, // site-wide configuration
	"POST /api/option/rest_model_ratio":        true, // resets model ratios
	"POST /api/system-task/legacy-log-reclaim": true,
}

func riskFor(method, path string) Risk {
	if method == "GET" || method == "HEAD" {
		if sideEffectGets[path] {
			return RiskWrite
		}
		return RiskRead
	}
	if method == "DELETE" || destructiveWrites[method+" "+path] {
		return RiskDestructive
	}
	if method == "POST" && readOnlyPosts[path] {
		return RiskRead
	}
	lower := strings.ToLower(path)
	for _, token := range []string{"delete", "reset", "revoke", "invalidate", "clear", "cleanup", "remove"} {
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

// defaultSummary derives a readable label from the route shape for operations
// without a hand-written entry in operationDocs.
func defaultSummary(method, path string) string {
	segments := strings.Split(strings.Trim(strings.TrimPrefix(path, "/api"), "/"), "/")
	words := make([]string, 0, len(segments))
	for _, segment := range segments {
		if segment != "" && !strings.HasPrefix(segment, ":") {
			words = append(words, strings.NewReplacer("_", " ", "-", " ").Replace(segment))
		}
	}
	subject := strings.Join(words, " ")
	last := segments[len(segments)-1]
	collection := strings.HasSuffix(path, "/")
	verb := method
	switch method {
	case "GET":
		verb = "Get"
		if collection {
			verb = "List"
		}
	case "POST":
		verb = "Run"
		if collection {
			verb = "Create"
		}
	case "PUT", "PATCH":
		verb = "Update"
	case "DELETE":
		verb = "Delete"
	}
	if strings.HasPrefix(last, ":") {
		return verb + " " + subject + " by " + strings.TrimPrefix(last, ":")
	}
	return verb + " " + subject
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

// Field-name fragments that mark a value as a credential. Matching is by
// substring so a newly added secret field is hidden by default.
var secretFieldFragments = []string{"password", "secret", "access_token", "refresh_token", "gotify_token", "key"}

// Names that contain a secret fragment but hold plain metadata.
var plainFieldNames = map[string]bool{
	"multi_key_mode": true, "key_mode": true, "is_multi_key": true, "multi_key_size": true,
	"multi_key_polling_index": true, "multi_key_status_list": true, "multi_key_disabled_reason": true,
	"multi_key_disabled_time": true, "key_index": true, "batch_add_set_key_prefix_2_name": true,
	"description_key": true, "benefit_key": true, "label_key": true, "active_key": true,
	"ref_key": true, "public_key": true, "keyword": true, "masked_key": true,
}

// IsSecretField reports whether a request/response field of this operation
// must be hidden from the model, the transcript, and confirmation cards.
func (op Operation) IsSecretField(name string) bool {
	lower := strings.ToLower(name)
	if plainFieldNames[lower] {
		return false
	}
	// A bare "token" is a credential value (for example the key search filter).
	if lower == "token" {
		return true
	}
	// Site options and authorization roles use "key" as the entry name.
	if lower == "key" && (op.Domain == "option" || op.Domain == "authz") {
		return false
	}
	for _, fragment := range secretFieldFragments {
		if strings.Contains(lower, fragment) {
			return true
		}
	}
	return false
}

// Operations whose successful response payload is itself a secret.
var secretResultNotes = map[string]string{
	"POST /api/redemption/": "Redemption codes were created. Code values are not shown in chat; copy them from the Redemption Codes page.",
}

// SecretResultNote returns the text that replaces the response payload of an
// operation whose result must not reach the model, or "" for normal results.
func (op Operation) SecretResultNote() string {
	return secretResultNotes[op.ID]
}

// IsSecretOptionKey mirrors the option list endpoint, which never returns
// options whose name marks them as credentials.
func IsSecretOptionKey(key string) bool {
	lower := strings.ToLower(key)
	for _, suffix := range []string{"token", "secret", "key", "password"} {
		if strings.HasSuffix(lower, suffix) {
			return true
		}
	}
	return false
}
