package model

// SystemSetting holds operator-managed workspace settings that are not tenant
// brand assets. It is a singleton row so updates can be audited atomically.
type SystemSetting struct {
	IdModel
	Announcement              string `json:"announcement" gorm:"type:text;not null"`
	GeoIPEnabled              bool   `json:"geoip_enabled" gorm:"not null;default:true"`
	GeoIPCityURL              string `json:"geoip_city_url" gorm:"size:1024;not null;default:''"`
	GeoIPCountryURL           string `json:"geoip_country_url" gorm:"size:1024;not null;default:''"`
	GeoIPASNURL               string `json:"geoip_asn_url" gorm:"size:1024;not null;default:''"`
	GeoIPUpdateHours          uint   `json:"geoip_update_hours" gorm:"not null;default:168"`
	GeoIPLastUpdatedAt        *int64 `json:"geoip_last_updated_at,omitempty"`
	GeoIPLastError            string `json:"geoip_last_error" gorm:"size:1000;not null;default:''"`
	GeoIPUpdating             bool   `json:"geoip_updating" gorm:"-"`
	WebLoginHours             uint   `json:"web_login_hours" gorm:"not null;default:168"`
	ClientLoginHours          uint   `json:"client_login_hours" gorm:"not null;default:168"`
	UserTokenRetentionDays    uint   `json:"user_token_retention_days" gorm:"not null;default:0"`
	LoginLogRetentionDays     uint   `json:"login_log_retention_days" gorm:"not null;default:0"`
	AuditConnRetentionDays    uint   `json:"audit_conn_retention_days" gorm:"not null;default:0"`
	AuditFileRetentionDays    uint   `json:"audit_file_retention_days" gorm:"not null;default:0"`
	ControlAuditRetentionDays uint   `json:"control_audit_retention_days" gorm:"not null;default:0"`
	LoginMaximumHours         uint   `json:"login_maximum_hours" gorm:"-"`
	UpdatedBy                 uint   `json:"updated_by" gorm:"not null;default:0;index"`
	TimeModel
}
