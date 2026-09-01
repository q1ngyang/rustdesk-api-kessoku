package database

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/q1ngyang/rustdesk-api-kessoku/v3/internal/buildinfo"
	"github.com/q1ngyang/rustdesk-api-kessoku/v3/model"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestInspectSchemaStates(t *testing.T) {
	tests := []struct {
		name      string
		setup     func(*testing.T, *gorm.DB)
		state     SchemaState
		installed *uint
		migration bool
		safe      bool
		wantError bool
	}{
		{name: "empty", state: StateEmpty, migration: true},
		{name: "partial initialization", setup: func(t *testing.T, db *gorm.DB) {
			if err := db.AutoMigrate(&model.Version{}, &model.User{}); err != nil {
				t.Fatal(err)
			}
		}, state: StateEmpty, migration: true},
		{name: "current", setup: schemaFixture(buildinfo.DatabaseSchema), state: StateCurrent, installed: uintPointer(buildinfo.DatabaseSchema), safe: true},
		{name: "upgrade", setup: schemaFixture(buildinfo.DatabaseSchema - 1), state: StateUpgradeRequired, installed: uintPointer(buildinfo.DatabaseSchema - 1), migration: true},
		{name: "future", setup: schemaFixture(buildinfo.DatabaseSchema + 1), state: StateNewerThanBinary, installed: uintPointer(buildinfo.DatabaseSchema + 1)},
		{name: "invalid zero", setup: schemaFixture(0), state: StateInvalid, wantError: true},
		{name: "application table without version", setup: func(t *testing.T, db *gorm.DB) {
			if err := db.AutoMigrate(&model.User{}); err != nil {
				t.Fatal(err)
			}
		}, state: StateInvalid, wantError: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "schema.db")), &gorm.Config{})
			if err != nil {
				t.Fatal(err)
			}
			if test.setup != nil {
				test.setup(t, db)
			}
			status, err := InspectSchema(db)
			if (err != nil) != test.wantError {
				t.Fatalf("InspectSchema error=%v wantError=%t", err, test.wantError)
			}
			if status.State != test.state || status.MigrationRequired != test.migration || status.SafeToStart != test.safe || !sameSchema(status.InstalledSchema, test.installed) || status.TargetSchema != buildinfo.DatabaseSchema {
				t.Fatalf("unexpected schema status: %+v", status)
			}
		})
	}
}

func TestRequireCurrentSchemaFailsClosed(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "future.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	schemaFixture(buildinfo.DatabaseSchema+1)(t, db)
	status, err := RequireCurrentSchema(db)
	if !errors.Is(err, ErrSchemaMismatch) || status.State != StateNewerThanBinary {
		t.Fatalf("future schema was not rejected: status=%+v err=%v", status, err)
	}
}

func schemaFixture(version uint) func(*testing.T, *gorm.DB) {
	return func(t *testing.T, db *gorm.DB) {
		t.Helper()
		if err := db.AutoMigrate(&model.Version{}); err != nil {
			t.Fatal(err)
		}
		if err := db.Create(&model.Version{Version: version}).Error; err != nil {
			t.Fatal(err)
		}
	}
}

func uintPointer(value uint) *uint { return &value }

func sameSchema(left, right *uint) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return *left == *right
}
