package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/q1ngyang/rustdesk-api-kessoku/v2/global"
	internalAuth "github.com/q1ngyang/rustdesk-api-kessoku/v2/internal/auth"
	"github.com/q1ngyang/rustdesk-api-kessoku/v2/service"
)

// WebClientCORS permits only the configured, independently hosted client
// origin. It never enables cookies and never permits the admin api-token header.
func WebClientCORS() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Cover authentication failures produced by later middleware as well as
		// successful controller responses.
		c.Header("Cache-Control", "no-store")
		c.Header("Pragma", "no-cache")
		origin := c.GetHeader("Origin")
		if origin == "" || global.Config.WebClient.Enabled() && origin == global.Config.WebClient.APIOrigin {
			if c.Request.Method == http.MethodOptions {
				c.AbortWithStatus(http.StatusForbidden)
				return
			}
			c.Next()
			return
		}
		if !global.Config.WebClient.Enabled() || origin != global.Config.WebClient.PublicOrigin {
			c.AbortWithStatus(http.StatusForbidden)
			return
		}
		c.Header("Access-Control-Allow-Origin", origin)
		c.Header("Vary", "Origin")
		if c.Request.Method == http.MethodOptions {
			if c.GetHeader("Access-Control-Request-Method") != http.MethodPost || !allowedWebClientHeaders(c.GetHeader("Access-Control-Request-Headers")) {
				c.AbortWithStatus(http.StatusForbidden)
				return
			}
			c.Header("Access-Control-Allow-Methods", http.MethodPost)
			c.Header("Access-Control-Allow-Headers", "Authorization, Content-Type")
			c.Header("Access-Control-Max-Age", "600")
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

func allowedWebClientHeaders(value string) bool {
	if strings.TrimSpace(value) == "" {
		return true
	}
	for _, header := range strings.Split(value, ",") {
		switch strings.ToLower(strings.TrimSpace(header)) {
		case "authorization", "content-type":
		default:
			return false
		}
	}
	return true
}

// WebClientConnectionAuth accepts only a DB-authoritative, non-revoked token
// with the Starry connection audience and scope. API/admin audience tokens are
// not needed by the logout route and are not accepted as a substitute.
func WebClientConnectionAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		parts := strings.Fields(c.GetHeader("Authorization"))
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || parts[1] == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
			c.Abort()
			return
		}
		token := parts[1]
		user, _, claims, err := service.AllService.UserService.AuthenticateAccessTokenContext(
			c.Request.Context(), token, internalAuth.ConnectionAudience, internalAuth.ConnectScope,
		)
		if err != nil || claims == nil || user.Id == 0 {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
			c.Abort()
			return
		}
		c.Set("curUser", user)
		c.Set("token", token)
		c.Next()
	}
}
