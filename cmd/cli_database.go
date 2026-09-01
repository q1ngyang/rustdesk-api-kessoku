package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"database/sql"
	"errors"
	"fmt"
	"hash/fnv"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	mysqlDriver "github.com/go-sql-driver/mysql"
	"github.com/q1ngyang/rustdesk-api-kessoku/v3/config"
	"github.com/q1ngyang/rustdesk-api-kessoku/v3/internal/buildinfo"
	databaseSchema "github.com/q1ngyang/rustdesk-api-kessoku/v3/internal/database"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	gormMysql "gorm.io/driver/mysql"
	gormPostgres "gorm.io/driver/postgres"
	gormSqlite "gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormLogger "gorm.io/gorm/logger"
)

const (
	mysqlTLSProfile    = "kessoku-verified-ca"
	sqliteDatabasePath = "./data/rustdeskapi.db"
)

var (
	errDatabaseDoesNotExist = errors.New("configured database does not exist")
	sqliteInMemoryLock      sync.Mutex
)

type databaseStatusOutput struct {
	SchemaVersion     int                        `json:"schema_version"`
	DatabaseType      string                     `json:"database_type"`
	ConnectionOK      bool                       `json:"connection_ok"`
	InstalledSchema   *uint                      `json:"installed_schema"`
	TargetSchema      uint                       `json:"target_schema"`
	State             databaseSchema.SchemaState `json:"state"`
	MigrationRequired bool                       `json:"migration_required"`
	SafeToStart       bool                       `json:"safe_to_start"`
	Error             *cliErrorDetail            `json:"error,omitempty"`
}

type migrationStep struct {
	Name   string `json:"name"`
	Status string `json:"status"`
}

type databaseMigrationOutput struct {
	SchemaVersion   int                        `json:"schema_version"`
	DatabaseType    string                     `json:"database_type"`
	FromSchema      *uint                      `json:"from_schema"`
	InstalledSchema *uint                      `json:"installed_schema"`
	TargetSchema    uint                       `json:"target_schema"`
	State           databaseSchema.SchemaState `json:"state"`
	Migrated        bool                       `json:"migrated"`
	Steps           []migrationStep            `json:"steps"`
	Error           *cliErrorDetail            `json:"error,omitempty"`
}

type databaseOpenMode int

const (
	databaseReadOnly databaseOpenMode = iota
	databaseReadWrite
	databaseCreate
)

func newDatabaseCommand(configPath *string) *cobra.Command {
	command := &cobra.Command{Use: "database", Short: "Inspect or explicitly migrate the Kessoku database", Args: noCommandArgs}
	command.AddCommand(newDatabaseStatusCommand(configPath), newDatabaseMigrateCommand(configPath))
	return command
}

