package api

import (
	"github.com/gin-gonic/gin"
	requstform "github.com/q1ngyang/rustdesk-api-kessoku/v3/http/request/api"
	"github.com/q1ngyang/rustdesk-api-kessoku/v3/http/response"
	"github.com/q1ngyang/rustdesk-api-kessoku/v3/service"
	"github.com/q1ngyang/rustdesk-api-kessoku/v3/utils"
	"net/http"
	"time"
)

type Index struct {
}

// Index 首页
// @Tags 首页
// @Summary 首页
// @Description 首页
// @Accept  json
// @Produce  json
// @Success 200 {object} response.Response
// @Failure 500 {object} response.Response
// @Router / [get]
func (i *Index) Index(c *gin.Context) {
	response.Success(
		c,
		"Hello Gwen",
	)
}

// Heartbeat 心跳
// @Tags 首页
// @Summary 心跳
// @Description 心跳
// @Accept  json
// @Produce  json
// @Success 200 {object} nil
// @Failure 500 {object} response.Response
// @Router /heartbeat [post]
func (i *Index) Heartbeat(c *gin.Context) {
	info := &requstform.PeerInfoInHeartbeat{}
	err := c.ShouldBindJSON(info)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{})
		return
	}
	info.Id = utils.NormalizeRustDeskID(info.Id)
	if info.Id == "" || info.Uuid == "" {
		c.JSON(http.StatusOK, gin.H{})
		return
	}
	peer, err := service.AllService.PeerService.ResolveReportIdentity(c.Request.Context(), info.Id, info.Uuid)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{})
		return
	}
	now := time.Now().Unix()
	refreshSysinfo := service.AllService.PeerService.NeedsSysinfoRefresh(peer, now)
	//如果在40s以内则不更新
	if now-peer.LastOnlineTime >= 30 {
		_ = service.AllService.PeerService.UpdatePresence(peer, c.ClientIP(), now)
	}
	if refreshSysinfo {
		c.JSON(http.StatusOK, gin.H{"sysinfo": true})
		return
	}
	c.JSON(http.StatusOK, gin.H{})
}

// Version 版本
// @Tags 首页
// @Summary 版本
// @Description 版本
// @Accept  json
// @Produce  json
// @Success 200 {object} response.Response
// @Failure 500 {object} response.Response
// @Router /version [get]
func (i *Index) Version(c *gin.Context) {
	//读取resources/version文件
	v := service.AllService.AppService.GetAppVersion()
	response.Success(
		c,
		v,
	)
}
