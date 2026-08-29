package model

// ShareRecord is retained only for backwards-compatible storage of the
// legacy WebClient guest-link feature. Address-book access is authorized by
// AddressBookCollectionRule and does not create or consume these rows.
type ShareRecord struct {
	IdModel
	UserId       uint   `json:"user_id" gorm:"default:0;not null;index"`
	PeerId       string `json:"peer_id" gorm:"default:'';not null;index"`
	ShareToken   string `json:"-" gorm:"default:'';not null;index"`
	PasswordType string `json:"password_type" gorm:"default:'';not null;"`
	Password     string `json:"-" gorm:"default:'';not null;"`
	Expire       int64  `json:"expire" gorm:"default:0;not null;"`
	TimeModel
}

// ShareRecordList is the legacy WebClient guest-share record list.
type ShareRecordList struct {
	ShareRecords []*ShareRecord `json:"list,omitempty"`
	Pagination
}
