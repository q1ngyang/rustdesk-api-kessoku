package config

import (
	"strings"
	"testing"
	"time"
)

func TestAuthDefaultsPreserveClientCompatibilityAndBoundInput(t *testing.T) {
	profile := Auth{}
	if got := profile.EffectiveAccessTokenTTL(); got != 7*24*time.Hour {
		t.Fatalf("default access token TTL = %s", got)
	}
	if got := profile.EffectiveMaximumTokenTTL(); got != 7*24*time.Hour {
		t.Fatalf("default maximum token TTL = %s", got)
	}
	if got := (Auth{MaxTokenBytes: MaxAccessTokenBytes + 1}).EffectiveMaxTokenBytes(); got != MaxAccessTokenBytes {
		t.Fatalf("effective token limit = %d", got)
	}
	internal := InternalAuth{}
	if internal.EffectiveGlobalRequestsPS() <= 0 || internal.EffectivePerCertRequestsPS() <= 0 {
		t.Fatal("enabled internal API would have an unlimited zero-value rate limit")
	}
}

func validAuthConfiguration() Auth {
	return Auth{
		Enabled:         true,
		Issuer:          "https://api.example.test/auth",
		Audiences:       []string{"kessoku-api", "rustdesk-connect"},
		AccessTokenTTL:  24 * time.Hour,
		MaximumTokenTTL: 7 * 24 * time.Hour,
		ClockSkew:       30 * time.Second,
		MaxTokenBytes:   MaxAccessTokenBytes,
		CurrentKey: AuthKey{
			ID:             "access-2026-08",
			PrivateKeyFile: "/run/secrets/access-key.pem",
		},
		PreviousKeys: []AuthKey{{
			ID:            "access-2026-07",
			PublicKeyFile: "/run/secrets/access-key-previous.pem",
		}},
		Internal: InternalAuth{
			Enabled:           true,
			Listen:            "127.0.0.1:21121",
			ServerCertFile:    "/run/secrets/internal-cert.pem",
			ServerKeyFile:     "/run/secrets/internal-key.pem",
			ClientCAFile:      "/run/secrets/client-ca.pem",
			AllowedURISANs:    []string{"spiffe://example.test/starry/production"},
			MaxBodyBytes:      1 << 20,
			RequestTimeout:    2 * time.Second,
			GlobalRequestsPS:  200,
			PerCertRequestsPS: 100,
		},
	}
}

func TestAuthValidationAcceptsBoundedMTLSProfile(t *testing.T) {
	if err := validAuthConfiguration().Validate(); err != nil {
		t.Fatalf("valid auth configuration rejected: %v", err)
	}
}

func TestAuthValidationRejectsUnsafeConfiguration(t *testing.T) {
	valid := validAuthConfiguration()
	tests := []struct {
		name   string
		mutate func(*Auth)
	}{
		{name: "internal without auth", mutate: func(a *Auth) { a.Enabled = false }},
		{name: "insecure issuer", mutate: func(a *Auth) { a.Issuer = "http://api.example.test" }},
		{name: "missing connection audience", mutate: func(a *Auth) { a.Audiences = []string{"kessoku-api"} }},
		{name: "duplicate audience", mutate: func(a *Auth) { a.Audiences = append(a.Audiences, "kessoku-api") }},
		{name: "TTL exceeds maximum", mutate: func(a *Auth) { a.AccessTokenTTL = 8 * 24 * time.Hour }},
		{name: "negative clock skew", mutate: func(a *Auth) { a.ClockSkew = -time.Second }},
		{name: "oversized token", mutate: func(a *Auth) { a.MaxTokenBytes = MaxAccessTokenBytes + 1 }},
		{name: "duplicate key", mutate: func(a *Auth) { a.PreviousKeys[0].ID = a.CurrentKey.ID }},
		{name: "missing private key", mutate: func(a *Auth) { a.CurrentKey.PrivateKeyFile = "" }},
		{name: "implicit listen host", mutate: func(a *Auth) { a.Internal.Listen = ":21121" }},
		{name: "missing TLS key", mutate: func(a *Auth) { a.Internal.ServerKeyFile = "" }},
		{name: "no SAN", mutate: func(a *Auth) { a.Internal.AllowedURISANs = nil }},
		{name: "URI SAN query", mutate: func(a *Auth) { a.Internal.AllowedURISANs[0] += "?role=admin" }},
		{name: "wildcard DNS SAN", mutate: func(a *Auth) { a.Internal.AllowedURISANs = nil; a.Internal.AllowedDNSSANs = []string{"*.example.test"} }},
		{name: "oversized body", mutate: func(a *Auth) { a.Internal.MaxBodyBytes = 1<<20 + 1 }},
		{name: "excessive timeout", mutate: func(a *Auth) { a.Internal.RequestTimeout = 11 * time.Second }},
		{name: "negative rate", mutate: func(a *Auth) { a.Internal.PerCertRequestsPS = -1 }},
		{name: "invalid key id", mutate: func(a *Auth) { a.CurrentKey.ID = strings.Repeat("x", 129) }},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			candidate := valid
			candidate.Audiences = append([]string(nil), valid.Audiences...)
			candidate.PreviousKeys = append([]AuthKey(nil), valid.PreviousKeys...)
			candidate.Internal.AllowedURISANs = append([]string(nil), valid.Internal.AllowedURISANs...)
			candidate.Internal.AllowedDNSSANs = append([]string(nil), valid.Internal.AllowedDNSSANs...)
			test.mutate(&candidate)
			if err := candidate.Validate(); err == nil {
				t.Fatal("unsafe auth configuration accepted")
			}
		})
	}
}
