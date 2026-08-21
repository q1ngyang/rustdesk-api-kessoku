package model

import (
	"strings"
)

type UserThird struct {
	IdModel
	UserId uint `json:"user_id" gorm:"not null;index;uniqueIndex:idx_user_thirds_user_op"`
	OauthUser
	UnionId string `json:"union_id" gorm:"default:'';not null;"`
	// OauthType  	   	string 		`json:"oauth_type" gorm:"not null;"`
	ThirdType string `json:"third_type" gorm:"default:'';not null;"` //deprecated
	OauthType string `json:"oauth_type" gorm:"default:'';not null;"`
	Op        string `json:"op" gorm:"default:'';not null;uniqueIndex:idx_user_thirds_user_op;uniqueIndex:idx_user_thirds_op_open_id"`
	TimeModel
}

func (u *UserThird) FromOauthUser(userId uint, oauthUser *OauthUser, oauthType string, op string) {
	u.UserId = userId
	u.OauthUser = *oauthUser
	u.OauthType = oauthType
	u.Op = op
	// make sure email is lower case
	u.Email = strings.ToLower(u.Email)
}
