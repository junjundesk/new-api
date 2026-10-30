package service

import (
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
)

// recordActivityTokenUsage 把本次请求的实际 token 用量累加到 Token 套餐授权。
// 只有计费解析选中了需要计量的套餐时才执行；失败只记日志，不影响请求结果。
func recordActivityTokenUsage(relayInfo *relaycommon.RelayInfo, tokens int64) {
	if relayInfo == nil || tokens <= 0 {
		return
	}
	grantId := relayInfo.PriceData.GroupRatioInfo.ActivityGrantId
	if grantId <= 0 {
		return
	}
	if err := model.AddActivityGrantTokenUsage(grantId, tokens); err != nil {
		common.SysError("failed to add activity grant token usage: " + err.Error())
	}
}
