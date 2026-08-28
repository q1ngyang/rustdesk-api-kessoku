package admin

import (
	"github.com/gin-gonic/gin"
	"github.com/q1ngyang/rustdesk-api-kessoku/v3/global"
	"github.com/q1ngyang/rustdesk-api-kessoku/v3/http/request/admin"
	"github.com/q1ngyang/rustdesk-api-kessoku/v3/http/response"
	adResp "github.com/q1ngyang/rustdesk-api-kessoku/v3/http/response/admin"
	"github.com/q1ngyang/rustdesk-api-kessoku/v3/model"
	"github.com/q1ngyang/rustdesk-api-kessoku/v3/service"
	"github.com/q1ngyang/rustdesk-api-kessoku/v3/utils"
	"gorm.io/gorm"
	"net/http"
	"strconv"
	"strings"
)

const maxRegistrationBodyBytes = 16 << 10

type User struct {
}

// Detail 管理员
// @Tags 用户
// @Summary 管理员详情
// @Description 管理员详情
// @Accept  json
// @Produce  json
// @Param id path int true "ID"
// @Success 200 {object} response.Response{data=model.User}
// @Failure 500 {object} response.Response
// @Router /admin/user/detail/{id} [get]
// @Security token
func (ct *User) Detail(c *gin.Context) {
	id := c.Param("id")
	iid, _ := strconv.Atoi(id)
	u := service.AllService.UserService.InfoById(uint(iid))
	if u.Id > 0 {
		actor := service.AllService.UserService.CurUser(c)
		if !service.AllService.AdminScopeService.CanManageUser(actor, u) {
			denyScopedAccess(c, "user", u.Id)
			return
		}
		response.Success(c, u)
		return
	}
	response.Fail(c, 101, response.TranslateMsg(c, "ItemNotFound"))
	return
}

// Create 管理员
// @Tags 用户
// @Summary 创建管理员
// @Description 创建管理员
// @Accept  json
// @Produce  json
// @Param body body admin.UserForm true "管理员信息"
// @Success 200 {object} response.Response{data=model.User}
// @Failure 500 {object} response.Response
// @Router /admin/user/create [post]
// @Security token
func (ct *User) Create(c *gin.Context) {
	f := &admin.UserForm{}
	if err := c.ShouldBindJSON(f); err != nil {
		response.Fail(c, 101, response.TranslateMsg(c, "ParamsError")+err.Error())
		return
	}
	errList := global.Validator.ValidStruct(c, f)
	if len(errList) > 0 {
		response.Fail(c, 101, errList[0])
		return
	}
	u := f.ToUser()
	actor := service.AllService.UserService.CurUser(c)
	if !service.AllService.UserService.IsSuperAdmin(actor) {
		if !service.AllService.AdminScopeService.CanManageGroup(actor, u.GroupId) {
			denyScopedAccess(c, "group", u.GroupId)
			return
		}
		u.Role = model.UserRoleUser
		u.NormalizeRole()
	}
	err := service.AllService.UserService.CreateContext(c.Request.Context(), actor.Id, controlRequestID(c), u)
	if err != nil {
		response.Fail(c, 101, response.TranslateMsg(c, "OperationFailed")+err.Error())
		return
	}
	response.Success(c, nil)
}

// List 列表
// @Tags 用户
// @Summary 管理员列表
// @Description 管理员列表
// @Accept  json
// @Produce  json
// @Param page query int false "页码"
// @Param page_size query int false "页大小"
// @Param username query int false "账户"
// @Success 200 {object} response.Response{data=model.UserList}
// @Failure 500 {object} response.Response
// @Router /admin/user/list [get]
// @Security token
func (ct *User) List(c *gin.Context) {
	query := &admin.UserQuery{}
	if err := c.ShouldBindQuery(query); err != nil {
		response.Fail(c, 101, response.TranslateMsg(c, "ParamsError")+err.Error())
		return
	}
	actor := service.AllService.UserService.CurUser(c)
	res := service.AllService.UserService.List(query.Page, query.PageSize, func(tx *gorm.DB) {
		service.AllService.AdminScopeService.ApplyUserScope(tx, actor)
		if query.Username != "" {
			tx.Where("username like ?", "%"+query.Username+"%")
		}
	})
	response.Success(c, res)
}

