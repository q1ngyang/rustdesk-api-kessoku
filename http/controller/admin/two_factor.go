package admin

import (
	"github.com/gin-gonic/gin"
	"github.com/q1ngyang/rustdesk-api-kessoku/v3/http/response"
	"github.com/q1ngyang/rustdesk-api-kessoku/v3/service"
	"github.com/q1ngyang/rustdesk-api-kessoku/v3/utils"
)

type twoFactorCodeRequest struct {
	Code string `json:"code" binding:"required,len=6,numeric"`
}

type twoFactorBeginRequest struct {
	Password string `json:"password" binding:"max=256"`
}

func (ct *User) TwoFactorStatus(c *gin.Context) {
	user := service.AllService.UserService.CurUser(c)
	response.Success(c, gin.H{"available": service.AllService.TwoFactorService.Available(), "enabled": service.AllService.TwoFactorService.Status(user.Id)})
}

func (ct *User) BeginTwoFactor(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	request := &twoFactorBeginRequest{}
	if err := c.ShouldBindJSON(request); err != nil {
		response.Fail(c, 101, response.TranslateMsg(c, "ParamsError"))
		return
	}
	user := service.AllService.UserService.CurUser(c)
	if !service.AllService.UserService.IsPasswordEmptyByUser(user) {
		ok, _, err := utils.VerifyPassword(user.Password, request.Password)
		if err != nil || !ok {
			response.Fail(c, 101, response.TranslateMsg(c, "OldPasswordError"))
			return
		}
	}
	secret, uri, err := service.AllService.TwoFactorService.BeginSetup(user)
	if err != nil {
		response.Fail(c, 101, err.Error())
		return
	}
	response.Success(c, gin.H{"secret": secret, "otpauth_uri": uri})
}

func (ct *User) ConfirmTwoFactor(c *gin.Context) {
	request := &twoFactorCodeRequest{}
	if err := c.ShouldBindJSON(request); err != nil {
		response.Fail(c, 101, response.TranslateMsg(c, "ParamsError"))
		return
	}
	user := service.AllService.UserService.CurUser(c)
	if err := service.AllService.TwoFactorService.ConfirmSetup(c.Request.Context(), user, controlRequestID(c), request.Code); err != nil {
		response.Fail(c, 101, err.Error())
		return
	}
	response.Success(c, gin.H{"enabled": true, "sessions_revoked": true})
}

func (ct *User) DisableTwoFactor(c *gin.Context) {
	request := &twoFactorCodeRequest{}
	if err := c.ShouldBindJSON(request); err != nil {
		response.Fail(c, 101, response.TranslateMsg(c, "ParamsError"))
		return
	}
	user := service.AllService.UserService.CurUser(c)
	if err := service.AllService.TwoFactorService.Disable(c.Request.Context(), user, controlRequestID(c), request.Code); err != nil {
		response.Fail(c, 101, err.Error())
		return
	}
	response.Success(c, gin.H{"enabled": false, "sessions_revoked": true})
}
