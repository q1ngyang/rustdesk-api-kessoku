package config

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/viper"
)

const (
	DebugMode     = "debug"
	ReleaseMode   = "release"
	DefaultConfig = "conf/config.yaml"
)

type App struct {
	// LegacyWebClient is parsed only to fail closed on obsolete deployments.
	// It never enables static Web Client routes.
	LegacyWebClient  int           `mapstructure:"web-client"`
	Register         bool          `mapstructure:"register"`
	RegisterStatus   int           `mapstructure:"register-status"`
	ShowSwagger      int           `mapstructure:"show-swagger"`
	TokenExpire      time.Duration `mapstructure:"token-expire"`
	WebSso           bool          `mapstructure:"web-sso"`
	DisablePwdLogin  bool          `mapstructure:"disable-pwd-login"`
	CaptchaThreshold int           `mapstructure:"captcha-threshold"`
	BanThreshold     int           `mapstructure:"ban-threshold"`
}
type Admin struct {
	Title           string `mapstructure:"title"`
	Hello           string `mapstructure:"hello"`
	HelloFile       string `mapstructure:"hello-file"`
	IdServerPort    int    `mapstructure:"id-server-port"`
	RelayServerPort int    `mapstructure:"relay-server-port"`
}
type Config struct {
	Lang          string `mapstructure:"lang"`
	App           App
	Admin         Admin
	Gorm          Gorm
	Mysql         Mysql
	Postgresql    Postgresql
	Gin           Gin
	Logger        Logger
	Redis         Redis
	Cache         Cache
	Oss           Oss
	Auth          Auth `mapstructure:"auth"`
	Rustdesk      Rustdesk
	Proxy         Proxy
	Ldap          Ldap
	ServerControl ServerControl `mapstructure:"server-control"`
	WebClient     WebClient     `mapstructure:"web-client"`
	Media         Media         `mapstructure:"media"`
	TwoFactor     TwoFactor     `mapstructure:"two-factor"`
	// DeprecatedWebClientProvider exists only to make every legacy root
	// web-client-provider block fail closed instead of being silently ignored.
	DeprecatedWebClientProvider map[string]interface{} `mapstructure:"web-client-provider"`
}

func (c Config) Validate() error {
	if c.App.LegacyWebClient != 0 {
		return errors.New("app.web-client is removed; configure web-client.mode instead")
	}
	if c.DeprecatedWebClientProvider != nil {
		return errors.New("web-client-provider is removed; delete it and configure web-client.mode instead")
	}
	if err := c.Auth.Validate(); err != nil {
		return err
	}
	if err := c.WebClient.Validate(c.Auth); err != nil {
		return err
	}
	if err := c.Ldap.Validate(); err != nil {
		return err
	}
	if err := c.Media.Validate(); err != nil {
		return err
	}
	if err := c.TwoFactor.Validate(); err != nil {
		return err
	}
	if err := c.Logger.Validate(); err != nil {
		return err
	}
	if c.Proxy.Enable {
		return errors.New("proxy.enable is not supported for OAuth/OIDC because a proxy can bypass destination address validation")
	}
	if err := c.validateDatabaseTransport(); err != nil {
		return err
	}
	return c.ServerControl.Validate()
}

func (a *Admin) Init() {
	if a.IdServerPort == 0 {
		a.IdServerPort = DefaultIdServerPort
	}
	if a.RelayServerPort == 0 {
		a.RelayServerPort = DefaultRelayServerPort
	}
}

