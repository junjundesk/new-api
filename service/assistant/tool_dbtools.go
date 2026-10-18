package assistant

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/model"
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
	Columns     []dbColumn
}

var adminDBTables = []dbTable{
	{Name: "users", Description: "User accounts and quota totals", Columns: []dbColumn{
		{Name: "id", Type: "int"}, {Name: "username", Type: "string"}, {Name: "display_name", Type: "string"},
		{Name: "role", Type: "int"}, {Name: "status", Type: "int"}, {Name: "group", Type: "string"},
		{Name: "quota", Type: "int"}, {Name: "used_quota", Type: "int"}, {Name: "request_count", Type: "int"},
		{Name: "created_at", Type: "timestamp"}, {Name: "last_login_at", Type: "timestamp"},
	}},
	{Name: "tokens", Description: "User API keys without secret key values", Columns: []dbColumn{
		{Name: "id", Type: "int"}, {Name: "user_id", Type: "int"}, {Name: "name", Type: "string"},
		{Name: "status", Type: "int"}, {Name: "unlimited_quota", Type: "bool"}, {Name: "remain_quota", Type: "int"},
		{Name: "used_quota", Type: "int"}, {Name: "expired_time", Type: "timestamp"}, {Name: "group", Type: "string"},
		{Name: "created_time", Type: "timestamp"},
	}},
	{Name: "channels", Description: "Upstream channels without secret key values", Columns: []dbColumn{
		{Name: "id", Type: "int"}, {Name: "type", Type: "int"}, {Name: "name", Type: "string"},
		{Name: "status", Type: "int"}, {Name: "base_url", Type: "string"}, {Name: "models", Type: "string"},
		{Name: "group", Type: "string"}, {Name: "priority", Type: "int"}, {Name: "weight", Type: "int"},
		{Name: "used_quota", Type: "int"}, {Name: "balance", Type: "float"},
	}},
	{Name: "logs", Description: "Consumption logs; content is not exposed through this tool", LogDB: true, Columns: []dbColumn{
		{Name: "id", Type: "int"}, {Name: "user_id", Type: "int"}, {Name: "username", Type: "string"},
		{Name: "type", Type: "int"}, {Name: "model_name", Type: "string"}, {Name: "token_name", Type: "string"},
		{Name: "group", Type: "string"}, {Name: "quota", Type: "int"}, {Name: "prompt_tokens", Type: "int"},
		{Name: "completion_tokens", Type: "int"}, {Name: "created_at", Type: "timestamp"}, {Name: "is_stream", Type: "bool"},
	}},
	{Name: "redemptions", Description: "Redemption codes without code values", Columns: []dbColumn{
		{Name: "id", Type: "int"}, {Name: "name", Type: "string"}, {Name: "status", Type: "int"},
		{Name: "quota", Type: "int"}, {Name: "created_time", Type: "timestamp"}, {Name: "redeemed_time", Type: "timestamp"},
	}},
	{Name: "top_ups", Description: "Top-up records", Columns: []dbColumn{
		{Name: "id", Type: "int"}, {Name: "user_id", Type: "int"}, {Name: "amount", Type: "float"},
		{Name: "money", Type: "float"}, {Name: "status", Type: "string"}, {Name: "payment_provider", Type: "string"},
		{Name: "create_time", Type: "timestamp"},
	}},
	{Name: "subscription_plans", Description: "Subscription plans", Columns: []dbColumn{
		{Name: "id", Type: "int"}, {Name: "title", Type: "string"}, {Name: "price_amount", Type: "float"},
		{Name: "currency", Type: "string"}, {Name: "enabled", Type: "bool"}, {Name: "created_at", Type: "timestamp"},
	}},
}

