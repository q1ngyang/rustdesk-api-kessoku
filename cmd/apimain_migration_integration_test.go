package main

import (
	"os"
	"strings"
	"testing"

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
