package admin

import (
	"github.com/gin-gonic/gin"
	"github.com/q1ngyang/rustdesk-api-kessoku/v3/global"
	"github.com/q1ngyang/rustdesk-api-kessoku/v3/http/request/admin"
	"github.com/q1ngyang/rustdesk-api-kessoku/v3/http/response"
	"github.com/q1ngyang/rustdesk-api-kessoku/v3/model"
	"github.com/q1ngyang/rustdesk-api-kessoku/v3/service"
	"gorm.io/gorm"
)

type UserToken struct {
}

// List 列表
// @Tags 登录凭证
// @Summary 登录凭证列表
// @Description 登录凭证列表
// @Accept  json
// @Produce  json
// @Param page query int false "页码"
// @Param page_size query int false "页大小"
// @Param user_id query int false "用户ID"
// @Success 200 {object} response.Response{data=model.UserTokenList}
// @Failure 500 {object} response.Response
// @Router /admin/user_token/list [get]
// @Security token
func (ct *UserToken) List(c *gin.Context) {
	query := &admin.LoginTokenQuery{}
	if err := c.ShouldBindQuery(query); err != nil {
		response.Fail(c, 101, response.TranslateMsg(c, "ParamsError")+err.Error())
		return
	}
	dateRange, err := service.ParseCreatedAtRange(query.CreatedFrom, query.CreatedTo)
	if err != nil {
		response.Fail(c, 101, response.TranslateMsg(c, "ParamsError"))
		return
	}
	res := service.AllService.UserService.TokenList(query.Page, query.PageSize, func(tx *gorm.DB) *gorm.DB {
		tx = dateRange.Apply(tx)
		if query.UserId > 0 {
			tx = tx.Where("user_id = ?", query.UserId)
		}
		return tx.Order("id desc")
	})
	response.Success(c, res)
}

// Delete 删除
// @Tags 登录凭证
// @Summary 登录凭证删除
// @Description 登录凭证删除
// @Accept  json
// @Produce  json
// @Param body body model.UserToken true "登录凭证信息"
// @Success 200 {object} response.Response
// @Failure 500 {object} response.Response
// @Router /admin/user_token/delete [post]
// @Security token
func (ct *UserToken) Delete(c *gin.Context) {
	f := &model.UserToken{}
	if err := c.ShouldBindJSON(f); err != nil {
		response.Fail(c, 101, response.TranslateMsg(c, "ParamsError")+err.Error())
		return
	}
	id := f.Id
	errList := global.Validator.ValidVar(c, id, "required,gt=0")
	if len(errList) > 0 {
		response.Fail(c, 101, errList[0])
		return
	}
	l := service.AllService.UserService.TokenInfoById(f.Id)
	u := service.AllService.UserService.CurUser(c)
	if !service.AllService.UserService.IsAdmin(u) && l.UserId != u.Id {
		response.Fail(c, 101, response.TranslateMsg(c, "NoAccess"))
		return
	}
	if l.Id > 0 {
		err := service.AllService.UserService.DeleteTokenContext(c.Request.Context(), u.Id, controlRequestID(c), l)
		if err == nil {
			response.Success(c, nil)
			return
		}
		response.Fail(c, 101, err.Error())
		return
	}
	response.Fail(c, 101, response.TranslateMsg(c, "ItemNotFound"))
}

// BatchDelete 批量删除
// @Tags 登录凭证
// @Summary 登录凭证批量删除
// @Description 登录凭证批量删除
// @Accept  json
// @Produce  json
// @Param body body admin.UserTokenBatchDeleteForm true "登录凭证信息"
// @Success 200 {object} response.Response
// @Failure 500 {object} response.Response
// @Router /admin/user_token/batchDelete [post]
// @Security token
func (ct *UserToken) BatchDelete(c *gin.Context) {
	f := &admin.UserTokenBatchDeleteForm{}
	if err := c.ShouldBindJSON(f); err != nil {
		response.Fail(c, 101, response.TranslateMsg(c, "ParamsError")+err.Error())
		return
	}
	ids := f.Ids
	if len(ids) == 0 {
		response.Fail(c, 101, response.TranslateMsg(c, "ParamsError"))
		return
	}
	actor := service.AllService.UserService.CurUser(c)
	err := service.AllService.UserService.BatchDeleteUserTokenContext(c.Request.Context(), actor.Id, controlRequestID(c), ids)
	if err == nil {
		response.Success(c, nil)
		return
	}
	response.Fail(c, 101, err.Error())
}
