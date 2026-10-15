package middleware

import (
	"fmt"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
)

// activityGrantRateLimitWindowSeconds 是活动码 RPM 的固定窗口长度：一分钟。
const activityGrantRateLimitWindowSeconds = 60

// ActivityGrantRateLimit 按活动码授权的 RPM 限制用户使用活动分组的请求频率。
// 它是活动码自身的额度，独立于全局的模型请求限流开关；超限返回 429 + Retry-After。
// 每个用户、每张授权单独计数，普通分组（没有活动授权）不受影响。
func ActivityGrantRateLimit() func(c *gin.Context) {
	return func(c *gin.Context) {
		userId := c.GetInt("id")
		if userId <= 0 {
			c.Next()
			return
		}
		group := common.GetContextKeyString(c, constant.ContextKeyUsingGroup)
		if group == "" {
			group = common.GetContextKeyString(c, constant.ContextKeyUserGroup)
		}
		if group == "" {
			c.Next()
			return
		}
		view := service.GetRequestActivityView(c, userId, group)
		if view == nil || view.RpmLimit <= 0 {
			c.Next()
			return
		}
		userRateLimitFactory(view.RpmLimit, activityGrantRateLimitWindowSeconds, fmt.Sprintf("AG:%d", view.GrantId))(c)
	}
}
