package config

import (
	"errors"
	"strings"
)

const (
	TypeSqlite     = "sqlite"
	TypeMysql      = "mysql"
	TypePostgresql = "postgresql"
)

type Gorm struct {
	Type         string `mapstructure:"type"`
	MaxIdleConns int    `mapstructure:"max-idle-conns"`
	MaxOpenConns int    `mapstructure:"max-open-conns"`
}

type Mysql struct {
	Addr     string `mapstructure:"addr"`
	Username string `mapstructure:"username"`
	Password string `mapstructure:"password"`
	Dbname   string `mapstructure:"dbname"`
	Tls      string `mapstructure:"tls"` // must be true for authenticated TLS
	CaFile   string `mapstructure:"ca-file"`
}

type Postgresql struct {
	Host        string `mapstructure:"host"`
	Port        string `mapstructure:"port"`
	User        string `mapstructure:"user"`
	Password    string `mapstructure:"password"`
	Dbname      string `mapstructure:"dbname"`
	Sslmode     string `mapstructure:"sslmode"` // "disable", "require", "verify-ca", "verify-full"
	Sslrootcert string `mapstructure:"ssl-root-cert"`
	TimeZone    string `mapstructure:"time-zone"` // e.g., "Asia/Shanghai"
}

func (c Config) validateDatabaseTransport() error {
	switch strings.ToLower(strings.TrimSpace(c.Gorm.Type)) {
	case "", TypeSqlite:
		return nil
	case TypeMysql:
		if !strings.EqualFold(strings.TrimSpace(c.Mysql.Tls), "true") {
			return errors.New("mysql.tls must be true; insecure and skip-verify database transport is not supported")
		}
		return nil
	case TypePostgresql:
		if !strings.EqualFold(strings.TrimSpace(c.Postgresql.Sslmode), "verify-full") {
			return errors.New("postgresql.sslmode must be verify-full")
		}
		return nil
	default:
		return errors.New("gorm.type must be sqlite, mysql, or postgresql")
	}
}
