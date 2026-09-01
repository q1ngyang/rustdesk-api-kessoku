package database

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/q1ngyang/rustdesk-api-kessoku/v3/internal/buildinfo"
	"github.com/q1ngyang/rustdesk-api-kessoku/v3/model"
	"gorm.io/gorm"
)

type SchemaState string

const (
	StateEmpty           SchemaState = "empty"
	StateCurrent         SchemaState = "current"
	StateUpgradeRequired SchemaState = "upgrade_required"
	StateNewerThanBinary SchemaState = "newer_than_binary"
	StateInvalid         SchemaState = "invalid"
)

var (
	ErrSchemaUnavailable = errors.New("database schema is unavailable")
	ErrSchemaMismatch    = errors.New("database schema does not match this binary")
)

type SchemaStatus struct {
	InstalledSchema   *uint       `json:"installed_schema"`
	TargetSchema      uint        `json:"target_schema"`
	State             SchemaState `json:"state"`
	MigrationRequired bool        `json:"migration_required"`
	SafeToStart       bool        `json:"safe_to_start"`
}

// InspectSchema performs metadata reads only. It deliberately does not call
// AutoMigrate, create the version table, or repair a partially initialized
// database.
func InspectSchema(db *gorm.DB) (SchemaStatus, error) {
	status := SchemaStatus{TargetSchema: buildinfo.DatabaseSchema, State: StateInvalid}
	if db == nil {
		return status, ErrSchemaUnavailable
	}
	tables, err := db.Migrator().GetTables()
	if err != nil {
		return status, fmt.Errorf("%w: list tables: %v", ErrSchemaUnavailable, err)
	}
	hasVersionTable := false
	hasApplicationTable := false
	for _, table := range tables {
		name := strings.TrimSpace(table)
		if name == "versions" {
			hasVersionTable = true
			continue
		}
		if name != "" && name != "sqlite_sequence" {
			hasApplicationTable = true
		}
	}
	if !hasVersionTable {
		if hasApplicationTable {
			return status, errors.New("database contains application tables but no schema version table")
		}
		status.State = StateEmpty
		status.MigrationRequired = true
		return status, nil
	}

	var maximum sql.NullInt64
	if err := db.Model(&model.Version{}).Select("MAX(version)").Scan(&maximum).Error; err != nil {
		return status, fmt.Errorf("%w: read installed schema: %v", ErrSchemaUnavailable, err)
	}
	// AutoMigrate creates versions before the final success marker is written.
	// An empty version table is therefore retryable initialization, not proof of
	// a successful migration.
	if !maximum.Valid {
		status.State = StateEmpty
		status.MigrationRequired = true
		return status, nil
	}
	if maximum.Int64 <= 0 || uint64(maximum.Int64) > uint64(^uint(0)) {
		return status, errors.New("database schema version is invalid")
	}
	installed := uint(maximum.Int64)
	status.InstalledSchema = &installed
	switch {
	case installed == status.TargetSchema:
		status.State = StateCurrent
		status.SafeToStart = true
	case installed < status.TargetSchema:
		status.State = StateUpgradeRequired
		status.MigrationRequired = true
	case installed > status.TargetSchema:
		status.State = StateNewerThanBinary
	}
	return status, nil
}

func RequireCurrentSchema(db *gorm.DB) (SchemaStatus, error) {
	status, err := InspectSchema(db)
	if err != nil {
		return status, err
	}
	if status.State != StateCurrent {
		return status, fmt.Errorf("%w: installed=%s target=%d state=%s", ErrSchemaMismatch, schemaLabel(status.InstalledSchema), status.TargetSchema, status.State)
	}
	return status, nil
}

func schemaLabel(value *uint) string {
	if value == nil {
		return "none"
	}
	return fmt.Sprintf("%d", *value)
}
