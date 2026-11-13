package middleware

import (
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func CORS() gin.HandlerFunc {
	config := cors.DefaultConfig()
	config.AllowAllOrigins = true
	// 不发送 Access-Control-Allow-Credentials：会话 cookie 为 SameSite=Strict，
	// 跨源请求本就不携带凭证，全开 Origins 叠加凭证放行只会扩大攻击面。
	config.AllowCredentials = false
	config.AllowMethods = []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"}
	config.AllowHeaders = []string{"*"}
	return cors.New(config)
}
