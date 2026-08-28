package api

import (
	"strings"

	"github.com/q1ngyang/rustdesk-api-kessoku/v3/global"
	"github.com/q1ngyang/rustdesk-api-kessoku/v3/model"
)

/*
	pub enum UserStatus {
	    Disabled = 0,
	    Normal = 1,
	    Unverified = -1,
	}
*/

/*
UserPayload
String name = ”;
String email = ”;
String note = ”;
UserStatus status;
bool isAdmin = false;
*/
type UserPayload struct {
	Name        string                 `json:"name"`
	Email       string                 `json:"email"`
	Note        string                 `json:"note"`
	IsAdmin     *bool                  `json:"is_admin"`
	Status      int                    `json:"status"`
	Info        map[string]interface{} `json:"info"`
	DisplayName string                 `json:"display_name"`
	Avatar      string                 `json:"avatar"`
}

func (up *UserPayload) FromUser(user *model.User) *UserPayload {
	up.Name = user.Username
	up.Email = user.Email
	up.Note = user.Remark
	isSuperAdmin := user.EffectiveRole() == model.UserRoleSuperAdmin
	up.IsAdmin = &isSuperAdmin
	up.Status = int(user.Status)
	up.Info = map[string]interface{}{}
	up.DisplayName = user.Nickname
	if up.DisplayName == "" {
		up.DisplayName = user.Username
	}
	if strings.HasPrefix(user.Avatar, "/media/avatars/") && !strings.Contains(user.Avatar, "..") {
		up.Avatar = strings.TrimRight(global.Config.Rustdesk.ApiServer, "/") + user.Avatar
	}
	return up
}

/*
	class HttpType {
	  static const kAuthReqTypeAccount = "account";
	  static const kAuthReqTypeMobile = "mobile";
	  static const kAuthReqTypeSMSCode = "sms_code";
	  static const kAuthReqTypeEmailCode = "email_code";
	  static const kAuthReqTypeTfaCode = "tfa_code";

	  static const kAuthResTypeToken = "access_token";
	  static const kAuthResTypeEmailCheck = "email_check";
	  static const kAuthResTypeTfaCheck = "tfa_check";
	}
*/
type LoginRes struct {
	Type        string      `json:"type"`
	AccessToken string      `json:"access_token"`
	User        UserPayload `json:"user"`
	Secret      string      `json:"secret,omitempty"`
	TfaType     string      `json:"tfa_type,omitempty"`
}
