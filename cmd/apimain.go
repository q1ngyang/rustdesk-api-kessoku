package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	"github.com/q1ngyang/rustdesk-api-kessoku/v3/config"
	"github.com/q1ngyang/rustdesk-api-kessoku/v3/global"
	internalAuth "github.com/q1ngyang/rustdesk-api-kessoku/v3/internal/auth"
	"github.com/q1ngyang/rustdesk-api-kessoku/v3/internal/buildinfo"
	databaseSchema "github.com/q1ngyang/rustdesk-api-kessoku/v3/internal/database"
	"github.com/q1ngyang/rustdesk-api-kessoku/v3/lib/cache"
	"github.com/q1ngyang/rustdesk-api-kessoku/v3/lib/lock"
	"github.com/q1ngyang/rustdesk-api-kessoku/v3/lib/logger"
	"github.com/q1ngyang/rustdesk-api-kessoku/v3/lib/upload"
	"github.com/q1ngyang/rustdesk-api-kessoku/v3/model"
	"github.com/q1ngyang/rustdesk-api-kessoku/v3/service"
	"github.com/q1ngyang/rustdesk-api-kessoku/v3/utils"
	"gorm.io/gorm"
)

const DatabaseVersion = buildinfo.DatabaseSchema

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

var rootCmd = newRootCommand()

func main() {
	if err := rootCmd.Execute(); err != nil {
		if !commandErrorReported(err) {
			_, _ = fmt.Fprintln(os.Stderr, err)
		}
		os.Exit(commandExitCode(err))
	}
}

func mysqlDSN(databaseName string) string {
	return mysqlDSNForConfig(&global.Config, databaseName)
}

func configureMySQLTLS() error {
	return configureMySQLTLSForConfig(&global.Config)
}

func postgresqlDSN() string {
	return postgresqlDSNForConfig(&global.Config)
}

func InitGlobal() {
	// Parse and validate configuration before any key generation, database
	// connection, migration, or background service is initialized.
	parsed := config.Config{}
	viperConfig, err := config.Load(&parsed, global.ConfigPath)
	if err != nil {
		panic(fmt.Errorf("initialize configuration: %w", err))
	}
	validationErrors, validationWarnings := validateConfigurationReferences(&parsed)
	if len(validationErrors) > 0 {
		panic(fmt.Errorf("initialize configuration: %s: %s", validationErrors[0].Field, validationErrors[0].Message))
	}
	global.Config = parsed
	global.Viper = viperConfig
	global.Config.Rustdesk.LoadKeyFile()

	//日志
	global.Logger = logger.New(&logger.Config{
		Path:         global.Config.Logger.Path,
		Level:        global.Config.Logger.Level,
		ReportCaller: global.Config.Logger.ReportCaller,
		MaxSizeMB:    global.Config.Logger.MaxSizeMB,
		MaxBackups:   global.Config.Logger.MaxBackups,
		MaxAgeDays:   global.Config.Logger.MaxAgeDays,
		Compress:     global.Config.Logger.Compress,
		LocalTime:    global.Config.Logger.LocalTime,
	})

	global.InitI18n()
	for _, warning := range validationWarnings {
		global.Logger.Warnf("configuration warning %s: %s", warning.Field, warning.Message)
	}

	// The schema guard and locked migration run before any service is allowed to
	// read application tables or create the TOTP key.
	global.DB, _, err = openConfiguredDatabase(context.Background(), &global.Config, databaseCreate, global.Logger)
	if err != nil {
		global.Logger.Fatalf("connect database: %v", err)
	}
	DatabaseAutoUpdate()

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
	if err := service.AllService.TwoFactorService.Init(); err != nil {
		global.Logger.Fatalf("initialize two-factor encryption: %v", err)
	}

	global.LoginLimiter = utils.NewLoginLimiter(utils.SecurityPolicy{
		CaptchaThreshold: global.Config.App.CaptchaThreshold,
		BanThreshold:     global.Config.App.BanThreshold,
		AttemptsWindow:   10 * time.Minute,
		BanDuration:      30 * time.Minute,
	})
	global.LoginLimiter.RegisterProvider(utils.B64StringCaptchaProvider{})
	service.AllService.DataRetentionService.Start()
	if err := service.AllService.GeoIPService.Init(); err != nil {
		global.Logger.Warnf("initialize GeoIP lookup: %v", err)
	}
	if err := service.RecordAuthKeyringStartup(global.Auth); err != nil {
		global.Logger.Fatalf("record authentication keyring audit: %v", err)
	}
}

