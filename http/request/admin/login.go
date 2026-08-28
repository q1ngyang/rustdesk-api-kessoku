package admin

type Login struct {
	Username  string `json:"username" validate:"required" label:"用户名"`
	Password  string `json:"password,omitempty" binding:"max=128" label:"密码"`
	Platform  string `json:"platform" label:"平台"`
	DeviceID  string `json:"device_id" binding:"max=128"`
	UUID      string `json:"uuid" binding:"max=128"`
	Captcha   string `json:"captcha,omitempty" label:"验证码"`
	CaptchaId string `json:"captcha_id,omitempty"`
	Challenge string `json:"challenge,omitempty" binding:"max=128"`
	TfaCode   string `json:"tfa_code,omitempty" binding:"max=16"`
}

type LoginLogQuery struct {
	UserId int `form:"user_id"`
	IsMy   int `form:"is_my"`
	PageQuery
}
type LoginTokenQuery struct {
	UserId int `form:"user_id"`
	PageQuery
}

type LoginLogIds struct {
	Ids []uint `json:"ids" binding:"required,max=1000,dive,gt=0"`
}
