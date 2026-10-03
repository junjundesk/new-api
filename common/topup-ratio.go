package common

import (
	"encoding/json"
	"sync"
)

var topupGroupRatio = map[string]float64{
	"default": 1,
	"vip":     1,
	"svip":     1,
}
var topupGroupRatioMutex sync.RWMutex

// 该字段已不再作为充值倍率使用：充值价格计算处将非 0 值视为 1（见 getPayMoney 等）。
// 现在 0 值表示该分组禁止非流式访问（relay 侧返回 403）。
func TopupGroupRatio2JSONString() string {
	topupGroupRatioMutex.RLock()
	defer topupGroupRatioMutex.RUnlock()
	jsonBytes, err := json.Marshal(topupGroupRatio)
	if err != nil {
		SysError("error marshalling topup group ratio: " + err.Error())
	}
	return string(jsonBytes)
}

func UpdateTopupGroupRatioByJSONString(jsonStr string) error {
	topupGroupRatioMutex.Lock()
	defer topupGroupRatioMutex.Unlock()
	topupGroupRatio = make(map[string]float64)
	return json.Unmarshal([]byte(jsonStr), &topupGroupRatio)
}

func GetTopupGroupRatio(name string) float64 {
	topupGroupRatioMutex.RLock()
	defer topupGroupRatioMutex.RUnlock()
	ratio, ok := topupGroupRatio[name]
	if !ok {
		SysError("topup group ratio not found: " + name)
		return 1
	}
	return ratio
}

// IsNonStreamingForbidden returns true when the group is configured to reject
// non-streaming API requests.
func IsNonStreamingForbidden(group string) bool {
	topupGroupRatioMutex.RLock()
	defer topupGroupRatioMutex.RUnlock()
	ratio, ok := topupGroupRatio[group]
	return ok && ratio == 0
}
