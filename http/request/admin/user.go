package admin

import (
	"github.com/q1ngyang/rustdesk-api-kessoku/v3/model"
)

type UserForm struct {
	Id       uint   `json:"id"`
	Username string `json:"username" validate:"required,gte=2,lte=32"`
	Email    string `json:"email"` //validate:"required,email" email不强制
	//Password string           `json:"password" validate:"required,gte=4,lte=20"`
	Nickname string           `json:"nickname"`
	Avatar   string           `json:"avatar"`
	GroupId  uint             `json:"group_id" validate:"required"`
	Role     model.UserRole   `json:"role" validate:"omitempty,oneof=user admin super_admin"`
	IsAdmin  *bool            `json:"is_admin"`
	Status   model.StatusCode `json:"status" validate:"required,oneof=1 2"`
	Remark   string           `json:"remark"`
}

func (uf *UserForm) FromUser(user *model.User) *UserForm {
	uf.Id = user.Id
	uf.Username = user.Username
	uf.Nickname = user.Nickname
	uf.Email = user.Email
	uf.Avatar = user.Avatar
	uf.GroupId = user.GroupId
	uf.Role = user.EffectiveRole()
	uf.IsAdmin = user.IsAdmin
	uf.Status = user.Status
	uf.Remark = user.Remark
	return uf
}
func (uf *UserForm) ToUser() *model.User {
	user := &model.User{}
	user.Id = uf.Id
	user.Username = uf.Username
	user.Nickname = uf.Nickname
	user.Email = uf.Email
	user.Avatar = uf.Avatar
	user.GroupId = uf.GroupId
	user.Role = uf.Role
	user.IsAdmin = uf.IsAdmin
	user.Status = uf.Status
	user.Remark = uf.Remark
	return user
}

type PageQuery struct {
	Page     uint `form:"page"`
	PageSize uint `form:"page_size"`
}

type UserQuery struct {
	PageQuery
	Username string `form:"username"`
}
type UserPasswordForm struct {
	Id       uint   `json:"id" validate:"required"`
	Password string `json:"password" validate:"required,gte=12,lte=128"`
}

type UserSessionRevokeForm struct {
	Id uint `json:"id" validate:"required,gt=0"`
}

type ChangeCurPasswordForm struct {
	OldPassword string `json:"old_password" validate:"required,gte=4,lte=32"`
	NewPassword string `json:"new_password" validate:"required,gte=12,lte=128"`
}

type CurrentProfileForm struct {
	Nickname string `json:"nickname" validate:"lte=64"`
	Email    string `json:"email" validate:"omitempty,email,lte=254"`
}

type UserPreferenceForm struct {
	Language string `json:"language" validate:"omitempty,oneof=zh-CN zh-TW en ja ko fr es ru"`
	Theme    string `json:"theme" validate:"omitempty,oneof=light dark"`
}

type GroupUsersQuery struct {
	IsMy   int  `json:"is_my"`
	UserId uint `json:"user_id"`
}

type RegisterForm struct {
	Username        string `json:"username" validate:"required,gte=2,lte=32"`
	Email           string `json:"email" validate:"omitempty,email,lte=254"`
	Password        string `json:"password" validate:"required,gte=12,lte=128"`
	ConfirmPassword string `json:"confirm_password" validate:"required,gte=12,lte=128"`
}

type UserTokenBatchDeleteForm struct {
	Ids []uint `json:"ids" binding:"required,max=1000,dive,gt=0"`
}
