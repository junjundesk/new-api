package assistant

import (
	"context"
	"math"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/model"
)

const (
	dbQueryDefaultLimit = 50
	dbQueryMaxLimit     = 100
	dbQueryMaxInValues  = 200
	dbQueryTimeout      = 5 * time.Second
)

type dbColumn struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Description string `json:"description,omitempty"`
}

type dbTable struct {
	Name        string
	Description string
	LogDB       bool
	// SoftDelete marks tables whose rows carry deleted_at. Deleted rows are
	// hidden unless the query sets include_deleted.
	SoftDelete bool
	Columns    []dbColumn
}

const (
	dbTypeInt    = "int"
	dbTypeFloat  = "float"
	dbTypeString = "string"
	dbTypeBool   = "bool"
	// dbTypeTime columns hold unix timestamps in seconds.
	dbTypeTime = "unix_seconds"
	// dbTypeDatetime columns hold a database datetime (NULL when unset).
	dbTypeDatetime = "datetime"
)

const quotaUnitNote = "internal quota units, not currency"

var deletedAtColumn = dbColumn{Name: "deleted_at", Type: dbTypeDatetime, Description: "NULL for live rows; set when the row was soft-deleted"}

// adminDBTables is the allow-list for the root database query tool. Secret
// columns (passwords, keys, tokens, code values) are deliberately absent.
var adminDBTables = []dbTable{
	{Name: "users", Description: "User accounts and quota totals", SoftDelete: true, Columns: []dbColumn{
		{Name: "id", Type: dbTypeInt}, {Name: "username", Type: dbTypeString}, {Name: "display_name", Type: dbTypeString},
		{Name: "email", Type: dbTypeString},
		{Name: "role", Type: dbTypeInt, Description: "1 common user, 10 admin, 100 root"},
		{Name: "status", Type: dbTypeInt, Description: "1 enabled, 2 disabled"},
		{Name: "group", Type: dbTypeString},
		{Name: "quota", Type: dbTypeInt, Description: "remaining balance, " + quotaUnitNote},
		{Name: "used_quota", Type: dbTypeInt, Description: "lifetime consumption, " + quotaUnitNote},
		{Name: "request_count", Type: dbTypeInt},
		{Name: "inviter_id", Type: dbTypeInt, Description: "user id of the inviter, 0 when none"},
		{Name: "created_at", Type: dbTypeTime}, {Name: "last_login_at", Type: dbTypeTime},
		deletedAtColumn,
	}},
	{Name: "tokens", Description: "User API keys without secret key values", SoftDelete: true, Columns: []dbColumn{
		{Name: "id", Type: dbTypeInt}, {Name: "user_id", Type: dbTypeInt}, {Name: "name", Type: dbTypeString},
		{Name: "status", Type: dbTypeInt, Description: "1 enabled, 2 disabled, 3 expired, 4 exhausted"},
		{Name: "unlimited_quota", Type: dbTypeBool},
		{Name: "remain_quota", Type: dbTypeInt, Description: quotaUnitNote},
		{Name: "used_quota", Type: dbTypeInt, Description: quotaUnitNote},
		{Name: "expired_time", Type: dbTypeTime, Description: "-1 means never expires"},
		{Name: "group", Type: dbTypeString},
		{Name: "created_time", Type: dbTypeTime}, {Name: "accessed_time", Type: dbTypeTime},
		deletedAtColumn,
	}},
	{Name: "channels", Description: "Upstream channels without secret key values", Columns: []dbColumn{
		{Name: "id", Type: dbTypeInt}, {Name: "type", Type: dbTypeInt, Description: "provider type id"},
		{Name: "name", Type: dbTypeString},
		{Name: "status", Type: dbTypeInt, Description: "1 enabled, 2 manually disabled, 3 auto disabled"},
		{Name: "base_url", Type: dbTypeString},
		{Name: "models", Type: dbTypeString, Description: "comma separated model names"},
		{Name: "group", Type: dbTypeString, Description: "comma separated group names"},
		{Name: "tag", Type: dbTypeString},
		{Name: "priority", Type: dbTypeInt}, {Name: "weight", Type: dbTypeInt},
		{Name: "used_quota", Type: dbTypeInt, Description: quotaUnitNote},
		{Name: "balance", Type: dbTypeFloat, Description: "upstream balance in USD"},
		{Name: "response_time", Type: dbTypeInt, Description: "last test latency in milliseconds"},
		{Name: "test_time", Type: dbTypeTime}, {Name: "created_time", Type: dbTypeTime},
	}},
	{Name: "logs", Description: "Request, top-up, management, and error logs; content is not exposed through this tool. Always filter by created_at, the table is large.", LogDB: true, Columns: []dbColumn{
		{Name: "id", Type: dbTypeInt}, {Name: "user_id", Type: dbTypeInt}, {Name: "username", Type: dbTypeString},
		{Name: "type", Type: dbTypeInt, Description: "1 top-up, 2 consume, 3 manage, 4 system, 5 error, 6 refund, 7 login"},
		{Name: "model_name", Type: dbTypeString}, {Name: "token_name", Type: dbTypeString},
		{Name: "token_id", Type: dbTypeInt}, {Name: "channel_id", Type: dbTypeInt},
		{Name: "group", Type: dbTypeString},
		{Name: "quota", Type: dbTypeInt, Description: quotaUnitNote},
		{Name: "prompt_tokens", Type: dbTypeInt}, {Name: "completion_tokens", Type: dbTypeInt},
		{Name: "use_time", Type: dbTypeInt, Description: "request duration in seconds"},
		{Name: "is_stream", Type: dbTypeBool},
		{Name: "created_at", Type: dbTypeTime},
	}},
	{Name: "redemptions", Description: "Redemption codes without code values", SoftDelete: true, Columns: []dbColumn{
		{Name: "id", Type: dbTypeInt}, {Name: "user_id", Type: dbTypeInt, Description: "admin who created the code"},
		{Name: "name", Type: dbTypeString},
		{Name: "status", Type: dbTypeInt, Description: "1 enabled, 2 disabled, 3 used"},
		{Name: "quota", Type: dbTypeInt, Description: quotaUnitNote},
		{Name: "used_user_id", Type: dbTypeInt, Description: "user who redeemed the code, 0 when unused"},
		{Name: "created_time", Type: dbTypeTime}, {Name: "redeemed_time", Type: dbTypeTime},
		{Name: "expired_time", Type: dbTypeTime, Description: "0 means never expires"},
		deletedAtColumn,
	}},
	{Name: "top_ups", Description: "Payment orders", Columns: []dbColumn{
		{Name: "id", Type: dbTypeInt}, {Name: "user_id", Type: dbTypeInt},
		{Name: "amount", Type: dbTypeInt, Description: "top-up amount requested"},
		{Name: "money", Type: dbTypeFloat, Description: "money paid"},
		{Name: "trade_no", Type: dbTypeString},
		{Name: "payment_method", Type: dbTypeString}, {Name: "payment_provider", Type: dbTypeString},
		{Name: "status", Type: dbTypeString, Description: "pending, success, expired, ..."},
		{Name: "create_time", Type: dbTypeTime}, {Name: "complete_time", Type: dbTypeTime},
	}},
	{Name: "subscription_plans", Description: "Subscription plans", Columns: []dbColumn{
		{Name: "id", Type: dbTypeInt}, {Name: "title", Type: dbTypeString},
		{Name: "price_amount", Type: dbTypeFloat}, {Name: "currency", Type: dbTypeString},
		{Name: "duration_unit", Type: dbTypeString}, {Name: "duration_value", Type: dbTypeInt},
		{Name: "total_amount", Type: dbTypeInt, Description: "quota included per period, " + quotaUnitNote},
		{Name: "enabled", Type: dbTypeBool}, {Name: "created_at", Type: dbTypeTime},
	}},
	{Name: "user_subscriptions", Description: "Subscriptions held by users", Columns: []dbColumn{
		{Name: "id", Type: dbTypeInt}, {Name: "user_id", Type: dbTypeInt}, {Name: "plan_id", Type: dbTypeInt},
		{Name: "amount_total", Type: dbTypeInt, Description: quotaUnitNote},
		{Name: "amount_used", Type: dbTypeInt, Description: quotaUnitNote},
		{Name: "status", Type: dbTypeString, Description: "active, expired, cancelled"},
		{Name: "source", Type: dbTypeString, Description: "order or admin"},
		{Name: "start_time", Type: dbTypeTime}, {Name: "end_time", Type: dbTypeTime},
	}},
}

