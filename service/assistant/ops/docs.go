package ops

import "sort"

// operationDoc is the hand-written documentation of one catalog operation.
// Field names and semantics mirror the controller request structs and the
// query parameters the handler reads; keep them in sync when an endpoint
// changes.
type operationDoc struct {
	Summary     string
	Description string
	Query       []BodyField
	Body        []BodyField
	Notes       string
}

var (
	pageField         = BodyField{Name: "p", Type: "integer", Description: "Page number, starting at 1. Defaults to 1."}
	pageSizeField     = BodyField{Name: "page_size", Type: "integer", Description: "Rows per page, 1-100. Defaults to the site page size (usually 10). Keep it small; large pages may exceed the tool result limit."}
	startField        = BodyField{Name: "start_timestamp", Type: "integer", Description: "Range start as a unix timestamp in seconds. 0 or omitted means no lower bound.", Example: 1730000000}
	endField          = BodyField{Name: "end_timestamp", Type: "integer", Description: "Range end as a unix timestamp in seconds. 0 or omitted means no upper bound.", Example: 1730086400}
	sortByField       = BodyField{Name: "sort_by", Type: "string", Description: "Sort column. Unknown values fall back to id."}
	sortOrderField    = BodyField{Name: "sort_order", Type: "string", Description: "asc or desc. Defaults to desc."}
	logTypeField      = BodyField{Name: "type", Type: "integer", Description: "Log type: 0 all, 1 top-up, 2 consume, 3 manage, 4 system, 5 error, 6 refund, 7 login."}
	excludeUsersField = BodyField{Name: "exclude_user_ids", Type: "string", Description: "Comma separated user ids to leave out.", Example: "1,2"}
)

const pagedResultNote = "The result is a page object: items, total, page, page_size. Read total to decide whether to request the next page with p."

