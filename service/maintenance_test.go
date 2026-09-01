package service

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/q1ngyang/rustdesk-api-kessoku/v3/config"
	"github.com/q1ngyang/rustdesk-api-kessoku/v3/internal/buildinfo"
	"github.com/q1ngyang/rustdesk-api-kessoku/v3/model"
	"github.com/q1ngyang/rustdesk-api-kessoku/v3/utils"
	"github.com/sirupsen/logrus"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestRecoverAdministratorAtomicallyRestoresAccessAndRevokesSessions(t *testing.T) {
	database := maintenanceTestDatabase(t, buildinfo.DatabaseSchema, true)
	user := maintenanceUserFixture(t, database, "recover-me", model.UserRoleUser, model.COMMON_STATUS_DISABLED)
	originalAuthVersion := user.AuthVersion
	activeTokens := []model.UserToken{{UserId: user.Id}, {UserId: user.Id}}
	if err := database.Create(&activeTokens).Error; err != nil {
		t.Fatal(err)
	}
	if err := database.Create(&[]model.AdminResourceScope{
		{AdminUserId: user.Id, ScopeType: model.AdminScopeTypeGroup, ScopeId: 11},
		{AdminUserId: user.Id, ScopeType: model.AdminScopeTypePeer, ScopeId: 12},
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := database.Create(&model.UserTwoFactor{UserID: user.Id, SecretCiphertext: "enabled-secret", PendingSecretCiphertext: "pending-secret", PendingExpiresAt: time.Now().Add(time.Minute).Unix(), Enabled: true}).Error; err != nil {
		t.Fatal(err)
	}
	if err := database.Create(&model.TwoFactorLoginChallenge{TokenHash: "recover-challenge", UserID: user.Id, Username: user.Username, ExpiresAt: time.Now().Add(time.Minute).Unix()}).Error; err != nil {
		t.Fatal(err)
	}

	const password = "replacement-password-123"
	const requestID = "0191f6a0-0000-7000-8000-000000000201"
	result, err := AllService.UserService.RecoverAdministratorContext(context.Background(), RecoverAdministratorOptions{
		Selector:  MaintenanceSelector{Username: user.Username, ConfirmUsername: user.Username},
		RequestID: requestID, Password: password, ResetTwoFactor: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.UserID != user.Id || result.Username != user.Username || result.AuthVersion != originalAuthVersion+1 || !result.PasswordReset || !result.TwoFactorReset || !result.TwoFactorWasConfigured || result.SessionsRevoked != 2 || result.ScopesCleared != 2 || result.LoginChallengesCleared != 1 {
		t.Fatalf("unexpected recovery result: %+v", result)
	}

	refreshed := &model.User{}
	if err := database.First(refreshed, user.Id).Error; err != nil {
		t.Fatal(err)
	}
	if refreshed.Status != model.COMMON_STATUS_ENABLE || refreshed.Role != model.UserRoleSuperAdmin || refreshed.IsAdmin == nil || !*refreshed.IsAdmin || refreshed.AuthVersion != originalAuthVersion+1 {
		t.Fatalf("administrator was not fully recovered: %+v", refreshed)
	}
	if valid, _, err := utils.VerifyPassword(refreshed.Password, password); err != nil || !valid {
		t.Fatalf("replacement password does not verify: valid=%t err=%v", valid, err)
	}
	assertMaintenanceRows(t, database, user.Id, 0, 0, 0)
	var revoked []model.UserToken
	if err := database.Where("user_id = ?", user.Id).Find(&revoked).Error; err != nil {
		t.Fatal(err)
	}
	for _, token := range revoked {
		if token.RevokedAt == nil || token.RevokedReason != "local_administrator_recovery" {
			t.Fatalf("session was not revoked safely: %+v", token)
		}
	}
	event := maintenanceAuditEvent(t, database, "maintenance.admin.recovered")
	if event.Result != "success" || event.ErrorCode != "" || event.RequestID != requestID || event.ActorUserID != 0 || event.TargetID != user.Username {
		t.Fatalf("unexpected recovery audit: %+v", event)
	}
	encoded, err := json.Marshal(event)
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{password, refreshed.Password, "enabled-secret", "pending-secret", "recover-challenge"} {
		if strings.Contains(string(encoded), secret) {
			t.Fatalf("recovery audit leaked sensitive material %q: %s", secret, encoded)
		}
	}
}

func TestRecoverAdministratorAlreadySuperAdminClearsLegacyScopeAndBumpsOnce(t *testing.T) {
	database := maintenanceTestDatabase(t, buildinfo.DatabaseSchema, true)
	user := maintenanceUserFixture(t, database, "existing-super", model.UserRoleSuperAdmin, model.COMMON_STATUS_ENABLE)
	if err := database.Create(&model.AdminResourceScope{AdminUserId: user.Id, ScopeType: model.AdminScopeTypeGroup, ScopeId: 42}).Error; err != nil {
		t.Fatal(err)
	}
	if err := database.Create(&model.UserTwoFactor{UserID: user.Id, SecretCiphertext: "keep-me", Enabled: true}).Error; err != nil {
		t.Fatal(err)
	}
	if err := database.Create(&model.TwoFactorLoginChallenge{TokenHash: "clear-me", UserID: user.Id, Username: user.Username, ExpiresAt: time.Now().Add(time.Minute).Unix()}).Error; err != nil {
		t.Fatal(err)
	}
	result, err := AllService.UserService.RecoverAdministratorContext(context.Background(), RecoverAdministratorOptions{
		Selector: MaintenanceSelector{UserID: user.Id, ConfirmUsername: user.Username}, RequestID: "0191f6a0-0000-7000-8000-000000000202",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.AuthVersion != user.AuthVersion+1 || result.ScopesCleared != 1 || result.LoginChallengesCleared != 1 || result.TwoFactorReset {
		t.Fatalf("unexpected no-option recovery result: %+v", result)
	}
	assertMaintenanceRows(t, database, user.Id, 0, 1, 0)
	refreshed := &model.User{}
	if err := database.First(refreshed, user.Id).Error; err != nil || refreshed.AuthVersion != user.AuthVersion+1 {
		t.Fatalf("auth version did not increase exactly once: user=%+v err=%v", refreshed, err)
	}
}

func TestResetTwoFactorMaintenanceIsIdempotentAndPreservesGlobalKey(t *testing.T) {
	database := maintenanceTestDatabase(t, buildinfo.DatabaseSchema, true)
	keyPath := filepath.Join(t.TempDir(), "totp.key")
	key := []byte("01234567890123456789012345678901")
	if err := os.WriteFile(keyPath, key, 0o600); err != nil {
		t.Fatal(err)
	}
	Config.TwoFactor = config.TwoFactor{Enabled: true, KeyFile: keyPath}
	user := maintenanceUserFixture(t, database, "two-factor-user", model.UserRoleUser, model.COMMON_STATUS_ENABLE)
	if err := database.Create(&model.UserTwoFactor{UserID: user.Id, SecretCiphertext: "ciphertext", PendingSecretCiphertext: "pending", Enabled: true}).Error; err != nil {
		t.Fatal(err)
	}
	if err := database.Create(&model.UserToken{UserId: user.Id}).Error; err != nil {
		t.Fatal(err)
	}

	first, err := AllService.UserService.ResetTwoFactorMaintenanceContext(context.Background(), ResetTwoFactorOptions{
		Selector: MaintenanceSelector{UserID: user.Id, ConfirmUsername: user.Username}, RequestID: "0191f6a0-0000-7000-8000-000000000203",
	})
	if err != nil {
		t.Fatal(err)
	}
	second, err := AllService.UserService.ResetTwoFactorMaintenanceContext(context.Background(), ResetTwoFactorOptions{
		Selector: MaintenanceSelector{Username: user.Username, ConfirmUsername: user.Username}, RequestID: "0191f6a0-0000-7000-8000-000000000204",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !first.TwoFactorWasConfigured || second.TwoFactorWasConfigured || first.AuthVersion != user.AuthVersion+1 || second.AuthVersion != user.AuthVersion+2 {
		t.Fatalf("reset was not idempotent: first=%+v second=%+v", first, second)
	}
	assertMaintenanceRows(t, database, user.Id, 0, 0, 0)
	if after, err := os.ReadFile(keyPath); err != nil || string(after) != string(key) {
		t.Fatalf("global TOTP key changed: %q err=%v", after, err)
	}
	var events int64
	if err := database.Model(&model.AdminAuditEvent{}).Where("action = ? AND result = ?", "maintenance.two_factor.reset", "success").Count(&events).Error; err != nil || events != 2 {
		t.Fatalf("two-factor audit events=%d err=%v", events, err)
	}
}

func TestMaintenanceSelectionFailuresDoNotModifyTarget(t *testing.T) {
	database := maintenanceTestDatabase(t, buildinfo.DatabaseSchema, true)
	user := maintenanceUserFixture(t, database, "confirmed-name", model.UserRoleUser, model.COMMON_STATUS_DISABLED)
	if err := database.Create(&model.UserToken{UserId: user.Id}).Error; err != nil {
		t.Fatal(err)
	}

	_, err := AllService.UserService.RecoverAdministratorContext(context.Background(), RecoverAdministratorOptions{
		Selector: MaintenanceSelector{UserID: user.Id, ConfirmUsername: "wrong-name"}, RequestID: "0191f6a0-0000-7000-8000-000000000205",
	})
	if serviceCode := MaintenanceErrorCode(err, ""); serviceCode != MaintenanceCodeConfirmationMismatch {
		t.Fatalf("confirmation error=%v code=%s", err, serviceCode)
	}
	_, err = AllService.UserService.RecoverAdministratorContext(context.Background(), RecoverAdministratorOptions{
		Selector: MaintenanceSelector{UserID: user.Id + 999, ConfirmUsername: "missing"}, RequestID: "0191f6a0-0000-7000-8000-000000000206",
	})
	if serviceCode := MaintenanceErrorCode(err, ""); serviceCode != MaintenanceCodeUserNotFound {
		t.Fatalf("not-found error=%v code=%s", err, serviceCode)
	}
	refreshed := &model.User{}
	if err := database.First(refreshed, user.Id).Error; err != nil {
		t.Fatal(err)
	}
	if refreshed.Status != user.Status || refreshed.Role != user.Role || refreshed.AuthVersion != user.AuthVersion {
		t.Fatalf("selection failure modified user: before=%+v after=%+v", user, refreshed)
	}
	var token model.UserToken
	if err := database.Where("user_id = ?", user.Id).First(&token).Error; err != nil || token.RevokedAt != nil {
		t.Fatalf("selection failure revoked token: %+v err=%v", token, err)
	}
	var failures []model.AdminAuditEvent
	if err := database.Where("result = ?", "failure").Order("id").Find(&failures).Error; err != nil {
		t.Fatal(err)
	}
	if len(failures) != 2 || failures[0].ErrorCode != MaintenanceCodeConfirmationMismatch || failures[1].ErrorCode != MaintenanceCodeUserNotFound {
		t.Fatalf("selection failure audits=%+v", failures)
	}
}

func TestMaintenanceFutureSchemaFailsWithoutAuditOrAccountMutation(t *testing.T) {
	database := maintenanceTestDatabase(t, buildinfo.DatabaseSchema+1, true)
	user := maintenanceUserFixture(t, database, "future-schema", model.UserRoleUser, model.COMMON_STATUS_DISABLED)
	_, err := AllService.UserService.RecoverAdministratorContext(context.Background(), RecoverAdministratorOptions{
		Selector: MaintenanceSelector{UserID: user.Id, ConfirmUsername: user.Username}, RequestID: "0191f6a0-0000-7000-8000-000000000207",
	})
	if MaintenanceErrorCode(err, "") != MaintenanceCodeSchemaMismatch {
		t.Fatalf("future schema error=%v", err)
	}
	refreshed := &model.User{}
	if err := database.First(refreshed, user.Id).Error; err != nil || refreshed.Status != user.Status || refreshed.Role != user.Role || refreshed.AuthVersion != user.AuthVersion {
		t.Fatalf("future schema modified user: %+v err=%v", refreshed, err)
	}
	var audits int64
	if err := database.Model(&model.AdminAuditEvent{}).Count(&audits).Error; err != nil || audits != 0 {
		t.Fatalf("future schema was written: audits=%d err=%v", audits, err)
	}
}

func TestRecoverAdministratorRollsBackEveryAccountChangeOnDatabaseFailure(t *testing.T) {
	database := maintenanceTestDatabase(t, buildinfo.DatabaseSchema, true)
	user := maintenanceUserFixture(t, database, "rollback-user", model.UserRoleUser, model.COMMON_STATUS_DISABLED)
	if err := database.Create(&model.UserToken{UserId: user.Id}).Error; err != nil {
		t.Fatal(err)
	}
	if err := database.Create(&model.AdminResourceScope{AdminUserId: user.Id, ScopeType: model.AdminScopeTypeGroup, ScopeId: 9}).Error; err != nil {
		t.Fatal(err)
	}
	if err := database.Create(&model.UserTwoFactor{UserID: user.Id, SecretCiphertext: "rollback-secret", Enabled: true}).Error; err != nil {
		t.Fatal(err)
	}
	if err := database.Exec(`CREATE TRIGGER fail_maintenance_token_update BEFORE UPDATE ON user_tokens
		BEGIN SELECT RAISE(ABORT, 'injected token failure'); END`).Error; err != nil {
		t.Fatal(err)
	}
	_, err := AllService.UserService.RecoverAdministratorContext(context.Background(), RecoverAdministratorOptions{
		Selector: MaintenanceSelector{UserID: user.Id, ConfirmUsername: user.Username}, RequestID: "0191f6a0-0000-7000-8000-000000000208", Password: "rollback-password-123", ResetTwoFactor: true,
	})
	if err == nil || !strings.Contains(err.Error(), "revoke recovered administrator sessions") {
		t.Fatalf("injected failure was not returned: %v", err)
	}
	refreshed := &model.User{}
	if err := database.First(refreshed, user.Id).Error; err != nil {
		t.Fatal(err)
	}
	if refreshed.Status != user.Status || refreshed.Role != user.Role || refreshed.AuthVersion != user.AuthVersion || refreshed.Password != user.Password {
		t.Fatalf("failed recovery was not rolled back: before=%+v after=%+v", user, refreshed)
	}
	assertMaintenanceRows(t, database, user.Id, 1, 1, 0)
	event := maintenanceAuditEvent(t, database, "maintenance.admin.recovered")
	if event.Result != "failure" || event.ErrorCode != MaintenanceCodeRecoveryFailed {
		t.Fatalf("rollback failure audit=%+v", event)
	}
}

func TestResetTwoFactorUsesOperationSpecificDatabaseFailureCode(t *testing.T) {
	database := maintenanceTestDatabase(t, buildinfo.DatabaseSchema, true)
	if err := database.Migrator().DropTable(&model.User{}); err != nil {
		t.Fatal(err)
	}
	_, err := AllService.UserService.ResetTwoFactorMaintenanceContext(context.Background(), ResetTwoFactorOptions{
		Selector: MaintenanceSelector{UserID: 7, ConfirmUsername: "missing-table"}, RequestID: "0191f6a0-0000-7000-8000-000000000210",
	})
	if MaintenanceErrorCode(err, "") != MaintenanceCodeTwoFactorResetFailed {
		t.Fatalf("two-factor query failure code=%s err=%v", MaintenanceErrorCode(err, ""), err)
	}
	event := maintenanceAuditEvent(t, database, "maintenance.two_factor.reset")
	if event.Result != "failure" || event.ErrorCode != MaintenanceCodeTwoFactorResetFailed {
		t.Fatalf("two-factor query failure audit=%+v", event)
	}
}

func TestExistingPasswordResetRotatesAuthAndAuditsWithoutSecret(t *testing.T) {
	database := maintenanceTestDatabase(t, buildinfo.DatabaseSchema, true)
	user := maintenanceUserFixture(t, database, "password-reset", model.UserRoleUser, model.COMMON_STATUS_ENABLE)
	if err := database.Create(&[]model.UserToken{{UserId: user.Id}, {UserId: user.Id}}).Error; err != nil {
		t.Fatal(err)
	}
	const password = "existing-command-password"
	originalAuthVersion := user.AuthVersion
	if err := AllService.UserService.UpdatePasswordContext(context.Background(), 0, "0191f6a0-0000-7000-8000-000000000209", user, password); err != nil {
		t.Fatal(err)
	}
	refreshed := &model.User{}
	if err := database.First(refreshed, user.Id).Error; err != nil || refreshed.AuthVersion != originalAuthVersion+1 {
		t.Fatalf("password reset auth version=%d err=%v", refreshed.AuthVersion, err)
	}
	var active int64
	if err := database.Model(&model.UserToken{}).Where("user_id = ? AND revoked_at IS NULL", user.Id).Count(&active).Error; err != nil || active != 0 {
		t.Fatalf("password reset active tokens=%d err=%v", active, err)
	}
	event := maintenanceAuditEvent(t, database, "auth.password.changed")
	encoded, _ := json.Marshal(event)
	if event.Result != "success" || strings.Contains(string(encoded), password) || strings.Contains(string(encoded), refreshed.Password) {
		t.Fatalf("password reset audit is unsafe: %+v", event)
	}
}

func maintenanceTestDatabase(t *testing.T, schema uint, includeAudit bool) *gorm.DB {
	t.Helper()
	oldConfig, oldDB, oldLogger, oldAuth, oldLock, oldServices := Config, DB, Logger, Auth, Lock, AllService
	t.Cleanup(func() {
		Config, DB, Logger, Auth, Lock, AllService = oldConfig, oldDB, oldLogger, oldAuth, oldLock, oldServices
	})
	path := filepath.Join(t.TempDir(), "maintenance.db")
	database, err := gorm.Open(sqlite.Open(path), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	models := []interface{}{&model.Version{}, &model.User{}, &model.UserToken{}, &model.AdminResourceScope{}, &model.UserTwoFactor{}, &model.TwoFactorLoginChallenge{}}
	if includeAudit {
		models = append(models, &model.AdminAuditEvent{})
	}
	if err := database.AutoMigrate(models...); err != nil {
		t.Fatal(err)
	}
	if err := database.Create(&model.Version{Version: schema}).Error; err != nil {
		t.Fatal(err)
	}
	NewMaintenance(&config.Config{}, database, logrus.New())
	return database
}

func maintenanceUserFixture(t *testing.T, database *gorm.DB, username string, role model.UserRole, status model.StatusCode) *model.User {
	t.Helper()
	isAdmin := role == model.UserRoleAdmin || role == model.UserRoleSuperAdmin
	hash, err := utils.EncryptPassword("original-password-123")
	if err != nil {
		t.Fatal(err)
	}
	user := &model.User{Username: username, Password: hash, Role: role, IsAdmin: &isAdmin, Status: status, AuthVersion: 7}
	if err := database.Create(user).Error; err != nil {
		t.Fatal(err)
	}
	return user
}

func assertMaintenanceRows(t *testing.T, database *gorm.DB, userID uint, scopes, factors, challenges int64) {
	t.Helper()
	for _, check := range []struct {
		model interface{}
		want  int64
	}{
		{model: &model.AdminResourceScope{}, want: scopes},
		{model: &model.UserTwoFactor{}, want: factors},
		{model: &model.TwoFactorLoginChallenge{}, want: challenges},
	} {
		var count int64
		column := "user_id"
		if _, ok := check.model.(*model.AdminResourceScope); ok {
			column = "admin_user_id"
		}
		if err := database.Model(check.model).Where(column+" = ?", userID).Count(&count).Error; err != nil || count != check.want {
			t.Fatalf("maintenance row count model=%T got=%d want=%d err=%v", check.model, count, check.want, err)
		}
	}
}

func maintenanceAuditEvent(t *testing.T, database *gorm.DB, action string) model.AdminAuditEvent {
	t.Helper()
	event := model.AdminAuditEvent{}
	if err := database.Where("action = ?", action).Order("id DESC").First(&event).Error; err != nil {
		t.Fatal(err)
	}
	return event
}