func newViper(path string) *viper.Viper {
	if path == "" {
		path = DefaultConfig
	}
	// A Relay identifier is an exact host:port string and commonly contains
	// dots. Viper's default "." key delimiter would otherwise reinterpret a
	// YAML key such as "relay.example.com:21117" as nested maps during
	// Unmarshal. Use a delimiter that cannot occur in validated configuration
	// keys while preserving the existing environment-variable contract.
	v := viper.NewWithOptions(viper.KeyDelimiter("::"))
	v.AutomaticEnv()
	v.SetEnvKeyReplacer(strings.NewReplacer("::", "_", ".", "_", "-", "_"))
	v.SetEnvPrefix("RUSTDESK_API")
	v.SetDefault("server-control::read-only", true)
	v.SetDefault("server-control::legacy-command-enabled", false)
	v.SetDefault("server-control::log-directory", "")
	v.SetDefault("server-control::registry-directory", "./data/server-control")
	v.SetDefault("server-control::host-identity-file", "")
	v.SetDefault("server-control::pairing::enabled", false)
	v.SetDefault("server-control::pairing::code-ttl", 10*time.Minute)
	v.SetDefault("server-control::pairing::recovery-ttl", 10*time.Minute)
	v.SetDefault("web-client::mode", WebClientDisabled)
	v.SetDefault("web-client::connection-token-ttl", defaultConnectionTokenTTL)
	v.SetDefault("media::directory", "./data/media")
	v.SetDefault("media::max-image-bytes", int64(1<<20))
	v.SetDefault("two-factor::enabled", true)
	v.SetDefault("two-factor::issuer", "RustDesk API Kessoku")
	v.SetDefault("two-factor::key-file", "./data/totp.key")
	v.SetDefault("two-factor::challenge-ttl", 5*time.Minute)
	v.SetDefault("logger::max-size-mb", 20)
	v.SetDefault("logger::max-backups", 5)
	v.SetDefault("logger::max-age-days", 14)
	v.SetDefault("logger::compress", true)
	v.SetDefault("logger::local-time", true)
	v.SetConfigFile(path)
	v.SetConfigType("yaml")
	return v
}

// Load reads and validates configuration without loading referenced key
// material or changing the filesystem. Runtime startup and local maintenance
// commands build their command-specific initialization stages on this common
// parser.
func Load(rowVal *Config, path string) (*viper.Viper, error) {
	if rowVal == nil {
		return nil, errors.New("configuration destination is required")
	}
	v := newViper(path)
	if legacyWebClientProviderEnvironmentPresent(os.Environ()) {
		return nil, errors.New("web-client-provider environment variables are removed; delete them and configure web-client instead")
	}
	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("read config file: %w", err)
	}
	/*
		v.WatchConfig()


			//监听配置修改没什么必要
			v.OnConfigChange(func(e fsnotify.Event) {
				//配置文件修改监听
				fmt.Println("config file changed:", e.Name)
				if err2 := v.Unmarshal(rowVal); err2 != nil {
					fmt.Println(err2)
				}
				rowVal.Rustdesk.LoadKeyFile()
				rowVal.Rustdesk.ParsePort()
			})
	*/
	if err := v.Unmarshal(rowVal); err != nil {
		return nil, fmt.Errorf("decode config: %w", err)
	}
	rowVal.Admin.Init()
	rowVal.Media.Init()
	if err := rowVal.Validate(); err != nil {
		return nil, fmt.Errorf("validate config: %w", err)
	}
	if err := rowVal.ValidateRuntime(); err != nil {
		return nil, fmt.Errorf("validate runtime config: %w", err)
	}
	return v, nil
}

// Init preserves the historical startup API. New command paths should use
// Load so ordinary validation errors can be returned instead of panicking.
func Init(rowVal *Config, path string) *viper.Viper {
	v, err := Load(rowVal, path)
	if err != nil {
		panic(fmt.Errorf("Fatal error config: %w", err))
	}
	rowVal.Rustdesk.LoadKeyFile()
	return v
}

func legacyWebClientProviderEnvironmentPresent(environment []string) bool {
	const removedRoot = "RUSTDESK_API_WEB_CLIENT_PROVIDER"
	for _, entry := range environment {
		name, _, _ := strings.Cut(entry, "=")
		name = strings.ToUpper(name)
		if name == removedRoot || strings.HasPrefix(name, removedRoot+"_") {
			return true
		}
	}
	return false
}

// ReadEnv 读取环境变量
func ReadEnv(rowVal interface{}) *viper.Viper {
	v := viper.New()
	v.AutomaticEnv()
	if err := v.Unmarshal(rowVal); err != nil {
		fmt.Println(err)
	}
	return v
}