func (e *DefaultToolExecutor) dbDefinitions() []map[string]any {
	return []map[string]any{
		functionTool(
			"admin_db_schema",
			"List database tables and safe columns available to the root database query tool. Secret columns are never listed.",
			objectSchema(map[string]any{}, nil),
		),
		functionTool(
			"admin_db_query",
			"Run a structured read-only database query as root. Choose one allowed table, safe columns, filters, optional ordering, limit/offset, and optional aggregate metrics. Raw SQL and writes are not supported.",
			objectSchema(map[string]any{
				"table":   map[string]any{"type": "string"},
				"columns": map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
				"filters": map[string]any{
					"type": "array",
					"items": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"column": map[string]any{"type": "string"},
							"op":     map[string]any{"type": "string", "enum": []string{"=", "!=", ">", ">=", "<", "<=", "in", "like", "is_null", "not_null"}},
							"value":  map[string]any{},
						},
						"required":             []string{"column", "op"},
						"additionalProperties": false,
					},
				},
				"order_by": map[string]any{
					"type": "array",
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
				"limit":  map[string]any{"type": "integer", "minimum": 1, "maximum": 100},
				"offset": map[string]any{"type": "integer", "minimum": 0},
				"aggregate": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"group_by": map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
						"metrics": map[string]any{
							"type": "array",
							"items": map[string]any{
								"type": "object",
								"properties": map[string]any{
									"func":   map[string]any{"type": "string", "enum": []string{"count", "sum", "avg", "min", "max"}},
									"column": map[string]any{"type": "string"},
								},
								"required":             []string{"func", "column"},
								"additionalProperties": false,
							},
						},
					},
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
		columns := make([]map[string]any, 0, len(table.Columns))
		for _, column := range table.Columns {
			columns = append(columns, map[string]any{"name": column.Name, "type": column.Type, "description": column.Description})
		}
		tables = append(tables, map[string]any{"table": table.Name, "description": table.Description, "columns": columns})
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
	Table     string       `json:"table"`
	Columns   []string     `json:"columns"`
	Filters   []dbFilter   `json:"filters"`
	OrderBy   []dbOrder    `json:"order_by"`
	Limit     *int         `json:"limit"`
	Offset    *int         `json:"offset"`
	Aggregate *dbAggregate `json:"aggregate"`
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
		return toolErrorf("table is not allowed")
	}
	allowed := make(map[string]bool, len(table.Columns))
	for _, column := range table.Columns {
		allowed[column.Name] = true
	}
	columns := args.Columns
	if len(columns) == 0 {
		for _, column := range table.Columns {
			columns = append(columns, column.Name)
		}
	}
	for _, column := range columns {
		if !allowed[column] {
			return toolErrorf("column is not allowed: %s", column)
		}
	}
	limit := 50
	if args.Limit != nil {
		limit = *args.Limit
	}
	if limit < 1 || limit > 100 {
		return toolErrorf("limit must be between 1 and 100")
	}
	offset := 0
	if args.Offset != nil {
		offset = *args.Offset
	}
	if offset < 0 {
		return toolErrorf("offset must not be negative")
	}
	db := model.DB
	if table.LogDB {
		db = model.LOG_DB
	}
	if db == nil {
		return toolErrorf("database is unavailable")
	}
	tx := db.WithContext(ctx).Table(table.Name)
	if args.Aggregate != nil && len(args.Aggregate.Metrics) > 0 {
		selects := make([]string, 0)
		for _, group := range args.Aggregate.GroupBy {
			if !allowed[group] {
				return toolErrorf("group_by column is not allowed: %s", group)
			}
			selects = append(selects, group)
		}
		for _, metric := range args.Aggregate.Metrics {
			fn := strings.ToLower(metric.Func)
			switch fn {
			case "count", "sum", "avg", "min", "max":
			default:
				return toolErrorf("aggregate function is not allowed: %s", fn)
			}
			if metric.Column == "*" && fn != "count" {
				return toolErrorf("aggregate * is only allowed with count")
			}
			if metric.Column != "*" && !allowed[metric.Column] {
				return toolErrorf("aggregate column is not allowed: %s", metric.Column)
			}
			selects = append(selects, fmt.Sprintf("%s(%s) AS %s_%s", fn, metric.Column, fn, strings.ReplaceAll(metric.Column, "*", "all")))
		}
		tx = tx.Select(strings.Join(selects, ", "))
		if len(args.Aggregate.GroupBy) > 0 {
			tx = tx.Group(strings.Join(args.Aggregate.GroupBy, ", "))
		}
	} else {
		tx = tx.Select(strings.Join(columns, ", "))
	}
	for _, filter := range args.Filters {
		if !allowed[filter.Column] {
			return toolErrorf("filter column is not allowed: %s", filter.Column)
		}
		switch filter.Op {
		case "=", "!=", ">", ">=", "<", "<=":
			tx = tx.Where(filter.Column+" "+filter.Op+" ?", filter.Value)
		case "in":
			values, ok := filter.Value.([]any)
			if !ok {
				return toolErrorf("in filter requires an array value")
			}
			tx = tx.Where(filter.Column+" IN ?", values)
		case "like":
			tx = tx.Where(filter.Column+" LIKE ?", "%"+fmt.Sprint(filter.Value)+"%")
		case "is_null":
			tx = tx.Where(filter.Column + " IS NULL")
		case "not_null":
			tx = tx.Where(filter.Column + " IS NOT NULL")
		default:
			return toolErrorf("filter operator is not allowed: %s", filter.Op)
		}
	}
	orderExpressions := make([]string, 0, len(args.OrderBy))
	for _, order := range args.OrderBy {
		if !allowed[order.Column] {
			return toolErrorf("order column is not allowed: %s", order.Column)
		}
		direction := strings.ToLower(order.Direction)
		if direction != "desc" {
			direction = "asc"
		}
		orderExpressions = append(orderExpressions, order.Column+" "+direction)
	}
	if len(orderExpressions) > 0 {
		tx = tx.Order(strings.Join(orderExpressions, ", "))
	}
	tx = tx.Limit(limit).Offset(offset)
	queryCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	tx = tx.WithContext(queryCtx)
	rows := make([]map[string]any, 0, limit)
	if err := tx.Find(&rows).Error; err != nil {
		return toolErrorf("query failed: %v", err)
	}
	return ToolResult{OK: true, Data: map[string]any{
		"table":  table.Name,
		"rows":   rows,
		"count":  len(rows),
		"limit":  limit,
		"offset": offset,
	}}
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
