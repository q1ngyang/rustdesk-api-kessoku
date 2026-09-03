package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/q1ngyang/rustdesk-api-kessoku/v3/config"
	"github.com/q1ngyang/rustdesk-api-kessoku/v3/global"
	"github.com/q1ngyang/rustdesk-api-kessoku/v3/internal/buildinfo"
	databaseSchema "github.com/q1ngyang/rustdesk-api-kessoku/v3/internal/database"
	"github.com/q1ngyang/rustdesk-api-kessoku/v3/model"
	"github.com/q1ngyang/rustdesk-api-kessoku/v3/service"
	"github.com/q1ngyang/rustdesk-api-kessoku/v3/utils"
	"github.com/sirupsen/logrus"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestVersionCommandNeedsNoConfigurationDatabaseOrKeys(t *testing.T) {
	directory := t.TempDir()
	withWorkingDirectory(t, directory)
	oldVersion, oldCommit, oldBuildTime := buildinfo.Version, buildinfo.GitCommit, buildinfo.BuildTime
	t.Cleanup(func() {
		buildinfo.Version, buildinfo.GitCommit, buildinfo.BuildTime = oldVersion, oldCommit, oldBuildTime
	})
	buildinfo.Version, buildinfo.GitCommit, buildinfo.BuildTime = "3.0.7-test", "fixture-commit", "2026-08-31T00:00:00Z"

	stdout, stderr, err := executeRoot("--config", filepath.Join(directory, "missing.yaml"), "version", "--json")
	if err != nil {
		t.Fatalf("version failed without runtime state: %v stderr=%s", err, stderr)
	}
	result := versionOutput{}
	if err := json.Unmarshal(stdout, &result); err != nil {
		t.Fatalf("decode version JSON: %v: %s", err, stdout)
	}
	if result.SchemaVersion != cliSchemaVersion || result.Component != buildinfo.Component || result.Version != "3.0.7-test" || result.DatabaseSchema != DatabaseVersion || result.GitCommit != "fixture-commit" || result.BuildTime != "2026-08-31T00:00:00Z" || result.GoVersion == "" {
		t.Fatalf("unexpected version result: %+v", result)
	}
	if entries, err := os.ReadDir(directory); err != nil || len(entries) != 0 {
		t.Fatalf("version created local state: entries=%v err=%v", entries, err)
	}
}

func TestConfigValidateIsReadOnlyAndReturnsStructuredWarnings(t *testing.T) {
	directory := t.TempDir()
	resources := filepath.Join(directory, "resources")
	if err := os.Mkdir(resources, 0o755); err != nil {
		t.Fatal(err)
	}
	configPath := writeCLIConfig(t, directory, true)
	before := directorySnapshot(t, directory)
	stdout, stderr, err := executeRoot("config", "validate", "--config", configPath, "--json")
	if err != nil {
		t.Fatalf("valid configuration rejected: %v stderr=%s stdout=%s", err, stderr, stdout)
	}
	result := configValidationOutput{}
	if err := json.Unmarshal(stdout, &result); err != nil {
		t.Fatal(err)
	}
	if !result.Valid || result.DatabaseType != config.TypeSqlite || len(result.Errors) != 0 || len(result.Warnings) != 1 || result.Warnings[0].Code != "TWO_FACTOR_KEY_PENDING" {
		t.Fatalf("unexpected validation result: %+v", result)
	}
	after := directorySnapshot(t, directory)
	if fmt.Sprint(before) != fmt.Sprint(after) {
		t.Fatalf("config validation changed filesystem: before=%v after=%v", before, after)
	}
	for _, forbidden := range []string{filepath.Join(directory, "totp.key"), filepath.Join(directory, "data"), filepath.Join(directory, "runtime")} {
		if _, err := os.Lstat(forbidden); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("config validation created %s: %v", forbidden, err)
		}
	}
}

func TestConfigValidateFailureUsesExitCodeAndDoesNotLeakSecrets(t *testing.T) {
	directory := t.TempDir()
	if err := os.Mkdir(filepath.Join(directory, "resources"), 0o755); err != nil {
		t.Fatal(err)
	}
	configPath := writeCLIConfig(t, directory, false)
	contents, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	const secret = "database-password-that-must-not-leak"
	invalid := strings.Replace(string(contents), "type: sqlite", "type: invalid", 1) + "\nmysql:\n  password: " + secret + "\n"
	if err := os.WriteFile(configPath, []byte(invalid), 0o600); err != nil {
		t.Fatal(err)
	}
	stdout, stderr, err := executeRoot("config", "validate", "--config", configPath, "--json")
	if err == nil || commandExitCode(err) != exitConfig {
		t.Fatalf("invalid config exit=%d err=%v stderr=%s", commandExitCode(err), err, stderr)
	}
	if bytes.Contains(stdout, []byte(secret)) || strings.Contains(stderr, secret) {
		t.Fatalf("configuration output leaked database password: stdout=%s stderr=%s", stdout, stderr)
	}
	result := configValidationOutput{}
	if decodeErr := json.Unmarshal(stdout, &result); decodeErr != nil || result.Valid || len(result.Errors) == 0 || result.Errors[0].Field != "gorm.type" {
		t.Fatalf("invalid config JSON=%+v decodeErr=%v", result, decodeErr)
	}
}

