package service

import (
	"testing"
	"time"

	"github.com/q1ngyang/rustdesk-api-kessoku/v3/config"
	"github.com/q1ngyang/rustdesk-api-kessoku/v3/model"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestEffectiveLoginTTLPreservesSubHourAuthConfiguration(t *testing.T) {
	previousConfig, previousDB := Config, DB
	t.Cleanup(func() {
		Config, DB = previousConfig, previousDB
	})

	Config = &config.Config{Auth: config.Auth{
		Enabled:         true,
		AccessTokenTTL:  10 * time.Minute,
		MaximumTokenTTL: 10 * time.Minute,
	}}
	DB = nil

	if got := (&SystemSettingService{}).EffectiveLoginTTL("client"); got != 10*time.Minute {
		t.Fatalf("effective login TTL = %s, want 10m", got)
	}
	if got := configuredDefaultLoginHours(); got != 1 {
		t.Fatalf("stored default login hours = %d, want minimum 1", got)
	}

	database, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := database.AutoMigrate(&model.SystemSetting{}); err != nil {
		t.Fatal(err)
	}
	if err := database.Create(&model.SystemSetting{
		IdModel:          model.IdModel{Id: systemSettingSingletonID},
		WebLoginHours:    168,
		ClientLoginHours: 168,
	}).Error; err != nil {
		t.Fatal(err)
	}
	DB = database
	if got := (&SystemSettingService{}).EffectiveLoginTTL("client"); got != 10*time.Minute {
		t.Fatalf("capped persisted login TTL = %s, want 10m", got)
	}
}