func DatabaseAutoUpdate() {
	before, after, migrated, _, err := migrateConfiguredDatabase(context.Background(), &global.Config, global.DB)
	if err != nil {
		if before.State == databaseSchema.StateNewerThanBinary || after.State == databaseSchema.StateNewerThanBinary {
			global.Logger.Fatalf("refuse database newer than binary: installed=%s target=%d", schemaPointerLabel(before.InstalledSchema), DatabaseVersion)
		}
		global.Logger.Fatalf("database migration failed: %v", err)
	}
	global.Logger.Infof("database schema ready: installed=%d migrated=%t", DatabaseVersion, migrated)
}

var migrationGlobals sync.Mutex

func Migrate(version uint) error {
	return MigrateDatabase(global.DB, version)
}

func MigrateDatabase(db *gorm.DB, version uint) error {
	if db == nil {
		return errors.New("database is unavailable")
	}
	migrationGlobals.Lock()
	defer migrationGlobals.Unlock()
	previous := global.DB
	global.DB = db
	defer func() { global.DB = previous }()
	return migrateUsingGlobals(version)
}

func migrateUsingGlobals(version uint) error {
	if global.Logger != nil {
		global.Logger.Infof("database migration step=inspect target=%d", version)
	}
	status, err := databaseSchema.InspectSchema(global.DB)
	if err != nil {
		return err
	}
	if status.State == databaseSchema.StateNewerThanBinary || status.State == databaseSchema.StateInvalid {
		return databaseSchema.ErrSchemaMismatch
	}
	initialDatabase := status.InstalledSchema == nil
	legacyVersion := uint(0)
	if status.InstalledSchema != nil {
		legacyVersion = *status.InstalledSchema
	}
	if legacyVersion > 0 && legacyVersion < 245 {
		if err := prepareLegacyOauthIdentityMigration(); err != nil {
			return fmt.Errorf("prepare legacy OAuth identity migration: %w", err)
		}
	}
	if err := validateOauthIdentityUniqueness(); err != nil {
		return err
	}
	if version >= 313 {
		if err := validatePeerIDUniqueness(); err != nil {
			return err
		}
	}
	// Reconcile the compatibility mirror on every v302+ migration. This also
	// recovers databases where an earlier process added the role column but
	// stopped before legacy is_admin rows were promoted.
	migrateLegacyRoles := version >= 302
	err = global.DB.AutoMigrate(
		&model.Version{},
		&model.User{},
		&model.UserToken{},
		&model.Tag{},
		&model.AddressBook{},
		&model.Peer{},
		&model.PeerPresenceLease{},
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
		&model.AdminResourceScope{},
		&model.BrandingSetting{},
		&model.UserTwoFactor{},
		&model.TwoFactorLoginChallenge{},
		&model.SystemSetting{},
	)
	if err != nil {
		return fmt.Errorf("schema migration: %w", err)
	}
	if global.Logger != nil {
		global.Logger.Infof("database migration step=schema target=%d complete", version)
	}
	if legacyVersion > 0 && legacyVersion < 245 {
		var identities []model.UserThird
		if err := global.DB.Where("oauth_type = ?", "google").Find(&identities).Error; err != nil {
			return fmt.Errorf("read legacy Google OAuth identities: %w", err)
		}
		for _, identity := range identities {
			if identity.UserId > 0 {
				if err := global.DB.Model(&model.User{}).Where("id = ?", identity.UserId).Update("email", identity.OpenId).Error; err != nil {
					return fmt.Errorf("migrate legacy Google OAuth email: %w", err)
				}
			}
		}
	}
	if legacyVersion > 0 && legacyVersion < 246 {
		if err := global.DB.Exec("UPDATE oauths SET issuer = 'https://accounts.google.com' WHERE op = 'google' AND issuer IS NULL").Error; err != nil {
			return fmt.Errorf("migrate legacy Google OAuth issuer: %w", err)
		}
	}
	if version >= 305 {
		// Move legacy announcements out of tenant branding without losing an
		// operator's existing message. LinuxDo is no longer a supported OAuth
		// provider; remove provider rows and their bindings atomically.
		if err := global.DB.Transaction(func(tx *gorm.DB) error {
			var legacy model.BrandingSetting
			_ = tx.First(&legacy, 1).Error
			setting := &model.SystemSetting{}
			if err := tx.First(setting, 1).Error; errors.Is(err, gorm.ErrRecordNotFound) {
				setting = &model.SystemSetting{IdModel: model.IdModel{Id: 1}, Announcement: legacy.Announcement, GeoIPEnabled: true, GeoIPCityURL: service.DefaultGeoIPCityURL, GeoIPCountryURL: service.DefaultGeoIPCountryURL, GeoIPASNURL: service.DefaultGeoIPASNURL, GeoIPUpdateHours: 168}
				if err := tx.Create(setting).Error; err != nil {
					return err
				}
			} else if err != nil {
				return err
			} else if setting.Announcement == "" && legacy.Announcement != "" {
				if err := tx.Model(setting).Update("announcement", legacy.Announcement).Error; err != nil {
					return err
				}
			}
			if err := tx.Where("oauth_type = ? OR op = ?", "linuxdo", "linuxdo").Delete(&model.UserThird{}).Error; err != nil {
				return err
			}
			return tx.Where("oauth_type = ? OR op = ?", "linuxdo", "linuxdo").Delete(&model.Oauth{}).Error
		}); err != nil {
			return fmt.Errorf("migrate v305 settings: %w", err)
		}
	}
	if version >= 306 {
		const defaultLoginFooter = `<a href="https://github.com/q1ngyang/rustdesk-api-kessoku" target="_blank" rel="noopener noreferrer"><span>RustDesk API Kessoku</span><span>Github</span></a>`
		if err := global.DB.Transaction(func(tx *gorm.DB) error {
			if err := tx.Model(&model.SystemSetting{}).
				Where("geo_ip_country_url IS NULL OR geo_ip_country_url = ''").
				Update("geo_ip_country_url", service.DefaultGeoIPCountryURL).Error; err != nil {
				return err
			}
			if err := tx.Model(&model.BrandingSetting{}).
				Where("login_kicker = ?", "RustDesk API KESSOKU").
				Update("login_kicker", "RustDesk API\nKESSOKU").Error; err != nil {
				return err
			}
			return tx.Model(&model.BrandingSetting{}).
				Where("login_footer = ?", "RustDesk API Kessoku · v3").
				Update("login_footer", defaultLoginFooter).Error
		}); err != nil {
			return fmt.Errorf("migrate v306 branding and GeoIP settings: %w", err)
		}
	}
	if version >= 307 {
		// A single uploaded asset used to serve both color schemes. Preserve it
		// in both themed slots, then let new clients save them independently.
		if err := global.DB.Transaction(func(tx *gorm.DB) error {
			pairs := [][2]string{
				{"admin_logo_light_url", "admin_logo_url"}, {"admin_logo_dark_url", "admin_logo_url"},
				{"admin_icon_light_url", "admin_icon_url"}, {"admin_icon_dark_url", "admin_icon_url"},
				{"login_logo_light_url", "login_logo_url"}, {"login_logo_dark_url", "login_logo_url"},
				{"web_client_logo_light_url", "web_client_logo_url"}, {"web_client_logo_dark_url", "web_client_logo_url"},
				{"web_client_icon_light_url", "web_client_icon_url"}, {"web_client_icon_dark_url", "web_client_icon_url"},
			}
			for _, pair := range pairs {
				if err := tx.Model(&model.BrandingSetting{}).
					Where(pair[0]+" = '' AND "+pair[1]+" <> ''").
					Update(pair[0], gorm.Expr(pair[1])).Error; err != nil {
					return err
				}
			}
			return nil
		}); err != nil {
			return fmt.Errorf("migrate v307 theme-aware branding: %w", err)
		}
	}
	if version >= 309 {
		// Consolidate the three v307 surface-specific identities into one themed
		// deployment identity. Prefer the administration assets because they were
		// already used by the persistent navigation and About surfaces. Preserve
		// the old columns as a rollback mirror.
		if err := global.DB.Transaction(func(tx *gorm.DB) error {
			setting := &model.BrandingSetting{}
			if err := tx.First(setting, 1).Error; errors.Is(err, gorm.ErrRecordNotFound) {
				return nil
			} else if err != nil {
				return err
			}
			first := func(values ...string) string {
				for _, value := range values {
					if value != "" {
						return value
					}
				}
				return ""
			}
			logoLight := first(setting.BrandLogoLightURL, setting.AdminLogoLightURL, setting.LoginLogoLightURL, setting.WebClientLogoLightURL, setting.AdminLogoURL, setting.LoginLogoURL, setting.WebClientLogoURL)
			logoDark := first(setting.BrandLogoDarkURL, setting.AdminLogoDarkURL, setting.LoginLogoDarkURL, setting.WebClientLogoDarkURL, setting.AdminLogoURL, setting.LoginLogoURL, setting.WebClientLogoURL)
			iconLight := first(setting.BrandIconLightURL, setting.AdminIconLightURL, setting.WebClientIconLightURL, setting.AdminIconURL, setting.WebClientIconURL)
			iconDark := first(setting.BrandIconDarkURL, setting.AdminIconDarkURL, setting.WebClientIconDarkURL, setting.AdminIconURL, setting.WebClientIconURL)
			loginBackgroundLight := first(setting.LoginBackgroundLightURL, setting.LoginBackgroundURL)
			loginBackgroundDark := first(setting.LoginBackgroundDarkURL, setting.LoginBackgroundURL)
			footer := first(setting.FooterHTML, setting.LoginFooter)
			return tx.Model(setting).Updates(map[string]interface{}{
				"brand_logo_light_url": logoLight, "brand_logo_dark_url": logoDark,
				"brand_icon_light_url": iconLight, "brand_icon_dark_url": iconDark,
				"login_background_light_url": loginBackgroundLight, "login_background_dark_url": loginBackgroundDark,
				"footer_html":          footer,
				"admin_logo_light_url": logoLight, "admin_logo_dark_url": logoDark,
				"admin_icon_light_url": iconLight, "admin_icon_dark_url": iconDark,
				"login_logo_light_url": logoLight, "login_logo_dark_url": logoDark,
				"web_client_logo_light_url": logoLight, "web_client_logo_dark_url": logoDark,
				"web_client_icon_light_url": iconLight, "web_client_icon_dark_url": iconDark,
				"login_background_url": loginBackgroundLight, "login_footer": footer,
			}).Error
		}); err != nil {
			return fmt.Errorf("migrate v309 unified branding: %w", err)
		}
	}
	// Normalize legacy authentication generations before any migration uses
	// them as an authority boundary. This ordering matters for direct upgrades
	// from releases that predate auth_version: a zero-valued active token must
	// first become generation 1 before v312 can compare it with its owner.
	if err := global.DB.Exec("UPDATE users SET auth_version = 1 WHERE auth_version IS NULL OR auth_version = 0").Error; err != nil {
		return fmt.Errorf("backfill user auth version: %w", err)
	}
	if err := global.DB.Exec("UPDATE user_tokens SET auth_version = 1 WHERE auth_version IS NULL OR auth_version = 0").Error; err != nil {
		return fmt.Errorf("backfill token auth version: %w", err)
	}
	if version >= 312 {
		// Persist native-client type on the token itself so login-log retention
		// cannot break device ownership recovery. Existing inventory timestamps
		// are backfilled conservatively; empty peers remain at zero and therefore
		// request a full sysinfo upload on their next heartbeat.
		if err := global.DB.Transaction(func(tx *gorm.DB) error {
			if err := tx.Exec(`UPDATE user_tokens
				SET client = COALESCE((
					SELECT login_logs.client FROM login_logs
					WHERE login_logs.user_token_id = user_tokens.id
					ORDER BY login_logs.id DESC LIMIT 1
				), '')
				WHERE client IS NULL OR client = ''`).Error; err != nil {
				return err
			}
			if err := tx.Exec(`UPDATE peers SET identity_source = CASE
				WHEN uuid <> '' AND user_id > 0 THEN ?
				ELSE ? END
				WHERE identity_source IS NULL OR identity_source = ''`, model.PeerIdentitySourceLogin, model.PeerIdentitySourceManual).Error; err != nil {
				return err
			}
			return tx.Exec(`UPDATE peers SET last_sysinfo_time = last_online_time
				WHERE last_sysinfo_time = 0 AND last_online_time > 0
				AND (cpu <> '' OR hostname <> '' OR memory <> '' OR os <> '' OR username <> '' OR version <> '')`).Error
		}); err != nil {
			return fmt.Errorf("migrate v312 device inventory metadata: %w", err)
		}
		if err := backfillActiveNativePeers(); err != nil {
			return fmt.Errorf("migrate v312 active native devices: %w", err)
		}
	}
	if err := global.DB.FirstOrCreate(&model.SecurityInvariantLock{Name: "enabled-admin"}).Error; err != nil {
		return fmt.Errorf("create enabled-administrator invariant lock: %w", err)
	}
	if migrateLegacyRoles {
		if err := global.DB.Exec("UPDATE users SET role = ? WHERE is_admin = ? AND (role IS NULL OR role = '' OR role NOT IN (?, ?))", model.UserRoleSuperAdmin, true, model.UserRoleAdmin, model.UserRoleSuperAdmin).Error; err != nil {
			return fmt.Errorf("migrate legacy administrators to super administrators: %w", err)
		}
		if err := global.DB.Exec("UPDATE users SET role = ? WHERE (is_admin = ? OR is_admin IS NULL) AND (role IS NULL OR role = '' OR role NOT IN (?, ?, ?))", model.UserRoleUser, false, model.UserRoleUser, model.UserRoleAdmin, model.UserRoleSuperAdmin).Error; err != nil {
			return fmt.Errorf("migrate legacy ordinary users: %w", err)
		}
	}
	if err := global.DB.Exec("UPDATE users SET role = ? WHERE role IS NULL OR role = '' OR role NOT IN (?, ?, ?)", model.UserRoleUser, model.UserRoleUser, model.UserRoleAdmin, model.UserRoleSuperAdmin).Error; err != nil {
		return fmt.Errorf("normalize invalid user roles: %w", err)
	}
	// Let each supported database evaluate the role predicate as its native
	// boolean type. Boolean placeholders inside CASE are inferred as text by
	// PostgreSQL when GORM prepares this cross-database statement.
	if err := global.DB.Exec("UPDATE users SET is_admin = (role IN (?, ?))", model.UserRoleAdmin, model.UserRoleSuperAdmin).Error; err != nil {
		return fmt.Errorf("synchronize legacy administrator flag: %w", err)
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
	if initialDatabase {
		if err := global.DB.Transaction(func(tx *gorm.DB) error {
			return bootstrapInitialDatabase(tx, version)
		}); err != nil {
			return err
		}
	} else if err := global.DB.Create(&model.Version{Version: version}).Error; err != nil {
		return fmt.Errorf("record database version: %w", err)
	}
	if global.Logger != nil {
		global.Logger.Infof("database migration step=record-version target=%d complete", version)
	}
	return nil
}

func validatePeerIDUniqueness() error {
	if global.DB == nil || !global.DB.Migrator().HasTable(&model.Peer{}) {
		return nil
	}
	var duplicateGroups int64
	query := global.DB.Table("peers").
		Select("id").
		Group("id").
		Having("COUNT(*) > 1")
	if err := global.DB.Table("(?) AS duplicate_peer_ids", query).Count(&duplicateGroups).Error; err != nil {
		return fmt.Errorf("validate peer identity uniqueness: %w", err)
	}
	if duplicateGroups > 0 {
		return fmt.Errorf("peer identity migration preflight: %d duplicate device ID group(s) require operator resolution", duplicateGroups)
	}
	return nil
}

func bootstrapInitialDatabase(tx *gorm.DB, version uint) error {
	defaultGroupName, shareGroupName := "Default Group", "Share Group"
	if global.Localizer != nil {
		localizer := global.Localizer("")
		if localized, err := localizer.LocalizeMessage(&i18n.Message{ID: "DefaultGroup"}); err == nil && localized != "" {
			defaultGroupName = localized
		}
		if localized, err := localizer.LocalizeMessage(&i18n.Message{ID: "ShareGroup"}); err == nil && localized != "" {
			shareGroupName = localized
		}
	}
	defaultGroup := &model.Group{}
	if err := tx.Where("type = ?", model.GroupTypeDefault).First(defaultGroup).Error; errors.Is(err, gorm.ErrRecordNotFound) {
		defaultGroup = &model.Group{Name: defaultGroupName, Type: model.GroupTypeDefault}
		if err := tx.Create(defaultGroup).Error; err != nil {
			return fmt.Errorf("create default group: %w", err)
		}
	} else if err != nil {
		return fmt.Errorf("read default group: %w", err)
	}
	shareGroup := &model.Group{}
	if err := tx.Where("type = ?", model.GroupTypeShare).First(shareGroup).Error; errors.Is(err, gorm.ErrRecordNotFound) {
		shareGroup = &model.Group{Name: shareGroupName, Type: model.GroupTypeShare}
		if err := tx.Create(shareGroup).Error; err != nil {
			return fmt.Errorf("create share group: %w", err)
		}
	} else if err != nil {
		return fmt.Errorf("read share group: %w", err)
	}
	admin := &model.User{}
	if err := tx.Where("username = ?", "admin").First(admin).Error; errors.Is(err, gorm.ErrRecordNotFound) {
		password := utils.RandomString(32)
		if password == "" {
			return errors.New("generate bootstrap administrator credential")
		}
		hash, err := utils.EncryptPassword(password)
		if err != nil {
			return fmt.Errorf("hash bootstrap administrator credential: %w", err)
		}
		isAdmin := true
		admin = &model.User{
			Username: "admin", Nickname: "Admin", Password: hash,
			Status: model.COMMON_STATUS_ENABLE, Role: model.UserRoleSuperAdmin,
			IsAdmin: &isAdmin, GroupId: defaultGroup.Id, AuthVersion: 1,
		}
		if err := tx.Create(admin).Error; err != nil {
			return fmt.Errorf("create bootstrap administrator: %w", err)
		}
		if global.Logger != nil {
			global.Logger.Info("bootstrap administrator created; set its password with reset-admin-pwd --password-file")
		}
	} else if err != nil {
		return fmt.Errorf("read bootstrap administrator: %w", err)
	}
	if err := tx.Create(&model.Version{Version: version}).Error; err != nil {
		return fmt.Errorf("record database version: %w", err)
	}
	return nil
}

func backfillActiveNativePeers() error {
	if global.DB == nil {
		return errors.New("database is unavailable")
	}
	now := time.Now().Unix()
	var tokens []model.UserToken
	if err := global.DB.Table("user_tokens").
		Select("user_tokens.*").
		Joins("JOIN users ON users.id = user_tokens.user_id").
		Where("user_tokens.client IN ?", []string{model.LoginLogClientNative, model.LoginLogClientApp}).
		Where("user_tokens.device_uuid <> '' AND user_tokens.device_id <> ''").
		Where("user_tokens.revoked_at IS NULL AND user_tokens.expired_at > ?", now).
		Where("users.status = ? AND users.auth_version = user_tokens.auth_version", model.COMMON_STATUS_ENABLE).
		Order("user_tokens.issued_at DESC, user_tokens.id DESC").
		Find(&tokens).Error; err != nil {
		return err
	}
	for i := range tokens {
		deviceID := utils.NormalizeRustDeskID(tokens[i].DeviceId)
		if deviceID == "" {
			continue
		}
		if deviceID != tokens[i].DeviceId {
			if err := global.DB.Model(&model.UserToken{}).Where("id = ?", tokens[i].Id).Update("device_id", deviceID).Error; err != nil {
				return err
			}
		}
		err := global.DB.Transaction(func(tx *gorm.DB) error {
			return bindMigratedNativePeer(tx, deviceID, tokens[i].DeviceUuid, tokens[i].UserId, now)
		})
		if errors.Is(err, service.ErrPeerIdentityConflict) {
			if global.Logger != nil {
				global.Logger.Warnf("skip conflicting active native device during v312 migration: token=%d id=%s", tokens[i].Id, deviceID)
			}
			continue
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func bindMigratedNativePeer(tx *gorm.DB, deviceID, deviceUUID string, userID uint, verifiedAt int64) error {
	var byIDs []model.Peer
	if err := tx.Where("id = ?", deviceID).Limit(2).Find(&byIDs).Error; err != nil {
		return err
	}
	var byUUIDs []model.Peer
	if err := tx.Where("uuid = ?", deviceUUID).Limit(2).Find(&byUUIDs).Error; err != nil {
		return err
	}
	if len(byIDs) > 1 || len(byUUIDs) > 1 {
		return service.ErrPeerIdentityConflict
	}
	var byID, byUUID *model.Peer
	if len(byIDs) == 1 {
		byID = &byIDs[0]
	}
	if len(byUUIDs) == 1 {
		byUUID = &byUUIDs[0]
	}
	updates := map[string]interface{}{
		"identity_source": model.PeerIdentitySourceLogin, "identity_verified_at": verifiedAt,
	}
	if byID != nil {
		if byID.Uuid != "" && byID.Uuid != deviceUUID || byID.UserId != 0 && byID.UserId != userID || byUUID != nil && byUUID.RowId != byID.RowId {
			return service.ErrPeerIdentityConflict
		}
		updates["uuid"] = deviceUUID
		updates["user_id"] = userID
		return tx.Model(byID).Updates(updates).Error
	}
	if byUUID != nil {
		if byUUID.UserId != 0 && byUUID.UserId != userID {
			return service.ErrPeerIdentityConflict
		}
		updates["id"] = deviceID
		updates["user_id"] = userID
		return tx.Model(byUUID).Updates(updates).Error
	}
	return tx.Create(&model.Peer{
		Id: deviceID, Uuid: deviceUUID, UserId: userID,
		IdentitySource: model.PeerIdentitySourceLogin, IdentityVerifiedAt: verifiedAt,
	}).Error
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
