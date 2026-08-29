package model

type LoginLog struct {
	IdModel
	UserId      uint   `json:"user_id" gorm:"default:0;not null;"`
	Client      string `json:"client"` //webadmin,webclient,app,
	DeviceId    string `json:"device_id"`
	Uuid        string `json:"uuid"`
	Ip          string `json:"ip"`
	Type        string `json:"type"`     //account,oauth
	Platform    string `json:"platform"` //windows,linux,mac,android,ios
	UserTokenId uint   `json:"user_token_id" gorm:"default:0;not null;"`
	IsDeleted   uint   `json:"is_deleted" gorm:"default:0;not null;"`
	TimeModel
}

const (
	LoginLogClientWebAdmin = "webadmin"
	LoginLogClientWeb      = "webclient"
	// LoginLogClientNative is the value sent by current official RustDesk
	// desktop and mobile clients in deviceInfo.type.
	LoginLogClientNative = "client"
	// LoginLogClientApp is retained for older API clients and stored records.
	LoginLogClientApp = "app"
)

func IsNativeLoginClient(client string) bool {
	return client == LoginLogClientNative || client == LoginLogClientApp
}

const (
	LoginLogTypeAccount = "account"
	LoginLogTypeOauth   = "oauth"
	LoginLogTypeGrant   = "grant"
)

const (
	IsDeletedNo  = 0
	IsDeletedYes = 1
)

type LoginLogList struct {
	LoginLogs []*LoginLog `json:"list"`
	Pagination
}
