package main

import (
	"context"
	"io"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/q1ngyang/rustdesk-api-kessoku/v3/config"
	databaseSchema "github.com/q1ngyang/rustdesk-api-kessoku/v3/internal/database"
	"github.com/q1ngyang/rustdesk-api-kessoku/v3/model"
	"github.com/q1ngyang/rustdesk-api-kessoku/v3/service"
	"github.com/sirupsen/logrus"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestMySQLLegacyMigration(t *testing.T) {
	dsn := os.Getenv("KESSOKU_TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("KESSOKU_TEST_MYSQL_DSN is not set")
	}
	database, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	requireEmptyIntegrationDatabase(t, database)
	testMigrationFixture(t, database)
	verifyDialectMaintenanceContract(t, database, config.TypeMysql)
	verifyDialectMigrationLock(t, database, config.TypeMysql)
}

func TestPostgreSQLLegacyMigration(t *testing.T) {
	dsn := os.Getenv("KESSOKU_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("KESSOKU_TEST_POSTGRES_DSN is not set")
	}
	database, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	requireEmptyIntegrationDatabase(t, database)
	testMigrationFixture(t, database)
	verifyDialectMaintenanceContract(t, database, config.TypePostgresql)
	verifyDialectMigrationLock(t, database, config.TypePostgresql)
}

func requireEmptyIntegrationDatabase(t *testing.T, database *gorm.DB) {
	t.Helper()
	databaseName := database.Migrator().CurrentDatabase()
	if !strings.HasPrefix(databaseName, "kessoku_test_") {
		t.Fatalf("refusing migration fixture against database %q; name must start with kessoku_test_", databaseName)
	}
	tables, err := database.Migrator().GetTables()
	if err != nil {
		t.Fatal(err)
	}
	if len(tables) != 0 {
		t.Fatalf("refusing migration fixture against non-empty database %q: %v", databaseName, tables)
	}
}

func verifyDialectMaintenanceContract(t *testing.T, database *gorm.DB, databaseType string) {
	t.Helper()
	beforeTables, err := database.Migrator().GetTables()
	if err != nil {
		t.Fatal(err)
	}
	status, err := databaseSchema.InspectSchema(database)
	if err != nil || status.State != databaseSchema.StateCurrent || status.InstalledSchema == nil || *status.InstalledSchema != DatabaseVersion {
		t.Fatalf("%s schema status=%+v err=%v", databaseType, status, err)
	}

	oldConfig, oldDB, oldLogger, oldAuth, oldLock, oldServices := service.Config, service.DB, service.Logger, service.Auth, service.Lock, service.AllService
	t.Cleanup(func() {
		service.Config, service.DB, service.Logger, service.Auth, service.Lock, service.AllService = oldConfig, oldDB, oldLogger, oldAuth, oldLock, oldServices
	})
	logger := logrus.New()
	logger.SetOutput(io.Discard)
	service.NewMaintenance(&config.Config{}, database, logger)

	isAdmin := false
	user := &model.User{
		Username: "dialect-recovery-" + databaseType,
		Password: "not-used-by-maintenance-fixture",
		Status:   model.COMMON_STATUS_DISABLED, Role: model.UserRoleUser,
		IsAdmin: &isAdmin, AuthVersion: 11,
	}
	if err := database.Create(user).Error; err != nil {
		t.Fatal(err)
	}
	for _, row := range []interface{}{
		&model.UserToken{UserId: user.Id},
		&model.AdminResourceScope{AdminUserId: user.Id, ScopeType: model.AdminScopeTypeGroup, ScopeId: 41},
		&model.UserTwoFactor{UserID: user.Id, SecretCiphertext: "dialect-secret", PendingSecretCiphertext: "dialect-pending", Enabled: true},
		&model.TwoFactorLoginChallenge{TokenHash: "dialect-challenge-" + databaseType, UserID: user.Id, Username: user.Username, ExpiresAt: time.Now().Add(time.Minute).Unix()},
	} {
		if err := database.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}

	recovered, err := service.AllService.UserService.RecoverAdministratorContext(context.Background(), service.RecoverAdministratorOptions{
		Selector:  service.MaintenanceSelector{UserID: user.Id, ConfirmUsername: user.Username},
		RequestID: "0191f6a0-0000-7000-8000-000000000301", ResetTwoFactor: true,
	})
	if err != nil {
		t.Fatalf("%s administrator recovery: %v", databaseType, err)
	}
	if recovered.AuthVersion != 12 || recovered.ScopesCleared != 1 || recovered.SessionsRevoked != 1 || !recovered.TwoFactorWasConfigured {
		t.Fatalf("%s administrator recovery result=%+v", databaseType, recovered)
	}
	reset, err := service.AllService.UserService.ResetTwoFactorMaintenanceContext(context.Background(), service.ResetTwoFactorOptions{
		Selector:  service.MaintenanceSelector{Username: user.Username, ConfirmUsername: user.Username},
		RequestID: "0191f6a0-0000-7000-8000-000000000302",
	})
	if err != nil {
		t.Fatalf("%s idempotent two-factor reset: %v", databaseType, err)
	}
	if reset.AuthVersion != 13 || reset.TwoFactorWasConfigured || reset.SessionsRevoked != 0 {
		t.Fatalf("%s two-factor reset result=%+v", databaseType, reset)
	}
	refreshed := &model.User{}
	if err := database.First(refreshed, user.Id).Error; err != nil {
		t.Fatal(err)
	}
	if refreshed.Status != model.COMMON_STATUS_ENABLE || refreshed.Role != model.UserRoleSuperAdmin || refreshed.IsAdmin == nil || !*refreshed.IsAdmin || refreshed.AuthVersion != 13 {
		t.Fatalf("%s recovered user=%+v", databaseType, refreshed)
	}
	for _, row := range []struct {
		model  interface{}
		column string
	}{
		{model: &model.AdminResourceScope{}, column: "admin_user_id"},
		{model: &model.UserTwoFactor{}, column: "user_id"},
		{model: &model.TwoFactorLoginChallenge{}, column: "user_id"},
	} {
		var count int64
		if err := database.Model(row.model).Where(row.column+" = ?", user.Id).Count(&count).Error; err != nil || count != 0 {
			t.Fatalf("%s residual maintenance rows model=%T count=%d err=%v", databaseType, row.model, count, err)
		}
	}
	var successfulAudits int64
	if err := database.Model(&model.AdminAuditEvent{}).
		Where("request_id IN ? AND result = ?", []string{"0191f6a0-0000-7000-8000-000000000301", "0191f6a0-0000-7000-8000-000000000302"}, "success").
		Count(&successfulAudits).Error; err != nil || successfulAudits != 2 {
		t.Fatalf("%s successful maintenance audits=%d err=%v", databaseType, successfulAudits, err)
	}
	status, err = databaseSchema.InspectSchema(database)
	if err != nil || status.State != databaseSchema.StateCurrent || status.InstalledSchema == nil || *status.InstalledSchema != DatabaseVersion {
		t.Fatalf("%s post-maintenance schema status=%+v err=%v", databaseType, status, err)
	}
	afterTables, err := database.Migrator().GetTables()
	if err != nil {
		t.Fatal(err)
	}
	if !sameTableSet(beforeTables, afterTables) {
		t.Fatalf("%s maintenance changed schema tables: before=%v after=%v", databaseType, beforeTables, afterTables)
	}
}

func verifyDialectMigrationLock(t *testing.T, database *gorm.DB, databaseType string) {
	t.Helper()
	cfg := &config.Config{Gorm: config.Gorm{Type: databaseType}}
	databaseName := database.Migrator().CurrentDatabase()
	if databaseType == config.TypeMysql {
		cfg.Mysql.Dbname = databaseName
	} else {
		cfg.Postgresql.Dbname = databaseName
	}
	start := make(chan struct{})
	errorsFound := make([]error, 2)
	var inside, maximum atomic.Int32
	var wait sync.WaitGroup
	for index := range errorsFound {
		wait.Add(1)
		go func(index int) {
			defer wait.Done()
			<-start
			errorsFound[index] = withMigrationLock(context.Background(), cfg, database, func(*gorm.DB) error {
				current := inside.Add(1)
				for {
					observed := maximum.Load()
					if current <= observed || maximum.CompareAndSwap(observed, current) {
						break
					}
				}
				time.Sleep(75 * time.Millisecond)
				inside.Add(-1)
				return nil
			})
		}(index)
	}
	close(start)
	wait.Wait()
	if errorsFound[0] != nil || errorsFound[1] != nil || maximum.Load() != 1 {
		t.Fatalf("%s migration lock did not serialize callbacks: max=%d errors=%v", databaseType, maximum.Load(), errorsFound)
	}
}

func sameTableSet(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	counts := make(map[string]int, len(left))
	for _, table := range left {
		counts[table]++
	}
	for _, table := range right {
		counts[table]--
	}
	for _, count := range counts {
		if count != 0 {
			return false
		}
	}
	return true
}
