package model

type UserToken struct {
	IdModel
	UserId        uint    `json:"user_id" gorm:"default:0;not null;index"`
	DeviceUuid    string  `json:"device_uuid" gorm:"default:'';omitempty;"`
	DeviceId      string  `json:"device_id" gorm:"default:'';omitempty;"`
	Token         string  `json:"-" gorm:"default:'';not null;index"` // compatibility read only; new rows stay empty
	JTI           *string `json:"jti,omitempty" gorm:"size:36;uniqueIndex"`
	Kid           string  `json:"kid,omitempty" gorm:"size:128;default:'';not null;index"`
	TokenHash     *string `json:"-" gorm:"size:64;uniqueIndex"`
	AuthVersion   uint64  `json:"auth_version" gorm:"default:1;not null;index"`
	IssuedAt      int64   `json:"issued_at" gorm:"default:0;not null;index"`
	ExpiredAt     int64   `json:"expired_at" gorm:"default:0;not null;index"`
	RevokedAt     *int64  `json:"revoked_at,omitempty" gorm:"index"`
	RevokedReason string  `json:"revoked_reason,omitempty" gorm:"default:'';not null;"`
	TimeModel
}

type UserTokenList struct {
	UserTokens []UserToken `json:"list"`
	Pagination
}