func newDatabaseStatusCommand(configPath *string) *cobra.Command {
	jsonOutput := false
	command := &cobra.Command{
		Use:   "status",
		Short: "Read database connectivity and schema state without migration",
		Args:  noCommandArgsJSON(&jsonOutput),
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, path, err := loadCommandConfig(configPath)
			if err != nil {
				return failCommand(cmd, jsonOutput, exitConfig, "CONFIG_INVALID", "configuration is invalid", "config", err)
			}
			_ = path
			result := databaseStatusOutput{
				SchemaVersion: cliSchemaVersion,
				DatabaseType:  cfg.DatabaseType(),
				TargetSchema:  buildinfo.DatabaseSchema,
				State:         databaseSchema.StateInvalid,
			}
			db, closeDatabase, err := openConfiguredDatabase(cmd.Context(), cfg, databaseReadOnly, commandLogger(cmd, jsonOutput))
			if errors.Is(err, errDatabaseDoesNotExist) {
				result.ConnectionOK = true
				result.State = databaseSchema.StateEmpty
				result.MigrationRequired = true
				if jsonOutput {
					return writeJSON(cmd.OutOrStdout(), result)
				}
				printDatabaseStatus(cmd, result)
				return nil
			}
			if err != nil {
				result.Error = &cliErrorDetail{Code: "DATABASE_CONNECTION_FAILED", Message: "database connection failed"}
				if jsonOutput {
					_ = writeJSON(cmd.OutOrStdout(), result)
				} else {
					_, _ = fmt.Fprintln(cmd.ErrOrStderr(), "database connection failed")
				}
				return &cliExitError{code: exitDatabase, reported: true, err: err}
			}
			defer closeDatabase()
			result.ConnectionOK = true
			status, err := inspectSchemaReadOnly(db)
			if err != nil {
				result.Error = &cliErrorDetail{Code: "DATABASE_SCHEMA_INVALID", Message: "database schema metadata is invalid"}
				if jsonOutput {
					_ = writeJSON(cmd.OutOrStdout(), result)
				} else {
					_, _ = fmt.Fprintln(cmd.ErrOrStderr(), result.Error.Message)
				}
				return &cliExitError{code: exitSchema, reported: true, err: err}
			}
			applySchemaStatus(&result, status)
			if jsonOutput {
				if err := writeJSON(cmd.OutOrStdout(), result); err != nil {
					return err
				}
			} else {
				printDatabaseStatus(cmd, result)
			}
			if status.State == databaseSchema.StateNewerThanBinary || status.State == databaseSchema.StateInvalid {
				return &cliExitError{code: exitSchema, reported: true, err: databaseSchema.ErrSchemaMismatch}
			}
			return nil
		},
	}
	command.Flags().BoolVar(&jsonOutput, "json", false, "emit stable JSON output")
	return command
}

func newDatabaseMigrateCommand(configPath *string) *cobra.Command {
	jsonOutput := false
	command := &cobra.Command{
		Use:   "migrate",
		Short: "Acquire the database migration lock and migrate to the binary schema",
		Args:  noCommandArgsJSON(&jsonOutput),
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, _, err := loadCommandConfig(configPath)
			if err != nil {
				return failCommand(cmd, jsonOutput, exitConfig, "CONFIG_INVALID", "configuration is invalid", "config", err)
			}
			result := databaseMigrationOutput{
				SchemaVersion: cliSchemaVersion,
				DatabaseType:  cfg.DatabaseType(),
				TargetSchema:  buildinfo.DatabaseSchema,
				State:         databaseSchema.StateInvalid,
				Steps:         []migrationStep{},
			}
			db, closeDatabase, err := openConfiguredDatabase(cmd.Context(), cfg, databaseCreate, commandLogger(cmd, jsonOutput))
			if err != nil {
				result.Error = &cliErrorDetail{Code: "DATABASE_CONNECTION_FAILED", Message: "database connection failed"}
				return writeMigrationFailure(cmd, jsonOutput, result, exitDatabase, err)
			}
			defer closeDatabase()
			before, after, migrated, steps, err := migrateConfiguredDatabase(cmd.Context(), cfg, db)
			result.FromSchema = before.InstalledSchema
			result.InstalledSchema = after.InstalledSchema
			result.State = after.State
			result.Migrated = migrated
			result.Steps = steps
			if err != nil {
				code, exitCode, message := "DATABASE_MIGRATION_FAILED", exitDatabase, "database migration failed"
				if after.State == databaseSchema.StateNewerThanBinary || errors.Is(err, databaseSchema.ErrSchemaMismatch) {
					code, exitCode, message = "DATABASE_SCHEMA_INCOMPATIBLE", exitSchema, "database schema is newer than this binary or otherwise incompatible"
				}
				result.Error = &cliErrorDetail{Code: code, Message: message}
				return writeMigrationFailure(cmd, jsonOutput, result, exitCode, err)
			}
			if jsonOutput {
				return writeJSON(cmd.OutOrStdout(), result)
			}
			_, err = fmt.Fprintf(cmd.OutOrStdout(), "database migration complete: %s -> %d (migrated: %t)\n", schemaPointerLabel(result.FromSchema), buildinfo.DatabaseSchema, result.Migrated)
			return err
		},
	}
	command.Flags().BoolVar(&jsonOutput, "json", false, "emit stable JSON output")
	return command
}

