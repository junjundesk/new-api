package model

import "github.com/QuantumNous/new-api/common"

// dbUnixTimeSQL is a trusted, dialect-specific SQL expression. SQLite's
// strftime returns text, so cast explicitly for numerical expiry comparisons.
func dbUnixTimeSQL() string {
	switch {
	case common.UsingMainDatabase(common.DatabaseTypePostgreSQL):
		return "EXTRACT(EPOCH FROM NOW())::bigint"
	case common.UsingMainDatabase(common.DatabaseTypeSQLite):
		return "CAST(strftime('%s','now') AS INTEGER)"
	default:
		return "UNIX_TIMESTAMP()"
	}
}

// GetDBTimestamp returns a UNIX timestamp from database time.
// Falls back to application time on error.
func GetDBTimestamp() int64 {
	var ts int64
	err := DB.Raw("SELECT " + dbUnixTimeSQL()).Scan(&ts).Error
	if err != nil || ts <= 0 {
		return common.GetTimestamp()
	}
	return ts
}
