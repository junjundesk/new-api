package operation_setting

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/config"
)

const (
	ErrorMappingEnabledOptionKey = "error_mapping_setting.enabled"
	ErrorMappingRulesOptionKey   = "error_mapping_setting.rules"

	MaxErrorMappingRules = 200
)

// ErrorMappingRule defines one client-facing error response rewrite rule.
// StatusCode == 0 matches any non-200 error status code (requires MatchMessage).
// TargetStatus == 0 keeps the original HTTP status code.
type ErrorMappingRule struct {
	Enabled       bool   `json:"enabled"`
	StatusCode    int    `json:"status_code"`
	MatchMessage  string `json:"match_message,omitempty"`
	TargetStatus  int    `json:"target_status,omitempty"`
	TargetMessage string `json:"target_message"`
}

type ErrorMappingSetting struct {
	Enabled bool               `json:"enabled"`
	Rules   []ErrorMappingRule `json:"rules"`
}

var (
	errorMappingMu      sync.Mutex
	errorMappingSetting = ErrorMappingSetting{
		Enabled: false,
		Rules:   []ErrorMappingRule{},
	}
)

type compiledErrorMappingRule struct {
	matchLower    string
	targetStatus  int
	targetMessage string
}

type errorMappingIndex struct {
	enabled             bool
	byStatusWithKeyword map[int][]compiledErrorMappingRule
	byStatus            map[int]compiledErrorMappingRule
	byKeyword           []compiledErrorMappingRule
}

var currentErrorMappingIndex atomic.Pointer[errorMappingIndex]

func init() {
	config.GlobalConfig.Register("error_mapping_setting", &errorMappingSetting)
	RebuildErrorMappingIndex()
}

func isValidMappedStatusCode(code int) bool {
	if code == 0 {
		return true
	}
	return code >= 100 && code <= 599 && code != http.StatusOK
}

func decodeErrorMappingRulesJSON(value string, ignoreInvalidEntries bool) ([]ErrorMappingRule, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return []ErrorMappingRule{}, nil
	}

	rawValue := json.RawMessage(trimmed)
	if common.GetJsonType(rawValue) != "array" {
		return nil, fmt.Errorf("错误码映射规则必须是 JSON 数组")
	}

	var rawRules []ErrorMappingRule
	if err := common.Unmarshal(rawValue, &rawRules); err != nil {
		return nil, fmt.Errorf("解析错误码映射规则失败: %w", err)
	}
	if len(rawRules) > MaxErrorMappingRules {
		return nil, fmt.Errorf("错误码映射规则数量不能超过 %d 条", MaxErrorMappingRules)
	}

	rules := make([]ErrorMappingRule, 0, len(rawRules))
	for i, rule := range rawRules {
		rule.MatchMessage = strings.TrimSpace(rule.MatchMessage)
		rule.TargetMessage = strings.TrimSpace(rule.TargetMessage)

		var entryErr error
		switch {
		case !isValidMappedStatusCode(rule.StatusCode):
			entryErr = fmt.Errorf("第 %d 条规则的匹配状态码 %d 无效（允许 0 或 100-599 且不能为 200）", i+1, rule.StatusCode)
		case !isValidMappedStatusCode(rule.TargetStatus):
			entryErr = fmt.Errorf("第 %d 条规则的映射后状态码 %d 无效（允许 0 或 100-599 且不能为 200）", i+1, rule.TargetStatus)
		case rule.StatusCode == 0 && rule.MatchMessage == "":
			entryErr = fmt.Errorf("第 %d 条规则必须至少填写匹配状态码或错误原因关键字", i+1)
		case rule.TargetMessage == "":
			entryErr = fmt.Errorf("第 %d 条规则的映射后错误原因不能为空", i+1)
		}

		if entryErr != nil {
			if !ignoreInvalidEntries {
				return nil, entryErr
			}
			common.SysError(entryErr.Error())
			continue
		}
		rules = append(rules, rule)
	}
	return rules, nil
}

// ValidateErrorMappingRulesJSON validates an admin-supplied JSON rule list.
func ValidateErrorMappingRulesJSON(value string) error {
	_, err := decodeErrorMappingRulesJSON(value, false)
	return err
}

// SetErrorMappingEnabled updates the global toggle and rebuilds the lookup index.
func SetErrorMappingEnabled(enabled bool) {
	errorMappingMu.Lock()
	errorMappingSetting.Enabled = enabled
	errorMappingMu.Unlock()
	RebuildErrorMappingIndex()
}

