package api

import (
	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	request "github.com/q1ngyang/rustdesk-api-kessoku/v3/http/request/api"
	"github.com/q1ngyang/rustdesk-api-kessoku/v3/http/response"
	"github.com/q1ngyang/rustdesk-api-kessoku/v3/model"
	"github.com/q1ngyang/rustdesk-api-kessoku/v3/service"
	"github.com/q1ngyang/rustdesk-api-kessoku/v3/utils"
	"net/http"
	"time"
)

type Audit struct {
}

// AuditConn
// @Tags 审计
// @Summary 审计连接
// @Description 审计连接
// @Accept  json
// @Produce  json
// @Param body body request.AuditConnForm true "审计连接"
// @Success 200 {string} string ""
// @Failure 500 {object} response.Response
// @Router /audit/conn [post]
func (a *Audit) AuditConn(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxClientReportBytes)
	af := &request.AuditConnForm{}
	err := c.ShouldBindBodyWith(af, binding.JSON)
	if err != nil {
		response.Error(c, response.TranslateMsg(c, "ParamsError")+err.Error())
		return
	}
	af.Id = utils.NormalizeRustDeskID(af.Id)
	if len(af.Peer) > 0 {
		af.Peer[0] = utils.NormalizeRustDeskID(af.Peer[0])
	}
	if !validAuditConnReport(af) || !peerReportIdentityMatches(af.Id, af.Uuid) {
		response.Error(c, response.TranslateMsg(c, "ParamsError"))
		return
	}
	/*ttt := &gin.H{}
	c.ShouldBindBodyWith(ttt, binding.JSON)
	fmt.Println(ttt)*/
	ac := af.ToAuditConn()
	if af.Action == model.AuditActionNew {
		if err := service.AllService.AuditService.CreateAuditConn(ac); err != nil {
			response.Error(c, response.TranslateMsg(c, "OperationFailed"))
			return
		}
	} else if af.Action == model.AuditActionClose {
		ex := service.AllService.AuditService.InfoByPeerIdAndConnId(af.Id, af.ConnId)
		if ex.Id != 0 {
			if ex.Uuid != af.Uuid {
				response.Error(c, response.TranslateMsg(c, "ParamsError"))
				return
			}
			ex.CloseTime = time.Now().Unix()
			if err := service.AllService.AuditService.UpdateAuditConn(ex); err != nil {
				response.Error(c, response.TranslateMsg(c, "OperationFailed"))
				return
			}
		}
	} else if af.Action == "" {
		ex := service.AllService.AuditService.InfoByPeerIdAndConnId(af.Id, af.ConnId)
		if ex.Id != 0 {
			up := &model.AuditConn{
				IdModel:   model.IdModel{Id: ex.Id},
				FromPeer:  ac.FromPeer,
				FromName:  ac.FromName,
				SessionId: ac.SessionId,
				Type:      ac.Type,
			}
			if ex.Uuid != af.Uuid {
				response.Error(c, response.TranslateMsg(c, "ParamsError"))
				return
			}
			if err := service.AllService.AuditService.UpdateAuditConn(up); err != nil {
				response.Error(c, response.TranslateMsg(c, "OperationFailed"))
				return
			}
		}
	}
	response.Success(c, "")
}

// AuditFile
// @Tags 审计
// @Summary 审计文件
// @Description 审计文件
// @Accept  json
// @Produce  json
// @Param body body request.AuditFileForm true "审计文件"
// @Success 200 {string} string ""
// @Failure 500 {object} response.Response
// @Router /audit/file [post]
func (a *Audit) AuditFile(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxClientReportBytes)
	aff := &request.AuditFileForm{}
	err := c.ShouldBindBodyWith(aff, binding.JSON)
	if err != nil {
		response.Error(c, response.TranslateMsg(c, "ParamsError")+err.Error())
		return
	}
	aff.Id = utils.NormalizeRustDeskID(aff.Id)
	aff.PeerId = utils.NormalizeRustDeskID(aff.PeerId)
	if !validAuditFileReport(aff) || !peerReportIdentityMatches(aff.Id, aff.Uuid) {
		response.Error(c, response.TranslateMsg(c, "ParamsError"))
		return
	}
	//ttt := &gin.H{}
	//c.ShouldBindBodyWith(ttt, binding.JSON)
	//fmt.Println(ttt)
	af := aff.ToAuditFile()
	if err := service.AllService.AuditService.CreateAuditFile(af); err != nil {
		response.Error(c, response.TranslateMsg(c, "OperationFailed"))
		return
	}
	response.Success(c, "")
}

func peerReportIdentityMatches(peerID, uuid string) bool {
	if !boundedReportField(peerID, 128) || !boundedReportField(uuid, 256) {
		return false
	}
	peer := service.AllService.PeerService.FindById(peerID)
	if peer.RowId != 0 && peer.Uuid == uuid {
		return true
	}
	// A client may emit its first connection/file audit before its next
	// sysinfo retry. Recover only from an exact prior native-login identity;
	// browser sessions and unknown UUIDs cannot claim a peer.
	userID := service.AllService.UserService.FindLatestUserIdFromLoginLogByUuid(uuid, peerID)
	if userID == 0 || service.AllService.PeerService.BindLoginIdentity(peerID, uuid, userID) != nil {
		return false
	}
	peer = service.AllService.PeerService.FindById(peerID)
	return peer.RowId != 0 && peer.Uuid == uuid && peer.UserId == userID
}

func validAuditConnReport(form *request.AuditConnForm) bool {
	if form == nil || !boundedReportField(form.Id, 128) || !boundedReportField(form.Uuid, 256) || form.ConnId <= 0 || len(form.Peer) > 2 {
		return false
	}
	if form.Action != model.AuditActionNew && form.Action != model.AuditActionClose && form.Action != "" {
		return false
	}
	if !boundedOptionalReportField(form.Ip, 128) {
		return false
	}
	for _, value := range form.Peer {
		if !boundedOptionalReportField(value, 512) {
			return false
		}
	}
	return true
}

func validAuditFileReport(form *request.AuditFileForm) bool {
	return form != nil && boundedReportField(form.Id, 128) && boundedReportField(form.Uuid, 256) &&
		boundedOptionalReportField(form.PeerId, 128) && boundedOptionalReportField(form.Info, 16<<10) &&
		boundedOptionalReportField(form.Path, 4096)
}
