package config

import "testing"

func TestDatabaseTransportValidation(t *testing.T) {
	for _, candidate := range []Config{
		{Proxy: Proxy{Enable: true, Host: "http://proxy.example.test:8080"}},
		{Gorm: Gorm{Type: TypeMysql}, Mysql: Mysql{Tls: "false"}},
		{Gorm: Gorm{Type: TypeMysql}, Mysql: Mysql{Tls: "skip-verify"}},
		{Gorm: Gorm{Type: TypePostgresql}, Postgresql: Postgresql{Sslmode: "disable"}},
		{Gorm: Gorm{Type: TypePostgresql}, Postgresql: Postgresql{Sslmode: "require"}},
		{Gorm: Gorm{Type: "unknown"}},
	} {
		if err := candidate.Validate(); err == nil {
			t.Fatalf("insecure database configuration was accepted: %+v", candidate)
		}
	}

	for _, candidate := range []Config{
		{Gorm: Gorm{Type: TypeSqlite}},
		{Gorm: Gorm{Type: TypeMysql}, Mysql: Mysql{Tls: "true"}},
		{Gorm: Gorm{Type: TypePostgresql}, Postgresql: Postgresql{Sslmode: "verify-full"}},
	} {
		if err := candidate.Validate(); err != nil {
			t.Fatalf("secure database configuration was rejected: %v", err)
		}
	}
}
