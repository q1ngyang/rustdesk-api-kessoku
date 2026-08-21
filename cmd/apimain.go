package main

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/go-redis/redis/v8"
	mysqlDriver "github.com/go-sql-driver/mysql"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	"github.com/q1ngyang/rustdesk-api-kessoku/v2/config"
	"github.com/q1ngyang/rustdesk-api-kessoku/v2/global"
	"github.com/q1ngyang/rustdesk-api-kessoku/v2/http"
	internalAuth "github.com/q1ngyang/rustdesk-api-kessoku/v2/internal/auth"
	"github.com/q1ngyang/rustdesk-api-kessoku/v2/lib/cache"
	"github.com/q1ngyang/rustdesk-api-kessoku/v2/lib/lock"
	"github.com/q1ngyang/rustdesk-api-kessoku/v2/lib/logger"
	"github.com/q1ngyang/rustdesk-api-kessoku/v2/lib/orm"
	"github.com/q1ngyang/rustdesk-api-kessoku/v2/lib/upload"
	"github.com/q1ngyang/rustdesk-api-kessoku/v2/model"
	"github.com/q1ngyang/rustdesk-api-kessoku/v2/service"
	"github.com/q1ngyang/rustdesk-api-kessoku/v2/utils"
	"github.com/spf13/cobra"
	"gorm.io/gorm"
)

const DatabaseVersion = 301

const mysqlTLSProfile = "kessoku-verified-ca"

// @title 管理系统API
// @version 1.0
// @description 接口
// @basePath /api
// @securityDefinitions.apikey token
// @in header
// @name api-token
// @securitydefinitions.apikey BearerAuth
// @in header
// @name Authorization

var rootCmd = &cobra.Command{
	Use:   "kessoku-api",
	Short: "Kessoku control plane for RustDesk deployments",
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		InitGlobal()
	},
	Run: func(cmd *cobra.Command, args []string) {
		global.Logger.Info("API SERVER START")
		http.ApiInit()
	},
}

var resetAdminPasswordFile string
var resetUserPasswordFile string
var resetUserID uint

var resetPwdCmd = &cobra.Command{
	Use:     "reset-admin-pwd --password-file PATH",
	Example: "kessoku-api reset-admin-pwd --password-file /run/secrets/bootstrap-admin-password",
	Short:   "Reset Admin Password",
	Args:    cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		pwd, err := passwordFromFile(resetAdminPasswordFile)
		if err != nil {
			return fmt.Errorf("read password file: %w", err)
		}
		admin := service.AllService.UserService.InfoById(1)
		if admin.Id == 0 {
			return errors.New("administrator user not found")
		}
		err = service.AllService.UserService.UpdatePassword(admin, pwd)
		if err != nil {
			return fmt.Errorf("reset administrator password: %w", err)
		}
		global.Logger.Info("reset password success! ")
		return nil
	},
}
var resetUserPwdCmd = &cobra.Command{
	Use:     "reset-pwd --user-id ID --password-file PATH",
	Example: "kessoku-api reset-pwd --user-id 2 --password-file /run/secrets/user-password",
	Short:   "Reset User Password",
	Args:    cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		if resetUserID == 0 {
			return errors.New("user-id must be greater than 0")
		}
		pwd, err := passwordFromFile(resetUserPasswordFile)
		if err != nil {
			return fmt.Errorf("read password file: %w", err)
		}
		u := service.AllService.UserService.InfoById(resetUserID)
		if u.Id == 0 {
			return errors.New("user not found")
		}
		err = service.AllService.UserService.UpdatePassword(u, pwd)
		if err != nil {
			return fmt.Errorf("reset user password: %w", err)
		}
		global.Logger.Info("reset password success!")
		return nil
	},
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&global.ConfigPath, "config", "c", "./conf/config.yaml", "choose config file")
	resetPwdCmd.Flags().StringVar(&resetAdminPasswordFile, "password-file", "", "owner-readable file containing the new password")
	_ = resetPwdCmd.MarkFlagRequired("password-file")
	resetUserPwdCmd.Flags().UintVar(&resetUserID, "user-id", 0, "user ID")
	resetUserPwdCmd.Flags().StringVar(&resetUserPasswordFile, "password-file", "", "owner-readable file containing the new password")
	_ = resetUserPwdCmd.MarkFlagRequired("user-id")
	_ = resetUserPwdCmd.MarkFlagRequired("password-file")
	rootCmd.AddCommand(resetPwdCmd, resetUserPwdCmd)
}

