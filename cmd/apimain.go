package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/go-redis/redis/v8"
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
)

const DatabaseVersion = 300

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

		dsn := fmt.Sprintf("%s:%s@(%s)/%s?charset=utf8mb4&parseTime=True&loc=Local&tls=%s",
			global.Config.Mysql.Username,
			global.Config.Mysql.Password,
			global.Config.Mysql.Addr,
			global.Config.Mysql.Dbname,
			global.Config.Mysql.Tls,
		)

		global.DB = orm.NewMysql(&orm.MysqlConfig{
			Dsn:          dsn,
			MaxIdleConns: global.Config.Gorm.MaxIdleConns,
			MaxOpenConns: global.Config.Gorm.MaxOpenConns,
		}, global.Logger)
	} else if global.Config.Gorm.Type == config.TypePostgresql {
		dsn := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s TimeZone=%s",
			global.Config.Postgresql.Host,
			global.Config.Postgresql.Port,
			global.Config.Postgresql.User,
			global.Config.Postgresql.Password,
			global.Config.Postgresql.Dbname,
			global.Config.Postgresql.Sslmode,
			global.Config.Postgresql.TimeZone,
		)
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
			dsnWithoutDB := fmt.Sprintf("%s:%s@(%s)/%s?charset=utf8mb4&parseTime=True&loc=Local&tls=%s",
				global.Config.Mysql.Username,
				global.Config.Mysql.Password,
				global.Config.Mysql.Addr,
				"",
				global.Config.Mysql.Tls,
			)

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
		if v.Version < uint(version) {
			if err := Migrate(uint(version)); err != nil {
				global.Logger.Fatalf("database migration failed: %v", err)
			}
		}

		// 245迁移
		if v.Version < 245 {
			//oauths 表的 oauth_type 字段设置为 op同样的值
			db.Exec("update oauths set oauth_type = op")
			db.Exec("update oauths set issuer = 'https://accounts.google.com' where op = 'google'")
			db.Exec("update user_thirds set oauth_type = third_type, op = third_type")
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
	)
	if err != nil {
		return fmt.Errorf("schema migration: %w", err)
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
