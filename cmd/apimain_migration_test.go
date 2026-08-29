package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/q1ngyang/rustdesk-api-kessoku/v3/global"
	internalAuth "github.com/q1ngyang/rustdesk-api-kessoku/v3/internal/auth"
	"github.com/q1ngyang/rustdesk-api-kessoku/v3/model"
	"github.com/sirupsen/logrus"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type legacyOauthBinding struct {
	ID     uint   `gorm:"column:id;primaryKey"`
	UserID uint   `gorm:"column:user_id"`
	Op     string `gorm:"column:op"`
	OpenID string `gorm:"column:open_id"`
}

func (legacyOauthBinding) TableName() string { return "user_thirds" }

type pre245OauthBinding struct {
	ID        uint   `gorm:"column:id;primaryKey"`
	UserID    uint   `gorm:"column:user_id"`
	OpenID    string `gorm:"column:open_id"`
	ThirdType string `gorm:"column:third_type"`
}

func (pre245OauthBinding) TableName() string { return "user_thirds" }

type legacyMigrationUser struct {
	ID       uint   `gorm:"column:id;primaryKey"`
	Username string `gorm:"column:username"`
	IsAdmin  bool   `gorm:"column:is_admin"`
}

func (legacyMigrationUser) TableName() string { return "users" }

type legacyMigrationToken struct {
	ID        uint   `gorm:"column:id;primaryKey"`
	UserID    uint   `gorm:"column:user_id"`
	Token     string `gorm:"column:token"`
	ExpiredAt int64  `gorm:"column:expired_at"`
}

func (legacyMigrationToken) TableName() string { return "user_tokens" }

type legacyMigrationServerCmd struct {
	ID     uint   `gorm:"column:id;primaryKey"`
	Cmd    string `gorm:"column:cmd"`
	Option string `gorm:"column:option"`
}

func (legacyMigrationServerCmd) TableName() string { return "server_cmds" }

func TestMigrateBackfillsAuthMetadataAndPreservesLegacyCommands(t *testing.T) {
	directory := t.TempDir()
	databasePath := filepath.Join(directory, "legacy.db")
	database, err := gorm.Open(sqlite.Open(databasePath), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	testMigrationFixture(t, database)
	sqlDatabase, err := database.DB()
	if err != nil {
		t.Fatal(err)
	}
	if err := sqlDatabase.Close(); err != nil {
		t.Fatal(err)
	}
	backup, err := os.ReadFile(databasePath)
	if err != nil {
		t.Fatal(err)
	}
	restoredPath := filepath.Join(directory, "restored.db")
	if err := os.WriteFile(restoredPath, backup, 0o600); err != nil {
		t.Fatal(err)
	}
	restored, err := gorm.Open(sqlite.Open(restoredPath), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	assertRestoredMigrationFixture(t, restored)
}

func TestMigrateRejectsDuplicateOauthIdentityBindingsBeforeAddingIndexes(t *testing.T) {
	database, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "duplicate-oauth.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := database.AutoMigrate(&legacyOauthBinding{}); err != nil {
		t.Fatal(err)
	}
	bindings := []legacyOauthBinding{
		{UserID: 1, Op: "oidc", OpenID: "subject-1"},
		{UserID: 1, Op: "oidc", OpenID: "subject-2"},
	}
	if err := database.Create(&bindings).Error; err != nil {
		t.Fatal(err)
	}

	oldDB, oldLogger := global.DB, global.Logger
	t.Cleanup(func() { global.DB, global.Logger = oldDB, oldLogger })
	global.DB = database
	global.Logger = logrus.New()
	err = Migrate(DatabaseVersion)
	if err == nil || !strings.Contains(err.Error(), "duplicate user/provider binding") {
		t.Fatalf("migration error = %v, want actionable duplicate binding failure", err)
	}
	if database.Migrator().HasIndex(&model.UserThird{}, "idx_user_thirds_user_op") {
		t.Fatal("unique OAuth index was created before duplicate preflight completed")
	}
}

func TestPre245OauthIdentityMigrationBackfillsBeforeUniqueIndexes(t *testing.T) {
	database, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "pre245-oauth.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := database.AutoMigrate(&pre245OauthBinding{}); err != nil {
		t.Fatal(err)
	}
	bindings := []pre245OauthBinding{
		{UserID: 1, ThirdType: "github", OpenID: "github-subject"},
		{UserID: 1, ThirdType: "google", OpenID: "google-subject"},
	}
	if err := database.Create(&bindings).Error; err != nil {
		t.Fatal(err)
	}

	oldDB, oldLogger := global.DB, global.Logger
	t.Cleanup(func() { global.DB, global.Logger = oldDB, oldLogger })
	global.DB = database
	global.Logger = logrus.New()
	if err := prepareLegacyOauthIdentityMigration(); err != nil {
		t.Fatal(err)
	}
	if err := validateOauthIdentityUniqueness(); err != nil {
		t.Fatalf("normalized multi-provider bindings were treated as duplicates: %v", err)
	}
	if err := database.AutoMigrate(&model.UserThird{}); err != nil {
		t.Fatal(err)
	}
	var migrated []model.UserThird
	if err := database.Order("id").Find(&migrated).Error; err != nil {
		t.Fatal(err)
	}
	if len(migrated) != 2 || migrated[0].Op != "github" || migrated[1].Op != "google" {
		t.Fatalf("legacy providers were not normalized: %+v", migrated)
	}
	for _, index := range []string{"idx_user_thirds_user_op", "idx_user_thirds_op_open_id"} {
		if !database.Migrator().HasIndex(&model.UserThird{}, index) {
			t.Fatalf("OAuth identity index %s is missing", index)
		}
	}
}

