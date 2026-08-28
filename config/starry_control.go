package config

import (
	"errors"
	"fmt"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
)

const MaxStarryControlPayloadBytes int64 = 4 << 20

// ServerControl configures Kessoku's versioned, typed Starry control plane.
// Runtime configuration defaults are intentionally secure: no instances, no
// write access, and no legacy command routes.
type ServerControl struct {
	LegacyCommandEnabled bool               `mapstructure:"legacy-command-enabled"`
	ReadOnly             bool               `mapstructure:"read-only"`
	RequestTimeout       time.Duration      `mapstructure:"request-timeout"`
	ResponseMaxBytes     int64              `mapstructure:"response-max-bytes"`
	Instances            []StarryInstance   `mapstructure:"instances"`
	LogDirectory         string             `mapstructure:"log-directory"`
	LogSources           []ControlLogSource `mapstructure:"log-sources"`
}

type ControlLogSource struct {
	ID         string `mapstructure:"id" json:"id"`
	Label      string `mapstructure:"label" json:"label"`
	Component  string `mapstructure:"component" json:"component"`
	InstanceID string `mapstructure:"instance-id" json:"instance_id"`
	File       string `mapstructure:"file" json:"-"`
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

func (c ServerControl) Validate() error {
	if c.RequestTimeout < 0 || c.RequestTimeout > 30*time.Second {
		return errors.New("server-control request-timeout must be between zero and 30 seconds")
	}
	if c.ResponseMaxBytes < 0 || c.ResponseMaxBytes > MaxStarryControlPayloadBytes {
		return fmt.Errorf("server-control response-max-bytes must not exceed %d", MaxStarryControlPayloadBytes)
	}
	seen := make(map[string]struct{}, len(c.Instances))
	for index, instance := range c.Instances {
		if !validProviderID(instance.ID) {
			return fmt.Errorf("server-control instance %d requires a valid deployment id", index)
		}
		if _, duplicate := seen[instance.ID]; duplicate {
			return fmt.Errorf("duplicate server-control instance id %q", instance.ID)
		}
		seen[instance.ID] = struct{}{}
		if !validGovernanceText(instance.Name, 256) {
			return fmt.Errorf("server-control instance %q requires a valid name", instance.ID)
		}
		if !instance.Enabled {
			continue
		}
		if err := validateControlOrigin(instance.BaseURL); err != nil {
			return fmt.Errorf("server-control instance %q base-url: %w", instance.ID, err)
		}
		if _, err := uuid.Parse(instance.ExpectedInstanceID); err != nil {
			return fmt.Errorf("server-control instance %q expected-instance-id must be a UUID", instance.ID)
		}
		if !validGovernanceText(instance.TLSServerName, 253) || strings.ContainsAny(instance.TLSServerName, "/:@") {
			return fmt.Errorf("server-control instance %q requires a valid TLS server name", instance.ID)
		}
		for field, value := range map[string]string{
			"ca-file":          instance.CAFile,
			"client-cert-file": instance.ClientCertFile,
			"client-key-file":  instance.ClientKeyFile,
			"control-key-file": instance.ControlKeyFile,
		} {
			if strings.TrimSpace(value) == "" || strings.TrimSpace(value) != value {
				return fmt.Errorf("server-control instance %q requires %s", instance.ID, field)
			}
		}
		if !validHeaderIdentifier(instance.ControlKeyID, 128) {
			return fmt.Errorf("server-control instance %q requires a valid control-key-id", instance.ID)
		}
		if _, err := fixedHTTPSURL(instance.ControlIssuer, false); err != nil {
			return fmt.Errorf("server-control instance %q control-issuer: %w", instance.ID, err)
		}
		if !validControlIdentityURI(instance.AuthorizedParty) {
			return fmt.Errorf("server-control instance %q authorized-party must be the client certificate URI SAN", instance.ID)
		}
	}
	if len(c.LogSources) > 0 && strings.TrimSpace(c.LogDirectory) == "" {
		return errors.New("server-control log-directory is required when log-sources are configured")
	}
	if len(c.LogSources) > 0 && (!filepath.IsAbs(c.LogDirectory) || strings.TrimSpace(c.LogDirectory) != c.LogDirectory) {
		return errors.New("server-control log-directory must be an absolute path without surrounding whitespace")
	}
	logIDs := make(map[string]struct{}, len(c.LogSources))
	for index, source := range c.LogSources {
		if !validProviderID(source.ID) {
			return fmt.Errorf("server-control log source %d requires a valid id", index)
		}
		if _, exists := logIDs[source.ID]; exists {
			return fmt.Errorf("duplicate server-control log source id %q", source.ID)
		}
		logIDs[source.ID] = struct{}{}
		if !validGovernanceText(source.Label, 120) {
			return fmt.Errorf("server-control log source %q requires a valid label", source.ID)
		}
		switch source.Component {
		case "kessoku", "starry", "relay", "control-agent":
		default:
			return fmt.Errorf("server-control log source %q has invalid component", source.ID)
		}
		if filepath.Base(source.File) != source.File || source.File == "." || strings.TrimSpace(source.File) != source.File {
			return fmt.Errorf("server-control log source %q file must be a simple filename", source.ID)
		}
		if source.InstanceID != "" && source.InstanceID != "*" {
			if _, exists := seen[source.InstanceID]; !exists {
				return fmt.Errorf("server-control log source %q references an unknown instance", source.ID)
			}
		}
	}
	return nil
}

func validControlIdentityURI(value string) bool {
	if len(value) > 256 || strings.TrimSpace(value) != value {
		return false
	}
	parsed, err := url.Parse(value)
	return err == nil && parsed.IsAbs() && parsed.Host != "" && parsed.User == nil && parsed.RawQuery == "" && parsed.Fragment == ""
}

func validateControlOrigin(value string) error {
	parsed, err := url.Parse(value)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil || parsed.Opaque != "" {
		return errors.New("must be an absolute HTTPS origin without credentials")
	}
	if parsed.RawQuery != "" || parsed.ForceQuery || parsed.Fragment != "" || parsed.RawPath != "" || parsed.Path != "" && parsed.Path != "/" {
		return errors.New("must not contain a path, query, or fragment")
	}
	return nil
}

func validHeaderIdentifier(value string, maximum int) bool {
	if value == "" || len(value) > maximum || strings.TrimSpace(value) != value {
		return false
	}
	for _, character := range value {
		if character < 0x21 || character > 0x7e {
			return false
		}
	}
	return true
}
