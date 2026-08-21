package middleware

import (
	"github.com/gin-gonic/gin"
	"github.com/q1ngyang/rustdesk-api-kessoku/v2/http/response"
	"github.com/q1ngyang/rustdesk-api-kessoku/v2/service"
	"net/http"
)

// BackendUserAuth 后台权限验证中间件
func BackendUserAuth() gin.HandlerFunc {
	return func(c *gin.Context) {

		//测试先关闭
		token := c.GetHeader("api-token")
		if token == "" {
			response.FailStatus(c, http.StatusUnauthorized, 401, response.TranslateMsg(c, "NeedLogin"))
			c.Abort()
			return
		}
		user, _ := service.AllService.UserService.InfoByAccessTokenContext(c.Request.Context(), token)
		if user.Id == 0 {
			response.FailStatus(c, http.StatusUnauthorized, 401, response.TranslateMsg(c, "NeedLogin"))
			c.Abort()
			return
		}

		if !service.AllService.UserService.CheckUserEnable(user) {
			c.JSON(401, gin.H{
				"error": "Unauthorized",
			})
			c.Abort()
			return
		}

		c.Set("curUser", user)
		c.Set("token", token)
		c.Next()
	}
}