func TestMigrateRecoversLegacyAdminAfterPartialRoleColumnAddition(t *testing.T) {
	database, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "partial-role.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := database.AutoMigrate(&model.Version{}, &legacyMigrationUser{}); err != nil {
		t.Fatal(err)
	}
	if err := database.Create(&model.Version{Version: DatabaseVersion - 1}).Error; err != nil {
		t.Fatal(err)
	}
	legacyAdmin := &legacyMigrationUser{Username: "partial-admin", IsAdmin: true}
	if err := database.Create(legacyAdmin).Error; err != nil {
		t.Fatal(err)
	}
	if err := database.Exec("ALTER TABLE users ADD COLUMN role text NOT NULL DEFAULT 'user'").Error; err != nil {
		t.Fatal(err)
	}

	oldDB, oldLogger := global.DB, global.Logger
	t.Cleanup(func() { global.DB, global.Logger = oldDB, oldLogger })
	global.DB = database
	global.Logger = logrus.New()
	if err := Migrate(DatabaseVersion); err != nil {
		t.Fatal(err)
	}
	migrated := &model.User{}
	if err := database.First(migrated, legacyAdmin.ID).Error; err != nil {
		t.Fatal(err)
	}
	if migrated.Role != model.UserRoleSuperAdmin {
		t.Fatalf("partially migrated legacy administrator role = %q, want %q", migrated.Role, model.UserRoleSuperAdmin)
	}
}

func assertRestoredMigrationFixture(t *testing.T, database *gorm.DB) {
	t.Helper()
	version := &model.Version{}
	if err := database.Order("id DESC").First(version).Error; err != nil {
		t.Fatal(err)
	}
	token := &model.UserToken{}
	if err := database.First(token).Error; err != nil {
		t.Fatal(err)
	}
	var commandCount int64
	if err := database.Model(&model.ServerCmd{}).Count(&commandCount).Error; err != nil {
		t.Fatal(err)
	}
	if version.Version != DatabaseVersion || token.Token != "" || token.TokenHash == nil || len(*token.TokenHash) != 64 || commandCount != 1 {
		t.Fatalf("restored migration fixture is incomplete: version=%d token=%+v commands=%d", version.Version, token, commandCount)
	}
}