func loadCommandConfig(configPath *string) (*config.Config, string, error) {
	path := config.DefaultConfig
	if configPath != nil && *configPath != "" {
		path = *configPath
	}
	cfg := &config.Config{}
	if _, err := config.Load(cfg, path); err != nil {
		return nil, path, err
	}
	return cfg, path, nil
}

func commandLogger(cmd *cobra.Command, jsonOutput bool) *logrus.Logger {
	logger := logrus.New()
	logger.SetLevel(logrus.InfoLevel)
	logger.SetFormatter(&logrus.TextFormatter{DisableColors: true, DisableTimestamp: true})
	if jsonOutput {
		logger.SetOutput(os.Stderr)
		logger.SetLevel(logrus.PanicLevel)
	} else {
		logger.SetOutput(cmd.ErrOrStderr())
	}
	return logger
}

func openConfiguredDatabase(ctx context.Context, cfg *config.Config, mode databaseOpenMode, logger *logrus.Logger) (*gorm.DB, func(), error) {
	if cfg == nil {
		return nil, func() {}, errors.New("configuration is unavailable")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	var dialector gorm.Dialector
	switch cfg.DatabaseType() {
	case config.TypeSqlite:
		absolute, err := filepath.Abs(sqliteDatabasePath)
		if err != nil {
			return nil, func() {}, err
		}
		if mode == databaseReadOnly || mode == databaseReadWrite {
			if _, err := os.Stat(absolute); errors.Is(err, os.ErrNotExist) {
				return nil, func() {}, errDatabaseDoesNotExist
			} else if err != nil {
				return nil, func() {}, err
			}
		} else if err := os.MkdirAll(filepath.Dir(absolute), 0o700); err != nil {
			return nil, func() {}, err
		}
		query := url.Values{"_busy_timeout": {"5000"}}
		switch mode {
		case databaseReadOnly:
			query.Set("mode", "ro")
			query.Set("_query_only", "1")
		case databaseReadWrite:
			query.Set("mode", "rw")
		default:
			query.Set("mode", "rwc")
		}
		dsn := (&url.URL{Scheme: "file", Path: absolute, RawQuery: query.Encode()}).String()
		dialector = gormSqlite.Open(dsn)
	case config.TypeMysql:
		if err := configureMySQLTLSForConfig(cfg); err != nil {
			return nil, func() {}, err
		}
		if mode == databaseCreate {
			if err := ensureMySQLDatabase(ctx, cfg, logger); err != nil {
				return nil, func() {}, err
			}
		}
		dialector = gormMysql.Open(mysqlDSNForConfig(cfg, cfg.Mysql.Dbname))
	case config.TypePostgresql:
		dialector = gormPostgres.Open(postgresqlDSNForConfig(cfg))
	default:
		return nil, func() {}, fmt.Errorf("unsupported database type %q", cfg.Gorm.Type)
	}
	database, err := gorm.Open(dialector, &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
		Logger:                                   configuredGormLogger(logger),
	})
	if err != nil {
		if cfg.DatabaseType() == config.TypeMysql && mode == databaseReadOnly && mysqlDatabaseMissing(err) {
			if serverErr := pingMySQLServer(ctx, cfg, logger); serverErr == nil {
				return nil, func() {}, errDatabaseDoesNotExist
			}
		}
		return nil, func() {}, err
	}
	sqlDatabase, err := database.DB()
	if err != nil {
		return nil, func() {}, err
	}
	// Preserve the existing configuration contract: zero is meaningful to
	// database/sql (no idle connections / no open-connection limit).
	sqlDatabase.SetMaxIdleConns(cfg.Gorm.MaxIdleConns)
	sqlDatabase.SetMaxOpenConns(cfg.Gorm.MaxOpenConns)
	pingContext, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := sqlDatabase.PingContext(pingContext); err != nil {
		_ = sqlDatabase.Close()
		return nil, func() {}, err
	}
	return database.WithContext(ctx), func() { _ = sqlDatabase.Close() }, nil
}

