package main

import (
	"encoding/pem"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"

	mysqlDriver "github.com/go-sql-driver/mysql"
	"github.com/q1ngyang/rustdesk-api-kessoku/v3/config"
	"github.com/q1ngyang/rustdesk-api-kessoku/v3/global"
)

func TestDatabaseDSNBuildersPreserveCredentialsAndTLS(t *testing.T) {
	oldConfig := global.Config
	t.Cleanup(func() { global.Config = oldConfig })

	global.Config.Mysql = config.Mysql{
		Addr:     "mysql.example.test:3306",
		Username: "user_name",
		Password: "p@:ss/word?&value",
	}
	parsedMySQL, err := mysqlDriver.ParseDSN(mysqlDSN("kessoku"))
	if err != nil {
		t.Fatal(err)
	}
	if parsedMySQL.User != global.Config.Mysql.Username || parsedMySQL.Passwd != global.Config.Mysql.Password || parsedMySQL.DBName != "kessoku" || parsedMySQL.TLSConfig != "true" {
		t.Fatalf("MySQL DSN lost values or TLS: %+v", parsedMySQL)
	}

	global.Config.Postgresql = config.Postgresql{
		Host:        "postgres.example.test",
		Port:        "5432",
		User:        "user:name",
		Password:    "p@ss/word?&value",
		Dbname:      "kessoku",
		Sslmode:     "verify-full",
		Sslrootcert: "/run/secrets/database ca.pem",
		TimeZone:    "Asia/Singapore",
	}
	parsedPostgres, err := url.Parse(postgresqlDSN())
	if err != nil {
		t.Fatal(err)
	}
	password, _ := parsedPostgres.User.Password()
	query := parsedPostgres.Query()
	if parsedPostgres.User.Username() != global.Config.Postgresql.User || password != global.Config.Postgresql.Password || parsedPostgres.Host != "postgres.example.test:5432" || query.Get("sslmode") != "verify-full" || query.Get("sslrootcert") != global.Config.Postgresql.Sslrootcert {
		t.Fatalf("PostgreSQL DSN lost values or TLS: %s", parsedPostgres.Redacted())
	}
}

func TestMySQLCustomCAUsesVerifiedNamedTLSProfile(t *testing.T) {
	oldConfig := global.Config
	t.Cleanup(func() {
		global.Config = oldConfig
		mysqlDriver.DeregisterTLSConfig(mysqlTLSProfile)
	})
	server := httptest.NewTLSServer(nil)
	defer server.Close()
	caFile := filepath.Join(t.TempDir(), "mysql-ca.pem")
	certificate := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: server.Certificate().Raw})
	if err := os.WriteFile(caFile, certificate, 0o600); err != nil {
		t.Fatal(err)
	}
	global.Config.Mysql = config.Mysql{Addr: "mysql.example.test:3306", Username: "user", Dbname: "kessoku", Tls: "true", CaFile: caFile}
	if err := configureMySQLTLS(); err != nil {
		t.Fatal(err)
	}
	parsed, err := mysqlDriver.ParseDSN(mysqlDSN("kessoku"))
	if err != nil {
		t.Fatal(err)
	}
	if parsed.TLSConfig != mysqlTLSProfile || parsed.TLS == nil || parsed.TLS.InsecureSkipVerify || parsed.TLS.ServerName != "mysql.example.test" {
		t.Fatalf("custom MySQL TLS profile is not certificate/hostname verified: %+v", parsed.TLS)
	}
}
