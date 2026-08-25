package admin

import (
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/q1ngyang/rustdesk-api-kessoku/v3/global"
	"github.com/q1ngyang/rustdesk-api-kessoku/v3/http/request/admin"
	"github.com/q1ngyang/rustdesk-api-kessoku/v3/http/response"
	"github.com/q1ngyang/rustdesk-api-kessoku/v3/model"
	"github.com/q1ngyang/rustdesk-api-kessoku/v3/service"
	"gorm.io/gorm"
)

type AdminScope struct{}

func (ct *AdminScope) Detail(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		response.Fail(c, 101, response.TranslateMsg(c, "ParamsError"))
		return
	}
	details, err := service.AllService.AdminScopeService.Details(uint(id))
	if err != nil {
		response.Fail(c, 101, response.TranslateMsg(c, "ItemNotFound"))
		return
	}
	response.Success(c, details)
}

func (ct *AdminScope) Update(c *gin.Context) {
	f := &admin.AdminScopeUpdateForm{}
	if err := c.ShouldBindJSON(f); err != nil {
		response.Fail(c, 101, response.TranslateMsg(c, "ParamsError")+err.Error())
		return
	}
	if errors := global.Validator.ValidStruct(c, f); len(errors) > 0 {
		response.Fail(c, 101, errors[0])
		return
	}
	actor := service.AllService.UserService.CurUser(c)
	if err := service.AllService.AdminScopeService.ReplaceScopesContext(c.Request.Context(), actor.Id, f.UserId, controlRequestID(c), f.ToScopeSet()); err != nil {
		response.Fail(c, 101, response.TranslateMsg(c, "OperationFailed")+err.Error())
		return
	}
	response.Success(c, nil)
}

// Options provides bounded remote-search results for the scope editor. It is
// intentionally super-administrator-only at the router layer.
func (ct *AdminScope) Options(c *gin.Context) {
	query := &admin.AdminScopeOptionQuery{}
	if err := c.ShouldBindQuery(query); err != nil || !query.Type.Valid() {
		response.Fail(c, 101, response.TranslateMsg(c, "ParamsError"))
		return
	}
	q := strings.TrimSpace(query.Q)
	switch query.Type {
	case model.AdminScopeTypeGroup:
		result := service.AllService.GroupService.List(query.Page, query.PageSize, func(tx *gorm.DB) {
			if q != "" {
				tx.Where("name LIKE ?", "%"+q+"%")
			}
		})
		response.Success(c, result)
	case model.AdminScopeTypeUser:
		result := service.AllService.UserService.List(query.Page, query.PageSize, func(tx *gorm.DB) {
			tx.Where("role = ?", model.UserRoleUser)
			if q != "" {
				tx.Where("username LIKE ? OR email LIKE ?", "%"+q+"%", "%"+q+"%")
			}
		})
		response.Success(c, result)
	case model.AdminScopeTypeCollection:
		result := service.AllService.AddressBookService.ListCollection(query.Page, query.PageSize, func(tx *gorm.DB) {
			if q != "" {
				tx.Where("name LIKE ?", "%"+q+"%")
			}
		})
		response.Success(c, result)
	case model.AdminScopeTypePeer:
		result := service.AllService.PeerService.List(query.Page, query.PageSize, func(tx *gorm.DB) {
			if q != "" {
				tx.Where("id LIKE ? OR hostname LIKE ? OR alias LIKE ?", "%"+q+"%", "%"+q+"%", "%"+q+"%")
			}
		})
		response.Success(c, result)
	}
}