func (e *DefaultToolExecutor) dbDefinitions() []map[string]any {
	return []map[string]any{
		functionTool(
			"admin_db_schema",
			"List the database tables and columns available to admin_db_query, with column types and units. Secret columns are never listed. Call this before the first query.",
			objectSchema(map[string]any{}, nil),
		),
		functionTool(
			"admin_db_query",
			"Run a structured read-only database query as root: one allowed table, safe columns, filters, ordering, limit/offset, or aggregate metrics with group_by. Raw SQL, joins, and writes are not supported. Soft-deleted rows are excluded unless include_deleted is true. Timestamps are unix seconds. The result reports has_more when another page exists.",
			objectSchema(map[string]any{
				"table":   map[string]any{"type": "string", "description": "Table name from admin_db_schema."},
				"columns": map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Columns to return. Defaults to every safe column. Ignored when aggregate is used."},
				"filters": map[string]any{
					"type":        "array",
					"description": "Conditions combined with AND.",
					"items": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"column": map[string]any{"type": "string"},
							"op":     map[string]any{"type": "string", "enum": []string{"=", "!=", ">", ">=", "<", "<=", "in", "like", "is_null", "not_null"}, "description": "like is a case-sensitivity-dependent substring match on string columns; in takes an array value."},
							"value":  map[string]any{"description": "Comparison value. Omit for is_null and not_null."},
						},
						"required":             []string{"column", "op"},
						"additionalProperties": false,
					},
				},
				"order_by": map[string]any{
					"type":        "array",
					"description": "Sort keys. With aggregate, use a group_by column or a metric alias such as sum_quota or count_all.",
					"items": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"column":    map[string]any{"type": "string"},
							"direction": map[string]any{"type": "string", "enum": []string{"asc", "desc"}},
						},
						"required":             []string{"column"},
						"additionalProperties": false,
					},
				},
				"limit":           map[string]any{"type": "integer", "minimum": 1, "maximum": dbQueryMaxLimit, "description": "Maximum rows. Defaults to 50."},
				"offset":          map[string]any{"type": "integer", "minimum": 0, "description": "Rows to skip."},
				"include_deleted": map[string]any{"type": "boolean", "description": "Include soft-deleted rows of users, tokens, and redemptions. Defaults to false."},
				"aggregate": map[string]any{
					"type":        "object",
					"description": "Return aggregated rows instead of raw rows. Each metric is returned as <func>_<column>, for example sum_quota; count of rows is count_all.",
					"properties": map[string]any{
						"group_by": map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
						"metrics": map[string]any{
							"type": "array",
							"items": map[string]any{
								"type": "object",
								"properties": map[string]any{
									"func":   map[string]any{"type": "string", "enum": []string{"count", "sum", "avg", "min", "max"}},
									"column": map[string]any{"type": "string", "description": "Column name, or * with count."},
								},
								"required":             []string{"func", "column"},
								"additionalProperties": false,
							},
						},
					},
					"required":             []string{"metrics"},
					"additionalProperties": false,
				},
			}, []string{"table"}),
		),
	}
}

