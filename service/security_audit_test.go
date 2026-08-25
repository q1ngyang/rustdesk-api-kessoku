package service

import (
	"context"
	"sync"
	"testing"

	"github.com/q1ngyang/rustdesk-api-kessoku/v3/config"
	"github.com/q1ngyang/rustdesk-api-kessoku/v3/lib/lock"
	"github.com/q1ngyang/rustdesk-api-kessoku/v3/model"
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

func TestRoleGrantRevokesSessionsAndCreatesAudit(t *testing.T) {
	database := securityAuditDatabase(t, true)
	isAdmin := false
	target := &model.User{Username: "role-target", Status: model.COMMON_STATUS_ENABLE, IsAdmin: &isAdmin, AuthVersion: 1}
	if err := database.Create(target).Error; err != nil {
		t.Fatal(err)
	}
	grantAdmin := true
	update := *target
	update.IsAdmin = &grantAdmin
	if err := AllService.UserService.UpdateContext(context.Background(), 77, "0191f6a0-0000-7000-8000-000000000011", &update); err != nil {
		t.Fatal(err)
	}
	refreshed := AllService.UserService.InfoById(target.Id)
	if !AllService.UserService.IsAdmin(refreshed) || refreshed.AuthVersion != 2 {
		t.Fatalf("role grant did not rotate authorization state: %+v", refreshed)
	}
	event := &model.AdminAuditEvent{}
	if err := database.Where("action = ?", "auth.user.role_changed").First(event).Error; err != nil {
		t.Fatal(err)
	}
	if event.Result != "success" || event.ActorUserID != 77 {
		t.Fatalf("role-change audit = %+v", event)
	}
}

func TestNoopUserUpdateStillProducesAuditIntentAndResult(t *testing.T) {
	database := securityAuditDatabase(t, true)
	isAdmin := false
	target := &model.User{Username: "no-op-target", Status: model.COMMON_STATUS_ENABLE, IsAdmin: &isAdmin, AuthVersion: 1}
	if err := database.Create(target).Error; err != nil {
		t.Fatal(err)
	}
	update := *target
	if err := AllService.UserService.UpdateContext(context.Background(), 77, "", &update); err != nil {
		t.Fatal(err)
	}
	event := &model.AdminAuditEvent{}
	if err := database.Where("action = ?", "auth.user.updated").First(event).Error; err != nil {
		t.Fatal(err)
	}
	if event.Result != "success" || event.ActorUserID != 77 {
		t.Fatalf("no-op user update audit = %+v", event)
	}
}

func TestConcurrentAdminDisablePreservesOneEnabledAdministrator(t *testing.T) {
	database := securityAuditDatabase(t, true)
	sqlDatabase, err := database.DB()
	if err != nil {
		t.Fatal(err)
	}
	sqlDatabase.SetMaxOpenConns(1)
	isAdmin := true
	admins := []*model.User{
		{Username: "admin-one", Status: model.COMMON_STATUS_ENABLE, IsAdmin: &isAdmin, AuthVersion: 1},
		{Username: "admin-two", Status: model.COMMON_STATUS_ENABLE, IsAdmin: &isAdmin, AuthVersion: 1},
	}
	if err := database.Create(&admins).Error; err != nil {
		t.Fatal(err)
	}

	start := make(chan struct{})
	errorsByAdmin := make([]error, len(admins))
	var wait sync.WaitGroup
	for index, admin := range admins {
		wait.Add(1)
		go func(index int, admin *model.User) {
			defer wait.Done()
			update := *admin
			update.Status = model.COMMON_STATUS_DISABLED
			<-start
			errorsByAdmin[index] = AllService.UserService.UpdateContext(context.Background(), 99, "", &update)
		}(index, admin)
	}
	close(start)
	wait.Wait()

	var enabledAdmins int64
	if err := database.Model(&model.User{}).Where("is_admin = ? AND status = ?", true, model.COMMON_STATUS_ENABLE).Count(&enabledAdmins).Error; err != nil {
		t.Fatal(err)
	}
	if enabledAdmins != 1 {
		t.Fatalf("enabled administrators = %d, want 1; operation errors=%v", enabledAdmins, errorsByAdmin)
	}
}

func TestDisabledAdministratorCanBeDeletedWhenAnotherEnabledAdminExists(t *testing.T) {
	database := securityAuditDatabase(t, true)
	if err := database.AutoMigrate(&model.UserThird{}, &model.LdapIdentity{}, &model.AddressBook{}, &model.AddressBookCollection{}, &model.AddressBookCollectionRule{}, &model.Tag{}, &model.Peer{}, &model.AdminResourceScope{}); err != nil {
		t.Fatal(err)
	}
	isAdmin := true
	enabled := &model.User{Username: "enabled-admin", Status: model.COMMON_STATUS_ENABLE, IsAdmin: &isAdmin, AuthVersion: 1}
	disabled := &model.User{Username: "disabled-admin", Status: model.COMMON_STATUS_DISABLED, IsAdmin: &isAdmin, AuthVersion: 1}
	if err := database.Create(&[]*model.User{enabled, disabled}).Error; err != nil {
		t.Fatal(err)
	}
	if err := AllService.UserService.DeleteContext(context.Background(), enabled.Id, "", disabled); err != nil {
		t.Fatal(err)
	}
	if deleted := AllService.UserService.InfoById(disabled.Id); deleted.Id != 0 {
		t.Fatal("disabled administrator was not deleted")
	}
}

func TestAuditDeletionLeavesAdministrativeAuditEvent(t *testing.T) {
	database := securityAuditDatabase(t, true)
	connection := &model.AuditConn{PeerId: "peer-1", FromPeer: "peer-2"}
	if err := database.Create(connection).Error; err != nil {
		t.Fatal(err)
	}
	if err := AllService.AuditService.DeleteAuditConnContext(context.Background(), 88, "0191f6a0-0000-7000-8000-000000000012", connection); err != nil {
		t.Fatal(err)
	}
	if err := database.First(&model.AuditConn{}, connection.Id).Error; err == nil {
		t.Fatal("connection audit was not deleted")
	}
	event := &model.AdminAuditEvent{}
	if err := database.Where("action = ?", "audit.connection.deleted").First(event).Error; err != nil {
		t.Fatal(err)
	}
	if event.Result != "success" || event.ActorUserID != 88 {
		t.Fatalf("audit-deletion event = %+v", event)
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
	database, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared&_busy_timeout=5000"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	models := []interface{}{&model.User{}, &model.UserToken{}, &model.SecurityInvariantLock{}}
	if migrateAudit {
		models = append(models, &model.AdminAuditEvent{}, &model.ControlOperationExpectation{}, &model.AuditConn{}, &model.AuditFile{})
	}
	if err := database.AutoMigrate(models...); err != nil {
		t.Fatal(err)
	}
	if err := database.Create(&model.SecurityInvariantLock{Name: "enabled-admin"}).Error; err != nil {
		t.Fatal(err)
	}
	New(&config.Config{}, database, logrus.New(), nil, lock.NewLocal())
	return database
}