func TestMigrateV312RecoversOnlyActiveNativeDevices(t *testing.T) {
	database, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "device-v312.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := database.AutoMigrate(&model.Version{}, &model.User{}, &model.UserToken{}, &model.LoginLog{}, &model.Peer{}); err != nil {
		t.Fatal(err)
	}
	if err := database.Create(&model.Version{Version: 311}).Error; err != nil {
		t.Fatal(err)
	}
	isAdmin := false
	user := &model.User{Username: "migration-native", Status: model.COMMON_STATUS_ENABLE, AuthVersion: 1, IsAdmin: &isAdmin}
	if err := database.Create(user).Error; err != nil {
		t.Fatal(err)
	}
	now := time.Now().Unix()
	active := &model.UserToken{UserId: user.Id, DeviceId: " 384 308 369 ", DeviceUuid: "uuid-active", AuthVersion: 1, IssuedAt: now, ExpiredAt: now + 3600}
	if err := database.Create(active).Error; err != nil {
		t.Fatal(err)
	}
	if err := database.Create(&model.LoginLog{UserId: user.Id, Client: model.LoginLogClientNative, DeviceId: active.DeviceId, Uuid: active.DeviceUuid, UserTokenId: active.Id}).Error; err != nil {
		t.Fatal(err)
	}
	revokedAt := now - 10
	revoked := &model.UserToken{UserId: user.Id, DeviceId: "999 000 111", DeviceUuid: "uuid-revoked", AuthVersion: 1, IssuedAt: now - 100, ExpiredAt: now + 3600, RevokedAt: &revokedAt}
	if err := database.Create(revoked).Error; err != nil {
		t.Fatal(err)
	}
	if err := database.Create(&model.LoginLog{UserId: user.Id, Client: model.LoginLogClientNative, DeviceId: revoked.DeviceId, Uuid: revoked.DeviceUuid, UserTokenId: revoked.Id}).Error; err != nil {
		t.Fatal(err)
	}
	disabledUser := &model.User{Username: "migration-disabled", Status: model.COMMON_STATUS_DISABLED, AuthVersion: 1, IsAdmin: &isAdmin}
	if err := database.Create(disabledUser).Error; err != nil {
		t.Fatal(err)
	}
	excluded := []struct {
		token  model.UserToken
		client string
	}{
		{token: model.UserToken{UserId: user.Id, DeviceId: "expired", DeviceUuid: "uuid-expired", AuthVersion: 1, IssuedAt: now - 7200, ExpiredAt: now - 1}, client: model.LoginLogClientNative},
		{token: model.UserToken{UserId: user.Id, DeviceId: "browser", DeviceUuid: "uuid-browser", AuthVersion: 1, IssuedAt: now, ExpiredAt: now + 3600}, client: model.LoginLogClientWebAdmin},
		{token: model.UserToken{UserId: user.Id, DeviceId: "stale-auth", DeviceUuid: "uuid-stale-auth", AuthVersion: 2, IssuedAt: now, ExpiredAt: now + 3600}, client: model.LoginLogClientNative},
		{token: model.UserToken{UserId: disabledUser.Id, DeviceId: "disabled", DeviceUuid: "uuid-disabled", AuthVersion: 1, IssuedAt: now, ExpiredAt: now + 3600}, client: model.LoginLogClientNative},
	}
	for i := range excluded {
		if err := database.Create(&excluded[i].token).Error; err != nil {
			t.Fatal(err)
		}
		if err := database.Create(&model.LoginLog{
			UserId: excluded[i].token.UserId, Client: excluded[i].client,
			DeviceId: excluded[i].token.DeviceId, Uuid: excluded[i].token.DeviceUuid,
			UserTokenId: excluded[i].token.Id,
		}).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := database.Create(&model.Peer{Id: "384308369", Alias: "preserve-me"}).Error; err != nil {
		t.Fatal(err)
	}
	// Simulate a direct upgrade from a release that predates auth_version. The
	// generic generation backfill must run before v312 evaluates this session.
	if err := database.Model(&model.User{}).Where("id = ?", user.Id).Update("auth_version", 0).Error; err != nil {
		t.Fatal(err)
	}
	if err := database.Model(&model.UserToken{}).Where("id = ?", active.Id).Update("auth_version", 0).Error; err != nil {
		t.Fatal(err)
	}

	oldDB, oldLogger := global.DB, global.Logger
	t.Cleanup(func() { global.DB, global.Logger = oldDB, oldLogger })
	global.DB, global.Logger = database, logrus.New()
	if err := Migrate(DatabaseVersion); err != nil {
		t.Fatal(err)
	}
	if err := database.First(active, active.Id).Error; err != nil {
		t.Fatal(err)
	}
	if active.Client != model.LoginLogClientNative || active.DeviceId != "384308369" || active.AuthVersion != 1 {
		t.Fatalf("active token migration incomplete: %+v", active)
	}
	peer := &model.Peer{}
	if err := database.Where("id = ?", "384308369").First(peer).Error; err != nil {
		t.Fatal(err)
	}
	if peer.Uuid != active.DeviceUuid || peer.UserId != user.Id || peer.IdentitySource != model.PeerIdentitySourceLogin || peer.LastSysinfoTime != 0 || peer.Alias != "preserve-me" {
		t.Fatalf("active native peer migration incomplete: %+v", peer)
	}
	excludedUUIDs := []string{revoked.DeviceUuid}
	for i := range excluded {
		excludedUUIDs = append(excludedUUIDs, excluded[i].token.DeviceUuid)
	}
	var excludedPeerCount int64
	if err := database.Model(&model.Peer{}).Where("uuid IN ?", excludedUUIDs).Count(&excludedPeerCount).Error; err != nil {
		t.Fatal(err)
	}
	if excludedPeerCount != 0 {
		t.Fatalf("%d ineligible sessions were resurrected as peers", excludedPeerCount)
	}
}