// Update 编辑
// @Tags 用户
// @Summary 管理员编辑
// @Description 管理员编辑
// @Accept  json
// @Produce  json
// @Param body body admin.UserForm true "用户信息"
// @Success 200 {object} response.Response{data=model.User}
// @Failure 500 {object} response.Response
// @Router /admin/user/update [post]
// @Security token
func (ct *User) Update(c *gin.Context) {
	f := &admin.UserForm{}
	if err := c.ShouldBindJSON(f); err != nil {
		response.Fail(c, 101, response.TranslateMsg(c, "ParamsError")+err.Error())
		return
	}
	if f.Id == 0 {
		response.Fail(c, 101, response.TranslateMsg(c, "ParamsError"))
		return
	}
	errList := global.Validator.ValidStruct(c, f)
	if len(errList) > 0 {
		response.Fail(c, 101, errList[0])
		return
	}
	actor := service.AllService.UserService.CurUser(c)
	current := service.AllService.UserService.InfoById(f.Id)
	if current.Id == 0 {
		response.Fail(c, 101, response.TranslateMsg(c, "ItemNotFound"))
		return
	}
	if !service.AllService.AdminScopeService.CanManageUser(actor, current) {
		denyScopedAccess(c, "user", current.Id)
		return
	}
	u := f.ToUser()
	if !service.AllService.UserService.IsSuperAdmin(actor) {
		u.Role = model.UserRoleUser
		u.NormalizeRole()
		if u.GroupId != current.GroupId && !service.AllService.AdminScopeService.CanManageGroup(actor, u.GroupId) {
			denyScopedAccess(c, "group", u.GroupId)
			return
		}
	}
	err := service.AllService.UserService.UpdateContext(c.Request.Context(), actor.Id, controlRequestID(c), u)
	if err != nil {
		response.Fail(c, 101, response.TranslateMsg(c, "OperationFailed")+err.Error())
		return
	}
	response.Success(c, nil)
}

// Delete 删除
// @Tags 用户
// @Summary 管理员删除
// @Description 管理员编删除
// @Accept  json
// @Produce  json
// @Param body body admin.UserForm true "用户信息"
// @Success 200 {object} response.Response
// @Failure 500 {object} response.Response
// @Router /admin/user/delete [post]
// @Security token
func (ct *User) Delete(c *gin.Context) {
	f := &admin.UserForm{}
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
	u := service.AllService.UserService.InfoById(f.Id)
	if u.Id > 0 {
		actor := service.AllService.UserService.CurUser(c)
		err := service.AllService.UserService.DeleteContext(c.Request.Context(), actor.Id, controlRequestID(c), u)
		if err == nil {
			response.Success(c, nil)
			return
		}
		response.Fail(c, 101, err.Error())
		return
	}
	response.Fail(c, 101, response.TranslateMsg(c, "ItemNotFound"))
}

// UpdatePassword 修改密码
// @Tags 用户
// @Summary 修改密码
// @Description 修改密码
// @Accept  json
// @Produce  json
// @Param body body admin.UserPasswordForm true "用户信息"
// @Success 200 {object} response.Response
// @Failure 500 {object} response.Response
// @Router /admin/user/updatePassword [post]
// @Security token
func (ct *User) UpdatePassword(c *gin.Context) {
	f := &admin.UserPasswordForm{}
	if err := c.ShouldBindJSON(f); err != nil {
		response.Fail(c, 101, response.TranslateMsg(c, "ParamsError")+err.Error())
		return
	}
	errList := global.Validator.ValidStruct(c, f)
	if len(errList) > 0 {
		response.Fail(c, 101, errList[0])
		return
	}
	u := service.AllService.UserService.InfoById(f.Id)
	if u.Id == 0 {
		response.Fail(c, 101, response.TranslateMsg(c, "ItemNotFound"))
		return
	}
	actor := service.AllService.UserService.CurUser(c)
	if !service.AllService.AdminScopeService.CanManageUser(actor, u) {
		denyScopedAccess(c, "user", u.Id)
		return
	}
	err := service.AllService.UserService.UpdatePasswordContext(c.Request.Context(), actor.Id, controlRequestID(c), u, f.Password)
	if err != nil {
		response.Fail(c, 101, response.TranslateMsg(c, "OperationFailed")+err.Error())
		return
	}
	response.Success(c, nil)
}