func configuredGormLogger(logger *logrus.Logger) gormLogger.Interface {
	if logger == nil {
		return gormLogger.Default.LogMode(gormLogger.Silent)
	}
	return gormLogger.New(logger, gormLogger.Config{
		SlowThreshold:             time.Second,
		LogLevel:                  gormLogger.Warn,
		IgnoreRecordNotFoundError: true,
		ParameterizedQueries:      true,
		Colorful:                  false,
	})
}

func mysqlDSNForConfig(cfg *config.Config, databaseName string) string {
	settings := mysqlDriver.NewConfig()
	settings.User = cfg.Mysql.Username
	settings.Passwd = cfg.Mysql.Password
	settings.Net = "tcp"
	settings.Addr = cfg.Mysql.Addr
	settings.DBName = databaseName
	settings.ParseTime = true
	settings.Loc = time.Local
	settings.TLSConfig = "true"
	if cfg.Mysql.CaFile != "" {
		settings.TLSConfig = mysqlTLSProfile
	}
	settings.Params = map[string]string{"charset": "utf8mb4"}
	return settings.FormatDSN()
}

func configureMySQLTLSForConfig(cfg *config.Config) error {
	if cfg.Mysql.CaFile == "" {
		return nil
	}
	caPEM, err := os.ReadFile(cfg.Mysql.CaFile)
	if err != nil {
		return fmt.Errorf("read MySQL CA file: %w", err)
	}
	roots, err := x509.SystemCertPool()
	if err != nil || roots == nil {
		roots = x509.NewCertPool()
	}
	if !roots.AppendCertsFromPEM(caPEM) {
		return errors.New("MySQL CA file contains no valid certificate")
	}
	host, _, err := net.SplitHostPort(cfg.Mysql.Addr)
	if err != nil {
		return fmt.Errorf("parse MySQL address: %w", err)
	}
	mysqlDriver.DeregisterTLSConfig(mysqlTLSProfile)
	return mysqlDriver.RegisterTLSConfig(mysqlTLSProfile, &tls.Config{
		MinVersion: tls.VersionTLS12,
		RootCAs:    roots,
		ServerName: host,
	})
}

func postgresqlDSNForConfig(cfg *config.Config) string {
	settings := cfg.Postgresql
	host := settings.Host
	if settings.Port != "" {
		host = net.JoinHostPort(settings.Host, settings.Port)
	}
	dsn := &url.URL{Scheme: "postgresql", Host: host, Path: "/" + settings.Dbname}
	if settings.User != "" {
		if settings.Password == "" {
			dsn.User = url.User(settings.User)
		} else {
			dsn.User = url.UserPassword(settings.User, settings.Password)
		}
	}
	query := dsn.Query()
	query.Set("sslmode", "verify-full")
	if settings.Sslrootcert != "" {
		query.Set("sslrootcert", settings.Sslrootcert)
	}
	if settings.TimeZone != "" {
		query.Set("TimeZone", settings.TimeZone)
	}
	dsn.RawQuery = query.Encode()
	return dsn.String()
}

func ensureMySQLDatabase(ctx context.Context, cfg *config.Config, logger *logrus.Logger) error {
	server, closeServer, err := openMySQLServer(ctx, cfg, logger)
	if err != nil {
		return err
	}
	defer closeServer()
	name := strings.ReplaceAll(cfg.Mysql.Dbname, "`", "``")
	return server.Exec("CREATE DATABASE IF NOT EXISTS `" + name + "` DEFAULT CHARSET utf8mb4").Error
}

func pingMySQLServer(ctx context.Context, cfg *config.Config, logger *logrus.Logger) error {
	_, closeServer, err := openMySQLServer(ctx, cfg, logger)
	if err != nil {
		return err
	}
	closeServer()
	return nil
}

func openMySQLServer(ctx context.Context, cfg *config.Config, logger *logrus.Logger) (*gorm.DB, func(), error) {
	db, err := gorm.Open(gormMysql.Open(mysqlDSNForConfig(cfg, "")), &gorm.Config{Logger: configuredGormLogger(logger)})
	if err != nil {
		return nil, func() {}, err
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, func() {}, err
	}
	pingContext, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := sqlDB.PingContext(pingContext); err != nil {
		_ = sqlDB.Close()
		return nil, func() {}, err
	}
	return db.WithContext(ctx), func() { _ = sqlDB.Close() }, nil
}

