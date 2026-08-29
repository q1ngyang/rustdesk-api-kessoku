package service

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/q1ngyang/rustdesk-api-kessoku/v3/model"
	"gorm.io/gorm"
)

const systemSettingSingletonID uint = 1

const (
	DefaultGeoIPCityURL    = "https://github.com/P3TERX/GeoLite.mmdb/raw/download/GeoLite2-City.mmdb"
	DefaultGeoIPCountryURL = "https://github.com/P3TERX/GeoLite.mmdb/raw/download/GeoLite2-Country.mmdb"
	DefaultGeoIPASNURL     = "https://github.com/P3TERX/GeoLite.mmdb/raw/download/GeoLite2-ASN.mmdb"
)

type SystemSettingService struct{}

func defaultSystemSetting() *model.SystemSetting {
	loginHours := configuredDefaultLoginHours()
	return &model.SystemSetting{
		IdModel:                   model.IdModel{Id: systemSettingSingletonID},
		GeoIPEnabled:              true,
		GeoIPCityURL:              DefaultGeoIPCityURL,
		GeoIPCountryURL:           DefaultGeoIPCountryURL,
		GeoIPASNURL:               DefaultGeoIPASNURL,
		GeoIPUpdateHours:          168,
		WebLoginHours:             loginHours,
		ClientLoginHours:          loginHours,
		UserTokenRetentionDays:    0,
		LoginLogRetentionDays:     0,
		AuditConnRetentionDays:    0,
		AuditFileRetentionDays:    0,
		ControlAuditRetentionDays: 0,
		LoginMaximumHours:         configuredMaximumLoginHours(),
	}
}

func configuredDefaultLoginHours() uint {
	if Config == nil {
		return 7 * 24
	}
	duration := Config.App.TokenExpire
	if Config.Auth.Enabled {
		duration = Config.Auth.EffectiveAccessTokenTTL()
	}
	if duration <= 0 {
		duration = 7 * 24 * time.Hour
	}
	return uint(duration / time.Hour)
}

func configuredMaximumLoginHours() uint {
	if Config == nil {
		return 7 * 24
	}
	duration := Config.App.TokenExpire
	if Config.Auth.Enabled {
		duration = Config.Auth.EffectiveMaximumTokenTTL()
	}
	if duration <= 0 {
		duration = 7 * 24 * time.Hour
	}
	hours := uint(duration / time.Hour)
	if hours == 0 {
		return 1
	}
	return hours
}

func (s *SystemSettingService) Get() (*model.SystemSetting, error) {
	setting := defaultSystemSetting()
	if DB == nil {
		return setting, errors.New("database is unavailable")
	}
	err := DB.First(setting, systemSettingSingletonID).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return setting, nil
	}
	if setting.GeoIPCityURL == "" {
		setting.GeoIPCityURL = DefaultGeoIPCityURL
	}
	if setting.GeoIPCountryURL == "" {
		setting.GeoIPCountryURL = DefaultGeoIPCountryURL
	}
	if setting.GeoIPASNURL == "" {
		setting.GeoIPASNURL = DefaultGeoIPASNURL
	}
	if setting.GeoIPUpdateHours == 0 {
		setting.GeoIPUpdateHours = 168
	}
	defaults := defaultSystemSetting()
	if setting.WebLoginHours == 0 {
		setting.WebLoginHours = defaults.WebLoginHours
	}
	if setting.ClientLoginHours == 0 {
		setting.ClientLoginHours = defaults.ClientLoginHours
	}
	setting.LoginMaximumHours = defaults.LoginMaximumHours
	return setting, err
}

func (s *SystemSettingService) EffectiveLoginTTL(client string) time.Duration {
	setting, err := s.Get()
	if err != nil {
		return time.Duration(configuredDefaultLoginHours()) * time.Hour
	}
	hours := setting.WebLoginHours
	if model.IsNativeLoginClient(client) {
		hours = setting.ClientLoginHours
	}
	maximum := configuredMaximumLoginHours()
	if hours < 1 || hours > maximum {
		hours = configuredDefaultLoginHours()
		if hours > maximum {
			hours = maximum
		}
	}
	return time.Duration(hours) * time.Hour
}