func TestConfigReferenceValidationRejectsUnsafeHostIdentity(t *testing.T) {
	directory := t.TempDir()
	valid := filepath.Join(directory, "machine-id")
	if err := os.WriteFile(valid, []byte("host-a\n"), 0o444); err != nil {
		t.Fatal(err)
	}
	configWithIdentity := func(path string) *config.Config {
		return &config.Config{ServerControl: config.ServerControl{
			HostIdentityFile: path,
			Pairing:          config.PairingBroker{Enabled: true},
		}}
	}
	if validationErrors, _ := validateConfigurationReferences(configWithIdentity(valid)); len(validationErrors) != 0 {
		t.Fatalf("valid host identity rejected: %+v", validationErrors)
	}

	symlink := filepath.Join(directory, "machine-id-link")
	if err := os.Symlink(valid, symlink); err != nil {
		t.Fatal(err)
	}
	empty := filepath.Join(directory, "empty-id")
	if err := os.WriteFile(empty, nil, 0o444); err != nil {
		t.Fatal(err)
	}
	large := filepath.Join(directory, "large-id")
	if err := os.WriteFile(large, bytes.Repeat([]byte{'x'}, 1025), 0o444); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{symlink, empty, large, filepath.Join(directory, "missing-id")} {
		validationErrors, _ := validateConfigurationReferences(configWithIdentity(path))
		if len(validationErrors) != 1 || validationErrors[0].Field != "server-control.host-identity-file" || validationErrors[0].Code != "HOST_IDENTITY_REFERENCE_INVALID" {
			t.Fatalf("unsafe host identity %s was not rejected precisely: %+v", path, validationErrors)
		}
	}
}

func TestDatabaseStatusStatesAreReadOnly(t *testing.T) {
	tests := []struct {
		name        string
		version     *uint
		state       databaseSchema.SchemaState
		migration   bool
		safe        bool
		exitCode    int
		missingFile bool
	}{
		{name: "missing is empty", state: databaseSchema.StateEmpty, migration: true, missingFile: true},
		{name: "current", version: schemaValue(DatabaseVersion), state: databaseSchema.StateCurrent, safe: true},
		{name: "upgrade required", version: schemaValue(DatabaseVersion - 1), state: databaseSchema.StateUpgradeRequired, migration: true},
		{name: "newer than binary", version: schemaValue(DatabaseVersion + 1), state: databaseSchema.StateNewerThanBinary, exitCode: exitSchema},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			directory := t.TempDir()
			withWorkingDirectory(t, directory)
			configPath := writeCLIConfig(t, directory, false)
			var beforeHash [32]byte
			var databasePath string
			if !test.missingFile {
				databasePath = filepath.Join(directory, "data", "rustdeskapi.db")
				if err := os.MkdirAll(filepath.Dir(databasePath), 0o700); err != nil {
					t.Fatal(err)
				}
				database, err := gorm.Open(sqlite.Open(databasePath), &gorm.Config{})
				if err != nil {
					t.Fatal(err)
				}
				if err := database.AutoMigrate(&model.Version{}); err != nil {
					t.Fatal(err)
				}
				if err := database.Create(&model.Version{Version: *test.version}).Error; err != nil {
					t.Fatal(err)
				}
				closeGormDatabase(t, database)
				beforeHash = fileDigest(t, databasePath)
			}
			stdout, stderr, err := executeRoot("database", "status", "--config", configPath, "--json")
			if commandExitCode(err) != test.exitCode {
				t.Fatalf("status exit=%d want=%d err=%v stderr=%s stdout=%s", commandExitCode(err), test.exitCode, err, stderr, stdout)
			}
			result := databaseStatusOutput{}
			if decodeErr := json.Unmarshal(stdout, &result); decodeErr != nil {
				t.Fatalf("decode status: %v: %s", decodeErr, stdout)
			}
			if result.State != test.state || result.MigrationRequired != test.migration || result.SafeToStart != test.safe || !result.ConnectionOK || result.DatabaseType != config.TypeSqlite {
				t.Fatalf("unexpected database status: %+v", result)
			}
			if test.missingFile {
				if _, err := os.Stat(filepath.Join(directory, "data")); !errors.Is(err, os.ErrNotExist) {
					t.Fatalf("status created SQLite state: %v", err)
				}
			} else if afterHash := fileDigest(t, databasePath); afterHash != beforeHash {
				t.Fatalf("database status modified SQLite database")
			}
		})
	}
}