var operationDocs = map[string]operationDoc{
	// ---- users (admin) ----
	"GET /api/user/": {
		Summary:     "List users",
		Description: "List users page by page, including soft-deleted ones. " + pagedResultNote,
		Query: []BodyField{pageField, pageSizeField,
			{Name: "sort_by", Type: "string", Description: "One of id, username, quota, group, created_at, last_login_at."},
			sortOrderField},
	},
	"GET /api/user/search": {
		Summary:     "Search users",
		Description: "Search users by keyword and filters. " + pagedResultNote,
		Query: []BodyField{
			{Name: "keyword", Type: "string", Description: "Matches username, email, or display name; a number also matches the user id."},
			{Name: "group", Type: "string", Description: "Exact user group."},
			{Name: "role", Type: "integer", Description: "1 common user, 10 admin, 100 root."},
			{Name: "status", Type: "integer", Description: "1 enabled, 2 disabled, -1 soft-deleted. Omit to include every user."},
			pageField, pageSizeField, sortByField, sortOrderField},
	},
	"GET /api/user/:id": {
		Summary:     "Get one user",
		Description: "Get one user's profile, quota, group, and bindings. quota and used_quota are internal quota units.",
	},
	"POST /api/user/": {
		Summary:     "Create a user",
		Description: "Create a user account. The new role must be lower than the caller's role.",
		Body: []BodyField{
			{Name: "username", Type: "string", Required: true, Description: "Login name."},
			{Name: "password", Type: "string", Required: true, Description: "Initial password, 8-20 characters."},
			{Name: "display_name", Type: "string", Description: "Defaults to the username."},
			{Name: "role", Type: "integer", Description: "1 common user (default), 10 admin."},
		},
	},
	"PUT /api/user/": {
		Summary:     "Edit a user's profile",
		Description: "Overwrite a user's username, display name, group, and remark. It does not change quota, role, or status; use POST /api/user/manage for those.",
		Body: []BodyField{
			{Name: "id", Type: "integer", Required: true, Description: "Target user id."},
			{Name: "username", Type: "string", Required: true, Description: "Must be sent even when unchanged."},
			{Name: "display_name", Type: "string", Description: "Written as given; omitting it clears the display name."},
			{Name: "group", Type: "string", Description: "User group. Written as given; omitting it clears the group."},
			{Name: "remark", Type: "string", Description: "Admin note. Written as given; omitting it clears the remark."},
			{Name: "password", Type: "string", Description: "New password. Omit to keep the current one."},
		},
		Notes: "This is a full overwrite of the four profile fields. Read the user with GET /api/user/:id first and send back the values that should stay.",
	},
	"POST /api/user/manage": {
		Summary:     "Manage a user (status, role, quota)",
		Description: "Enable, disable, soft-delete, promote, demote, or change the quota of one user.",
		Body: []BodyField{
			{Name: "id", Type: "integer", Required: true, Description: "Target user id."},
			{Name: "action", Type: "string", Required: true, Description: "One of disable, enable, delete, promote, demote, add_quota."},
			{Name: "mode", Type: "string", Description: "Only for add_quota: add, subtract, or override."},
			{Name: "value", Type: "integer", Description: "Only for add_quota: amount in internal quota units (not currency). For override it is the new total quota.", Example: 500000},
		},
		Notes: "Quota is in internal quota units; convert a currency amount with quota_per_unit from any *_display value before calling. delete is a soft delete. promote requires root.",
	},
	"DELETE /api/user/:id": {
		Summary:     "Permanently delete a user",
		Description: "Hard-delete a user. This cannot be undone. For a reversible removal use POST /api/user/manage with action=delete.",
	},
	"DELETE /api/user/:id/bindings/:binding_type": {
		Summary:     "Clear one login binding of a user",
		Description: "Remove a third-party binding from a user. binding_type is one of email, github, discord, oidc, wechat, telegram, linuxdo.",
	},
	"DELETE /api/user/:id/reset_passkey": {
		Summary:     "Remove a user's passkey",
		Description: "Delete the passkey registered by one user so they must enroll again.",
	},
	"POST /api/user/email/send": {
		Summary:     "Send an email to users",
		Description: "Send one email to selected users or to every user.",
		Body: []BodyField{
			{Name: "user_ids", Type: "array of integer", Description: "Recipients. Required unless all_users is true."},
			{Name: "all_users", Type: "boolean", Description: "Send to every user."},
			{Name: "subject", Type: "string", Required: true, Description: "Email subject."},
			{Name: "content", Type: "string", Required: true, Description: "Email body."},
			{Name: "include_unsubscribed", Type: "boolean", Description: "Also send to users who opted out of operational email. Defaults to false."},
		},
	},
	"POST /api/user/inviter/bind": {
		Summary:     "Set the inviter of users",
		Description: "Repoint the inviter of up to 100 users. It never grants invitation rewards.",
		Body: []BodyField{
			{Name: "user_ids", Type: "array of integer", Required: true, Description: "Users whose inviter changes."},
			{Name: "inviter_id", Type: "integer", Required: true, Description: "The new inviter's user id."},
		},
	},
	"GET /api/user/inviter/list": {
		Summary: "List inviters",
		Query:   []BodyField{{Name: "keyword", Type: "string", Description: "Optional inviter search keyword."}},
	},
	"GET /api/user/:id/invitees": {Summary: "List the users invited by one user"},
	"GET /api/user/topup": {
		Summary:     "List top-up orders of all users",
		Description: "List payment orders. " + pagedResultNote,
		Query: []BodyField{pageField, pageSizeField,
			{Name: "keyword", Type: "string", Description: "Optional order search keyword (for example the trade number)."}},
	},
	"POST /api/user/topup/complete": {
		Summary:     "Manually complete a top-up order",
		Description: "Mark a pending payment order as paid and credit the user. Use only after confirming the payment arrived.",
		Body:        []BodyField{{Name: "trade_no", Type: "string", Required: true, Description: "Order trade number from GET /api/user/topup."}},
	},

	// ---- bonus ----
	"GET /api/user/:id/bonus": {
		Summary:     "List user bonus grants",
		Description: "List every bonus grant for one user (active, used, expired, revoked) plus the active balance summary.",
	},
	"POST /api/user/:id/bonus": {
		Summary:     "Grant bonus to one user",
		Description: "Grant bonus quota to one user. amount is quota units, not currency; expire_input controls the expiry.",
		Body: []BodyField{
			{Name: "amount", Type: "integer", Required: true, Description: "Bonus quota to grant, in internal quota units (not currency). Must be greater than 0.", Example: 500000},
			{Name: "expire_input", Type: "string", Required: false, Description: "Expiry duration. A bare number means hours (\"1\" = 1 hour); units may be combined (\"1天\" = 24 hours, \"1天2小时\" = 26 hours). Empty or omitted means the grant never expires.", Example: "24"},
			{Name: "remark", Type: "string", Required: false, Description: "Optional admin note stored on the grant.", Example: "活动赠金"},
			{Name: "send_email", Type: "boolean", Required: false, Description: "Send the arrival notification email. Defaults to true.", Example: true},
		},
		Notes: "The target user id comes from the path, not the body; all_users is rejected here. amount is quota units, not dollars. expire_input empty means permanent.",
	},
	"POST /api/user/bonus/grant_all": {
		Summary:     "Grant bonus to all users",
		Description: "Grant bonus quota to every non-deleted user. The same expiry applies to each grant.",
		Body: []BodyField{
			{Name: "amount", Type: "integer", Required: true, Description: "Bonus quota granted to each user, in internal quota units (not currency). Must be greater than 0.", Example: 500000},
			{Name: "expire_input", Type: "string", Required: false, Description: "Expiry duration applied to every grant. A bare number means hours (\"1\" = 1 hour); units may be combined. Empty or omitted means the grants never expire.", Example: "24"},
			{Name: "remark", Type: "string", Required: false, Description: "Optional admin note stored on each grant.", Example: "活动赠金"},
			{Name: "send_email", Type: "boolean", Required: false, Description: "Send the arrival notification email. Defaults to true.", Example: true},
		},
		Notes: "This endpoint always targets every non-deleted user; user_id is rejected here. Use POST /api/user/:id/bonus for one user.",
	},
	"POST /api/user/:id/bonus/:grant_id/revoke": {
		Summary:     "Revoke a user bonus grant",
		Description: "Revoke one active bonus grant. The unconsumed amount leaves the user's bonus balance; amounts already consumed are not refunded.",
		Body: []BodyField{
			{Name: "remark", Type: "string", Required: false, Description: "Optional reason stored on the revoked grant.", Example: "expiry was entered incorrectly"},
		},
		Notes: "Only an active grant can be revoked. The unconsumed amount is removed from the user's bonus balance; already consumed amounts are not refunded. Use GET /api/user/:id/bonus to find grant_id.",
	},

	// ---- logs and usage data ----
	"GET /api/log/": {
		Summary:     "List logs of all users",
		Description: "List request, top-up, management, and error logs across all users. " + pagedResultNote,
		Query: []BodyField{pageField, pageSizeField, logTypeField, startField, endField,
			{Name: "username", Type: "string", Description: "Exact username."},
			{Name: "token_name", Type: "string", Description: "Exact API key name."},
			{Name: "model_name", Type: "string", Description: "Model name filter."},
			{Name: "channel", Type: "integer", Description: "Channel id."},
			{Name: "group", Type: "string", Description: "Group used by the request."},
			{Name: "request_id", Type: "string", Description: "Exact request id."},
			{Name: "upstream_request_id", Type: "string", Description: "Exact upstream request id."}},
	},
	"GET /api/log/stat": {
		Summary:     "Sum consumption for a log filter",
		Description: "Return total quota plus current RPM and TPM for the logs matching the filter. quota is internal quota units.",
		Query: []BodyField{logTypeField, startField, endField,
			{Name: "username", Type: "string", Description: "Exact username."},
			{Name: "token_name", Type: "string", Description: "Exact API key name."},
			{Name: "model_name", Type: "string", Description: "Model name filter."},
			{Name: "channel", Type: "integer", Description: "Channel id."},
			{Name: "group", Type: "string", Description: "Group used by the request."}},
	},
	"GET /api/log/self": {
		Summary:     "List my logs",
		Description: "List the caller's own logs. " + pagedResultNote,
		Query: []BodyField{pageField, pageSizeField, logTypeField, startField, endField,
			{Name: "token_name", Type: "string", Description: "Exact API key name."},
			{Name: "model_name", Type: "string", Description: "Model name filter."},
			{Name: "group", Type: "string", Description: "Group used by the request."},
			{Name: "request_id", Type: "string", Description: "Exact request id."}},
	},
	"GET /api/log/self/stat": {
		Summary: "Sum my consumption for a log filter",
		Query: []BodyField{logTypeField, startField, endField,
			{Name: "token_name", Type: "string", Description: "Exact API key name."},
			{Name: "model_name", Type: "string", Description: "Model name filter."},
			{Name: "group", Type: "string", Description: "Group used by the request."}},
	},
	"GET /api/data/": {
		Summary:     "Usage statistics by model over time (all users)",
		Description: "Hourly usage buckets per model for all users or one username.",
		Query: []BodyField{startField, endField,
			{Name: "username", Type: "string", Description: "Limit to one username."}, excludeUsersField},
	},
	"GET /api/data/users": {
		Summary: "Usage statistics grouped by user",
		Query:   []BodyField{startField, endField},
	},
	"GET /api/data/user-breakdown": {
		Summary: "Per-user usage breakdown",
		Query: []BodyField{startField, endField,
			{Name: "username", Type: "string", Description: "Limit to one username."}, excludeUsersField},
	},
	"GET /api/data/self": {
		Summary:     "My usage statistics by model over time",
		Description: "Hourly usage buckets per model for the caller. The range may not exceed 30 days.",
		Query:       []BodyField{startField, endField},
	},
	"GET /api/task/": {
		Summary:     "List async tasks of all users",
		Description: "List video/music generation tasks. " + pagedResultNote,
		Query: []BodyField{pageField, pageSizeField, startField, endField,
			{Name: "platform", Type: "string", Description: "Task platform."},
			{Name: "task_id", Type: "string", Description: "Exact task id."},
			{Name: "status", Type: "string", Description: "Task status."},
			{Name: "action", Type: "string", Description: "Task action."},
			{Name: "channel_id", Type: "string", Description: "Channel id."}},
	},

	// ---- channels ----
	"GET /api/channel/": {
		Summary:     "List channels",
		Description: "List upstream channels without their keys. " + pagedResultNote + " The result also includes type_counts.",
		Query: []BodyField{pageField, pageSizeField,
			{Name: "group", Type: "string", Description: "Channel group filter."},
			{Name: "status", Type: "string", Description: "enabled or disabled (disabled covers manual and automatic). Omit for all."},
			{Name: "type", Type: "integer", Description: "Channel type id."},
			{Name: "tag_mode", Type: "boolean", Description: "Group rows by tag."},
			{Name: "id_sort", Type: "boolean", Description: "Sort by id instead of priority."},
			{Name: "sort_by", Type: "string", Description: "One of id, name, priority, balance, response_time, test_time."},
			sortOrderField},
		Notes: "Channel rows are large (model lists, mappings). Use a small page_size.",
	},
	"GET /api/channel/search": {
		Summary: "Search channels",
		Query: []BodyField{
			{Name: "keyword", Type: "string", Description: "Matches channel id, name, key, or base URL."},
			{Name: "group", Type: "string", Description: "Channel group filter."},
			{Name: "model", Type: "string", Description: "Model name the channel must serve."},
			{Name: "status", Type: "string", Description: "enabled or disabled. Omit for all."},
			{Name: "type", Type: "integer", Description: "Channel type id."},
			pageField, pageSizeField},
	},
	"GET /api/channel/:id": {Summary: "Get one channel", Description: "Get one channel's configuration. The key is never returned."},
	"GET /api/channel/test": {
		Summary:     "Test every channel",
		Description: "Start a background test of all channels. Each test sends a real request upstream, which costs channel quota, and failing channels may be auto-disabled.",
	},
	"GET /api/channel/test/:id": {
		Summary:     "Test one channel",
		Description: "Send one real test request through a channel and report latency or the upstream error. It consumes a small amount of upstream quota.",
		Query: []BodyField{
			{Name: "model", Type: "string", Description: "Model to test. Defaults to the channel's test model."},
			{Name: "endpoint_type", Type: "string", Description: "Optional endpoint type to test."},
			{Name: "stream", Type: "boolean", Description: "Test with streaming."}},
	},
	"GET /api/channel/update_balance": {
		Summary:     "Refresh the balance of every channel",
		Description: "Query each upstream for its balance and store the result.",
	},
	"GET /api/channel/update_balance/:id": {
		Summary:     "Refresh one channel's balance",
		Description: "Query the upstream balance of one channel and store the result.",
	},
	"POST /api/channel/": {
		Summary:     "Create a channel",
		Description: "Create one or more upstream channels.",
		Body: []BodyField{
			{Name: "mode", Type: "string", Required: true, Description: "single (one channel), batch (one channel per key line), or multi_to_single (one multi-key channel)."},
			{Name: "channel", Type: "object", Required: true, Description: "Channel fields: type (integer), name, key, base_url, models (comma separated), group (comma separated), priority, weight, tag, model_mapping, remark."},
		},
		Notes: "The channel key is a credential: ask the administrator to create the channel on the Channels page instead of pasting keys into chat.",
	},
	"PUT /api/channel/": {
		Summary:     "Edit a channel",
		Description: "Update the given fields of one channel. Fields that are omitted keep their value.",
		Body: []BodyField{
			{Name: "id", Type: "integer", Required: true, Description: "Channel id."},
			{Name: "name", Type: "string", Description: "Channel name."},
			{Name: "models", Type: "string", Description: "Comma separated model list (replaces the whole list)."},
			{Name: "group", Type: "string", Description: "Comma separated group list (replaces the whole list)."},
			{Name: "priority", Type: "integer", Description: "Higher is tried first."},
			{Name: "weight", Type: "integer", Description: "Load-balancing weight within the same priority."},
			{Name: "base_url", Type: "string", Description: "Upstream base URL."},
			{Name: "tag", Type: "string", Description: "Channel tag."},
			{Name: "model_mapping", Type: "string", Description: "JSON object string mapping requested to upstream model names."},
			{Name: "remark", Type: "string", Description: "Admin note."},
		},
		Notes: "status is rejected here; use POST /api/channel/:id/status to enable or disable a channel.",
	},
	"POST /api/channel/:id/status": {
		Summary:     "Enable or disable one channel",
		Description: "Set the status of one channel. This is reversible.",
		Body:        []BodyField{{Name: "status", Type: "integer", Required: true, Description: "1 to enable, 2 to disable."}},
	},
	"POST /api/channel/status/batch": {
		Summary:     "Enable or disable several channels",
		Description: "Set the status of the listed channels. This is reversible.",
		Body: []BodyField{
			{Name: "ids", Type: "array of integer", Required: true, Description: "Channel ids."},
			{Name: "status", Type: "integer", Required: true, Description: "1 to enable, 2 to disable."},
		},
	},
	"POST /api/channel/batch": {
		Summary:     "Delete several channels",
		Description: "Permanently delete the listed channels.",
		Body:        []BodyField{{Name: "ids", Type: "array of integer", Required: true, Description: "Channel ids to delete."}},
	},
	"DELETE /api/channel/:id":      {Summary: "Delete one channel", Description: "Permanently delete one channel."},
	"DELETE /api/channel/disabled": {Summary: "Delete every disabled channel", Description: "Permanently delete all channels that are currently disabled."},
	"POST /api/channel/batch/tag": {
		Summary: "Set the tag of several channels",
		Body: []BodyField{
			{Name: "ids", Type: "array of integer", Required: true, Description: "Channel ids."},
			{Name: "tag", Type: "string", Description: "New tag. Omit or null to clear the tag."},
		},
	},
	"POST /api/channel/tag/disabled": {
		Summary: "Disable every channel with a tag",
		Body:    []BodyField{{Name: "tag", Type: "string", Required: true, Description: "Channel tag."}},
	},
	"POST /api/channel/tag/enabled": {
		Summary: "Enable every channel with a tag",
		Body:    []BodyField{{Name: "tag", Type: "string", Required: true, Description: "Channel tag."}},
	},
	"POST /api/channel/rename_group": {
		Summary:     "Rename a group across channels",
		Description: "Replace one group name with another in every channel's group list.",
		Body: []BodyField{
			{Name: "old_group", Type: "string", Required: true, Description: "Existing group name."},
			{Name: "new_group", Type: "string", Required: true, Description: "Replacement group name."},
		},
	},
	"POST /api/channel/multi_key/manage": {
		Summary:     "Manage the keys of a multi-key channel",
		Description: "Inspect, enable, disable, or delete individual keys of a multi-key channel. Keys are addressed by index and never returned.",
		Body: []BodyField{
			{Name: "channel_id", Type: "integer", Required: true, Description: "Multi-key channel id."},
			{Name: "action", Type: "string", Required: true, Description: "One of get_key_status, disable_key, enable_key, enable_all_keys, disable_all_keys, delete_key, delete_disabled_keys."},
			{Name: "key_index", Type: "integer", Description: "Key position for disable_key, enable_key, delete_key."},
			{Name: "page", Type: "integer", Description: "Page for get_key_status."},
			{Name: "page_size", Type: "integer", Description: "Page size for get_key_status."},
			{Name: "status", Type: "integer", Description: "get_key_status filter: 1 enabled, 2 manually disabled, 3 auto disabled."},
		},
		Notes: "delete_key and delete_disabled_keys permanently remove keys.",
	},

	// ---- API keys (own account) ----
	"GET /api/token/": {
		Summary:     "List my API keys",
		Description: "List the caller's API keys with masked values. " + pagedResultNote,
		Query:       []BodyField{pageField, pageSizeField},
	},
	"GET /api/token/search": {
		Summary: "Search my API keys",
		Query: []BodyField{
			{Name: "keyword", Type: "string", Description: "Matches the key name."},
			pageField, pageSizeField},
	},
	"POST /api/token/": {
		Summary:     "Create an API key for my account",
		Description: "Create an API key owned by the caller. The full key is not returned.",
		Body: []BodyField{
			{Name: "name", Type: "string", Required: true, Description: "Key name, at most 50 characters."},
			{Name: "expired_time", Type: "integer", Description: "Expiry as a unix timestamp in seconds; -1 means never."},
			{Name: "unlimited_quota", Type: "boolean", Description: "True for no per-key quota cap."},
			{Name: "remain_quota", Type: "integer", Description: "Per-key quota cap in internal quota units when unlimited_quota is false."},
			{Name: "group", Type: "string", Description: "Group the key uses. Empty means the account group."},
			{Name: "model_limits_enabled", Type: "boolean", Description: "Restrict the key to model_limits."},
			{Name: "model_limits", Type: "string", Description: "Comma separated models the key may call."},
			{Name: "allow_ips", Type: "string", Description: "Newline separated IP allow list. Empty allows all."},
		},
	},
	"PUT /api/token/": {
		Summary:     "Edit one of my API keys",
		Description: "Edit an API key owned by the caller. Without a query flag every editable field is overwritten.",
		Query: []BodyField{
			{Name: "status_only", Type: "string", Description: "Set to 1 to change only status; all other body fields are ignored."},
			{Name: "group_only", Type: "string", Description: "Set to 1 to change only group."},
		},
		Body: []BodyField{
			{Name: "id", Type: "integer", Required: true, Description: "Key id."},
			{Name: "status", Type: "integer", Description: "With status_only: 1 enabled, 2 disabled."},
			{Name: "name", Type: "string", Description: "Key name."},
			{Name: "expired_time", Type: "integer", Description: "Unix timestamp in seconds; -1 means never."},
			{Name: "unlimited_quota", Type: "boolean", Description: "True for no per-key quota cap."},
			{Name: "remain_quota", Type: "integer", Description: "Per-key quota cap in internal quota units."},
			{Name: "group", Type: "string", Description: "Group the key uses."},
			{Name: "model_limits_enabled", Type: "boolean", Description: "Restrict the key to model_limits."},
			{Name: "model_limits", Type: "string", Description: "Comma separated models the key may call."},
			{Name: "allow_ips", Type: "string", Description: "Newline separated IP allow list."},
		},
		Notes: "A full edit (no status_only/group_only) overwrites name, expiry, quota, limits, and group with the body, so omitted fields are reset. Read the key with GET /api/token/:id first, or use status_only=1 to just enable/disable.",
	},
	"DELETE /api/token/:id": {Summary: "Delete one of my API keys", Description: "Permanently delete an API key owned by the caller."},
	"POST /api/token/batch": {
		Summary:     "Delete several of my API keys",
		Description: "Permanently delete the listed API keys owned by the caller (at most 100).",
		Body:        []BodyField{{Name: "ids", Type: "array of integer", Required: true, Description: "Key ids to delete."}},
	},

	// ---- redemption codes ----
	"GET /api/redemption/": {
		Summary:     "List redemption codes",
		Description: "List redemption codes. Code values are hidden. " + pagedResultNote,
		Query:       []BodyField{pageField, pageSizeField},
	},
	"GET /api/redemption/search": {
		Summary: "Search redemption codes",
		Query: []BodyField{
			{Name: "keyword", Type: "string", Description: "Matches the code name or id."},
			{Name: "status", Type: "string", Description: "Optional status filter."},
			pageField, pageSizeField},
	},
	"POST /api/redemption/": {
		Summary:     "Create redemption codes",
		Description: "Generate redemption codes. The code values are secrets and are not shown in chat; the administrator copies them from the Redemption Codes page.",
		Body: []BodyField{
			{Name: "name", Type: "string", Required: true, Description: "Batch name, 1-20 characters."},
			{Name: "count", Type: "integer", Required: true, Description: "How many codes to create, 1-100."},
			{Name: "quota", Type: "integer", Required: true, Description: "Quota each code grants, in internal quota units (not currency).", Example: 500000},
			{Name: "expired_time", Type: "integer", Description: "Expiry as a unix timestamp in seconds; 0 means never."},
		},
	},
	"PUT /api/redemption/": {
		Summary:     "Edit a redemption code",
		Description: "Edit one redemption code. Without status_only the name, quota, and expiry are all overwritten.",
		Query:       []BodyField{{Name: "status_only", Type: "string", Description: "Set to 1 to change only status."}},
		Body: []BodyField{
			{Name: "id", Type: "integer", Required: true, Description: "Redemption code id."},
			{Name: "status", Type: "integer", Description: "With status_only: 1 enabled, 2 disabled."},
			{Name: "name", Type: "string", Description: "Code name."},
			{Name: "quota", Type: "integer", Description: "Quota in internal quota units."},
			{Name: "expired_time", Type: "integer", Description: "Unix timestamp in seconds; 0 means never."},
		},
	},
	"DELETE /api/redemption/:id":     {Summary: "Delete one redemption code"},
	"DELETE /api/redemption/invalid": {Summary: "Delete used, disabled, and expired redemption codes", Description: "Permanently delete every redemption code that can no longer be redeemed."},
	"POST /api/user/topup": {
		Summary:     "Redeem a redemption code for my account",
		Description: "Redeem one code and add its quota to the caller's balance.",
		Body:        []BodyField{{Name: "key", Type: "string", Required: true, Description: "The redemption code."}},
	},

	// ---- site options (root) ----
	"GET /api/option/": {
		Summary:     "List site options",
		Description: "List every site configuration option as key/value pairs. Credential options are never returned.",
		Notes:       "The list is long. Values are strings; JSON-valued options are returned as JSON text.",
	},
	"PUT /api/option/": {
		Summary:     "Change one site option",
		Description: "Set one site configuration option. This takes effect site-wide immediately.",
		Body: []BodyField{
			{Name: "key", Type: "string", Required: true, Description: "Exact option name as returned by GET /api/option/."},
			{Name: "value", Type: "string", Required: true, Description: "New value. Booleans are \"true\"/\"false\"; JSON options take the full JSON text."},
		},
		Notes: "Read the current value first and change only what was asked. JSON-valued options are replaced as a whole, so send the complete document. Values of credential options (names ending in Key, Secret, Token, Password) are hidden in the transcript; prefer the Settings page for those.",
	},
	"POST /api/option/rest_model_ratio": {Summary: "Reset model ratios to defaults", Description: "Discard every customized model ratio and restore the built-in defaults."},
	"POST /api/system-task/log-cleanup": {
		Summary:     "Delete logs older than a timestamp",
		Description: "Start a background task that permanently deletes logs created before target_timestamp.",
		Query:       []BodyField{{Name: "target_timestamp", Type: "integer", Required: true, Description: "Unix timestamp in seconds; logs older than this are deleted."}},
	},

	// ---- subscriptions ----
	"GET /api/subscription/admin/plans":                   {Summary: "List subscription plans (admin view)"},
	"GET /api/subscription/admin/users/:id/subscriptions": {Summary: "List one user's subscriptions"},
	"POST /api/subscription/admin/bind": {
		Summary:     "Give a user a subscription plan",
		Description: "Bind a subscription plan to a user without payment.",
		Body: []BodyField{
			{Name: "user_id", Type: "integer", Required: true, Description: "Target user id."},
			{Name: "plan_id", Type: "integer", Required: true, Description: "Plan id from GET /api/subscription/admin/plans."},
		},
	},
	"POST /api/subscription/admin/users/:id/subscriptions": {
		Summary:     "Create a subscription for one user",
		Description: "Give the user in the path a subscription plan without payment.",
		Body:        []BodyField{{Name: "plan_id", Type: "integer", Required: true, Description: "Plan id from GET /api/subscription/admin/plans."}},
	},
	"POST /api/subscription/admin/users/:id/subscriptions/reset": {
		Summary:     "Reset one user's usage on a plan",
		Description: "Reset the used amount of the user's active subscriptions of one plan.",
		Body: []BodyField{
			{Name: "plan_id", Type: "integer", Required: true, Description: "Plan id."},
			{Name: "advance_reset_time", Type: "boolean", Description: "Also move the next scheduled reset forward. Defaults to true."},
		},
	},
	"POST /api/subscription/admin/plans/:id/subscriptions/reset": {
		Summary:     "Reset usage for every subscriber of a plan",
		Description: "Reset the used amount of every active subscription of the plan in the path.",
		Body:        []BodyField{{Name: "advance_reset_time", Type: "boolean", Description: "Also move the next scheduled reset forward. Defaults to true."}},
	},
	"POST /api/subscription/admin/user_subscriptions/:id/invalidate": {Summary: "Invalidate one user subscription", Description: "End one subscription immediately. id is the user subscription id, not the user id."},
	"DELETE /api/subscription/admin/user_subscriptions/:id":          {Summary: "Delete one user subscription", Description: "Permanently delete one subscription record. id is the user subscription id, not the user id."},
	"PATCH /api/subscription/admin/plans/:id": {
		Summary: "Enable or disable a subscription plan",
		Body:    []BodyField{{Name: "enabled", Type: "boolean", Required: true, Description: "True to enable, false to disable."}},
	},

	// ---- activity codes ----
	"GET /api/activity/": {
		Summary: "List activity codes",
		Query:   []BodyField{pageField, pageSizeField, {Name: "keyword", Type: "string", Description: "Optional search keyword."}},
	},
	"GET /api/activity/:id/grants": {Summary: "List the grants of one activity code"},
	"POST /api/activity/:id/grants": {
		Summary:     "Grant an activity code to users",
		Description: "Give the activity in the path to the listed users.",
		Body: []BodyField{
			{Name: "user_ids", Type: "array of integer", Required: true, Description: "Users who receive the grant."},
			{Name: "remark", Type: "string", Description: "Optional admin note."},
		},
	},
	"POST /api/activity/grants/:grant_id/revoke": {
		Summary: "Revoke an activity grant",
		Body:    []BodyField{{Name: "remark", Type: "string", Description: "Optional reason."}},
	},
	"PUT /api/activity/:id/status": {
		Summary: "Enable or disable an activity code",
		Body:    []BodyField{{Name: "status", Type: "integer", Required: true, Description: "1 enabled, 2 disabled."}},
	},
	"POST /api/user/activity/verify": {
		Summary:     "Preview an activity code",
		Description: "Check an activity code and show what redeeming it would grant. Nothing is redeemed.",
		Body:        []BodyField{{Name: "code", Type: "string", Required: true, Description: "Activity code."}},
	},
	"POST /api/user/activity/redeem": {
		Summary:     "Redeem an activity code for my account",
		Description: "Redeem an activity code. Paid activities deduct quota from the caller's balance.",
		Body: []BodyField{
			{Name: "code", Type: "string", Required: true, Description: "Activity code."},
			{Name: "confirm", Type: "boolean", Description: "Must be true to redeem a paid activity."},
			{Name: "expected_price_quota", Type: "integer", Description: "Price in quota units shown by the preview; the redeem fails if the price changed."},
		},
	},

	// ---- own account ----
	"GET /api/user/self":                    {Summary: "Get my account", Description: "Get the caller's profile, quota, used quota, group, and settings. Quota values are internal quota units."},
	"GET /api/user/sessions":                {Summary: "List my login sessions"},
	"DELETE /api/user/sessions/:sid":        {Summary: "Sign out one of my sessions"},
	"POST /api/user/sessions/revoke-others": {Summary: "Sign out all my other sessions"},
	"POST /api/user/aff_transfer": {
		Summary:     "Move invitation rewards into my balance",
		Description: "Transfer invitation reward quota to the caller's main balance.",
		Body:        []BodyField{{Name: "quota", Type: "integer", Required: true, Description: "Amount to transfer, in internal quota units."}},
	},
	"POST /api/user/checkin":     {Summary: "Do today's check-in"},
	"GET /api/user/self/bonus":   {Summary: "Get my bonus balance and grants"},
	"GET /api/subscription/self": {Summary: "Get my subscriptions"},
	"GET /api/pricing":           {Summary: "Get model pricing", Description: "List the models and prices available to the caller. The list can be large."},
	"GET /api/group/":            {Summary: "List all group names"},
	"PUT /api/user/self": {
		Summary:     "Edit my profile",
		Description: "Change the caller's display name, or set the interface language. Password changes are not available from chat.",
		Body: []BodyField{
			{Name: "username", Type: "string", Description: "Must be sent together with display_name; use the current username."},
			{Name: "display_name", Type: "string", Description: "New display name, at most 20 characters."},
			{Name: "language", Type: "string", Description: "Interface language code such as en or zh. When present, only the language is updated."},
		},
	},
	"PUT /api/subscription/self/preference": {
		Summary:     "Choose how my requests are billed",
		Description: "Set whether requests draw from the subscription or the wallet balance first.",
		Body:        []BodyField{{Name: "billing_preference", Type: "string", Required: true, Description: "One of subscription_first, wallet_first, subscription_only, wallet_only."}},
	},
	"POST /api/channel/copy/:id": {
		Summary:     "Duplicate a channel",
		Description: "Create a copy of one channel, including its key.",
		Query: []BodyField{
			{Name: "suffix", Type: "string", Description: "Text appended to the copied channel's name. Defaults to a copy marker."},
			{Name: "reset_balance", Type: "boolean", Description: "Start the copy with zero balance and used quota. Defaults to true."},
		},
	},
	"POST /api/channel/fix": {
		Summary:     "Rebuild channel abilities",
		Description: "Recompute the model/group routing table from the channel list. Safe to run when routing looks out of sync.",
	},
	"DELETE /api/option/channel_affinity_cache": {
		Summary:     "Clear the channel affinity cache",
		Description: "Drop cached channel affinity entries so requests are routed afresh.",
		Query: []BodyField{
			{Name: "all", Type: "string", Description: "Set to true to clear every rule."},
			{Name: "rule_name", Type: "string", Description: "Clear one rule by name. Required unless all is true."},
		},
	},
	"DELETE /api/performance/disk_cache": {Summary: "Clear the request body disk cache"},
	"POST /api/performance/gc":           {Summary: "Force a garbage collection", Description: "Trigger a Go garbage collection on this node. It changes no data."},
	"POST /api/performance/reset_stats":  {Summary: "Reset performance counters"},
	"GET /api/performance/stats":         {Summary: "Get runtime performance statistics"},
	"GET /api/models/": {
		Summary: "List model metadata",
		Query: []BodyField{pageField, pageSizeField,
			{Name: "status", Type: "string", Description: "Optional status filter."}},
	},
	"GET /api/models/search": {
		Summary: "Search model metadata",
		Query: []BodyField{
			{Name: "keyword", Type: "string", Description: "Matches the model name."},
			{Name: "vendor", Type: "string", Description: "Vendor filter."},
			pageField, pageSizeField},
	},
	"POST /api/models/": {
		Summary: "Create model metadata",
		Body: []BodyField{
			{Name: "model_name", Type: "string", Required: true, Description: "Model name or pattern."},
			{Name: "description", Type: "string", Description: "Shown on the pricing page."},
			{Name: "tags", Type: "string", Description: "Comma separated tags."},
			{Name: "vendor_id", Type: "integer", Description: "Vendor id from GET /api/vendors/."},
			{Name: "status", Type: "integer", Description: "1 enabled, 0 disabled."},
			{Name: "name_rule", Type: "integer", Description: "0 exact, 1 prefix, 2 contains, 3 suffix."},
		},
	},
	"PUT /api/models/": {
		Summary: "Edit model metadata",
		Query:   []BodyField{{Name: "status_only", Type: "string", Description: "Set to true to change only status."}},
		Body: []BodyField{
			{Name: "id", Type: "integer", Required: true, Description: "Model metadata id."},
			{Name: "model_name", Type: "string", Description: "Model name or pattern."},
			{Name: "description", Type: "string", Description: "Shown on the pricing page."},
			{Name: "tags", Type: "string", Description: "Comma separated tags."},
			{Name: "vendor_id", Type: "integer", Description: "Vendor id."},
			{Name: "status", Type: "integer", Description: "1 enabled, 0 disabled."},
		},
		Notes: "Without status_only the record is overwritten with the body; read it with GET /api/models/:id first.",
	},
	"GET /api/vendors/": {Summary: "List model vendors", Query: []BodyField{pageField, pageSizeField}},
	"POST /api/vendors/": {
		Summary: "Create a model vendor",
		Body: []BodyField{
			{Name: "name", Type: "string", Required: true, Description: "Vendor name."},
			{Name: "description", Type: "string", Description: "Optional description."},
			{Name: "icon", Type: "string", Description: "Icon name."},
			{Name: "status", Type: "integer", Description: "1 enabled, 0 disabled."},
		},
	},
	"PUT /api/vendors/": {
		Summary: "Edit a model vendor",
		Body: []BodyField{
			{Name: "id", Type: "integer", Required: true, Description: "Vendor id."},
			{Name: "name", Type: "string", Description: "Vendor name."},
			{Name: "description", Type: "string", Description: "Optional description."},
			{Name: "icon", Type: "string", Description: "Icon name."},
			{Name: "status", Type: "integer", Description: "1 enabled, 0 disabled."},
		},
	},
	"POST /api/subscription/admin/plans": {
		Summary:     "Create a subscription plan",
		Description: "Create a subscription plan. The body wraps the plan fields in a plan object.",
		Body: []BodyField{
			{Name: "plan", Type: "object", Required: true, Description: "Plan fields: title, subtitle, price_amount (number), currency, duration_unit (for example month), duration_value, total_amount (quota units per period), enabled, upgrade_group, quota_reset_period."},
		},
	},
	"PUT /api/subscription/admin/plans/:id": {
		Summary:     "Edit a subscription plan",
		Description: "Overwrite a subscription plan. The body wraps the plan fields in a plan object.",
		Body: []BodyField{
			{Name: "plan", Type: "object", Required: true, Description: "Full plan fields: title, subtitle, price_amount, currency, duration_unit, duration_value, total_amount (quota units per period), enabled, upgrade_group, quota_reset_period."},
		},
		Notes: "Send the complete plan; read it from GET /api/subscription/admin/plans first. To only enable or disable a plan use PATCH on the same path.",
	},
	"GET /api/system-info/instances":    {Summary: "List running server instances"},
	"GET /api/system-task/list":         {Summary: "List background system tasks"},
	"GET /api/system-task/current":      {Summary: "Get the running background system task"},
	"GET /api/data/flow":                {Summary: "Request flow statistics (all users)", Query: []BodyField{startField, endField, {Name: "username", Type: "string", Description: "Limit to one username."}}},
	"GET /api/channel/models_enabled":   {Summary: "List models served by enabled channels"},
	"GET /api/channel/fetch_models/:id": {Summary: "Fetch the upstream model list of one channel", Description: "Ask the upstream provider of one channel which models it offers. Nothing is saved."},
	"POST /api/activity/": {
		Summary:     "Create an activity code",
		Description: "Create an activity (promotion) code that grants a group, discount ratio, or benefit bundle for a limited time.",
		Body: []BodyField{
			{Name: "code", Type: "string", Required: true, Description: "The code users enter."},
			{Name: "name", Type: "string", Required: true, Description: "Display name."},
			{Name: "description", Type: "string", Description: "Shown to users."},
			{Name: "type", Type: "string", Description: "ratio (group/price based) or benefit (bundle of benefits)."},
			{Name: "group_name", Type: "string", Description: "Group the activity grants access to."},
			{Name: "duration_hours", Type: "number", Required: true, Description: "How long a grant lasts after redemption, in hours."},
			{Name: "limit_mode", Type: "string", Description: "rpm or concurrency."},
			{Name: "rpm_limit", Type: "integer", Description: "Requests per minute allowed under the grant."},
			{Name: "concurrency_limit", Type: "integer", Description: "Concurrent requests allowed when limit_mode is concurrency."},
			{Name: "price_quota", Type: "integer", Description: "Price users pay to redeem, in internal quota units. 0 is free."},
			{Name: "ratio_mode", Type: "string", Description: "inherit (use the group's ratio) or override."},
			{Name: "ratio_override", Type: "number", Description: "Group ratio when ratio_mode is override."},
			{Name: "start_time", Type: "integer", Description: "Redeemable from this unix timestamp; 0 means immediately."},
			{Name: "end_time", Type: "integer", Description: "Redeemable until this unix timestamp; 0 means no end."},
			{Name: "total_limit", Type: "integer", Description: "Maximum redemptions overall; 0 means unlimited."},
			{Name: "per_user_limit", Type: "integer", Description: "Maximum redemptions per user."},
			{Name: "public_visible", Type: "integer", Description: "1 to list the activity publicly, 0 to keep it code-only."},
			{Name: "status", Type: "integer", Description: "1 enabled, 2 disabled."},
		},
	},
	"PUT /api/activity/": {
		Summary:     "Edit an activity code",
		Description: "Edit an activity code. Takes the same fields as creation plus id.",
		Body: []BodyField{
			{Name: "id", Type: "integer", Required: true, Description: "Activity code id."},
			{Name: "code", Type: "string", Required: true, Description: "The code users enter."},
			{Name: "name", Type: "string", Required: true, Description: "Display name."},
			{Name: "duration_hours", Type: "number", Required: true, Description: "Grant duration in hours."},
		},
		Notes: "Most fields are overwritten by the body. Read the activity with GET /api/activity/:id and send back every value that should stay; see POST /api/activity/ for the full field list. To only enable or disable it use PUT /api/activity/:id/status.",
	},
	"PUT /api/channel/tag": {
		Summary:     "Edit every channel with a tag",
		Description: "Apply the given settings to all channels that carry one tag. Fields that are omitted stay unchanged.",
		Body: []BodyField{
			{Name: "tag", Type: "string", Required: true, Description: "Tag selecting the channels."},
			{Name: "new_tag", Type: "string", Description: "Rename the tag."},
			{Name: "priority", Type: "integer", Description: "New priority."},
			{Name: "weight", Type: "integer", Description: "New weight."},
			{Name: "models", Type: "string", Description: "Comma separated model list (replaces the list)."},
			{Name: "groups", Type: "string", Description: "Comma separated group list (replaces the list)."},
			{Name: "model_mapping", Type: "string", Description: "JSON object string of model name mappings."},
		},
	},
	"POST /api/channel/upstream_updates/detect": {
		Summary:     "Check one channel for upstream model changes",
		Description: "Ask the upstream provider which models it now offers and report models to add or remove. Nothing is changed.",
		Body:        []BodyField{{Name: "id", Type: "integer", Required: true, Description: "Channel id."}},
	},
	"POST /api/channel/upstream_updates/detect_all": {
		Summary:     "Check every channel for upstream model changes",
		Description: "Run the upstream model check for all channels. It contacts every upstream provider.",
	},
	"POST /api/channel/upstream_updates/apply": {
		Summary:     "Apply upstream model changes to one channel",
		Description: "Add or remove models on a channel based on a previous detection.",
		Body: []BodyField{
			{Name: "id", Type: "integer", Required: true, Description: "Channel id."},
			{Name: "add_models", Type: "array of string", Description: "Models to add to the channel."},
			{Name: "remove_models", Type: "array of string", Description: "Models to remove from the channel."},
			{Name: "ignore_models", Type: "array of string", Description: "Models to stop suggesting."},
		},
	},
	"POST /api/channel/upstream_updates/apply_all": {
		Summary:     "Apply upstream model changes to every channel",
		Description: "Apply all pending detected model additions and removals across channels.",
	},
	"POST /api/models/sync_upstream": {
		Summary:     "Sync model metadata from the official source",
		Description: "Fetch official model metadata and update local model records. Preview first with GET /api/models/sync_upstream/preview.",
	},
	"POST /api/prefill_group/": {
		Summary: "Create a prefill group",
		Body: []BodyField{
			{Name: "name", Type: "string", Required: true, Description: "Unique group name."},
			{Name: "type", Type: "string", Required: true, Description: "What the items are, for example model, tag, or endpoint."},
			{Name: "items", Type: "array", Description: "The prefilled values."},
			{Name: "description", Type: "string", Description: "Optional description."},
		},
	},
	"PUT /api/prefill_group/": {
		Summary: "Edit a prefill group",
		Body: []BodyField{
			{Name: "id", Type: "integer", Required: true, Description: "Prefill group id."},
			{Name: "name", Type: "string", Required: true, Description: "Unique group name."},
			{Name: "type", Type: "string", Description: "Item type."},
			{Name: "items", Type: "array", Description: "The prefilled values (replaces the list)."},
			{Name: "description", Type: "string", Description: "Optional description."},
		},
	},
	"POST /api/system-task/log-migration": {
		Summary:     "Start the log migration task",
		Description: "Start a background task that migrates legacy logs to the current log storage.",
		Body: []BodyField{
			{Name: "batch_size", Type: "integer", Description: "Rows per batch. Omit for the default."},
			{Name: "delete_source", Type: "boolean", Description: "Delete migrated rows from the source. Defaults to false."},
		},
	},
	"POST /api/system-task/legacy-log-reclaim": {Summary: "Reclaim legacy log storage", Description: "Drop the already-migrated legacy log data to free space. This cannot be undone."},
	"PUT /api/user/setting": {
		Summary:     "Update my notification settings",
		Description: "Set how the caller is warned about low quota and which notifications they receive.",
		Body: []BodyField{
			{Name: "notify_type", Type: "string", Required: true, Description: "One of email, webhook, bark, gotify."},
			{Name: "quota_warning_threshold", Type: "number", Required: true, Description: "Warn when the balance drops to or below this many internal quota units."},
			{Name: "notification_email", Type: "string", Description: "Address for email notifications. Defaults to the account email."},
			{Name: "operational_notify_enabled", Type: "boolean", Description: "Receive operational emails from the site."},
			{Name: "accept_unset_model_ratio_model", Type: "boolean", Description: "Allow calling models that have no configured price."},
			{Name: "record_ip_log", Type: "boolean", Description: "Record the client IP in usage logs."},
		},
		Notes: "The boolean fields are overwritten on every call; send the current values for the ones that should stay. Webhook, Bark, and Gotify targets contain credentials and are set on the Settings page, not in chat.",
	},
	"POST /api/user/self/channel_chains":             {Summary: "Create a channel chain for my account"},
	"PUT /api/user/self/channel_chains/:chain_id":    {Summary: "Edit one of my channel chains"},
	"DELETE /api/user/self/channel_chains/:chain_id": {Summary: "Delete one of my channel chains"},
	"POST /api/channel/:id/codex/refresh":            {Summary: "Refresh a Codex channel credential"},
	"POST /api/channel/:id/codex/usage/reset":        {Summary: "Reset a Codex channel's usage window"},
	"POST /api/channel/fetch_models":                 {Summary: "Fetch a model list with unsaved channel settings", Description: "Used by the channel form before saving; it needs the channel key, so prefer GET /api/channel/fetch_models/:id for an existing channel."},
}

// UnmatchedDocs lists documented operation ids that the current catalog does
// not contain: the route was removed, renamed, or excluded. The catalog test
// uses it to keep the documentation from drifting away from the router.
func UnmatchedDocs() []string {
	unmatched := make([]string, 0)
	for id := range operationDocs {
		if _, ok := Find(id); !ok {
			unmatched = append(unmatched, id)
		}
	}
	sort.Strings(unmatched)
	return unmatched
}
