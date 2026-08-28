package model

type UserTwoFactor struct {
	IdModel
	UserID                  uint   `json:"user_id" gorm:"not null;uniqueIndex"`
	SecretCiphertext        string `json:"-" gorm:"type:text;not null"`
	PendingSecretCiphertext string `json:"-" gorm:"type:text;not null"`
	PendingExpiresAt        int64  `json:"-" gorm:"not null;default:0"`
	Enabled                 bool   `json:"enabled" gorm:"not null;default:false;index"`
	LastUsedStep            int64  `json:"-" gorm:"not null;default:0"`
	TimeModel
}

type TwoFactorLoginChallenge struct {
	IdModel
	TokenHash string `json:"-" gorm:"size:64;not null;uniqueIndex"`
	UserID    uint   `json:"-" gorm:"not null;index"`
	Username  string `json:"-" gorm:"size:64;not null"`
	Client    string `json:"-" gorm:"size:64;not null"`
	DeviceID  string `json:"-" gorm:"size:128;not null"`
	UUID      string `json:"-" gorm:"size:256;not null"`
	Platform  string `json:"-" gorm:"size:64;not null"`
	ExpiresAt int64  `json:"-" gorm:"not null;index"`
	Attempts  uint   `json:"-" gorm:"not null;default:0"`
	UsedAt    int64  `json:"-" gorm:"not null;default:0"`
	TimeModel
}
