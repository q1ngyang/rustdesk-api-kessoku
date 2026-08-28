package admin

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/q1ngyang/rustdesk-api-kessoku/v3/http/response"
	"github.com/q1ngyang/rustdesk-api-kessoku/v3/service"
)

func (ct *User) UploadAvatar(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		response.Fail(c, 101, response.TranslateMsg(c, "ParamsError"))
		return
	}
	opened, err := file.Open()
	if err != nil {
		response.Fail(c, 101, response.TranslateMsg(c, "OperationFailed"))
		return
	}
	defer opened.Close()
	mediaURL, err := service.StoreImage(opened, "avatars")
	if err != nil {
		response.Fail(c, http.StatusUnprocessableEntity, err.Error())
		return
	}
	user := service.AllService.UserService.CurUser(c)
	if err := service.AllService.UserService.UpdateAvatarContext(c.Request.Context(), user, controlRequestID(c), mediaURL); err != nil {
		response.Fail(c, 101, err.Error())
		return
	}
	absolute := strings.TrimRight(service.Config.Rustdesk.ApiServer, "/") + mediaURL
	response.Success(c, gin.H{"avatar": mediaURL, "official_client_avatar": absolute})
}