func mysqlDatabaseMissing(err error) bool {
	var driverErr *mysqlDriver.MySQLError
	return errors.As(err, &driverErr) && driverErr.Number == 1049
}

func inspectSchemaReadOnly(db *gorm.DB) (databaseSchema.SchemaStatus, error) {
	if db == nil {
		return databaseSchema.SchemaStatus{TargetSchema: buildinfo.DatabaseSchema, State: databaseSchema.StateInvalid}, errors.New("database is unavailable")
	}
	tx := db.Begin(&sql.TxOptions{ReadOnly: true})
	if tx.Error != nil {
		return databaseSchema.SchemaStatus{TargetSchema: buildinfo.DatabaseSchema, State: databaseSchema.StateInvalid}, tx.Error
	}
	defer tx.Rollback()
	status, err := databaseSchema.InspectSchema(tx)
	if err != nil {
		return status, err
	}
	if err := tx.Commit().Error; err != nil {
		return status, err
	}
	return status, nil
}

func migrateConfiguredDatabase(ctx context.Context, cfg *config.Config, db *gorm.DB) (before, after databaseSchema.SchemaStatus, migrated bool, steps []migrationStep, operationErr error) {
	before = databaseSchema.SchemaStatus{TargetSchema: buildinfo.DatabaseSchema, State: databaseSchema.StateInvalid}
	after = before
	steps = []migrationStep{{Name: "acquire_lock", Status: "pending"}, {Name: "inspect_schema", Status: "pending"}, {Name: "apply_migration", Status: "pending"}, {Name: "verify_schema", Status: "pending"}}
	err := withMigrationLock(ctx, cfg, db, func(lockedDB *gorm.DB) error {
		steps[0].Status = "complete"
		var err error
		before, err = databaseSchema.InspectSchema(lockedDB)
		if err != nil {
			steps[1].Status = "failed"
			return err
		}
		steps[1].Status = "complete"
		after = before
		if before.State == databaseSchema.StateNewerThanBinary || before.State == databaseSchema.StateInvalid {
			steps[2].Status = "refused"
			return databaseSchema.ErrSchemaMismatch
		}
		if before.State == databaseSchema.StateCurrent {
			steps[2].Status = "not_required"
			steps[3].Status = "complete"
			return nil
		}
		if err := MigrateDatabase(lockedDB, buildinfo.DatabaseSchema); err != nil {
			steps[2].Status = "failed"
			return err
		}
		steps[2].Status = "complete"
		migrated = true
		after, err = databaseSchema.InspectSchema(lockedDB)
		if err != nil {
			steps[3].Status = "failed"
			return err
		}
		if after.State != databaseSchema.StateCurrent {
			steps[3].Status = "failed"
			return databaseSchema.ErrSchemaMismatch
		}
		steps[3].Status = "complete"
		return nil
	})
	if err != nil && steps[0].Status == "pending" {
		steps[0].Status = "failed"
	}
	return before, after, migrated, steps, err
}

