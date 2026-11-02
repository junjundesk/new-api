package middleware

import (
	"strconv"

	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
)

// ActivityGrantRateLimit 在 Distribute 之前建立活动限流的请求级状态：
// 把请求套进一层可取消 context（并发槽失租即取消请求），并把会话挂到 gin context，
// 供实际分组确定后的 service.ActivityLimitGuard 使用；请求结束时统一释放该请求在
// 各分组持有的并发槽（每请求每分组至多一个）。RPM 与并发的额度检查发生在实际定组后、
// 上游调用前，覆盖 playground、auto/chain 选组、跨组 fallback 与渠道重试。
func ActivityGrantRateLimit() func(c *gin.Context) {
	return func(c *gin.Context) {
		session := service.BeginActivityConcurrencySession(c)
		if session == nil {
			c.Next()
			return
		}
		defer session.Release()
		c.Next()
	}
}

// abortWithActivityLimit 输出活动限流拒绝：并发满/超频返回 429，暂时无法验证限制
// 返回 503。RPM 超限带真实窗口剩余秒数的 Retry-After，并发超限不伪造。
func abortWithActivityLimit(c *gin.Context, limitErr *service.ActivityLimitError) {
	if limitErr.RetryAfterSeconds > 0 {
		c.Header("Retry-After", strconv.FormatInt(limitErr.RetryAfterSeconds, 10))
	}
	abortWithOpenAiMessage(c, limitErr.StatusCode, limitErr.Message, types.ErrorCode(limitErr.Code))
}
