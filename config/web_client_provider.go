package config

import (
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	WebClientProviderDisabled = "disabled"
	WebClientProviderExternal = "external"
)

// WebClientProvider configures a separately hosted, independently governed
// browser client. Kessoku never fetches, proxies, or injects credentials into
// this origin.
type WebClientProvider struct {
	Mode                string                    `mapstructure:"mode"`
	AuthorizationRecord string                    `mapstructure:"authorization-record"`
	Manifest            WebClientProviderManifest `mapstructure:"manifest"`
}

// WebClientProviderManifest is the complete public provider descriptor. Keep
// governance notes and any deployment-only values outside this type so they
// cannot accidentally enter API responses.
type WebClientProviderManifest struct {
	ID            string `json:"id" mapstructure:"id"`
	Name          string `json:"name" mapstructure:"name"`
	LaunchURL     string `json:"launch_url" mapstructure:"launch-url"`
	AllowedOrigin string `json:"allowed_origin" mapstructure:"allowed-origin"`
	License       string `json:"license" mapstructure:"license"`
	SourceURL     string `json:"source_url" mapstructure:"source-url"`
	Version       string `json:"version" mapstructure:"version"`
	Digest        string `json:"digest" mapstructure:"digest"`
}

func (p WebClientProvider) EffectiveMode() string {
	if p.Mode == "" {
		return WebClientProviderDisabled
	}
	return p.Mode
}

func (p WebClientProvider) Enabled() bool {
	return p.EffectiveMode() == WebClientProviderExternal
}

func (p WebClientProvider) Validate() error {
	switch p.EffectiveMode() {
	case WebClientProviderDisabled:
		return nil
	case WebClientProviderExternal:
	default:
		return fmt.Errorf("web-client-provider mode must be %q or %q", WebClientProviderDisabled, WebClientProviderExternal)
	}

	if !validProviderID(p.Manifest.ID) {
		return errors.New("external web-client provider requires a valid id")
	}
	for field, value := range map[string]string{
		"name":                 p.Manifest.Name,
		"license":              p.Manifest.License,
		"version":              p.Manifest.Version,
		"authorization-record": p.AuthorizationRecord,
	} {
		maximum := 256
		if field == "authorization-record" {
			maximum = 2048
		}
		if !validGovernanceText(value, maximum) {
			return fmt.Errorf("external web-client provider requires valid %s", field)
		}
	}

	launch, err := fixedHTTPSURL(p.Manifest.LaunchURL, false)
	if err != nil {
		return fmt.Errorf("invalid external web-client launch-url: %w", err)
	}
	origin, err := fixedHTTPSURL(p.Manifest.AllowedOrigin, true)
	if err != nil {
		return fmt.Errorf("invalid external web-client allowed-origin: %w", err)
	}
	if canonicalOrigin(launch) != canonicalOrigin(origin) {
		return errors.New("external web-client launch-url must use allowed-origin")
	}
	if _, err := fixedHTTPSURL(p.Manifest.SourceURL, false); err != nil {
		return fmt.Errorf("invalid external web-client source-url: %w", err)
	}
	if !validSHA256Digest(p.Manifest.Digest) {
		return errors.New("external web-client digest must be lowercase sha256:<64 hex characters>")
	}
	return nil
}

func validProviderID(value string) bool {
	if value == "" || len(value) > 64 || strings.TrimSpace(value) != value {
		return false
	}
	for index, character := range value {
		if character >= 'a' && character <= 'z' || character >= '0' && character <= '9' || index > 0 && strings.ContainsRune("._-", character) {
			continue
		}
		return false
	}
	return true
}

func validGovernanceText(value string, maximum int) bool {
	if value == "" || len(value) > maximum || strings.TrimSpace(value) != value || !utf8.ValidString(value) {
		return false
	}
	for _, character := range value {
		if unicode.IsControl(character) {
			return false
		}
	}
	return true
}

func fixedHTTPSURL(value string, originOnly bool) (*url.URL, error) {
	if value == "" || strings.TrimSpace(value) != value || len(value) > 2048 {
		return nil, errors.New("URL is required")
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil || parsed.Opaque != "" {
		return nil, errors.New("URL must be an absolute HTTPS URL without credentials")
	}
	if parsed.RawQuery != "" || parsed.ForceQuery || parsed.Fragment != "" {
		return nil, errors.New("URL must not contain a query or fragment")
	}
	if originOnly && (parsed.Path != "" || parsed.RawPath != "") {
		return nil, errors.New("origin must not contain a path")
	}
	return parsed, nil
}

func canonicalOrigin(value *url.URL) string {
	return strings.ToLower(value.Scheme + "://" + value.Host)
}

func validSHA256Digest(value string) bool {
	const prefix = "sha256:"
	if !strings.HasPrefix(value, prefix) || len(value) != len(prefix)+64 || value != strings.ToLower(value) {
		return false
	}
	_, err := hex.DecodeString(strings.TrimPrefix(value, prefix))
	return err == nil
}
