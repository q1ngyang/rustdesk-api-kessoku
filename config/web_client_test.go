package config

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/spf13/viper"
)

func TestWebClientDefaultsDisabledAndLegacyProviderFailsClosed(t *testing.T) {
	if (WebClient{}).Enabled() || (WebClient{}).Validate(Auth{}) != nil {
		t.Fatal("zero-value web client is not securely disabled")
	}
	legacy := Config{DeprecatedWebClientProvider: map[string]interface{}{"mode": "disabled"}}
	if err := legacy.Validate(); err == nil || !strings.Contains(err.Error(), "web-client-provider is removed") {
		t.Fatalf("legacy provider block did not fail closed: %v", err)
	}
}

func TestLegacyWebClientProviderYAMLIsDetected(t *testing.T) {
	decoder := viper.New()
	decoder.SetConfigType("yaml")
	if err := decoder.ReadConfig(strings.NewReader("web-client-provider:\n  mode: external\n")); err != nil {
		t.Fatal(err)
	}
	decoded := Config{}
	if err := decoder.Unmarshal(&decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.DeprecatedWebClientProvider == nil {
		t.Fatal("legacy web-client-provider YAML was silently discarded")
	}
	if err := decoded.Validate(); err == nil || !strings.Contains(err.Error(), "web-client-provider is removed") {
		t.Fatalf("legacy provider YAML did not fail closed: %v", err)
	}
}

func TestLegacyWebClientProviderEnvironmentIsDetected(t *testing.T) {
	for _, environment := range [][]string{
		{"RUSTDESK_API_WEB_CLIENT_PROVIDER_MODE=external"},
		{"rustdesk_api_web_client_provider_manifest_launch_url=https://legacy.invalid"},
		{"RUSTDESK_API_WEB_CLIENT_PROVIDER="},
	} {
		if !legacyWebClientProviderEnvironmentPresent(environment) {
			t.Fatalf("legacy provider environment was silently ignored: %v", environment)
		}
	}
	if legacyWebClientProviderEnvironmentPresent([]string{"RUSTDESK_API_WEB_CLIENT_MODE=builtin"}) {
		t.Fatal("current web-client environment was mistaken for a legacy provider")
	}
}

func TestInitPreservesExactRelayMapKeysFromDefaultYAML(t *testing.T) {
	t.Setenv("RUSTDESK_API_AUTH_MAXIMUM_TOKEN_TTL", "10m")
	decoded := Config{}
	Init(&decoded, "../conf/config.yaml")
	const relay = "rustdesk.example.com:21117"
	if got := decoded.WebClient.RelayWSSURLs[relay]; got != "wss://rustdesk.example.com/ws/relay" {
		t.Fatalf("exact Relay map key was not preserved: %#v", decoded.WebClient.RelayWSSURLs)
	}
	if decoded.Auth.MaximumTokenTTL != 10*time.Minute {
		t.Fatalf("nested environment override was not preserved: %s", decoded.Auth.MaximumTokenTTL)
	}
}

func TestBuiltinWebClientValidationAndPublicContract(t *testing.T) {
	auth := Auth{Enabled: true, MaximumTokenTTL: time.Hour}
	valid := WebClient{
		Mode:               WebClientBuiltin,
		Listen:             "0.0.0.0:21122",
		PublicOrigin:       "https://client.example.test",
		APIOrigin:          "https://api.example.test",
		RendezvousWSSURL:   "wss://starry.example.test/ws/id",
		RelayWSSURLs:       map[string]string{"relay.example.test:21117": "wss://starry.example.test/ws/relay"},
		ServerPublicKey:    base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{1}, 32)),
		ProfileGeneration:  7,
		ConnectionTokenTTL: 15 * time.Minute,
	}
	if err := valid.Validate(auth); err != nil {
		t.Fatalf("valid web client rejected: %v", err)
	}
	public := valid.PublicConfig()
	if public.SchemaVersion != 1 || public.ProfileGeneration != 7 || public.APIOrigin != valid.APIOrigin || public.RendezvousWSSURL != valid.RendezvousWSSURL {
		t.Fatalf("unexpected public contract: %+v", public)
	}
	if len(public.RelayWSSURLs) != 1 || public.ServerPublicKey != valid.ServerPublicKey || !strings.HasPrefix(public.ServerKeyFingerprint, "sha256:") {
		t.Fatalf("public contract omitted fixed trust data: %+v", public)
	}
	rawKey := valid
	rawKey.ServerPublicKey = strings.TrimRight(valid.ServerPublicKey, "=")
	if got := rawKey.PublicConfig().ServerPublicKey; got != valid.ServerPublicKey {
		t.Fatalf("public key was not emitted in canonical padded base64: %q", got)
	}
	public.RelayWSSURLs["mutated"] = "wss://attacker.invalid/ws/relay"
	if _, exists := valid.RelayWSSURLs["mutated"]; exists {
		t.Fatal("public DTO aliases mutable deployment configuration")
	}

	tests := []struct {
		name   string
		mutate func(*WebClient, *Auth)
	}{
		{name: "auth disabled", mutate: func(_ *WebClient, a *Auth) { a.Enabled = false }},
		{name: "same effective origin", mutate: func(w *WebClient, _ *Auth) { w.APIOrigin = "https://client.example.test:443" }},
		{name: "public origin path", mutate: func(w *WebClient, _ *Auth) { w.PublicOrigin += "/client" }},
		{name: "insecure API", mutate: func(w *WebClient, _ *Auth) { w.APIOrigin = "http://api.example.test" }},
		{name: "rendezvous query", mutate: func(w *WebClient, _ *Auth) { w.RendezvousWSSURL += "?token=x" }},
		{name: "rendezvous path", mutate: func(w *WebClient, _ *Auth) { w.RendezvousWSSURL = "wss://starry.example.test/other" }},
		{name: "relay missing", mutate: func(w *WebClient, _ *Auth) { w.RelayWSSURLs = nil }},
		{name: "relay wrong path", mutate: func(w *WebClient, _ *Auth) {
			w.RelayWSSURLs = map[string]string{"relay": "wss://starry.example.test/other"}
		}},
		{name: "bad key", mutate: func(w *WebClient, _ *Auth) { w.ServerPublicKey = base64.StdEncoding.EncodeToString([]byte("short")) }},
		{name: "zero generation", mutate: func(w *WebClient, _ *Auth) { w.ProfileGeneration = 0 }},
		{name: "unsafe generation", mutate: func(w *WebClient, _ *Auth) { w.ProfileGeneration = maximumWebClientGeneration + 1 }},
		{name: "too many relays", mutate: func(w *WebClient, _ *Auth) {
			w.RelayWSSURLs = make(map[string]string, 65)
			for index := 0; index < 65; index++ {
				w.RelayWSSURLs[fmt.Sprintf("relay-%d", index)] = "wss://starry.example.test/ws/relay"
			}
		}},
		{name: "relay name too long", mutate: func(w *WebClient, _ *Auth) {
			w.RelayWSSURLs = map[string]string{strings.Repeat("r", 256): "wss://starry.example.test/ws/relay"}
		}},
		{name: "long token", mutate: func(w *WebClient, _ *Auth) { w.ConnectionTokenTTL = time.Hour + time.Second }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			candidate := valid
			candidate.RelayWSSURLs = map[string]string{"relay.example.test:21117": "wss://starry.example.test/ws/relay"}
			candidateAuth := auth
			test.mutate(&candidate, &candidateAuth)
			if err := candidate.Validate(candidateAuth); err == nil {
				t.Fatal("invalid web client accepted")
			}
		})
	}
}
