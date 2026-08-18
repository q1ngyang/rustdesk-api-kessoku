package config

import "time"

// ServerControl configures Kessoku's versioned, typed Starry control plane.
// Zero values are intentionally secure: no instances, no write access, and no
// legacy command routes.
type ServerControl struct {
	LegacyCommandEnabled bool             `mapstructure:"legacy-command-enabled"`
	ReadOnly             bool             `mapstructure:"read-only"`
	RequestTimeout       time.Duration    `mapstructure:"request-timeout"`
	ResponseMaxBytes     int64            `mapstructure:"response-max-bytes"`
	Instances            []StarryInstance `mapstructure:"instances"`
}

// StarryInstance is deployment-owned. Agent URLs and credential paths are not
// accepted from browser requests, which prevents the provider from becoming an
// SSRF proxy.
type StarryInstance struct {
	ID                 string `mapstructure:"id"`
	Name               string `mapstructure:"name"`
	Enabled            bool   `mapstructure:"enabled"`
	BaseURL            string `mapstructure:"base-url"`
	ExpectedInstanceID string `mapstructure:"expected-instance-id"`
	TLSServerName      string `mapstructure:"tls-server-name"`
	CAFile             string `mapstructure:"ca-file"`
	ClientCertFile     string `mapstructure:"client-cert-file"`
	ClientKeyFile      string `mapstructure:"client-key-file"`
	ControlKeyFile     string `mapstructure:"control-key-file"`
	ControlKeyID       string `mapstructure:"control-key-id"`
	ControlIssuer      string `mapstructure:"control-issuer"`
	AuthorizedParty    string `mapstructure:"authorized-party"`
	AllowedCertificate string `mapstructure:"allowed-certificate-identity"`
}

func (c ServerControl) Timeout() time.Duration {
	if c.RequestTimeout <= 0 {
		return 5 * time.Second
	}
	return c.RequestTimeout
}

func (c ServerControl) MaxResponseBytes() int64 {
	if c.ResponseMaxBytes <= 0 {
		return 1 << 20
	}
	return c.ResponseMaxBytes
}
