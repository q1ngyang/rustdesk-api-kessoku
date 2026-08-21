package config

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const MaxAccessTokenBytes = 8 << 10

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
		return 7 * 24 * time.Hour
	}
	return a.AccessTokenTTL
}

func (a Auth) EffectiveMaximumTokenTTL() time.Duration {
	if a.MaximumTokenTTL <= 0 {
		return 7 * 24 * time.Hour
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
	if a.MaxTokenBytes <= 0 || a.MaxTokenBytes > MaxAccessTokenBytes {
		return MaxAccessTokenBytes
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

func (a InternalAuth) EffectiveGlobalRequestsPS() int {
	if a.GlobalRequestsPS <= 0 {
		return 200
	}
	return a.GlobalRequestsPS
}

func (a InternalAuth) EffectivePerCertRequestsPS() int {
	if a.PerCertRequestsPS <= 0 {
		return 100
	}
	return a.PerCertRequestsPS
}

func (a Auth) Validate() error {
	if !a.Enabled {
		if a.Internal.Enabled {
			return errors.New("auth.internal cannot be enabled while auth is disabled")
		}
		return nil
	}
	if _, err := fixedHTTPSURL(a.Issuer, false); err != nil {
		return fmt.Errorf("auth issuer: %w", err)
	}
	seenAudiences := make(map[string]struct{}, len(a.Audiences))
	for _, audience := range a.Audiences {
		if !validHeaderIdentifier(audience, 256) {
			return errors.New("auth audiences contain an invalid value")
		}
		if _, duplicate := seenAudiences[audience]; duplicate {
			return fmt.Errorf("duplicate auth audience %q", audience)
		}
		seenAudiences[audience] = struct{}{}
	}
	for _, required := range []string{"kessoku-api", "rustdesk-connect"} {
		if _, exists := seenAudiences[required]; !exists {
			return fmt.Errorf("auth audiences must include %q", required)
		}
	}
	if a.AccessTokenTTL < 0 || a.MaximumTokenTTL < 0 || a.EffectiveAccessTokenTTL() > a.EffectiveMaximumTokenTTL() {
		return errors.New("auth access-token-ttl must be positive and not exceed maximum-token-ttl")
	}
	if a.ClockSkew < 0 || a.ClockSkew > 5*time.Minute {
		return errors.New("auth clock-skew must be between zero and five minutes")
	}
	if a.MaxTokenBytes < 0 || a.MaxTokenBytes > MaxAccessTokenBytes {
		return fmt.Errorf("auth max-token-bytes must not exceed %d", MaxAccessTokenBytes)
	}
	if !validHeaderIdentifier(a.CurrentKey.ID, 128) || !validFileReference(a.CurrentKey.PrivateKeyFile) {
		return errors.New("auth current key requires a valid id and private-key-file")
	}
	seenKeys := map[string]struct{}{a.CurrentKey.ID: {}}
	for _, previous := range a.PreviousKeys {
		if !validHeaderIdentifier(previous.ID, 128) || !validFileReference(previous.PublicKeyFile) {
			return errors.New("each previous auth key requires a valid id and public-key-file")
		}
		if _, duplicate := seenKeys[previous.ID]; duplicate {
			return fmt.Errorf("duplicate auth key id %q", previous.ID)
		}
		seenKeys[previous.ID] = struct{}{}
	}
	return a.Internal.validateEnabled()
}

func (a InternalAuth) validateEnabled() error {
	if !a.Enabled {
		return nil
	}
	host, port, err := net.SplitHostPort(a.Listen)
	if err != nil || host == "" {
		return errors.New("auth.internal listen must be an explicit host and port")
	}
	portNumber, err := strconv.ParseUint(port, 10, 16)
	if err != nil || portNumber == 0 {
		return errors.New("auth.internal listen port must be between 1 and 65535")
	}
	for field, value := range map[string]string{
		"server-cert-file": a.ServerCertFile,
		"server-key-file":  a.ServerKeyFile,
		"client-ca-file":   a.ClientCAFile,
	} {
		if !validFileReference(value) {
			return fmt.Errorf("auth.internal requires a valid %s", field)
		}
	}
	if len(a.AllowedURISANs) == 0 && len(a.AllowedDNSSANs) == 0 {
		return errors.New("auth.internal requires at least one exact allowed client SAN")
	}
	seenSANs := make(map[string]struct{}, len(a.AllowedURISANs)+len(a.AllowedDNSSANs))
	for _, value := range a.AllowedURISANs {
		parsed, err := url.Parse(value)
		if err != nil || !parsed.IsAbs() || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" || strings.TrimSpace(value) != value {
			return fmt.Errorf("auth.internal URI SAN %q is invalid", value)
		}
		key := "uri:" + value
		if _, duplicate := seenSANs[key]; duplicate {
			return fmt.Errorf("duplicate auth.internal URI SAN %q", value)
		}
		seenSANs[key] = struct{}{}
	}
	for _, value := range a.AllowedDNSSANs {
		if !validDNSName(value) {
			return fmt.Errorf("auth.internal DNS SAN %q is invalid", value)
		}
		key := "dns:" + value
		if _, duplicate := seenSANs[key]; duplicate {
			return fmt.Errorf("duplicate auth.internal DNS SAN %q", value)
		}
		seenSANs[key] = struct{}{}
	}
	if a.MaxBodyBytes < 0 || a.MaxBodyBytes > 1<<20 {
		return errors.New("auth.internal max-body-bytes must not exceed 1048576")
	}
	if a.RequestTimeout < 0 || a.RequestTimeout > 10*time.Second {
		return errors.New("auth.internal request-timeout must be between zero and ten seconds")
	}
	if a.GlobalRequestsPS < 0 || a.PerCertRequestsPS < 0 {
		return errors.New("auth.internal request rate limits cannot be negative")
	}
	return nil
}

func validFileReference(value string) bool {
	return validGovernanceText(value, 4096)
}

func validDNSName(value string) bool {
	if value == "" || len(value) > 253 || strings.TrimSpace(value) != value || strings.HasSuffix(value, ".") || net.ParseIP(value) != nil {
		return false
	}
	for _, label := range strings.Split(value, ".") {
		if label == "" || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
		for _, character := range label {
			if character >= 'a' && character <= 'z' || character >= 'A' && character <= 'Z' || character >= '0' && character <= '9' || character == '-' {
				continue
			}
			return false
		}
	}
	return true
}