// RevokeSessions invalidates every current access token for one user.
// @Tags 用户
// @Summary 撤销用户全部登录会话
// @Description 原子递增 auth_version 并撤销该用户全部现有 token
// @Accept json
// @Produce json
// @Param body body admin.UserSessionRevokeForm true "用户 ID"
// @Success 200 {object} response.Response
// @Failure 500 {object} response.Response
// @Router /admin/user/revokeSessions [post]
// @Security token
func (ct *User) RevokeSessions(c *gin.Context) {
	f := &admin.UserSessionRevokeForm{}
	if err := c.ShouldBindJSON(f); err != nil {
		response.Fail(c, 101, response.TranslateMsg(c, "ParamsError"))
		return
	}
	errList := global.Validator.ValidStruct(c, f)
	if len(errList) > 0 {
		response.Fail(c, 101, errList[0])
		return
	}
	u := service.AllService.UserService.InfoById(f.Id)
	if u.Id == 0 {
		response.Fail(c, 101, response.TranslateMsg(c, "ItemNotFound"))
		return
	}
	actor := service.AllService.UserService.CurUser(c)
	if !service.AllService.AdminScopeService.CanManageUser(actor, u) {
		denyScopedAccess(c, "user", u.Id)
		return
	}
	if err := service.AllService.UserService.FlushTokenContext(c.Request.Context(), actor.Id, controlRequestID(c), u); err != nil {
		response.Fail(c, 101, response.TranslateMsg(c, "OperationFailed"))
		return
	}
	response.Success(c, nil)
}

// Current 当前用户
// @Tags 用户
// @Summary 当前用户
// @Description 当前用户
// @Accept  json
// @Produce  json
// @Success 200 {object} response.Response{data=adResp.LoginPayload}
// @Failure 500 {object} response.Response
// @Router /admin/user/current [get]
// @Security token
func (ct *User) Current(c *gin.Context) {
	u := service.AllService.UserService.CurUser(c)
	token, _ := c.Get("token")
	t := token.(string)
	responseLoginSuccess(c, u, t)
}

// UpdateCurrentProfile lets a signed-in user maintain only their own public
// identity fields. Role, username, group and security state remain outside
// this narrowly scoped endpoint.
func (ct *User) UpdateCurrentProfile(c *gin.Context) {
	f := &admin.CurrentProfileForm{}
	if err := c.ShouldBindJSON(f); err != nil {
		response.Fail(c, 101, response.TranslateMsg(c, "ParamsError"))
		return
	}
	f.Nickname = strings.TrimSpace(f.Nickname)
	f.Email = strings.TrimSpace(f.Email)
	if errList := global.Validator.ValidStruct(c, f); len(errList) > 0 {
		response.Fail(c, 101, errList[0])
		return
	}
	u := service.AllService.UserService.CurUser(c)
	if err := service.AllService.UserService.UpdateCurrentProfileContext(c.Request.Context(), u, controlRequestID(c), f.Nickname, f.Email); err != nil {
		response.Fail(c, 101, response.TranslateMsg(c, "OperationFailed")+err.Error())
		return
	}
	response.Success(c, gin.H{"nickname": u.Nickname, "email": u.Email})
}

// UpdatePreferences synchronizes non-sensitive presentation preferences for
// the current account. It never changes authorization or identity fields.
func (ct *User) UpdatePreferences(c *gin.Context) {
	f := &admin.UserPreferenceForm{}
	if err := c.ShouldBindJSON(f); err != nil || f.Language == "" && f.Theme == "" {
		response.Fail(c, 101, response.TranslateMsg(c, "ParamsError"))
		return
	}
	if errList := global.Validator.ValidStruct(c, f); len(errList) > 0 {
		response.Fail(c, 101, errList[0])
		return
	}
	u := service.AllService.UserService.CurUser(c)
	if err := service.AllService.UserService.UpdatePreferencesContext(c.Request.Context(), u, f.Language, f.Theme); err != nil {
		response.Fail(c, 101, response.TranslateMsg(c, "OperationFailed"))
		return
	}
	response.Success(c, gin.H{"preference_language": u.PreferenceLanguage, "preference_theme": u.PreferenceTheme})
}

// ChangeCurPwd 修改当前用户密码
// @Tags 用户
// @Summary 修改当前用户密码
// @Description 修改当前用户密码
// @Accept  json
// @Produce  json
// @Param body body admin.ChangeCurPasswordForm true "用户信息"
// @Success 200 {object} response.Response
// @Failure 500 {object} response.Response
// @Router /admin/user/changeCurPwd [post]
// @Security token
func (ct *User) ChangeCurPwd(c *gin.Context) {
	f := &admin.ChangeCurPasswordForm{}
	if err := c.ShouldBindJSON(f); err != nil {
		response.Fail(c, 101, response.TranslateMsg(c, "ParamsError")+err.Error())
		return
	}

	errList := global.Validator.ValidStruct(c, f)
	if len(errList) > 0 {
		response.Fail(c, 101, errList[0])
		return
	}
	u := service.AllService.UserService.CurUser(c)
	// Verify the old password only when the account already has one set
	if !service.AllService.UserService.IsPasswordEmptyByUser(u) {
		ok, _, err := utils.VerifyPassword(u.Password, f.OldPassword)
		if err != nil || !ok {
			response.Fail(c, 101, response.TranslateMsg(c, "OldPasswordError"))
			return
		}
	}
	err := service.AllService.UserService.UpdatePasswordContext(c.Request.Context(), u.Id, controlRequestID(c), u, f.NewPassword)
	if err != nil {
		response.Fail(c, 101, response.TranslateMsg(c, "OperationFailed")+err.Error())
		return
	}
	response.Success(c, nil)
}

