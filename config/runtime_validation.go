package config

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
)

// ValidateRuntime covers the fields required by the executable itself. File
// existence and cryptographic parsing are checked separately so `config
// validate` can remain read-only and database-only commands do not need to
// initialize unrelated secrets.
func (c Config) ValidateRuntime() error {
	if c.Gorm.MaxIdleConns < 0 || c.Gorm.MaxOpenConns < 0 {
		return errors.New("gorm.max-idle-conns and gorm.max-open-conns cannot be negative")
	}
	if c.Gorm.MaxOpenConns > 0 && c.Gorm.MaxIdleConns > c.Gorm.MaxOpenConns {
		return errors.New("gorm.max-idle-conns must not exceed gorm.max-open-conns")
	}
	switch c.DatabaseType() {
	case TypeSqlite:
	case TypeMysql:
		if err := requiredHostPort("mysql.addr", c.Mysql.Addr); err != nil {
			return err
		}
		if strings.TrimSpace(c.Mysql.Username) == "" || strings.TrimSpace(c.Mysql.Dbname) == "" {
			return errors.New("mysql.username and mysql.dbname are required")
		}
	case TypePostgresql:
		if strings.TrimSpace(c.Postgresql.Host) == "" || strings.TrimSpace(c.Postgresql.User) == "" || strings.TrimSpace(c.Postgresql.Dbname) == "" {
			return errors.New("postgresql.host, postgresql.user, and postgresql.dbname are required")
		}
		port, err := strconv.ParseUint(c.Postgresql.Port, 10, 16)
		if err != nil || port == 0 {
			return errors.New("postgresql.port must be between 1 and 65535")
		}
	}
	if err := requiredHostPort("gin.api-addr", c.Gin.ApiAddr); err != nil {
		return err
	}
	switch c.Gin.Mode {
	case "", DebugMode, ReleaseMode, "test":
	default:
		return errors.New("gin.mode must be debug, release, or test")
	}
	if err := cleanPath("gin.resources-path", c.Gin.ResourcesPath, true); err != nil {
		return err
	}
	for field, port := range map[string]int{
		"admin.id-server-port":    c.Admin.IdServerPort,
		"admin.relay-server-port": c.Admin.RelayServerPort,
	} {
		if port < 1 || port > 65535 {
			return fmt.Errorf("%s must be between 1 and 65535", field)
		}
	}
	if err := requiredHostPort("rustdesk.id-server", c.Rustdesk.IdServer); err != nil {
		return err
	}
	if err := requiredHostPort("rustdesk.relay-server", c.Rustdesk.RelayServer); err != nil {
		return err
	}
	if err := absoluteHTTPURL("rustdesk.api-server", c.Rustdesk.ApiServer); err != nil {
		return err
	}
	if strings.TrimSpace(c.Rustdesk.Key) == "" && strings.TrimSpace(c.Rustdesk.KeyFile) == "" {
		return errors.New("rustdesk.key or rustdesk.key-file is required")
	}
	if c.Rustdesk.KeyFile != "" {
		if err := cleanPath("rustdesk.key-file", c.Rustdesk.KeyFile, false); err != nil {
			return err
		}
	}
	if c.Admin.HelloFile != "" {
		if err := cleanPath("admin.hello-file", c.Admin.HelloFile, false); err != nil {
			return err
		}
	}
	if c.Logger.Path != "" {
		if err := cleanPath("logger.path", c.Logger.Path, false); err != nil {
			return err
		}
	}
	switch strings.TrimSpace(c.Cache.Type) {
	case "", "memory":
	case "file":
		if err := cleanPath("cache.file-dir", c.Cache.FileDir, true); err != nil {
			return err
		}
	case "redis":
		if err := requiredHostPort("cache.redis-addr", c.Cache.RedisAddr); err != nil {
			return err
		}
	default:
		return errors.New("cache.type must be memory, file, or redis")
	}
	return nil
}

func (c Config) DatabaseType() string {
	typ := strings.ToLower(strings.TrimSpace(c.Gorm.Type))
	if typ == "" {
		return TypeSqlite
	}
	return typ
}

func requiredHostPort(field, value string) error {
	host, port, err := net.SplitHostPort(strings.TrimSpace(value))
	if err != nil || host == "" {
		return fmt.Errorf("%s must be an explicit host and port", field)
	}
	number, err := strconv.ParseUint(port, 10, 16)
	if err != nil || number == 0 {
		return fmt.Errorf("%s port must be between 1 and 65535", field)
	}
	return nil
}

func absoluteHTTPURL(field, value string) error {
	parsed, err := url.Parse(value)
	if err != nil || parsed.Host == "" || parsed.User != nil || parsed.Opaque != "" || parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("%s must be an absolute HTTP or HTTPS URL without credentials", field)
	}
	if parsed.RawQuery != "" || parsed.Fragment != "" {
		return fmt.Errorf("%s must not contain a query or fragment", field)
	}
	return nil
}

func cleanPath(field, value string, rejectDot bool) error {
	if value == "" || strings.TrimSpace(value) != value || strings.ContainsRune(value, '\x00') {
		return fmt.Errorf("%s must be a clean path", field)
	}
	cleaned := filepath.Clean(value)
	if cleaned == string(filepath.Separator) || rejectDot && cleaned == "." {
		return fmt.Errorf("%s must not be the working directory or filesystem root", field)
	}
	return nil
}
