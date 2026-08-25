package middleware

import (
	"github.com/gin-gonic/gin"
	"github.com/q1ngyang/rustdesk-api-kessoku/v3/global"
	"github.com/q1ngyang/rustdesk-api-kessoku/v3/http/response"
	"net/http"
)

func Limiter() gin.HandlerFunc {
	return func(c *gin.Context) {
		loginLimiter := global.LoginLimiter
		clientIp := c.ClientIP()
		banned, _ := loginLimiter.CheckSecurityStatus(clientIp)
		if banned {
			response.Fail(c, http.StatusLocked, response.TranslateMsg(c, "Banned"))
			c.Abort()
			return
		}
		c.Next()
	}
}