// LoadErrorMappingRulesFromJSONString updates the configured rule list and rebuilds the lookup index.
func LoadErrorMappingRulesFromJSONString(value string) {
	rules, err := decodeErrorMappingRulesJSON(value, true)
	if err != nil {
		common.SysError("加载错误码映射规则失败，已回退为空列表: " + err.Error())
		rules = []ErrorMappingRule{}
	}
	errorMappingMu.Lock()
	errorMappingSetting.Rules = rules
	errorMappingMu.Unlock()
	RebuildErrorMappingIndex()
}

// RebuildErrorMappingIndex compiles the current setting into an immutable atomic lookup table.
func RebuildErrorMappingIndex() {
	errorMappingMu.Lock()
	enabled := errorMappingSetting.Enabled
	rules := make([]ErrorMappingRule, len(errorMappingSetting.Rules))
	copy(rules, errorMappingSetting.Rules)
	errorMappingMu.Unlock()

	idx := &errorMappingIndex{
		enabled:             enabled,
		byStatusWithKeyword: make(map[int][]compiledErrorMappingRule),
		byStatus:            make(map[int]compiledErrorMappingRule),
	}

	if enabled {
		for _, rule := range rules {
			if !rule.Enabled {
				continue
			}
			targetMsg := strings.TrimSpace(rule.TargetMessage)
			matchMsg := strings.TrimSpace(rule.MatchMessage)
			if targetMsg == "" || !isValidMappedStatusCode(rule.StatusCode) || !isValidMappedStatusCode(rule.TargetStatus) {
				continue
			}
			compiled := compiledErrorMappingRule{
				matchLower:    strings.ToLower(matchMsg),
				targetStatus:  rule.TargetStatus,
				targetMessage: targetMsg,
			}
			switch {
			case rule.StatusCode > 0 && compiled.matchLower != "":
				idx.byStatusWithKeyword[rule.StatusCode] = append(idx.byStatusWithKeyword[rule.StatusCode], compiled)
			case rule.StatusCode > 0:
				if _, exists := idx.byStatus[rule.StatusCode]; !exists {
					idx.byStatus[rule.StatusCode] = compiled
				}
			case compiled.matchLower != "":
				idx.byKeyword = append(idx.byKeyword, compiled)
			}
		}
	}

	currentErrorMappingIndex.Store(idx)
}

// MatchErrorMapping matches an error status code and raw message against the
// active rules. Priority:
//  1. Exact status code + message keyword (first matching rule)
//  2. Exact status code catch-all (first configured rule)
//  3. Any status code + message keyword (first matching rule)
func MatchErrorMapping(statusCode int, message string) (int, string, bool) {
	if statusCode == http.StatusOK {
		return statusCode, message, false
	}
	idx := currentErrorMappingIndex.Load()
	if idx == nil || !idx.enabled {
		return statusCode, message, false
	}

	lowerMsg := strings.ToLower(message)

	if candidates := idx.byStatusWithKeyword[statusCode]; len(candidates) > 0 && lowerMsg != "" {
		for _, rule := range candidates {
			if strings.Contains(lowerMsg, rule.matchLower) {
				return resolveMappedStatus(statusCode, rule.targetStatus), rule.targetMessage, true
			}
		}
	}

	if rule, ok := idx.byStatus[statusCode]; ok {
		return resolveMappedStatus(statusCode, rule.targetStatus), rule.targetMessage, true
	}

	if len(idx.byKeyword) > 0 && lowerMsg != "" {
		for _, rule := range idx.byKeyword {
			if strings.Contains(lowerMsg, rule.matchLower) {
				return resolveMappedStatus(statusCode, rule.targetStatus), rule.targetMessage, true
			}
		}
	}

	return statusCode, message, false
}

func resolveMappedStatus(originalStatus int, targetStatus int) int {
	if targetStatus > 0 {
		return targetStatus
	}
	return originalStatus
}

// SetErrorMappingForTest sets the active error mapping state and returns a
// restore callback for deterministic tests.
func SetErrorMappingForTest(enabled bool, rules []ErrorMappingRule) func() {
	errorMappingMu.Lock()
	prevEnabled := errorMappingSetting.Enabled
	prevRules := make([]ErrorMappingRule, len(errorMappingSetting.Rules))
	copy(prevRules, errorMappingSetting.Rules)

	errorMappingSetting.Enabled = enabled
	errorMappingSetting.Rules = make([]ErrorMappingRule, len(rules))
	copy(errorMappingSetting.Rules, rules)
	errorMappingMu.Unlock()
	RebuildErrorMappingIndex()

	return func() {
		errorMappingMu.Lock()
		errorMappingSetting.Enabled = prevEnabled
		errorMappingSetting.Rules = prevRules
		errorMappingMu.Unlock()
		RebuildErrorMappingIndex()
	}
}