func passwordFromFile(path string) (string, error) {
	if strings.TrimSpace(path) == "" {
		return "", errors.New("password-file is required")
	}
	pathInfo, err := os.Lstat(path)
	if err != nil {
		return "", err
	}
	if !pathInfo.Mode().IsRegular() {
		return "", errors.New("password-file must be a regular file")
	}
	if pathInfo.Mode().Perm()&0o077 != 0 {
		return "", errors.New("password-file must not be accessible by group or other users")
	}
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	fileInfo, err := file.Stat()
	if err != nil {
		return "", err
	}
	if !fileInfo.Mode().IsRegular() || !os.SameFile(pathInfo, fileInfo) {
		return "", errors.New("password-file changed while opening")
	}
	if fileInfo.Mode().Perm()&0o077 != 0 {
		return "", errors.New("password-file must not be accessible by group or other users")
	}
	contents, err := io.ReadAll(io.LimitReader(file, 131))
	if err != nil {
		return "", err
	}
	if len(contents) > 130 {
		return "", errors.New("password must contain 12 to 128 bytes")
	}
	password := strings.TrimSuffix(strings.TrimSuffix(string(contents), "\n"), "\r")
	if len(password) < 12 || len(password) > 128 {
		return "", errors.New("password must contain 12 to 128 bytes")
	}
	return password, nil
}
func main() {
	if err := rootCmd.Execute(); err != nil {
		global.Logger.Error(err)
		os.Exit(1)
	}
}

func mysqlDSN(databaseName string) string {
	settings := mysqlDriver.NewConfig()
	settings.User = global.Config.Mysql.Username
	settings.Passwd = global.Config.Mysql.Password
	settings.Net = "tcp"
	settings.Addr = global.Config.Mysql.Addr
	settings.DBName = databaseName
	settings.ParseTime = true
	settings.Loc = time.Local
	settings.TLSConfig = "true"
	if global.Config.Mysql.CaFile != "" {
		settings.TLSConfig = mysqlTLSProfile
	}
	settings.Params = map[string]string{"charset": "utf8mb4"}
	return settings.FormatDSN()
}

func configureMySQLTLS() error {
	if global.Config.Mysql.CaFile == "" {
		return nil
	}
	caPEM, err := os.ReadFile(global.Config.Mysql.CaFile)
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
	mysqlDriver.DeregisterTLSConfig(mysqlTLSProfile)
	if err := mysqlDriver.RegisterTLSConfig(mysqlTLSProfile, &tls.Config{
		MinVersion: tls.VersionTLS12,
		RootCAs:    roots,
	}); err != nil {
		return fmt.Errorf("register MySQL TLS profile: %w", err)
	}
	return nil
}

