package service

import (
	"context"
	"testing"
	"time"

	"github.com/q1ngyang/rustdesk-api-kessoku/v3/config"
	"github.com/q1ngyang/rustdesk-api-kessoku/v3/lib/lock"
	"github.com/q1ngyang/rustdesk-api-kessoku/v3/model"
	"github.com/q1ngyang/rustdesk-api-kessoku/v3/model/custom_types"
	"github.com/sirupsen/logrus"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestDataRetentionRemovesOnlyExpiredRecordsAndStarryAudit(t *testing.T) {
	oldConfig, oldDB, oldLogger, oldAuth, oldLock, oldServices := Config, DB, Logger, Auth, Lock, AllService
	t.Cleanup(func() {
		Config, DB, Logger, Auth, Lock, AllService = oldConfig, oldDB, oldLogger, oldAuth, oldLock, oldServices
	})
	database, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := database.AutoMigrate(&model.SystemSetting{}, &model.UserToken{}, &model.LoginLog{}, &model.AuditConn{}, &model.AuditFile{}, &model.AdminAuditEvent{}); err != nil {
		t.Fatal(err)
	}
	New(&config.Config{App: config.App{TokenExpire: 168 * time.Hour}}, database, logrus.New(), nil, lock.NewLocal())
	setting := defaultSystemSetting()
	setting.UserTokenRetentionDays, setting.LoginLogRetentionDays = 30, 30
	setting.AuditConnRetentionDays, setting.AuditFileRetentionDays, setting.ControlAuditRetentionDays = 30, 30, 30
	if err := database.Create(setting).Error; err != nil {
		t.Fatal(err)
	}

	now := time.Date(2026, 8, 29, 12, 0, 0, 0, time.UTC)
	old := custom_types.AutoTime(now.Add(-45 * 24 * time.Hour))
	recent := custom_types.AutoTime(now.Add(-2 * 24 * time.Hour))
	revokedOld := now.Add(-45 * 24 * time.Hour).Unix()
	rows := []model.UserToken{
		{UserId: 1, ExpiredAt: now.Add(24 * time.Hour).Unix()},
		{UserId: 1, ExpiredAt: now.Add(-45 * 24 * time.Hour).Unix()},
		{UserId: 1, ExpiredAt: now.Add(24 * time.Hour).Unix(), RevokedAt: &revokedOld},
	}
	if err := database.Create(&rows).Error; err != nil {
		t.Fatal(err)
	}
	if err := database.Create(&model.LoginLog{TimeModel: model.TimeModel{CreatedAt: old}}).Error; err != nil {
		t.Fatal(err)
	}
	if err := database.Create(&model.AuditConn{TimeModel: model.TimeModel{CreatedAt: old}}).Error; err != nil {
		t.Fatal(err)
	}
	if err := database.Create(&model.AuditFile{TimeModel: model.TimeModel{CreatedAt: recent}}).Error; err != nil {
		t.Fatal(err)
	}
	metadata := custom_types.AutoJson([]byte(`{}`))
	if err := database.Create(&[]model.AdminAuditEvent{
		{TargetType: "starry_instance", TargetID: "s1", Action: "read", Result: "success", Metadata: metadata, TimeModel: model.TimeModel{CreatedAt: old}},
		{TargetType: "auth_keyring", TargetID: "keys", Action: "read", Result: "success", Metadata: metadata, TimeModel: model.TimeModel{CreatedAt: old}},
	}).Error; err != nil {
		t.Fatal(err)
	}

	counts, err := AllService.DataRetentionService.Cleanup(context.Background(), now)
	if err != nil {
		t.Fatal(err)
	}
	if counts["user_tokens"] != 2 || counts["login_logs"] != 1 || counts["connection_logs"] != 1 || counts["file_logs"] != 0 || counts["control_audit"] != 1 {
		t.Fatalf("unexpected cleanup counts: %#v", counts)
	}
	var activeTokens, authAudit int64
	if err := database.Model(&model.UserToken{}).Count(&activeTokens).Error; err != nil {
		t.Fatal(err)
	}
	if err := database.Model(&model.AdminAuditEvent{}).Where("target_type = ?", "auth_keyring").Count(&authAudit).Error; err != nil {
		t.Fatal(err)
	}
	if activeTokens != 1 || authAudit != 1 {
		t.Fatalf("active data was removed: tokens=%d auth_audit=%d", activeTokens, authAudit)
	}
}

func TestCreatedAtRangeUsesExclusiveEnd(t *testing.T) {
	rangeValue, err := ParseCreatedAtRange("2026-08-01T00:00:00Z", "2026-09-01T00:00:00Z")
	if err != nil || rangeValue.From == nil || rangeValue.To == nil {
		t.Fatalf("range parse failed: %#v %v", rangeValue, err)
	}
	if _, err := ParseCreatedAtRange("2026-09-01T00:00:00Z", "2026-08-01T00:00:00Z"); err == nil {
		t.Fatal("accepted reversed range")
	}
}

func TestZeroRetentionDisablesEveryCleanupCategory(t *testing.T) {
	oldConfig, oldDB, oldLogger, oldAuth, oldLock, oldServices := Config, DB, Logger, Auth, Lock, AllService
	t.Cleanup(func() {
		Config, DB, Logger, Auth, Lock, AllService = oldConfig, oldDB, oldLogger, oldAuth, oldLock, oldServices
	})
	database, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := database.AutoMigrate(&model.SystemSetting{}, &model.UserToken{}, &model.LoginLog{}, &model.AuditConn{}, &model.AuditFile{}, &model.AdminAuditEvent{}); err != nil {
		t.Fatal(err)
	}
	New(&config.Config{App: config.App{TokenExpire: 168 * time.Hour}}, database, logrus.New(), nil, lock.NewLocal())
	if err := database.Create(defaultSystemSetting()).Error; err != nil {
		t.Fatal(err)
	}
	old := custom_types.AutoTime(time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC))
	revoked := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC).Unix()
	metadata := custom_types.AutoJson([]byte(`{}`))
	rows := []interface{}{
		&model.UserToken{UserId: 1, ExpiredAt: revoked, RevokedAt: &revoked},
		&model.LoginLog{TimeModel: model.TimeModel{CreatedAt: old}},
		&model.AuditConn{TimeModel: model.TimeModel{CreatedAt: old}},
		&model.AuditFile{TimeModel: model.TimeModel{CreatedAt: old}},
		&model.AdminAuditEvent{TargetType: "starry_instance", TargetID: "s1", Action: "read", Result: "success", Metadata: metadata, TimeModel: model.TimeModel{CreatedAt: old}},
	}
	for _, row := range rows {
		if err := database.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
	counts, err := AllService.DataRetentionService.Cleanup(context.Background(), time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	for category, count := range counts {
		if count != 0 {
			t.Fatalf("zero retention deleted %d rows from %s", count, category)
		}
	}
	for _, table := range []interface{}{&model.UserToken{}, &model.LoginLog{}, &model.AuditConn{}, &model.AuditFile{}, &model.AdminAuditEvent{}} {
		var count int64
		if err := database.Model(table).Count(&count).Error; err != nil || count != 1 {
			t.Fatalf("row was not preserved for %T: count=%d err=%v", table, count, err)
		}
	}
}
