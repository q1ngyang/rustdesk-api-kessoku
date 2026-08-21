package config

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	WebClientDisabled          = "disabled"
	WebClientBuiltin           = "builtin"
	WebClientSchemaVersion     = 1
	defaultConnectionTokenTTL  = 15 * time.Minute
	maximumConnectionTokenTTL  = time.Hour
	maximumWebClientGeneration = uint64(1<<53 - 1)
	webClientRendezvousWSSPath = "/ws/id"
	webClientRelayWSSPath      = "/ws/relay"
)

// WebClient configures the repository-owned browser client on a listener and
// HTTPS origin that are deliberately distinct from the API/admin origin.
type WebClient struct {
	Mode               string            `mapstructure:"mode"`
	Listen             string            `mapstructure:"listen"`
	PublicOrigin       string            `mapstructure:"public-origin"`
	APIOrigin          string            `mapstructure:"api-origin"`
	RendezvousWSSURL   string            `mapstructure:"rendezvous-wss-url"`
	RelayWSSURLs       map[string]string `mapstructure:"relay-wss-urls"`
	ServerPublicKey    string            `mapstructure:"server-public-key"`
	ProfileGeneration  uint64            `mapstructure:"profile-generation"`
	ConnectionTokenTTL time.Duration     `mapstructure:"connection-token-ttl"`
}

type WebClientPublicConfig struct {
	SchemaVersion        int               `json:"schema_version"`
	ProfileGeneration    uint64            `json:"profile_generation"`
	APIOrigin            string            `json:"api_origin"`
	RendezvousWSSURL     string            `json:"rendezvous_wss_url"`
	RelayWSSURLs         map[string]string `json:"relay_wss_urls"`
	ServerPublicKey      string            `json:"server_public_key"`
	ServerKeyFingerprint string            `json:"server_key_fingerprint"`
}

func (w WebClient) EffectiveMode() string {
	if w.Mode == "" {
		return WebClientDisabled
	}
	return w.Mode
}

func (w WebClient) Enabled() bool {
	return w.EffectiveMode() == WebClientBuiltin
}

func (w WebClient) EffectiveConnectionTokenTTL() time.Duration {
	if w.ConnectionTokenTTL <= 0 {
		return defaultConnectionTokenTTL
	}
	return w.ConnectionTokenTTL
}

func (w WebClient) Validate(auth Auth) error {
	switch w.EffectiveMode() {
	case WebClientDisabled:
		return nil
	case WebClientBuiltin:
	default:
		return fmt.Errorf("web-client mode must be %q or %q", WebClientDisabled, WebClientBuiltin)
	}
	if !auth.Enabled {
		return errors.New("web-client builtin mode requires auth.enabled")
	}
	if err := validExplicitListen(w.Listen); err != nil {
		return fmt.Errorf("web-client listen: %w", err)
	}
	publicOrigin, err := fixedHTTPSURL(w.PublicOrigin, true)
	if err != nil {
		return fmt.Errorf("web-client public-origin: %w", err)
	}
	apiOrigin, err := fixedHTTPSURL(w.APIOrigin, true)
	if err != nil {
		return fmt.Errorf("web-client api-origin: %w", err)
	}
	if canonicalOrigin(publicOrigin) == canonicalOrigin(apiOrigin) {
		return errors.New("web-client public-origin must be different from api-origin")
	}
	if w.PublicOrigin != canonicalOrigin(publicOrigin) || w.APIOrigin != canonicalOrigin(apiOrigin) {
		return errors.New("web-client public-origin and api-origin must use canonical lowercase origins without a default port")
	}
	if _, err := fixedWSSURL(w.RendezvousWSSURL, webClientRendezvousWSSPath); err != nil {
		return fmt.Errorf("web-client rendezvous-wss-url: %w", err)
	}
	if len(w.RelayWSSURLs) == 0 || len(w.RelayWSSURLs) > 64 {
		return errors.New("web-client relay-wss-urls must contain between 1 and 64 exact mappings")
	}
	for relayName, relayURL := range w.RelayWSSURLs {
		if !validGovernanceText(relayName, 255) {
			return errors.New("web-client relay-wss-urls contains an invalid relay name")
		}
		if _, err := fixedWSSURL(relayURL, webClientRelayWSSPath); err != nil {
			return fmt.Errorf("web-client relay-wss-urls[%q]: %w", relayName, err)
		}
	}
	if _, err := decodeWebClientPublicKey(w.ServerPublicKey); err != nil {
		return fmt.Errorf("web-client server-public-key: %w", err)
	}
	if w.ProfileGeneration == 0 || w.ProfileGeneration > maximumWebClientGeneration {
		return errors.New("web-client profile-generation must be a positive JavaScript-safe integer")
	}
	ttl := w.EffectiveConnectionTokenTTL()
	if w.ConnectionTokenTTL < 0 || ttl <= 0 || ttl > maximumConnectionTokenTTL {
		return errors.New("web-client connection-token-ttl must be positive and not exceed one hour")
	}
	if ttl > auth.EffectiveMaximumTokenTTL() {
		return errors.New("web-client connection-token-ttl must not exceed auth.maximum-token-ttl")
	}
	return nil
}

