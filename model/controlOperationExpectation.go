package model

// ControlOperationExpectation binds every asynchronous Starry operation to
// the exact instance, operation kind, source digest, and originating audit
// event that Kessoku authorized.
type ControlOperationExpectation struct {
	IdModel
	OperationID          string `json:"operation_id" gorm:"size:64;not null;index:idx_control_operation,priority:2"`
	InstanceID           string `json:"instance_id" gorm:"size:191;not null;index:idx_control_operation,priority:1"`
	Kind                 string `json:"kind" gorm:"size:32;not null"`
	ExpectedSourceDigest string `json:"expected_source_digest" gorm:"size:71;not null"`
	AuditEventID         uint   `json:"audit_event_id" gorm:"not null;uniqueIndex"`
	ExpiresAt            int64  `json:"expires_at" gorm:"not null;index"`
	TimeModel
}