func (e *DefaultToolExecutor) adminDBSchema() ToolResult {
	if !e.canUseDBTools() {
		return toolErrorf("database tools are available only to root")
	}
	tables := make([]map[string]any, 0, len(adminDBTables))
	for _, table := range adminDBTables {
		entry := map[string]any{"table": table.Name, "description": table.Description, "columns": table.Columns}
		if table.SoftDelete {
			entry["soft_delete"] = "rows with deleted_at set are hidden unless include_deleted is true"
		}
		tables = append(tables, entry)
	}
	return ToolResult{OK: true, Data: map[string]any{"tables": tables}}
}

type dbFilter struct {
	Column string `json:"column"`
	Op     string `json:"op"`
	Value  any    `json:"value"`
}

type dbOrder struct {
	Column    string `json:"column"`
	Direction string `json:"direction"`
}

type dbMetric struct {
	Func   string `json:"func"`
	Column string `json:"column"`
}

type dbAggregate struct {
	GroupBy []string   `json:"group_by"`
	Metrics []dbMetric `json:"metrics"`
}

type dbQueryArguments struct {
	Table          string       `json:"table"`
	Columns        []string     `json:"columns"`
	Filters        []dbFilter   `json:"filters"`
	OrderBy        []dbOrder    `json:"order_by"`
	Limit          *int         `json:"limit"`
	Offset         *int         `json:"offset"`
	IncludeDeleted bool         `json:"include_deleted"`
	Aggregate      *dbAggregate `json:"aggregate"`
}

