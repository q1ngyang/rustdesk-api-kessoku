package config

import (
	"strings"
	"testing"
)

func TestWebClientProviderDefaultsDisabled(t *testing.T) {
	provider := WebClientProvider{}
	if provider.Enabled() || provider.Validate() != nil {
		t.Fatal("zero-value web-client provider is not securely disabled")
	}
	if err := (Config{App: App{LegacyWebClient: 1}}).Validate(); err == nil {
		t.Fatal("legacy bundled web-client flag was accepted")
	}
}

func TestExternalWebClientProviderValidation(t *testing.T) {
	valid := WebClientProvider{
		Mode:                WebClientProviderExternal,
		AuthorizationRecord: "Approved under change KES-42; deployment owner retains the license record.",
		Manifest: WebClientProviderManifest{
			ID:            "approved-client",
			Name:          "Approved browser client",
			LaunchURL:     "https://client.example.test/launch",
			AllowedOrigin: "https://client.example.test",
			License:       "Apache-2.0",
			SourceURL:     "https://source.example.test/approved/client",
			Version:       "1.2.3",
			Digest:        "sha256:" + strings.Repeat("a", 64),
		},
	}
	if err := valid.Validate(); err != nil {
		t.Fatalf("valid provider rejected: %v", err)
	}

	tests := []struct {
		name   string
		mutate func(*WebClientProvider)
	}{
		{name: "unknown mode", mutate: func(p *WebClientProvider) { p.Mode = "bundled" }},
		{name: "missing authorization", mutate: func(p *WebClientProvider) { p.AuthorizationRecord = "" }},
		{name: "origin mismatch", mutate: func(p *WebClientProvider) { p.Manifest.AllowedOrigin = "https://other.example.test" }},
		{name: "origin path", mutate: func(p *WebClientProvider) { p.Manifest.AllowedOrigin += "/path" }},
		{name: "launch query", mutate: func(p *WebClientProvider) { p.Manifest.LaunchURL += "?token=forbidden" }},
		{name: "insecure launch", mutate: func(p *WebClientProvider) { p.Manifest.LaunchURL = "http://client.example.test/launch" }},
		{name: "source credentials", mutate: func(p *WebClientProvider) { p.Manifest.SourceURL = "https://user:pass@source.example.test/client" }},
		{name: "invalid digest", mutate: func(p *WebClientProvider) { p.Manifest.Digest = "sha256:latest" }},
		{name: "invalid id", mutate: func(p *WebClientProvider) { p.Manifest.ID = "../client" }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			candidate := valid
			test.mutate(&candidate)
			if err := candidate.Validate(); err == nil {
				t.Fatal("invalid provider accepted")
			}
		})
	}
}
