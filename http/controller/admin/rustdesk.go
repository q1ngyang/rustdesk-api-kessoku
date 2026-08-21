package admin

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Rustdesk only exists for one compatibility release. Even when an operator
// explicitly exposes these routes, Kessoku never accepts or forwards a raw
// command or option. All control operations are implemented by the typed
// /server-control/v1 API.
type Rustdesk struct{}

func (r *Rustdesk) deprecated(c *gin.Context) {
	c.Header("Deprecation", "true")
	c.Header("Sunset", "Wed, 18 Feb 2027 00:00:00 GMT")
	c.JSON(http.StatusGone, gin.H{
		"error": gin.H{
			"code":       "LEGACY_SERVER_COMMAND_REMOVED",
			"message":    "legacy server commands are removed; use /api/admin/server-control/v1",
			"deprecated": true,
		},
	})
}

func (r *Rustdesk) SendCmd(c *gin.Context)   { r.deprecated(c) }
func (r *Rustdesk) CmdList(c *gin.Context)   { r.deprecated(c) }
func (r *Rustdesk) CmdDelete(c *gin.Context) { r.deprecated(c) }
func (r *Rustdesk) CmdCreate(c *gin.Context) { r.deprecated(c) }
