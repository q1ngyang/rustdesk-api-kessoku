package service

import (
	"context"
	"testing"

	"github.com/q1ngyang/rustdesk-api-kessoku/v2/config"
	"github.com/q1ngyang/rustdesk-api-kessoku/v2/lib/lock"
	"github.com/q1ngyang/rustdesk-api-kessoku/v2/model"
	"github.com/sirupsen/logrus"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestRejectedSecurityMutationRetainsFailureAudit(t *testing.T) {
	database := securityAuditDatabase(t, true)
	isAdmin := true
	admin := &model.User{Username: "only-admin", Status: model.COMMON_STATUS_ENABLE, IsAdmin: &isAdmin, AuthVersion: 1}
	if err := database.Create(admin).Error; err != nil {
		t.Fatal(err)
	}

	err := AllService.UserService.DeleteContext(context.Background(), admin.Id, "0191f6a0-0000-7000-8000-000000000010", admin)
	if err == nil {
		t.Fatal("last administrator deletion unexpectedly succeeded")
	}
	event := &model.AdminAuditEvent{}
	if err := database.Where("action = ?", "auth.user.deleted").First(event).Error; err != nil {
		t.Fatal(err)
	}
	if event.Result != "failure" || event.ErrorCode != "AUTH_USER_DELETE_FAILED" || event.ActorUserID != admin.Id {
		t.Fatalf("failure audit = %+v", event)
	}
	if user := AllService.UserService.InfoById(admin.Id); user.Id == 0 {
		t.Fatal("rejected mutation deleted the administrator")
	}
}

func TestSecurityMutationFailsClosedWhenAuditTableIsUnavailable(t *testing.T) {
	database := securityAuditDatabase(t, false)
	isAdmin := false
	user := &model.User{Username: "audit-required", Status: model.COMMON_STATUS_ENABLE, IsAdmin: &isAdmin, AuthVersion: 1}
	if err := database.Create(user).Error; err != nil {
		t.Fatal(err)
	}
	if err := AllService.UserService.FlushToken(user); err == nil {
		t.Fatal("global revocation proceeded without its required audit table")
	}
	refreshed := AllService.UserService.InfoById(user.Id)
	if refreshed.AuthVersion != 1 {
		t.Fatalf("auth version changed without audit: %d", refreshed.AuthVersion)
	}
}

func TestAuthKeyringStartupRecordsLoadAndRotationWithoutKeyMaterial(t *testing.T) {
	database := securityAuditDatabase(t, true)
	first := lifecycleAuthManager(t)
	if err := RecordAuthKeyringStartup(first); err != nil {
		t.Fatal(err)
	}
	if err := RecordAuthKeyringStartup(first); err != nil {
		t.Fatal(err)
	}
	if err := RecordAuthKeyringStartup(lifecycleAuthManager(t)); err != nil {
		t.Fatal(err)
	}
	events := []model.AdminAuditEvent{}
	if err := database.Where("target_type = ?", "auth_keyring").Order("id").Find(&events).Error; err != nil {
		t.Fatal(err)
	}
	if len(events) != 3 || events[0].Action != "auth.keyring.loaded" || events[1].Action != "auth.keyring.loaded" || events[2].Action != "auth.keyring.rotated" {
		t.Fatalf("keyring audit sequence = %+v", events)
	}
	for _, event := range events {
		if event.Result != "success" || event.TargetID == "" || len(event.Metadata) == 0 {
			t.Fatalf("incomplete keyring audit event: %+v", event)
		}
	}
}

func securityAuditDatabase(t *testing.T, migrateAudit bool) *gorm.DB {
	t.Helper()
	oldConfig, oldDB, oldLogger, oldAuth, oldLock, oldServices := Config, DB, Logger, Auth, Lock, AllService
	t.Cleanup(func() {
		Config, DB, Logger, Auth, Lock, AllService = oldConfig, oldDB, oldLogger, oldAuth, oldLock, oldServices
	})
	database, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	models := []interface{}{&model.User{}, &model.UserToken{}}
	if migrateAudit {
		models = append(models, &model.AdminAuditEvent{}, &model.ControlOperationExpectation{})
	}
	if err := database.AutoMigrate(models...); err != nil {
		t.Fatal(err)
	}
	New(&config.Config{}, database, logrus.New(), nil, lock.NewLocal())
	return database
}
