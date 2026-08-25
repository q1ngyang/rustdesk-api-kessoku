package web

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/q1ngyang/rustdesk-api-kessoku/v3/global"
)

// ClientConfig returns only immutable public connection trust data. Listener,
// token lifetime, users, credentials, and internal/control endpoints are not
// represented in this DTO.
func (i *Index) ClientConfig(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	c.Header("Pragma", "no-cache")
	c.JSON(http.StatusOK, global.Config.WebClient.PublicConfig())
}
