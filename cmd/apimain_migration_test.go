package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/q1ngyang/rustdesk-api-kessoku/v2/global"
	internalAuth "github.com/q1ngyang/rustdesk-api-kessoku/v2/internal/auth"
	"github.com/q1ngyang/rustdesk-api-kessoku/v2/model"
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
	user := legacyMigrationUser{Username: "legacy-user"}
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
