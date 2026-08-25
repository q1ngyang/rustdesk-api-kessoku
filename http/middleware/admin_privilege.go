package middleware

import (
	"github.com/gin-gonic/gin"
	"github.com/q1ngyang/rustdesk-api-kessoku/v3/http/response"
	"github.com/q1ngyang/rustdesk-api-kessoku/v3/service"
	"net/http"
)

// AdminPrivilege ...
func AdminPrivilege() gin.HandlerFunc {
	return func(c *gin.Context) {
		u := service.AllService.UserService.CurUser(c)

		if !service.AllService.UserService.IsAdmin(u) {
			response.FailStatus(c, http.StatusForbidden, 403, response.TranslateMsg(c, "NoAccess"))
			c.Abort()
			return
		}

		c.Next()
	}
}

// SuperAdminPrivilege protects global configuration and delegation endpoints.
func SuperAdminPrivilege() gin.HandlerFunc {
	return func(c *gin.Context) {
		u := service.AllService.UserService.CurUser(c)
		if !service.AllService.UserService.IsSuperAdmin(u) {
			response.FailStatus(c, http.StatusForbidden, 403, response.TranslateMsg(c, "NoAccess"))
			c.Abort()
			return
		}
		c.Next()
	}
}
