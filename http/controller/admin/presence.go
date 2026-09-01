package admin

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/q1ngyang/rustdesk-api-kessoku/v3/http/response"
	"github.com/q1ngyang/rustdesk-api-kessoku/v3/service"
)

type Presence struct{}

// Metrics returns process-local request counters and database-wide active
// lease gauges. It contains no device identifiers, IP addresses, or tokens.
// @Tags Presence
// @Summary Read Presence Lease v2 metrics
// @Produce json
// @Success 200 {object} response.Response{data=service.PresenceMetricsSnapshot}
// @Failure 401 {object} response.Response
// @Failure 403 {object} response.Response
// @Failure 500 {object} response.ErrorResponse
// @Router /admin/presence/v2/metrics [get]
// @Security token
func (p *Presence) Metrics(c *gin.Context) {
	result, err := service.AllService.PeerService.PresenceMetricsSnapshot(c.Request.Context(), time.Now().Unix())
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorResponse{Error: "presence metrics unavailable"})
		return
	}
	response.Success(c, result)
}