// MyOauth
// @Tags 用户
// @Summary 我的授权
// @Description 我的授权
// @Accept  json
// @Produce  json
// @Success 200 {object} response.Response{data=[]adResp.UserOauthItem}
// @Failure 500 {object} response.Response
// @Router /admin/user/myOauth [get]
// @Security token
func (ct *User) MyOauth(c *gin.Context) {
	u := service.AllService.UserService.CurUser(c)
	oal := service.AllService.OauthService.List(1, 100, nil)
	ops := make([]string, 0)
	for _, oa := range oal.Oauths {
		ops = append(ops, oa.Op)
	}
	uts := service.AllService.UserService.UserThirdsByUserId(u.Id)
	var res []*adResp.UserOauthItem
	for _, oa := range oal.Oauths {
		item := &adResp.UserOauthItem{
			Op: oa.Op,
		}
		for _, ut := range uts {
			if ut.Op == oa.Op {
				item.Status = 1
				break
			}
		}
		res = append(res, item)
	}
	response.Success(c, res)
}

// groupUsers
func (ct *User) GroupUsers(c *gin.Context) {
	actor := service.AllService.UserService.CurUser(c)
	groupFilter := func(tx *gorm.DB) {}
	userFilter := func(tx *gorm.DB) {}
	if service.AllService.UserService.Role(actor) == model.UserRoleAdmin {
		groupFilter = func(tx *gorm.DB) { service.AllService.AdminScopeService.ApplyGroupScope(tx, actor) }
		userFilter = func(tx *gorm.DB) { service.AllService.AdminScopeService.ApplyUserScope(tx, actor) }
	}
	aG := service.AllService.GroupService.List(1, 999, groupFilter)
	aU := service.AllService.UserService.List(1, 9999, userFilter)
	groups := make([]adResp.GroupDirectoryGroup, 0, len(aG.Groups))
	for _, group := range aG.Groups {
		groups = append(groups, adResp.GroupDirectoryGroup{Id: group.Id, Name: group.Name})
	}
	users := make([]adResp.GroupDirectoryUser, 0, len(aU.Users))
	for _, user := range aU.Users {
		users = append(users, adResp.GroupDirectoryUser{Id: user.Id, Username: user.Username, GroupId: user.GroupId})
	}
	response.Success(c, gin.H{
		"groups": groups,
		"users":  users,
	})
}

// Register
func (ct *User) Register(c *gin.Context) {
	if !global.Config.App.Register {
		response.Fail(c, 101, response.TranslateMsg(c, "RegisterClosed"))
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxRegistrationBodyBytes)
	f := &admin.RegisterForm{}
	if err := c.ShouldBindJSON(f); err != nil {
		response.Fail(c, 101, response.TranslateMsg(c, "ParamsError")+err.Error())
		return
	}
	errList := global.Validator.ValidStruct(c, f)
	if len(errList) > 0 {
		response.Fail(c, 101, errList[0])
		return
	}
	if f.Password != f.ConfirmPassword {
		response.Fail(c, 101, response.TranslateMsg(c, "ParamsError"))
		return
	}
	regStatus := model.StatusCode(global.Config.App.RegisterStatus)
	// 注册状态可能未配置，默认启用
	if regStatus != model.COMMON_STATUS_DISABLED && regStatus != model.COMMON_STATUS_ENABLE {
		regStatus = model.COMMON_STATUS_ENABLE
	}

	u := service.AllService.UserService.Register(f.Username, f.Email, f.Password, regStatus)
	if u == nil || u.Id == 0 {
		response.Fail(c, 101, response.TranslateMsg(c, "OperationFailed"))
		return
	}
	if regStatus == model.COMMON_STATUS_DISABLED {
		// 需要管理员审核
		response.Fail(c, 101, response.TranslateMsg(c, "RegisterSuccessWaitAdminConfirm"))
		return
	}
	// 注册成功后自动登录
	ut := service.AllService.UserService.Login(u, &model.LoginLog{
		UserId: u.Id,
		Client: model.LoginLogClientWebAdmin,
		Uuid:   "",
		Ip:     c.ClientIP(),
		Type:   model.LoginLogTypeAccount,
	})
	if ut == nil {
		response.Fail(c, 101, response.TranslateMsg(c, "SystemError"))
		return
	}
	responseLoginSuccess(c, u, ut.Token)
}
