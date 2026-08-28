package service

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"

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
	return &model.SystemSetting{
		IdModel:          model.IdModel{Id: systemSettingSingletonID},
		GeoIPEnabled:     true,
		GeoIPCityURL:     DefaultGeoIPCityURL,
		GeoIPCountryURL:  DefaultGeoIPCountryURL,
		GeoIPASNURL:      DefaultGeoIPASNURL,
		GeoIPUpdateHours: 168,
	}
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
	return setting, err
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
			"announcement":        next.Announcement,
			"geo_ip_enabled":      next.GeoIPEnabled,
			"geo_ip_city_url":     next.GeoIPCityURL,
			"geo_ip_country_url":  next.GeoIPCountryURL,
			"geo_ip_asn_url":      next.GeoIPASNURL,
			"geo_ip_update_hours": next.GeoIPUpdateHours,
			"updated_by":          actor,
		}).Error
	})
}

func validateSystemSetting(setting *model.SystemSetting) error {
	if len(setting.Announcement) > 16<<10 || strings.ContainsRune(setting.Announcement, '\x00') {
		return errors.New("announcement is invalid or too long")
	}
	if setting.GeoIPUpdateHours < 1 || setting.GeoIPUpdateHours > 24*90 {
		return errors.New("geoip_update_hours must be between 1 and 2160")
	}
	for name, value := range map[string]string{"geoip_city_url": setting.GeoIPCityURL, "geoip_country_url": setting.GeoIPCountryURL, "geoip_asn_url": setting.GeoIPASNURL} {
		parsed, err := url.Parse(value)
		if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil || parsed.Fragment != "" || len(value) > 1024 {
			return fmt.Errorf("%s must be a valid HTTPS URL", name)
		}
	}
	return nil
}
