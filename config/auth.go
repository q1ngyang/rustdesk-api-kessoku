package config

import "time"

// Auth configures the connection/API access-token profile. Ed25519 private
// material is loaded only from a file and is never represented in YAML values,
// database rows, or API responses.
type Auth struct {
	Enabled                bool          `mapstructure:"enabled"`
	Issuer                 string        `mapstructure:"issuer"`
	Audiences              []string      `mapstructure:"audiences"`
	AccessTokenTTL         time.Duration `mapstructure:"access-token-ttl"`
	MaximumTokenTTL        time.Duration `mapstructure:"maximum-token-ttl"`
	ClockSkew              time.Duration `mapstructure:"clock-skew"`
	MaxTokenBytes          int           `mapstructure:"max-token-bytes"`
	LegacyTokenReadEnabled bool          `mapstructure:"legacy-token-read-enabled"`
	CurrentKey             AuthKey       `mapstructure:"current-key"`
	PreviousKeys           []AuthKey     `mapstructure:"previous-keys"`
	Internal               InternalAuth  `mapstructure:"internal"`
}

type AuthKey struct {
	ID             string `mapstructure:"id"`
	PrivateKeyFile string `mapstructure:"private-key-file"`
	PublicKeyFile  string `mapstructure:"public-key-file"`
}

// InternalAuth is served on a dedicated TLS listener. It is intentionally not
// inherited from the public API listener or reverse-proxy headers.
type InternalAuth struct {
	Enabled           bool          `mapstructure:"enabled"`
	Listen            string        `mapstructure:"listen"`
	ServerCertFile    string        `mapstructure:"server-cert-file"`
	ServerKeyFile     string        `mapstructure:"server-key-file"`
	ClientCAFile      string        `mapstructure:"client-ca-file"`
	AllowedURISANs    []string      `mapstructure:"allowed-uri-sans"`
	AllowedDNSSANs    []string      `mapstructure:"allowed-dns-sans"`
	MaxBodyBytes      int64         `mapstructure:"max-body-bytes"`
	RequestTimeout    time.Duration `mapstructure:"request-timeout"`
	GlobalRequestsPS  int           `mapstructure:"global-requests-per-second"`
	PerCertRequestsPS int           `mapstructure:"per-cert-requests-per-second"`
}

func (a Auth) EffectiveAccessTokenTTL() time.Duration {
	if a.AccessTokenTTL <= 0 {
		return 15 * time.Minute
	}
	return a.AccessTokenTTL
}

func (a Auth) EffectiveMaximumTokenTTL() time.Duration {
	if a.MaximumTokenTTL <= 0 {
		return time.Hour
	}
	return a.MaximumTokenTTL
}

func (a Auth) EffectiveClockSkew() time.Duration {
	if a.ClockSkew < 0 {
		return 0
	}
	if a.ClockSkew == 0 {
		return 30 * time.Second
	}
	return a.ClockSkew
}

func (a Auth) EffectiveMaxTokenBytes() int {
	if a.MaxTokenBytes <= 0 {
		return 8 << 10
	}
	return a.MaxTokenBytes
}

func (a InternalAuth) EffectiveMaxBodyBytes() int64 {
	if a.MaxBodyBytes <= 0 || a.MaxBodyBytes > 1<<20 {
		return 1 << 20
	}
	return a.MaxBodyBytes
}

func (a InternalAuth) EffectiveRequestTimeout() time.Duration {
	if a.RequestTimeout <= 0 {
		return 2 * time.Second
	}
	return a.RequestTimeout
}
