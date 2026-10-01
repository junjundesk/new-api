package model

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// ParseBonusDuration parses a human-friendly bonus validity period.
//
// A bare number is interpreted as hours ("1" => 1 hour), matching the admin
// form default. Explicit units may be mixed and are resolved to their calendar
// equivalents: 年/月/周/天/小时/分钟/秒 (also accepts y/mo/w/d/h/m/s and the
// English words). Examples that parse:
//
//	"1"          => 1h
//	"1.5"        => 1h30m
//	"1天"        => 24h
//	"1天2小时30分" => 26h30m
//	"2周"        => 14 days
//	"1年"        => 365 days
func ParseBonusDuration(input string) (time.Duration, error) {
	s := strings.TrimSpace(input)
	if s == "" {
		return 0, errors.New("bonus duration is empty")
	}

	// Bare number => hours (default unit).
	if v, err := strconv.ParseFloat(s, 64); err == nil {
		if v <= 0 {
			return 0, errors.New("bonus duration must be positive")
		}
		return time.Duration(v * float64(time.Hour)), nil
	}

	matches := bonusDurationSegmentPattern.FindAllStringSubmatch(s, -1)
	if len(matches) == 0 {
		return 0, fmt.Errorf("invalid bonus duration: %q", input)
	}

	// Every non-space, non-separator character must belong to a parsed segment;
	// otherwise "1天abc" would silently parse as 1 day.
	remainder := s
	var totalSeconds float64
	for _, m := range matches {
		value, err := strconv.ParseFloat(m[1], 64)
		if err != nil {
			return 0, fmt.Errorf("invalid bonus duration value %q", m[1])
		}
		if value <= 0 {
			return 0, errors.New("bonus duration must be positive")
		}
		seconds, ok := bonusDurationUnitSeconds(m[2])
		if !ok {
			return 0, fmt.Errorf("unknown bonus duration unit: %q", m[2])
		}
		totalSeconds += value * seconds
		remainder = strings.Replace(remainder, m[0], "", 1)
	}
	if strings.Trim(remainder, " \t,") != "" {
		return 0, fmt.Errorf("invalid bonus duration: %q", input)
	}
	if totalSeconds <= 0 {
		return 0, errors.New("bonus duration must be positive")
	}
	return time.Duration(totalSeconds * float64(time.Second)), nil
}

var bonusDurationSegmentPattern = regexp.MustCompile(`(\d+(?:\.\d+)?)\s*([^\d\s,]+)`)

// bonusDurationUnitSeconds maps a unit token to its second count. Calendar
// months/years use the conventional 30/365 day approximations because bonus
// expiry is a business window, not a calendar arithmetic operation.
func bonusDurationUnitSeconds(unit string) (float64, bool) {
	u := strings.ToLower(strings.TrimSpace(unit))
	u = strings.TrimPrefix(u, "个")
	u = strings.TrimSuffix(u, "钟")
	switch u {
	case "年", "y", "yr", "year", "years":
		return 365 * 24 * 3600, true
	case "月", "mo", "mon", "month", "months":
		return 30 * 24 * 3600, true
	case "周", "星期", "w", "wk", "week", "weeks":
		return 7 * 24 * 3600, true
	case "天", "日", "d", "day", "days":
		return 24 * 3600, true
	case "小时", "时", "h", "hr", "hour", "hours":
		return 3600, true
	case "分", "分钟", "m", "min", "mins", "minute", "minutes":
		return 60, true
	case "秒", "s", "sec", "secs", "second", "seconds":
		return 1, true
	default:
		return 0, false
	}
}
