package api

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	requstform "github.com/q1ngyang/rustdesk-api-kessoku/v3/http/request/api"
	"github.com/q1ngyang/rustdesk-api-kessoku/v3/http/response"
	"github.com/q1ngyang/rustdesk-api-kessoku/v3/service"
	"net/http"
	"strings"
)

const maxClientReportBytes = 64 << 10

type Peer struct {
}

// SysInfo
// @Tags System
// @Summary 提交系统信息
// @Description 提交系统信息
// @Accept  json
// @Produce  json
// @Param body body requstform.PeerForm true "系统信息表单"
// @Success 200 {string} string "SYSINFO_UPDATED,ID_NOT_FOUND"
// @Failure 500 {object} response.ErrorResponse
// @Router /sysinfo [post]
func (p *Peer) SysInfo(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxClientReportBytes)
	f := &requstform.PeerForm{}
	err := c.ShouldBindBodyWith(f, binding.JSON)
	if err != nil {
		response.Error(c, response.TranslateMsg(c, "ParamsError")+err.Error())
		return
	}
	if !validPeerReport(f) {
		response.Error(c, response.TranslateMsg(c, "ParamsError"))
		return
	}
	fpe := f.ToPeer()
	pe := service.AllService.PeerService.FindById(f.Id)
	if pe.RowId == 0 {
		userID := service.AllService.UserService.FindLatestUserIdFromLoginLogByUuid(f.Uuid, f.Id)
		if userID == 0 {
			c.String(http.StatusOK, "ID_NOT_FOUND")
			return
		}
		pe = fpe
		pe.UserId = userID
		err = service.AllService.PeerService.Create(pe)
		if err != nil {
			response.Error(c, response.TranslateMsg(c, "OperationFailed")+err.Error())
			return
		}
	} else {
		if pe.Uuid == "" || pe.Uuid != f.Uuid {
			c.String(http.StatusOK, "ID_NOT_FOUND")
			return
		}
		if pe.UserId == 0 {
			pe.UserId = service.AllService.UserService.FindLatestUserIdFromLoginLogByUuid(pe.Uuid, pe.Id)
		}
		fpe.RowId = pe.RowId
		fpe.UserId = pe.UserId
		err = service.AllService.PeerService.Update(fpe)
		if err != nil {
			response.Error(c, response.TranslateMsg(c, "OperationFailed")+err.Error())
			return
		}
	}
	//SYSINFO_UPDATED 上传成功
	//ID_NOT_FOUND 下次心跳会上传
	//直接响应文本
	c.String(http.StatusOK, "SYSINFO_UPDATED")
}

func validPeerReport(form *requstform.PeerForm) bool {
	return form != nil && boundedReportField(form.Id, 128) && boundedReportField(form.Uuid, 256) &&
		boundedOptionalReportField(form.Cpu, 1024) && boundedOptionalReportField(form.Hostname, 512) &&
		boundedOptionalReportField(form.Memory, 128) && boundedOptionalReportField(form.Os, 128) &&
		boundedOptionalReportField(form.Username, 512) && boundedOptionalReportField(form.Version, 128)
}

func boundedReportField(value string, maximum int) bool {
	return value != "" && boundedOptionalReportField(value, maximum)
}

func boundedOptionalReportField(value string, maximum int) bool {
	return len(value) <= maximum && !strings.ContainsRune(value, '\x00')
}

// SysInfoVer
// @Tags System
// @Summary 获取系统版本信息
// @Description 获取系统版本信息
// @Accept  json
// @Produce  json
// @Success 200 {string} string ""
// @Failure 500 {object} response.ErrorResponse
// @Router /sysinfo_ver [post]
func (p *Peer) SysInfoVer(c *gin.Context) {
	//读取resources/version文件
	v := service.AllService.AppService.GetAppVersion()
	// 加上启动时间，方便client上传信息
	v = fmt.Sprintf("%s\n%s", v, service.AllService.AppService.GetStartTime())
	c.String(http.StatusOK, v)
}