func withMigrationLock(ctx context.Context, cfg *config.Config, db *gorm.DB, operation func(*gorm.DB) error) error {
	if cfg == nil || db == nil || operation == nil {
		return errors.New("migration lock requires configuration, database, and operation")
	}
	switch cfg.DatabaseType() {
	case config.TypeMysql:
		return db.Connection(func(connection *gorm.DB) error {
			lockContext, cancel := context.WithTimeout(ctx, 60*time.Second)
			defer cancel()
			var acquired sql.NullInt64
			if err := connection.WithContext(lockContext).Raw("SELECT GET_LOCK(?, ?)", migrationLockName(cfg), 60).Scan(&acquired).Error; err != nil {
				return fmt.Errorf("acquire MySQL migration lock: %w", err)
			}
			if !acquired.Valid || acquired.Int64 != 1 {
				return errors.New("MySQL migration lock was not acquired")
			}
			defer func() {
				unlockContext, unlockCancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer unlockCancel()
				_ = connection.WithContext(unlockContext).Exec("SELECT RELEASE_LOCK(?)", migrationLockName(cfg)).Error
			}()
			return operation(connection.WithContext(ctx))
		})
	case config.TypePostgresql:
		return db.Connection(func(connection *gorm.DB) error {
			lockContext, cancel := context.WithTimeout(ctx, 60*time.Second)
			defer cancel()
			key := migrationAdvisoryKey(cfg)
			if err := connection.WithContext(lockContext).Exec("SELECT pg_advisory_lock(?)", key).Error; err != nil {
				return fmt.Errorf("acquire PostgreSQL migration lock: %w", err)
			}
			defer func() {
				unlockContext, unlockCancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer unlockCancel()
				_ = connection.WithContext(unlockContext).Exec("SELECT pg_advisory_unlock(?)", key).Error
			}()
			return operation(connection.WithContext(ctx))
		})
	case config.TypeSqlite:
		filename, err := sqliteFilename(db)
		if err != nil {
			return err
		}
		if filename == "" {
			sqliteInMemoryLock.Lock()
			defer sqliteInMemoryLock.Unlock()
			return operation(db)
		}
		return withSQLiteFileLock(ctx, filename+".migrate.lock", func() error { return operation(db) })
	default:
		return fmt.Errorf("unsupported database type %q", cfg.Gorm.Type)
	}
}

func sqliteFilename(db *gorm.DB) (string, error) {
	var rows []struct {
		Sequence int    `gorm:"column:seq"`
		Name     string `gorm:"column:name"`
		File     string `gorm:"column:file"`
	}
	if err := db.Raw("PRAGMA database_list").Scan(&rows).Error; err != nil {
		return "", err
	}
	for _, row := range rows {
		if row.Name == "main" {
			return row.File, nil
		}
	}
	return "", nil
}

func migrationLockName(cfg *config.Config) string {
	identity := cfg.DatabaseType() + ":"
	if cfg.DatabaseType() == config.TypeMysql {
		identity += cfg.Mysql.Dbname
	} else {
		identity += cfg.Postgresql.Dbname
	}
	hash := fnv.New64a()
	_, _ = hash.Write([]byte(identity))
	// MySQL caps GET_LOCK names at 64 characters. A domain-separated fixed
	// length digest supports every valid database name without truncation
	// collisions while remaining identical across Kessoku processes.
	return fmt.Sprintf("kessoku:migration:%016x", hash.Sum64())
}

func migrationAdvisoryKey(cfg *config.Config) int64 {
	hash := fnv.New64a()
	_, _ = hash.Write([]byte(migrationLockName(cfg)))
	return int64(hash.Sum64() & uint64(^uint64(0)>>1))
}

func applySchemaStatus(result *databaseStatusOutput, status databaseSchema.SchemaStatus) {
	result.InstalledSchema = status.InstalledSchema
	result.TargetSchema = status.TargetSchema
	result.State = status.State
	result.MigrationRequired = status.MigrationRequired
	result.SafeToStart = status.SafeToStart
}

func printDatabaseStatus(cmd *cobra.Command, result databaseStatusOutput) {
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "database: %s\nconnection: %t\ninstalled schema: %s\ntarget schema: %d\nstate: %s\nmigration required: %t\nsafe to start: %t\n",
		result.DatabaseType, result.ConnectionOK, schemaPointerLabel(result.InstalledSchema), result.TargetSchema, result.State, result.MigrationRequired, result.SafeToStart)
}

func schemaPointerLabel(value *uint) string {
	if value == nil {
		return "none"
	}
	return fmt.Sprintf("%d", *value)
}

func writeMigrationFailure(cmd *cobra.Command, jsonOutput bool, result databaseMigrationOutput, exitCode int, cause error) error {
	if jsonOutput {
		_ = writeJSON(cmd.OutOrStdout(), result)
	} else if result.Error != nil {
		_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "%s: %s\n", result.Error.Code, result.Error.Message)
	}
	return &cliExitError{code: exitCode, reported: true, err: cause}
}