func TestDatabaseStatusReportsInvalidUnversionedApplicationSchema(t *testing.T) {
	directory := t.TempDir()
	withWorkingDirectory(t, directory)
	configPath := writeCLIConfig(t, directory, false)
	databasePath := filepath.Join(directory, "data", "rustdeskapi.db")
	if err := os.MkdirAll(filepath.Dir(databasePath), 0o700); err != nil {
		t.Fatal(err)
	}
	database, err := gorm.Open(sqlite.Open(databasePath), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := database.AutoMigrate(&model.User{}); err != nil {
		t.Fatal(err)
	}
	closeGormDatabase(t, database)
	before := fileDigest(t, databasePath)
	stdout, stderr, err := executeRoot("database", "status", "--config", configPath, "--json")
	if err == nil || commandExitCode(err) != exitSchema || stderr != "" {
		t.Fatalf("invalid schema exit=%d err=%v stdout=%s stderr=%s", commandExitCode(err), err, stdout, stderr)
	}
	result := databaseStatusOutput{}
	if decodeErr := json.Unmarshal(stdout, &result); decodeErr != nil || !result.ConnectionOK || result.State != databaseSchema.StateInvalid || result.Error == nil || result.Error.Code != "DATABASE_SCHEMA_INVALID" || result.SafeToStart {
		t.Fatalf("invalid schema JSON=%+v decodeErr=%v", result, decodeErr)
	}
	if after := fileDigest(t, databasePath); after != before {
		t.Fatal("invalid database status modified the database")
	}
}

func TestDatabaseMigrateRejectsFutureSchemaWithoutDatabaseWrite(t *testing.T) {
	directory := t.TempDir()
	withWorkingDirectory(t, directory)
	configPath := writeCLIConfig(t, directory, false)
	databasePath := filepath.Join(directory, "data", "rustdeskapi.db")
	if err := os.MkdirAll(filepath.Dir(databasePath), 0o700); err != nil {
		t.Fatal(err)
	}
	database, err := gorm.Open(sqlite.Open(databasePath), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := database.AutoMigrate(&model.Version{}); err != nil {
		t.Fatal(err)
	}
	if err := database.Create(&model.Version{Version: DatabaseVersion + 1}).Error; err != nil {
		t.Fatal(err)
	}
	closeGormDatabase(t, database)
	before := fileDigest(t, databasePath)
	stdout, stderr, err := executeRoot("database", "migrate", "--config", configPath, "--json")
	if err == nil || commandExitCode(err) != exitSchema || stderr != "" {
		t.Fatalf("future migration exit=%d err=%v stdout=%s stderr=%s", commandExitCode(err), err, stdout, stderr)
	}
	result := databaseMigrationOutput{}
	if decodeErr := json.Unmarshal(stdout, &result); decodeErr != nil || result.State != databaseSchema.StateNewerThanBinary || result.FromSchema == nil || *result.FromSchema != DatabaseVersion+1 || result.Migrated || len(result.Steps) != 4 || result.Steps[2].Status != "refused" || result.Error == nil || result.Error.Code != "DATABASE_SCHEMA_INCOMPATIBLE" {
		t.Fatalf("future migration JSON=%+v decodeErr=%v", result, decodeErr)
	}
	if after := fileDigest(t, databasePath); after != before {
		t.Fatal("future migration refusal modified the database")
	}
}

func TestDatabaseMigrateInitializesAndIsIdempotent(t *testing.T) {
	directory := t.TempDir()
	withWorkingDirectory(t, directory)
	configPath := writeCLIConfig(t, directory, false)
	stdout, stderr, err := executeRoot("database", "migrate", "--config", configPath, "--json")
	if err != nil {
		t.Fatalf("initial migration failed: %v stderr=%s stdout=%s", err, stderr, stdout)
	}
	first := databaseMigrationOutput{}
	if err := json.Unmarshal(stdout, &first); err != nil {
		t.Fatal(err)
	}
	if !first.Migrated || first.State != databaseSchema.StateCurrent || first.InstalledSchema == nil || *first.InstalledSchema != DatabaseVersion || len(first.Steps) != 4 {
		t.Fatalf("unexpected initial migration: %+v", first)
	}
	stdout, stderr, err = executeRoot("database", "migrate", "--config", configPath, "--json")
	if err != nil {
		t.Fatalf("idempotent migration failed: %v stderr=%s", err, stderr)
	}
	second := databaseMigrationOutput{}
	if err := json.Unmarshal(stdout, &second); err != nil {
		t.Fatal(err)
	}
	if second.Migrated || second.State != databaseSchema.StateCurrent || second.Steps[2].Status != "not_required" {
		t.Fatalf("migration was not idempotent: %+v", second)
	}
	database, err := gorm.Open(sqlite.Open(filepath.Join(directory, "data", "rustdeskapi.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	defer closeGormDatabase(t, database)
	var versions int64
	if err := database.Model(&model.Version{}).Count(&versions).Error; err != nil || versions != 1 {
		t.Fatalf("version markers=%d err=%v", versions, err)
	}
	admin := &model.User{}
	if err := database.Where("username = ?", "admin").First(admin).Error; err != nil || admin.Role != model.UserRoleSuperAdmin {
		t.Fatalf("bootstrap administrator=%+v err=%v", admin, err)
	}
}

func TestSQLiteConcurrentMigrationHasExactlyOneMigrator(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "concurrent.db")
	connections := make([]*gorm.DB, 2)
	for index := range connections {
		database, err := gorm.Open(sqlite.Open(path), &gorm.Config{})
		if err != nil {
			t.Fatal(err)
		}
		connections[index] = database
		defer closeGormDatabase(t, database)
	}
	oldLogger := global.Logger
	logger := logrus.New()
	logger.SetOutput(io.Discard)
	global.Logger = logger
	t.Cleanup(func() { global.Logger = oldLogger })
	cfg := &config.Config{Gorm: config.Gorm{Type: config.TypeSqlite}}
	start := make(chan struct{})
	migrated := make([]bool, 2)
	errorsFound := make([]error, 2)
	var wait sync.WaitGroup
	for index := range connections {
		wait.Add(1)
		go func(index int) {
			defer wait.Done()
			<-start
			_, _, migrated[index], _, errorsFound[index] = migrateConfiguredDatabase(t.Context(), cfg, connections[index])
		}(index)
	}
	close(start)
	wait.Wait()
	for _, err := range errorsFound {
		if err != nil {
			t.Fatalf("concurrent migration failed: %v", errorsFound)
		}
	}
	count := 0
	for _, value := range migrated {
		if value {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("migrators=%d results=%v", count, migrated)
	}
	var versions int64
	if err := connections[0].Model(&model.Version{}).Where("version = ?", DatabaseVersion).Count(&versions).Error; err != nil || versions != 1 {
		t.Fatalf("current version markers=%d err=%v", versions, err)
	}
}

func TestMigrationFailureNeverRecordsTargetVersion(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "failed.db")
	database, err := gorm.Open(sqlite.Open(path), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	defer closeGormDatabase(t, database)
	oldLogger := global.Logger
	logger := logrus.New()
	logger.SetOutput(io.Discard)
	global.Logger = logger
	t.Cleanup(func() { global.Logger = oldLogger })
	if err := MigrateDatabase(database, DatabaseVersion); err != nil {
		t.Fatal(err)
	}
	if err := database.Where("1 = 1").Delete(&model.Version{}).Error; err != nil {
		t.Fatal(err)
	}
	if err := database.Create(&model.Version{Version: DatabaseVersion - 1}).Error; err != nil {
		t.Fatal(err)
	}
	if err := database.Exec(`CREATE TRIGGER reject_version_success BEFORE INSERT ON versions
		BEGIN SELECT RAISE(ABORT, 'injected version failure'); END`).Error; err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{Gorm: config.Gorm{Type: config.TypeSqlite}}
	_, _, _, _, err = migrateConfiguredDatabase(t.Context(), cfg, database)
	if err == nil {
		t.Fatal("injected migration failure unexpectedly succeeded")
	}
	latest := &model.Version{}
	if err := database.Order("id DESC").First(latest).Error; err != nil || latest.Version != DatabaseVersion-1 {
		t.Fatalf("failed migration marked success: version=%+v err=%v", latest, err)
	}
}

func TestSQLiteFileMigrationLockSerializesCallbacks(t *testing.T) {
	path := filepath.Join(t.TempDir(), "lock.db")
	databases := make([]*gorm.DB, 2)
	for index := range databases {
		db, err := gorm.Open(sqlite.Open(path), &gorm.Config{})
		if err != nil {
			t.Fatal(err)
		}
		databases[index] = db
		defer closeGormDatabase(t, db)
	}
	cfg := &config.Config{Gorm: config.Gorm{Type: config.TypeSqlite}}
	var inside, maximum atomic.Int32
	start := make(chan struct{})
	errorsFound := make([]error, 2)
	var wait sync.WaitGroup
	for index := range databases {
		wait.Add(1)
		go func(index int) {
			defer wait.Done()
			<-start
			errorsFound[index] = withMigrationLock(t.Context(), cfg, databases[index], func(*gorm.DB) error {
				current := inside.Add(1)
				for current > maximum.Load() && !maximum.CompareAndSwap(maximum.Load(), current) {
				}
				time.Sleep(50 * time.Millisecond)
				inside.Add(-1)
				return nil
			})
		}(index)
	}
	close(start)
	wait.Wait()
	if errorsFound[0] != nil || errorsFound[1] != nil || maximum.Load() != 1 {
		t.Fatalf("SQLite lock did not serialize: max=%d errors=%v", maximum.Load(), errorsFound)
	}
}

func TestSQLiteMigrationLockRejectsUnsafeLockFiles(t *testing.T) {
	directory := t.TempDir()
	target := filepath.Join(directory, "target")
	if err := os.WriteFile(target, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	symlink := filepath.Join(directory, "symlink.lock")
	if err := os.Symlink(target, symlink); err != nil {
		t.Fatal(err)
	}
	if err := withSQLiteFileLock(context.Background(), symlink, func() error { return nil }); err == nil {
		t.Fatal("SQLite migration lock followed a symbolic link")
	}
	unsafe := filepath.Join(directory, "unsafe.lock")
	if err := os.WriteFile(unsafe, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := withSQLiteFileLock(context.Background(), unsafe, func() error { return nil }); err == nil || !strings.Contains(err.Error(), "group or other") {
		t.Fatalf("unsafe SQLite lock permissions were accepted: %v", err)
	}
}

func TestMigrationLockNameIsStableBoundedAndDatabaseSpecific(t *testing.T) {
	first := &config.Config{
		Gorm:  config.Gorm{Type: config.TypeMysql},
		Mysql: config.Mysql{Dbname: strings.Repeat("a", 64)},
	}
	second := &config.Config{
		Gorm:  config.Gorm{Type: config.TypeMysql},
		Mysql: config.Mysql{Dbname: strings.Repeat("a", 63) + "b"},
	}
	name := migrationLockName(first)
	if name != migrationLockName(first) {
		t.Fatal("migration lock name is not stable")
	}
	if len(name) > 64 {
		t.Fatalf("migration lock name exceeds MySQL limit: %d", len(name))
	}
	if name == migrationLockName(second) {
		t.Fatal("different databases received the same migration lock name")
	}
	postgres := &config.Config{
		Gorm:       config.Gorm{Type: config.TypePostgresql},
		Postgresql: config.Postgresql{Dbname: first.Mysql.Dbname},
	}
	if name == migrationLockName(postgres) {
		t.Fatal("different database dialects received the same migration lock name")
	}
}

func TestMaintenanceCLIJSONRecoveryResetAndFailureCodes(t *testing.T) {
	directory := t.TempDir()
	withWorkingDirectory(t, directory)
	configPath := writeCLIConfig(t, directory, false)
	if stdout, stderr, err := executeRoot("database", "migrate", "--config", configPath, "--json"); err != nil {
		t.Fatalf("prepare maintenance database: %v stdout=%s stderr=%s", err, stdout, stderr)
	}
	databasePath := filepath.Join(directory, "data", "rustdeskapi.db")
	database, err := gorm.Open(sqlite.Open(databasePath), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	isAdmin := false
	user := &model.User{Username: "cli-recovery", Password: "legacy-hash-placeholder", Status: model.COMMON_STATUS_DISABLED, Role: model.UserRoleUser, IsAdmin: &isAdmin, AuthVersion: 4}
	if err := database.Create(user).Error; err != nil {
		t.Fatal(err)
	}
	for _, row := range []interface{}{
		&model.UserToken{UserId: user.Id},
		&model.AdminResourceScope{AdminUserId: user.Id, ScopeType: model.AdminScopeTypeGroup, ScopeId: 88},
		&model.UserTwoFactor{UserID: user.Id, SecretCiphertext: "cli-secret", PendingSecretCiphertext: "cli-pending", Enabled: true},
		&model.TwoFactorLoginChallenge{TokenHash: "cli-challenge", UserID: user.Id, Username: user.Username, ExpiresAt: time.Now().Add(time.Minute).Unix()},
	} {
		if err := database.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
	closeGormDatabase(t, database)

	passwordPath := filepath.Join(directory, "recovery-password")
	const replacementPassword = "cli-recovery-password-123"
	if err := os.WriteFile(passwordPath, []byte(replacementPassword+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	stdout, stderr, err := executeRoot("maintenance", "recover-admin",
		"--config", configPath,
		"--username", user.Username,
		"--confirm-username", user.Username,
		"--password-file", passwordPath,
		"--reset-2fa",
		"--json",
	)
	if err != nil || stderr != "" {
		t.Fatalf("recover-admin failed: %v stdout=%s stderr=%s", err, stdout, stderr)
	}
	recovery := maintenanceCommandOutput{}
	if err := json.Unmarshal(stdout, &recovery); err != nil {
		t.Fatal(err)
	}
	if !recovery.Success || recovery.Operation != "recover_admin" || recovery.RequestID == "" || recovery.UserID != user.Id || recovery.Username != user.Username || recovery.AuthVersion != 5 || !recovery.PasswordReset || !recovery.TwoFactorReset || !recovery.TwoFactorWasConfigured || recovery.ScopesCleared != 1 || recovery.SessionsRevoked != 1 || recovery.LoginChallengesCleared != 1 || recovery.Error != nil {
		t.Fatalf("unexpected recover-admin JSON: %+v", recovery)
	}
	if bytes.Contains(stdout, []byte(replacementPassword)) || bytes.Contains(stdout, []byte("cli-secret")) {
		t.Fatalf("recover-admin JSON leaked secret: %s", stdout)
	}

	database, err = gorm.Open(sqlite.Open(databasePath), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	refreshed := &model.User{}
	if err := database.First(refreshed, user.Id).Error; err != nil {
		t.Fatal(err)
	}
	validPassword, _, err := utils.VerifyPassword(refreshed.Password, replacementPassword)
	if err != nil || !validPassword || refreshed.AuthVersion != 5 || refreshed.Status != model.COMMON_STATUS_ENABLE || refreshed.Role != model.UserRoleSuperAdmin || refreshed.IsAdmin == nil || !*refreshed.IsAdmin {
		t.Fatalf("recovered CLI user=%+v passwordValid=%t err=%v", refreshed, validPassword, err)
	}
	closeGormDatabase(t, database)

	stdout, stderr, err = executeRoot("maintenance", "reset-2fa",
		"--config", configPath,
		"--user-id", fmt.Sprint(user.Id),
		"--confirm-username", user.Username,
		"--json",
	)
	if err != nil || stderr != "" {
		t.Fatalf("reset-2fa failed: %v stdout=%s stderr=%s", err, stdout, stderr)
	}
	reset := maintenanceCommandOutput{}
	if err := json.Unmarshal(stdout, &reset); err != nil {
		t.Fatal(err)
	}
	if !reset.Success || reset.Operation != "reset_2fa" || reset.AuthVersion != 6 || !reset.TwoFactorReset || reset.TwoFactorWasConfigured || reset.SessionsRevoked != 0 || reset.Error != nil {
		t.Fatalf("unexpected reset-2fa JSON: %+v", reset)
	}

	stdout, stderr, err = executeRoot("maintenance", "recover-admin",
		"--config", configPath,
		"--user-id", fmt.Sprint(user.Id),
		"--confirm-username", "wrong-name",
		"--json",
	)
	if err == nil || commandExitCode(err) != exitMaintenance || stderr != "" {
		t.Fatalf("confirmation mismatch exit=%d err=%v stdout=%s stderr=%s", commandExitCode(err), err, stdout, stderr)
	}
	failure := maintenanceCommandOutput{}
	if decodeErr := json.Unmarshal(stdout, &failure); decodeErr != nil || failure.Success || failure.Error == nil || failure.Error.Code != service.MaintenanceCodeConfirmationMismatch {
		t.Fatalf("unexpected maintenance failure JSON=%+v decodeErr=%v", failure, decodeErr)
	}
	database, err = gorm.Open(sqlite.Open(databasePath), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	defer closeGormDatabase(t, database)
	if err := database.First(refreshed, user.Id).Error; err != nil || refreshed.AuthVersion != 6 {
		t.Fatalf("confirmation mismatch modified account: %+v err=%v", refreshed, err)
	}
	var failureAudits int64
	if err := database.Model(&model.AdminAuditEvent{}).Where("action = ? AND result = ? AND error_code = ?", "maintenance.admin.recovered", "failure", service.MaintenanceCodeConfirmationMismatch).Count(&failureAudits).Error; err != nil || failureAudits != 1 {
		t.Fatalf("confirmation failure audits=%d err=%v", failureAudits, err)
	}
}

func TestMaintenanceCLIFailsClosedOnFutureSchema(t *testing.T) {
	directory := t.TempDir()
	withWorkingDirectory(t, directory)
	configPath := writeCLIConfig(t, directory, false)
	if stdout, stderr, err := executeRoot("database", "migrate", "--config", configPath, "--json"); err != nil {
		t.Fatalf("prepare future-schema fixture: %v stdout=%s stderr=%s", err, stdout, stderr)
	}
	databasePath := filepath.Join(directory, "data", "rustdeskapi.db")
	database, err := gorm.Open(sqlite.Open(databasePath), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	user := &model.User{}
	if err := database.Where("username = ?", "admin").First(user).Error; err != nil {
		t.Fatal(err)
	}
	if err := database.Create(&model.Version{Version: DatabaseVersion + 1}).Error; err != nil {
		t.Fatal(err)
	}
	var auditsBefore int64
	if err := database.Model(&model.AdminAuditEvent{}).Count(&auditsBefore).Error; err != nil {
		t.Fatal(err)
	}
	closeGormDatabase(t, database)

	stdout, stderr, err := executeRoot("maintenance", "reset-2fa",
		"--config", configPath,
		"--user-id", fmt.Sprint(user.Id),
		"--confirm-username", user.Username,
		"--json",
	)
	if err == nil || commandExitCode(err) != exitSchema || stderr != "" {
		t.Fatalf("future schema exit=%d err=%v stdout=%s stderr=%s", commandExitCode(err), err, stdout, stderr)
	}
	result := maintenanceCommandOutput{}
	if decodeErr := json.Unmarshal(stdout, &result); decodeErr != nil || result.Error == nil || result.Error.Code != service.MaintenanceCodeSchemaMismatch {
		t.Fatalf("future schema JSON=%+v decodeErr=%v", result, decodeErr)
	}
	database, err = gorm.Open(sqlite.Open(databasePath), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	defer closeGormDatabase(t, database)
	var auditsAfter int64
	if err := database.Model(&model.AdminAuditEvent{}).Count(&auditsAfter).Error; err != nil || auditsAfter != auditsBefore {
		t.Fatalf("future schema wrote audit: before=%d after=%d err=%v", auditsBefore, auditsAfter, err)
	}
}

func TestCLIJSONGoldenContracts(t *testing.T) {
	installed := uint(DatabaseVersion)
	cases := []struct {
		name  string
		value interface{}
	}{
		{name: "version", value: versionOutput{SchemaVersion: cliSchemaVersion, Details: buildinfo.Details{
			Component: buildinfo.Component, Version: "3.0.7", DatabaseSchema: DatabaseVersion,
			GitCommit: "0123456789abcdef", BuildTime: "2026-08-31T00:00:00Z", GoVersion: "go1.26.6",
		}}},
		{name: "config-validation", value: configValidationOutput{
			SchemaVersion: cliSchemaVersion, Valid: true, ConfigPath: "/etc/kessoku/config.yaml", DatabaseType: config.TypeSqlite,
			Errors: []configValidationIssue{}, Warnings: []configValidationIssue{{Field: "two-factor.key-file", Code: "TWO_FACTOR_KEY_PENDING", Message: "key will be created by full service startup"}},
		}},
		{name: "database-status", value: databaseStatusOutput{
			SchemaVersion: cliSchemaVersion, DatabaseType: config.TypePostgresql, ConnectionOK: true,
			InstalledSchema: &installed, TargetSchema: DatabaseVersion, State: databaseSchema.StateCurrent, SafeToStart: true,
		}},
		{name: "database-migration", value: databaseMigrationOutput{
			SchemaVersion: cliSchemaVersion, DatabaseType: config.TypeMysql, FromSchema: &installed, InstalledSchema: &installed,
			TargetSchema: DatabaseVersion, State: databaseSchema.StateCurrent, Migrated: false,
			Steps: []migrationStep{{Name: "acquire_lock", Status: "complete"}, {Name: "inspect_schema", Status: "complete"}, {Name: "apply_migration", Status: "not_required"}, {Name: "verify_schema", Status: "complete"}},
		}},
		{name: "maintenance-success", value: maintenanceCommandOutput{
			SchemaVersion: cliSchemaVersion, Operation: "recover_admin", Success: true,
			RequestID: "0191f6a0-0000-7000-8000-000000000401", UserID: 7, Username: "recovered-user", AuthVersion: 9,
			PasswordReset: true, TwoFactorReset: true, TwoFactorWasConfigured: true,
			LoginChallengesCleared: 1, ScopesCleared: 2, SessionsRevoked: 3,
		}},
		{name: "maintenance-failure", value: maintenanceCommandOutput{
			SchemaVersion: cliSchemaVersion, Operation: "reset_2fa", RequestID: "0191f6a0-0000-7000-8000-000000000402",
			Error: &cliErrorDetail{Code: service.MaintenanceCodeConfirmationMismatch, Message: "confirm-username does not exactly match the stored username"},
		}},
		{name: "command-error", value: cliErrorOutput{
			SchemaVersion: cliSchemaVersion, OK: false,
			Error: cliErrorDetail{Code: "CONFIG_INVALID", Message: "configuration is invalid", Field: "config"},
		}},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			actual := &bytes.Buffer{}
			if err := writeJSON(actual, test.value); err != nil {
				t.Fatal(err)
			}
			expected, err := os.ReadFile(filepath.Join("testdata", "cli-json", test.name+".golden.json"))
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(actual.Bytes(), expected) {
				t.Fatalf("JSON contract changed\nactual:   %s\nexpected: %s", actual.Bytes(), expected)
			}
		})
	}
}

func TestCLIUsageErrorsUseStableExitCodeAndJSON(t *testing.T) {
	stdout, stderr, err := executeRoot("version", "unexpected", "--json")
	if err == nil || commandExitCode(err) != exitUsage || stderr != "" {
		t.Fatalf("version usage exit=%d err=%v stdout=%s stderr=%s", commandExitCode(err), err, stdout, stderr)
	}
	result := cliErrorOutput{}
	if decodeErr := json.Unmarshal(stdout, &result); decodeErr != nil || result.OK || result.Error.Code != "USAGE_INVALID" || result.Error.Field != "arguments" {
		t.Fatalf("version usage JSON=%+v decodeErr=%v", result, decodeErr)
	}
	_, _, err = executeRoot("unknown-command")
	if err == nil || commandExitCode(err) != exitUsage {
		t.Fatalf("unknown command exit=%d err=%v", commandExitCode(err), err)
	}
}

func TestLegacyPasswordResetCommandsRemainCompatible(t *testing.T) {
	directory := t.TempDir()
	withWorkingDirectory(t, directory)
	configPath := writeCLIConfig(t, directory, false)
	if stdout, stderr, err := executeRoot("database", "migrate", "--config", configPath, "--json"); err != nil {
		t.Fatalf("prepare legacy-command database: %v stdout=%s stderr=%s", err, stdout, stderr)
	}
	databasePath := filepath.Join(directory, "data", "rustdeskapi.db")
	database, err := gorm.Open(sqlite.Open(databasePath), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	admin := &model.User{}
	if err := database.Where("username = ?", "admin").First(admin).Error; err != nil {
		t.Fatal(err)
	}
	isAdmin := false
	user := &model.User{Username: "legacy-reset-user", Password: "old-placeholder", Status: model.COMMON_STATUS_ENABLE, Role: model.UserRoleUser, IsAdmin: &isAdmin, AuthVersion: 3}
	if err := database.Create(user).Error; err != nil {
		t.Fatal(err)
	}
	if err := database.Create(&[]model.UserToken{{UserId: admin.Id}, {UserId: user.Id}}).Error; err != nil {
		t.Fatal(err)
	}
	adminVersion, userVersion := admin.AuthVersion, user.AuthVersion
	closeGormDatabase(t, database)

	adminPasswordPath := filepath.Join(directory, "admin-password")
	userPasswordPath := filepath.Join(directory, "user-password")
	const adminPassword = "legacy-admin-password-123"
	const userPassword = "legacy-user-password-456"
	if err := os.WriteFile(adminPasswordPath, []byte(adminPassword), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(userPasswordPath, []byte(userPassword), 0o600); err != nil {
		t.Fatal(err)
	}
	stdout, stderr, err := executeRoot("reset-admin-pwd", "--config", configPath, "--password-file", adminPasswordPath)
	if err != nil || !bytes.Contains(stdout, []byte("reset password success!")) || strings.Contains(stderr, adminPassword) {
		t.Fatalf("reset-admin-pwd compatibility failure: err=%v stdout=%s stderr=%s", err, stdout, stderr)
	}
	stdout, stderr, err = executeRoot("reset-pwd", "--config", configPath, "--user-id", fmt.Sprint(user.Id), "--password-file", userPasswordPath)
	if err != nil || !bytes.Contains(stdout, []byte("reset password success!")) || strings.Contains(stderr, userPassword) {
		t.Fatalf("reset-pwd compatibility failure: err=%v stdout=%s stderr=%s", err, stdout, stderr)
	}

	database, err = gorm.Open(sqlite.Open(databasePath), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	defer closeGormDatabase(t, database)
	for _, check := range []struct {
		id       uint
		password string
		version  uint64
	}{
		{id: admin.Id, password: adminPassword, version: adminVersion + 1},
		{id: user.Id, password: userPassword, version: userVersion + 1},
	} {
		refreshed := &model.User{}
		if err := database.First(refreshed, check.id).Error; err != nil {
			t.Fatal(err)
		}
		valid, _, verifyErr := utils.VerifyPassword(refreshed.Password, check.password)
		if verifyErr != nil || !valid || refreshed.AuthVersion != check.version {
			t.Fatalf("legacy password result user=%+v valid=%t err=%v", refreshed, valid, verifyErr)
		}
		var active int64
		if err := database.Model(&model.UserToken{}).Where("user_id = ? AND revoked_at IS NULL", check.id).Count(&active).Error; err != nil || active != 0 {
			t.Fatalf("legacy reset active tokens user=%d count=%d err=%v", check.id, active, err)
		}
	}
	var successfulAudits int64
	if err := database.Model(&model.AdminAuditEvent{}).Where("action = ? AND result = ?", "auth.password.changed", "success").Count(&successfulAudits).Error; err != nil || successfulAudits != 2 {
		t.Fatalf("legacy reset success audits=%d err=%v", successfulAudits, err)
	}
}

func TestPasswordFromFileRejectsReplacementRace(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "password")
	if err := os.WriteFile(path, []byte("original-password-123"), 0o600); err != nil {
		t.Fatal(err)
	}
	openOriginal := openPasswordFileNoFollow
	openPasswordFileNoFollow = func(candidate string) (*os.File, error) {
		if err := os.Rename(candidate, candidate+".original"); err != nil {
			return nil, err
		}
		if err := os.WriteFile(candidate, []byte("attacker-password-456"), 0o600); err != nil {
			return nil, err
		}
		return openOriginal(candidate)
	}
	t.Cleanup(func() { openPasswordFileNoFollow = openOriginal })
	if _, err := passwordFromFile(path); err == nil || !strings.Contains(err.Error(), "changed while opening") {
		t.Fatalf("replacement race was not rejected: %v", err)
	}
}

func executeRoot(arguments ...string) ([]byte, string, error) {
	stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
	command := newRootCommand()
	command.SetOut(stdout)
	command.SetErr(stderr)
	command.SetArgs(arguments)
	err := command.Execute()
	return stdout.Bytes(), stderr.String(), err
}

func writeCLIConfig(t *testing.T, directory string, enableTwoFactor bool) string {
	t.Helper()
	keyFile := filepath.Join(directory, "totp.key")
	contents := fmt.Sprintf(`lang: en
app:
  web-client: 0
gin:
  api-addr: "127.0.0.1:21114"
  mode: test
  resources-path: %q
admin:
  id-server-port: 21116
  relay-server-port: 21117
gorm:
  type: sqlite
  max-idle-conns: 1
  max-open-conns: 2
rustdesk:
  id-server: "rustdesk.example.test:21116"
  relay-server: "rustdesk.example.test:21117"
  api-server: "https://api.example.test"
  key: "public-rustdesk-key"
logger:
  path: ""
  level: info
two-factor:
  enabled: %t
  issuer: "Kessoku Test"
  key-file: %q
  challenge-ttl: 5m
auth:
  enabled: false
server-control:
  read-only: true
  instances: []
`, filepath.Join(directory, "resources"), enableTwoFactor, keyFile)
	path := filepath.Join(directory, "config.yaml")
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func withWorkingDirectory(t *testing.T, directory string) {
	t.Helper()
	previous, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(directory); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(previous); err != nil {
			t.Errorf("restore working directory: %v", err)
		}
	})
}

func directorySnapshot(t *testing.T, root string) []string {
	t.Helper()
	result := []string{}
	if err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		result = append(result, fmt.Sprintf("%s:%s:%o:%d", relative, entry.Type(), info.Mode().Perm(), info.Size()))
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return result
}

func fileDigest(t *testing.T, path string) [32]byte {
	t.Helper()
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return sha256.Sum256(contents)
}

func closeGormDatabase(t *testing.T, database *gorm.DB) {
	t.Helper()
	sqlDatabase, err := database.DB()
	if err != nil {
		t.Fatal(err)
	}
	if err := sqlDatabase.Close(); err != nil {
		t.Fatal(err)
	}
}

func schemaValue(value uint) *uint { return &value }