func postgresqlDSN() string {
	settings := global.Config.Postgresql
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

func InitGlobal() {
	//配置解析
	global.Viper = config.Init(&global.Config, global.ConfigPath)

	//日志
	global.Logger = logger.New(&logger.Config{
		Path:         global.Config.Logger.Path,
		Level:        global.Config.Logger.Level,
		ReportCaller: global.Config.Logger.ReportCaller,
	})

	global.InitI18n()

	//redis
	global.Redis = redis.NewClient(&redis.Options{
		Addr:     global.Config.Redis.Addr,
		Password: global.Config.Redis.Password,
		DB:       global.Config.Redis.Db,
	})

	//cache
	if global.Config.Cache.Type == cache.TypeFile {
		fc := cache.NewFileCache()
		fc.SetDir(global.Config.Cache.FileDir)
		global.Cache = fc
	} else if global.Config.Cache.Type == cache.TypeRedis {
		global.Cache = cache.NewRedis(&redis.Options{
			Addr:     global.Config.Cache.RedisAddr,
			Password: global.Config.Cache.RedisPwd,
			DB:       global.Config.Cache.RedisDb,
		})
	}
	//gorm
	if global.Config.Gorm.Type == config.TypeMysql {
		if err := configureMySQLTLS(); err != nil {
			global.Logger.Fatalf("configure MySQL TLS: %v", err)
		}

		dsn := mysqlDSN(global.Config.Mysql.Dbname)

		global.DB = orm.NewMysql(&orm.MysqlConfig{
			Dsn:          dsn,
			MaxIdleConns: global.Config.Gorm.MaxIdleConns,
			MaxOpenConns: global.Config.Gorm.MaxOpenConns,
		}, global.Logger)
	} else if global.Config.Gorm.Type == config.TypePostgresql {
		dsn := postgresqlDSN()
		global.DB = orm.NewPostgresql(&orm.PostgresqlConfig{
			Dsn:          dsn,
			MaxIdleConns: global.Config.Gorm.MaxIdleConns,
			MaxOpenConns: global.Config.Gorm.MaxOpenConns,
		}, global.Logger)
	} else {
		//sqlite
		global.DB = orm.NewSqlite(&orm.SqliteConfig{
			MaxIdleConns: global.Config.Gorm.MaxIdleConns,
			MaxOpenConns: global.Config.Gorm.MaxOpenConns,
		}, global.Logger)
	}

	//validator
	global.ApiInitValidator()

	//oss
	global.Oss = &upload.Oss{
		AccessKeyId:     global.Config.Oss.AccessKeyId,
		AccessKeySecret: global.Config.Oss.AccessKeySecret,
		Host:            global.Config.Oss.Host,
		CallbackUrl:     global.Config.Oss.CallbackUrl,
		ExpireTime:      global.Config.Oss.ExpireTime,
		MaxByte:         global.Config.Oss.MaxByte,
	}

	// Access-token signer/verifier. An enabled profile fails closed when its
	// secret file or contract settings are invalid.
	var authErr error
	global.Auth, authErr = internalAuth.NewManager(global.Config.Auth)
	if authErr != nil {
		global.Logger.Fatalf("initialize Ed25519 auth profile: %v", authErr)
	}
	//locker
	global.Lock = lock.NewLocal()

	//service
	service.New(&global.Config, global.DB, global.Logger, global.Auth, global.Lock)

	global.LoginLimiter = utils.NewLoginLimiter(utils.SecurityPolicy{
		CaptchaThreshold: global.Config.App.CaptchaThreshold,
		BanThreshold:     global.Config.App.BanThreshold,
		AttemptsWindow:   10 * time.Minute,
		BanDuration:      30 * time.Minute,
	})
	global.LoginLimiter.RegisterProvider(utils.B64StringCaptchaProvider{})
	DatabaseAutoUpdate()
	if err := service.RecordAuthKeyringStartup(global.Auth); err != nil {
		global.Logger.Fatalf("record authentication keyring audit: %v", err)
	}
}

func DatabaseAutoUpdate() {
	version := DatabaseVersion

	db := global.DB

	if global.Config.Gorm.Type == config.TypeMysql {
		//检查存不存在数据库，不存在则创建
		dbName := db.Migrator().CurrentDatabase()
		if dbName == "" {
			dbName = global.Config.Mysql.Dbname
			// 移除 DSN 中的数据库名称，以便初始连接时不指定数据库
			dsnWithoutDB := mysqlDSN("")

			//新链接
			dbWithoutDB := orm.NewMysql(&orm.MysqlConfig{
				Dsn: dsnWithoutDB,
			}, global.Logger)
			// 获取底层的 *sql.DB 对象，并确保在程序退出时关闭连接
			sqlDBWithoutDB, err := dbWithoutDB.DB()
			if err != nil {
				global.Logger.Errorf("获取底层 *sql.DB 对象失败: %v", err)
				return
			}
			defer func() {
				if err := sqlDBWithoutDB.Close(); err != nil {
					global.Logger.Errorf("关闭连接失败: %v", err)
				}
			}()

			err = dbWithoutDB.Exec("CREATE DATABASE IF NOT EXISTS `" + strings.ReplaceAll(dbName, "`", "``") + "` DEFAULT CHARSET utf8mb4").Error
			if err != nil {
				global.Logger.Error(err)
				return
			}
		}
	}

	if !db.Migrator().HasTable(&model.Version{}) {
		if err := Migrate(uint(version)); err != nil {
			global.Logger.Fatalf("database migration failed: %v", err)
		}
	} else {
		//查找最后一个version
		var v model.Version
		db.Last(&v)
		if v.Version < 245 {
			if err := prepareLegacyOauthIdentityMigration(); err != nil {
				global.Logger.Fatalf("prepare legacy OAuth identity migration: %v", err)
			}
		}
		if v.Version < uint(version) {
			if err := Migrate(uint(version)); err != nil {
				global.Logger.Fatalf("database migration failed: %v", err)
			}
		}

		// 245迁移
		if v.Version < 245 {
			//通过email迁移旧的google授权
			uts := make([]model.UserThird, 0)
			db.Where("oauth_type = ?", "google").Find(&uts)
			for _, ut := range uts {
				if ut.UserId > 0 {
					db.Model(&model.User{}).Where("id = ?", ut.UserId).Update("email", ut.OpenId)
				}
			}
		}
		if v.Version < 246 {
			db.Exec("update oauths set issuer = 'https://accounts.google.com' where op = 'google' and issuer is null")
		}
	}

}
func Migrate(version uint) error {
	global.Logger.Info("Migrating....", version)
	if err := validateOauthIdentityUniqueness(); err != nil {
		return err
	}
	err := global.DB.AutoMigrate(
		&model.Version{},
		&model.User{},
		&model.UserToken{},
		&model.Tag{},
		&model.AddressBook{},
		&model.Peer{},
		&model.Group{},
		&model.UserThird{},
		&model.Oauth{},
		&model.LoginLog{},
		&model.ShareRecord{},
		&model.AuditConn{},
		&model.AuditFile{},
		&model.AddressBookCollection{},
		&model.AddressBookCollectionRule{},
		&model.ServerCmd{},
		&model.DeviceGroup{},
		&model.AdminAuditEvent{},
		&model.LdapIdentity{},
		&model.ControlOperationExpectation{},
		&model.SecurityInvariantLock{},
	)
	if err != nil {
		return fmt.Errorf("schema migration: %w", err)
	}
	if err := global.DB.FirstOrCreate(&model.SecurityInvariantLock{Name: "enabled-admin"}).Error; err != nil {
		return fmt.Errorf("create enabled-administrator invariant lock: %w", err)
	}
	if err := global.DB.Exec("UPDATE users SET auth_version = 1 WHERE auth_version IS NULL OR auth_version = 0").Error; err != nil {
		return fmt.Errorf("backfill user auth version: %w", err)
	}
	if err := global.DB.Exec("UPDATE user_tokens SET auth_version = 1 WHERE auth_version IS NULL OR auth_version = 0").Error; err != nil {
		return fmt.Errorf("backfill token auth version: %w", err)
	}
	var legacyTokens []model.UserToken
	if err := global.DB.Where("token <> '' AND token_hash IS NULL").Find(&legacyTokens).Error; err != nil {
		return fmt.Errorf("read legacy tokens: %w", err)
	}
	for i := range legacyTokens {
		hash := internalAuth.TokenHashHex(legacyTokens[i].Token)
		if err := global.DB.Model(&legacyTokens[i]).Updates(map[string]interface{}{
			"token_hash": hash,
			"token":      "",
		}).Error; err != nil {
			return fmt.Errorf("backfill token hash for row %d: %w", legacyTokens[i].Id, err)
		}
	}
	if err := global.DB.Create(&model.Version{Version: version}).Error; err != nil {
		return fmt.Errorf("record database version: %w", err)
	}
	//如果是初次则创建一个默认用户
	var vc int64
	global.DB.Model(&model.Version{}).Count(&vc)
	if vc == 1 {
		localizer := global.Localizer("")
		defaultGroup, _ := localizer.LocalizeMessage(&i18n.Message{
			ID: "DefaultGroup",
		})
		group := &model.Group{
			Name: defaultGroup,
			Type: model.GroupTypeDefault,
		}
		service.AllService.GroupService.Create(group)

		shareGroup, _ := localizer.LocalizeMessage(&i18n.Message{
			ID: "ShareGroup",
		})
		groupShare := &model.Group{
			Name: shareGroup,
			Type: model.GroupTypeShare,
		}
		service.AllService.GroupService.Create(groupShare)
		//是true
		is_admin := true
		admin := &model.User{
			Username: "admin",
			Nickname: "Admin",
			Status:   model.COMMON_STATUS_ENABLE,
			IsAdmin:  &is_admin,
			GroupId:  1,
		}

		// Create an unreachable bootstrap credential. Operators must set the
		// initial password explicitly through reset-admin-pwd --password-file;
		// reusable credentials are never emitted to application logs.
		pwd := utils.RandomString(32)
		if pwd == "" {
			return errors.New("generate bootstrap administrator credential")
		}
		var err error
		admin.Password, err = utils.EncryptPassword(pwd)
		if err != nil {
			return fmt.Errorf("hash bootstrap administrator credential: %w", err)
		}
		if err := global.DB.Create(admin).Error; err != nil {
			return fmt.Errorf("create bootstrap administrator: %w", err)
		}
		global.Logger.Info("bootstrap administrator created; set its password with reset-admin-pwd --password-file")
	}
	return nil
}

func prepareLegacyOauthIdentityMigration() error {
	migrator := global.DB.Migrator()
	if migrator.HasTable(&model.Oauth{}) {
		for _, field := range []string{"OauthType", "Issuer"} {
			if !migrator.HasColumn(&model.Oauth{}, field) {
				if err := migrator.AddColumn(&model.Oauth{}, field); err != nil {
					return fmt.Errorf("add oauths.%s: %w", field, err)
				}
			}
		}
		if err := global.DB.Exec("UPDATE oauths SET oauth_type = op WHERE oauth_type IS NULL OR oauth_type = ''").Error; err != nil {
			return fmt.Errorf("backfill oauth provider type: %w", err)
		}
		if err := global.DB.Exec("UPDATE oauths SET issuer = 'https://accounts.google.com' WHERE op = 'google' AND (issuer IS NULL OR issuer = '')").Error; err != nil {
			return fmt.Errorf("backfill Google OAuth issuer: %w", err)
		}
	}
	if migrator.HasTable(&model.UserThird{}) {
		for _, field := range []string{"OauthType", "Op"} {
			if !migrator.HasColumn(&model.UserThird{}, field) {
				if err := migrator.AddColumn(&model.UserThird{}, field); err != nil {
					return fmt.Errorf("add user_thirds.%s: %w", field, err)
				}
			}
		}
		if migrator.HasColumn(&model.UserThird{}, "third_type") {
			if err := global.DB.Exec("UPDATE user_thirds SET oauth_type = third_type WHERE oauth_type IS NULL OR oauth_type = ''").Error; err != nil {
				return fmt.Errorf("backfill OAuth identity type: %w", err)
			}
			if err := global.DB.Exec("UPDATE user_thirds SET op = third_type WHERE op IS NULL OR op = ''").Error; err != nil {
				return fmt.Errorf("backfill OAuth identity provider: %w", err)
			}
		}
	}
	return nil
}

func validateOauthIdentityUniqueness() error {
	migrator := global.DB.Migrator()
	if !migrator.HasTable(&model.UserThird{}) ||
		!migrator.HasColumn(&model.UserThird{}, "user_id") ||
		!migrator.HasColumn(&model.UserThird{}, "op") ||
		!migrator.HasColumn(&model.UserThird{}, "open_id") {
		return nil
	}

	type duplicateBinding struct {
		Count int64 `gorm:"column:duplicate_count"`
	}
	checks := []struct {
		columns string
		name    string
	}{
		{columns: "user_id, op", name: "user/provider"},
		{columns: "op, open_id", name: "provider/subject"},
	}
	for _, check := range checks {
		var duplicate duplicateBinding
		err := global.DB.Table("user_thirds").
			Select("COUNT(*) AS duplicate_count").
			Group(check.columns).
			Having("COUNT(*) > ?", 1).
			Limit(1).
			Take(&duplicate).Error
		if err == nil {
			return fmt.Errorf("OAuth identity migration preflight: duplicate %s binding with %d rows; back up the database, review and merge the duplicate identity rows, then retry", check.name, duplicate.Count)
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("OAuth identity migration preflight for %s binding: %w", check.name, err)
		}
	}
	return nil
}
