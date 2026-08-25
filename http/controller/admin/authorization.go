package admin

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/q1ngyang/rustdesk-api-kessoku/v3/http/response"
	"github.com/q1ngyang/rustdesk-api-kessoku/v3/service"
)

func denyScopedAccess(c *gin.Context, targetType string, targetID uint) {
	actor := service.AllService.UserService.CurUser(c)
	actorID := uint(0)
	if actor != nil {
		actorID = actor.Id
	}
	service.AllService.AdminScopeService.RecordDenied(c.Request.Context(), actorID, controlRequestID(c), targetType, strconv.FormatUint(uint64(targetID), 10))
	response.FailStatus(c, http.StatusForbidden, 403, response.TranslateMsg(c, "NoAccess"))
}
