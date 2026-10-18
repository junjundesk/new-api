package assistant

import (
	"time"

	"github.com/QuantumNous/new-api/model"
)

func dailyTokenUsageDay(now time.Time) int {
	return now.Year()*10000 + int(now.Month())*100 + now.Day()
}

// CheckDailyTokenQuota reports whether a user already reached the daily token
// limit and, if so, how many seconds remain until the counter resets.
func CheckDailyTokenQuota(userID, limit int) (bool, int64, error) {
	if userID <= 0 || limit <= 0 {
		return false, 0, nil
	}
	now := time.Now()
	used, err := model.GetAssistantDailyUsage(userID, dailyTokenUsageDay(now))
	if err != nil {
		return false, 0, err
	}
	if used < int64(limit) {
		return false, 0, nil
	}
	nextDay := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, now.Location())
	return true, int64(nextDay.Sub(now) / time.Second), nil
}

// RecordTokenUsage adds one run's upstream token usage to the user's daily
// counter. Runs without reported usage are ignored.
func RecordTokenUsage(userID int, usage Usage) error {
	if userID <= 0 {
		return nil
	}
	total := usage.TotalTokens
	if total <= 0 {
		total = usage.InputTokens + usage.OutputTokens
	}
	if total <= 0 {
		return nil
	}
	return model.AddAssistantDailyUsage(userID, dailyTokenUsageDay(time.Now()), int64(usage.InputTokens), int64(usage.OutputTokens), int64(total))
}
