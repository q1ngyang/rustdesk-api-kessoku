package model

// LdapIdentity binds one immutable directory subject to one local account.
// Provider and Subject are SHA-256 fingerprints, so directory DNs are not
// duplicated into the application database.
type LdapIdentity struct {
	IdModel
	UserId   uint   `json:"user_id" gorm:"not null;index;uniqueIndex:ux_ldap_provider_user,priority:2"`
	Provider string `json:"-" gorm:"size:64;not null;uniqueIndex:ux_ldap_provider_subject,priority:1;uniqueIndex:ux_ldap_provider_user,priority:1"`
	Subject  string `json:"-" gorm:"size:64;not null;uniqueIndex:ux_ldap_provider_subject,priority:2"`
	TimeModel
}
