package api

type OidcAuthRequest struct {
	DeviceInfo DeviceInfoInLogin `json:"deviceInfo" binding:"required" label:"设备信息"`
	Id         string            `json:"id" binding:"required,max=128" label:"id"`
	Op         string            `json:"op" binding:"required,max=64" label:"op"`
	Uuid       string            `json:"uuid" binding:"required,max=256" label:"uuid"`
}

type OidcAuthQuery struct {
	Code string `json:"code" form:"code" binding:"required,max=128" label:"code"`
	Id   string `json:"id" form:"id" binding:"required,max=128" label:"id"`
	Uuid string `json:"uuid" form:"uuid" binding:"required,max=256" label:"uuid"`
}