func testMigrationFixture(t *testing.T, database *gorm.DB) {
	t.Helper()
	oldDB, oldLogger := global.DB, global.Logger
	t.Cleanup(func() {
		global.DB, global.Logger = oldDB, oldLogger
	})
	if err := database.AutoMigrate(
		&model.Version{},
		&legacyMigrationUser{},
		&legacyMigrationToken{},
		&legacyMigrationServerCmd{},
	); err != nil {
		t.Fatal(err)
	}
	if err := database.Create(&model.Version{Version: DatabaseVersion - 1}).Error; err != nil {
		t.Fatal(err)
	}
	user := legacyMigrationUser{Username: "legacy-admin", IsAdmin: true}
	if err := database.Create(&user).Error; err != nil {
		t.Fatal(err)
	}
	const legacyPlaintextToken = "legacy-token-that-must-be-hashed"
	legacyToken := legacyMigrationToken{
		UserID:    user.ID,
		Token:     legacyPlaintextToken,
		ExpiredAt: time.Now().Add(time.Hour).Unix(),
	}
	if err := database.Create(&legacyToken).Error; err != nil {
		t.Fatal(err)
	}
	legacyCommand := legacyMigrationServerCmd{Cmd: "arbitrary-command", Option: "--legacy"}
	if err := database.Create(&legacyCommand).Error; err != nil {
		t.Fatal(err)
	}

	global.DB = database
	global.Logger = logrus.New()
	if err := Migrate(DatabaseVersion); err != nil {
		t.Fatal(err)
	}

	migratedUser := &model.User{}
	if err := database.First(migratedUser, user.ID).Error; err != nil {
		t.Fatal(err)
	}
	if migratedUser.AuthVersion != 1 {
		t.Fatalf("auth_version = %d, want 1", migratedUser.AuthVersion)
	}
	if migratedUser.Role != model.UserRoleSuperAdmin || migratedUser.IsAdmin == nil || !*migratedUser.IsAdmin {
		t.Fatalf("legacy administrator role was not preserved as super administrator: %+v", migratedUser)
	}
	migratedToken := &model.UserToken{}
	if err := database.First(migratedToken, legacyToken.ID).Error; err != nil {
		t.Fatal(err)
	}
	if migratedToken.AuthVersion != 1 || migratedToken.TokenHash == nil {
		t.Fatalf("token auth metadata was not backfilled: %+v", migratedToken)
	}
	if !internalAuth.ConstantTimeHashEqual(legacyPlaintextToken, *migratedToken.TokenHash) {
		t.Fatal("legacy token hash does not match the stored credential")
	}
	if migratedToken.Token != "" {
		t.Fatal("legacy plaintext token was not cleared after its hash was backfilled")
	}
	var commandCount int64
	if err := database.Model(&model.ServerCmd{}).Where("id = ?", legacyCommand.ID).Count(&commandCount).Error; err != nil {
		t.Fatal(err)
	}
	if commandCount != 1 {
		t.Fatal("legacy server command row was not preserved for compatibility/export")
	}
	latest := &model.Version{}
	if err := database.Last(latest).Error; err != nil {
		t.Fatal(err)
	}
	if latest.Version != DatabaseVersion {
		t.Fatalf("database version = %d, want %d", latest.Version, DatabaseVersion)
	}
}
