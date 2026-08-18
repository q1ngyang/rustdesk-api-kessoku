package model

import "github.com/q1ngyang/rustdesk-api-kessoku/v2/model/custom_types"

type AdminAuditEvent struct {
	IdModel
	ActorUserID uint                  `json:"actor_user_id" gorm:"not null;index"`
	Action      string                `json:"action" gorm:"size:96;not null;index"`
	TargetType  string                `json:"target_type" gorm:"size:64;not null;index"`
	TargetID    string                `json:"target_id" gorm:"size:191;not null;index"`
	RequestID   string                `json:"request_id" gorm:"size:64;not null;index"`
	Result      string                `json:"result" gorm:"size:32;not null;index"`
	ErrorCode   string                `json:"error_code,omitempty" gorm:"size:96;not null;default:'';index"`
	Metadata    custom_types.AutoJson `json:"metadata" gorm:"type:text;not null"`
	TimeModel
}

type AdminAuditEventList struct {
	Events []AdminAuditEvent `json:"list"`
	Pagination
}