func (w WebClient) PublicConfig() WebClientPublicConfig {
	relays := make(map[string]string, len(w.RelayWSSURLs))
	for name, endpoint := range w.RelayWSSURLs {
		relays[name] = endpoint
	}
	key, _ := decodeWebClientPublicKey(w.ServerPublicKey)
	digest := sha256.Sum256(key)
	return WebClientPublicConfig{
		SchemaVersion:        WebClientSchemaVersion,
		ProfileGeneration:    w.ProfileGeneration,
		APIOrigin:            w.APIOrigin,
		RendezvousWSSURL:     w.RendezvousWSSURL,
		RelayWSSURLs:         relays,
		ServerPublicKey:      base64.StdEncoding.EncodeToString(key),
		ServerKeyFingerprint: "sha256:" + hex.EncodeToString(digest[:]),
	}
}

func (w WebClient) CSPConnectSources() []string {
	values := []string{w.APIOrigin}
	if parsed, err := url.Parse(w.RendezvousWSSURL); err == nil {
		values = append(values, canonicalOrigin(parsed))
	}
	for _, endpoint := range w.RelayWSSURLs {
		if parsed, err := url.Parse(endpoint); err == nil {
			values = append(values, canonicalOrigin(parsed))
		}
	}
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func fixedWSSURL(value, requiredPath string) (*url.URL, error) {
	if value == "" || strings.TrimSpace(value) != value || len(value) > 2048 {
		return nil, errors.New("URL is required")
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.Scheme != "wss" || parsed.Host == "" || parsed.User != nil || parsed.Opaque != "" {
		return nil, errors.New("URL must be an absolute WSS URL without credentials")
	}
	if parsed.Path != requiredPath || parsed.RawPath != "" || parsed.RawQuery != "" || parsed.ForceQuery || parsed.Fragment != "" {
		return nil, fmt.Errorf("URL must use exactly path %q without query or fragment", requiredPath)
	}
	if !validURLHost(parsed) {
		return nil, errors.New("URL host is invalid")
	}
	return parsed, nil
}

func validExplicitListen(value string) error {
	host, port, err := net.SplitHostPort(value)
	if err != nil || host == "" {
		return errors.New("must be an explicit host and port")
	}
	portNumber, err := strconv.ParseUint(port, 10, 16)
	if err != nil || portNumber == 0 {
		return errors.New("port must be between 1 and 65535")
	}
	return nil
}

func decodeWebClientPublicKey(value string) (ed25519.PublicKey, error) {
	if value == "" || strings.TrimSpace(value) != value || len(value) > 128 {
		return nil, errors.New("must be a base64 Ed25519 public key")
	}
	decoded, err := base64.StdEncoding.DecodeString(value)
	if err != nil {
		decoded, err = base64.RawStdEncoding.DecodeString(value)
	}
	if err != nil || len(decoded) != ed25519.PublicKeySize {
		return nil, errors.New("must decode to exactly 32 bytes")
	}
	return append(ed25519.PublicKey(nil), decoded...), nil
}