func (e *DefaultToolExecutor) adminDBQuery(ctx context.Context, rawArguments string) ToolResult {
	if !e.canUseDBTools() {
		return toolErrorf("database tools are available only to root")
	}
	args := dbQueryArguments{}
	if err := decodeArguments(rawArguments, &args); err != nil {
		return toolErrorf("invalid arguments: %v", err)
	}
	table, ok := findDBTable(args.Table)
	if !ok {
		return toolErrorf("table is not allowed: %s. Call admin_db_schema for the available tables.", args.Table)
	}
	columnTypes := make(map[string]string, len(table.Columns))
	for _, column := range table.Columns {
		columnTypes[column.Name] = column.Type
	}
	limit, err := normalizeLimit(args.Limit, dbQueryDefaultLimit, dbQueryMaxLimit)
	if err != nil {
		return toolError(err)
	}
	offset, err := normalizeOffset(args.Offset)
	if err != nil {
		return toolError(err)
	}
	db := model.DB
	if table.LogDB {
		db = model.LOG_DB
	}
	if db == nil {
		return toolErrorf("database is unavailable")
	}
	queryCtx, cancel := context.WithTimeout(ctx, dbQueryTimeout)
	defer cancel()
	tx := db.WithContext(queryCtx).Table(table.Name)
	// Column names come from the allow-list, but several are reserved words
	// ("group", "type"), so every identifier is quoted for the active dialect.
	quote := func(name string) string { return tx.Statement.Quote(name) }

	// sortable maps a name accepted by order_by to its SQL expression.
	sortable := map[string]string{}
	aggregated := args.Aggregate != nil && len(args.Aggregate.Metrics) > 0
	if args.Aggregate != nil && !aggregated {
		return toolErrorf("aggregate requires at least one metric")
	}
	if aggregated {
		selects := make([]string, 0, len(args.Aggregate.GroupBy)+len(args.Aggregate.Metrics))
		for _, group := range args.Aggregate.GroupBy {
			if _, allowed := columnTypes[group]; !allowed {
				return toolErrorf("group_by column is not allowed: %s", group)
			}
			if _, duplicate := sortable[group]; duplicate {
				return toolErrorf("duplicate group_by column: %s", group)
			}
			selects = append(selects, quote(group))
			sortable[group] = quote(group)
		}
		for _, metric := range args.Aggregate.Metrics {
			fn := strings.ToLower(metric.Func)
			switch fn {
			case "count", "sum", "avg", "min", "max":
			default:
				return toolErrorf("aggregate function is not allowed: %s", metric.Func)
			}
			alias := fn + "_" + metric.Column
			expression := ""
			if metric.Column == "*" {
				if fn != "count" {
					return toolErrorf("aggregate * is only allowed with count")
				}
				alias = "count_all"
				expression = "count(*)"
			} else {
				columnType, allowed := columnTypes[metric.Column]
				if !allowed {
					return toolErrorf("aggregate column is not allowed: %s", metric.Column)
				}
				if (fn == "sum" || fn == "avg") && columnType != dbTypeInt && columnType != dbTypeFloat {
					return toolErrorf("%s requires a numeric column, %s is %s", fn, metric.Column, columnType)
				}
				expression = fn + "(" + quote(metric.Column) + ")"
			}
			if _, duplicate := sortable[alias]; duplicate {
				return toolErrorf("duplicate metric: %s", alias)
			}
			selects = append(selects, expression+" AS "+quote(alias))
			// Order by the expression, not the alias: it works on every dialect.
			sortable[alias] = expression
		}
		tx = tx.Select(strings.Join(selects, ", "))
		for _, group := range args.Aggregate.GroupBy {
			// Group quotes a plain column name itself; one call per column
			// accumulates into a single GROUP BY clause.
			tx = tx.Group(group)
		}
	} else {
		columns := args.Columns
		if len(columns) == 0 {
			for _, column := range table.Columns {
				columns = append(columns, column.Name)
			}
		}
		selects := make([]string, 0, len(columns))
		for _, column := range columns {
			if _, allowed := columnTypes[column]; !allowed {
				return toolErrorf("column is not allowed: %s", column)
			}
			selects = append(selects, quote(column))
		}
		tx = tx.Select(strings.Join(selects, ", "))
		for name := range columnTypes {
			sortable[name] = quote(name)
		}
	}

	if table.SoftDelete && !args.IncludeDeleted {
		tx = tx.Where(quote("deleted_at") + " IS NULL")
	}
	for _, filter := range args.Filters {
		columnType, allowed := columnTypes[filter.Column]
		if !allowed {
			return toolErrorf("filter column is not allowed: %s", filter.Column)
		}
		column := quote(filter.Column)
		switch filter.Op {
		case "=", "!=", ">", ">=", "<", "<=":
			if filter.Value == nil {
				return toolErrorf("filter %s %s requires a value; use is_null or not_null for NULL checks", filter.Column, filter.Op)
			}
			operator := filter.Op
			if operator == "!=" {
				operator = "<>"
			}
			tx = tx.Where(column+" "+operator+" ?", dbFilterValue(filter.Value))
		case "in":
			values, isList := filter.Value.([]any)
			if !isList || len(values) == 0 {
				return toolErrorf("in filter requires a non-empty array value")
			}
			if len(values) > dbQueryMaxInValues {
				return toolErrorf("in filter accepts at most %d values", dbQueryMaxInValues)
			}
			normalized := make([]any, 0, len(values))
			for _, value := range values {
				normalized = append(normalized, dbFilterValue(value))
			}
			tx = tx.Where(column+" IN ?", normalized)
		case "like":
			if columnType != dbTypeString {
				return toolErrorf("like is only supported on string columns, %s is %s", filter.Column, columnType)
			}
			pattern, isString := filter.Value.(string)
			if !isString || pattern == "" {
				return toolErrorf("like filter requires a non-empty string value")
			}
			tx = tx.Where(column+" LIKE ?", "%"+pattern+"%")
		case "is_null":
			tx = tx.Where(column + " IS NULL")
		case "not_null":
			tx = tx.Where(column + " IS NOT NULL")
		default:
			return toolErrorf("filter operator is not allowed: %s", filter.Op)
		}
	}

	orderExpressions := make([]string, 0, len(args.OrderBy))
	for _, order := range args.OrderBy {
		expression, allowed := sortable[order.Column]
		if !allowed {
			if aggregated {
				return toolErrorf("order column is not allowed: %s. With aggregate, order by a group_by column or a metric alias such as sum_quota or count_all.", order.Column)
			}
			return toolErrorf("order column is not allowed: %s", order.Column)
		}
		direction := "ASC"
		if strings.EqualFold(order.Direction, "desc") {
			direction = "DESC"
		}
		orderExpressions = append(orderExpressions, expression+" "+direction)
	}
	if len(orderExpressions) > 0 {
		tx = tx.Order(strings.Join(orderExpressions, ", "))
	}

	// Fetch one extra row to learn whether another page exists.
	rows := make([]map[string]any, 0, limit+1)
	if err := tx.Limit(limit + 1).Offset(offset).Find(&rows).Error; err != nil {
		return toolErrorf("query failed: %v", err)
	}
	hasMore := len(rows) > limit
	if hasMore {
		rows = rows[:limit]
	}
	for _, row := range rows {
		for key, value := range row {
			// Some drivers return text columns as bytes, which would be
			// marshalled as base64.
			if raw, isBytes := value.([]byte); isBytes {
				row[key] = string(raw)
			}
		}
	}
	data := map[string]any{
		"table":    table.Name,
		"rows":     rows,
		"count":    len(rows),
		"limit":    limit,
		"offset":   offset,
		"has_more": hasMore,
	}
	if hasMore {
		data["next_offset"] = offset + len(rows)
	}
	return ToolResult{OK: true, Data: data}
}

// dbFilterValue turns whole JSON numbers into integers so they bind as integer
// parameters; a float parameter against an integer column defeats indexes on
// some databases.
func dbFilterValue(value any) any {
	number, ok := value.(float64)
	if !ok || number != math.Trunc(number) || math.Abs(number) >= 1e18 {
		return value
	}
	return int64(number)
}

func (e *DefaultToolExecutor) canUseDBTools() bool {
	return e.adminScope && e.adminDBTools && e.actorRole >= 100
}

func findDBTable(name string) (dbTable, bool) {
	for _, table := range adminDBTables {
		if table.Name == name {
			return table, true
		}
	}
	return dbTable{}, false
}