func (s *SystemSettingService) UpdateContext(ctx context.Context, actor uint, requestID string, next *model.SystemSetting) (operationErr error) {
	if next == nil {
		return errors.New("system settings payload is required")
	}
	if err := validateSystemSetting(next); err != nil {
		return err
	}
	event, err := beginSecurityAudit(ctx, actor, requestID, "system_settings.updated", "system_settings", "1", nil)
	if err != nil {
		return err
	}
	defer finalizeSecurityAudit(event, &operationErr, "system_settings_update_failed")
	current := defaultSystemSetting()
	return DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.FirstOrCreate(current, model.SystemSetting{IdModel: model.IdModel{Id: systemSettingSingletonID}}).Error; err != nil {
			return err
		}
		return tx.Model(current).Updates(map[string]interface{}{
			"announcement":                 next.Announcement,
			"geo_ip_enabled":               next.GeoIPEnabled,
			"geo_ip_city_url":              next.GeoIPCityURL,
			"geo_ip_country_url":           next.GeoIPCountryURL,
			"geo_ip_asn_url":               next.GeoIPASNURL,
			"geo_ip_update_hours":          next.GeoIPUpdateHours,
			"web_login_hours":              next.WebLoginHours,
			"client_login_hours":           next.ClientLoginHours,
			"user_token_retention_days":    next.UserTokenRetentionDays,
			"login_log_retention_days":     next.LoginLogRetentionDays,
			"audit_conn_retention_days":    next.AuditConnRetentionDays,
			"audit_file_retention_days":    next.AuditFileRetentionDays,
			"control_audit_retention_days": next.ControlAuditRetentionDays,
			"updated_by":                   actor,
		}).Error
	})
}

func validateSystemSetting(setting *model.SystemSetting) error {
	defaults := defaultSystemSetting()
	if setting.WebLoginHours == 0 {
		setting.WebLoginHours = defaults.WebLoginHours
	}
	if setting.ClientLoginHours == 0 {
		setting.ClientLoginHours = defaults.ClientLoginHours
	}
	if len(setting.Announcement) > 16<<10 || strings.ContainsRune(setting.Announcement, '\x00') {
		return errors.New("announcement is invalid or too long")
	}
	if setting.GeoIPUpdateHours < 1 || setting.GeoIPUpdateHours > 24*90 {
		return errors.New("geoip_update_hours must be between 1 and 2160")
	}
	maximumHours := configuredMaximumLoginHours()
	if setting.WebLoginHours < 1 || setting.WebLoginHours > maximumHours || setting.ClientLoginHours < 1 || setting.ClientLoginHours > maximumHours {
		return fmt.Errorf("login validity must be between 1 and %d hours", maximumHours)
	}
	for name, days := range map[string]uint{
		"user_token_retention_days":    setting.UserTokenRetentionDays,
		"login_log_retention_days":     setting.LoginLogRetentionDays,
		"audit_conn_retention_days":    setting.AuditConnRetentionDays,
		"audit_file_retention_days":    setting.AuditFileRetentionDays,
		"control_audit_retention_days": setting.ControlAuditRetentionDays,
	} {
		if days > 3650 {
			return fmt.Errorf("%s must be zero or between 1 and 3650", name)
		}
	}
	for name, value := range map[string]string{"geoip_city_url": setting.GeoIPCityURL, "geoip_country_url": setting.GeoIPCountryURL, "geoip_asn_url": setting.GeoIPASNURL} {
		parsed, err := url.Parse(value)
		if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil || parsed.Fragment != "" || len(value) > 1024 {
			return fmt.Errorf("%s must be a valid HTTPS URL", name)
		}
	}
	return nil
}
