package config

import (
	"strings"
	"testing"
	"time"
)

func validStarryControlConfig() ServerControl {
	return ServerControl{
		ReadOnly:         true,
		RequestTimeout:   5 * time.Second,
		ResponseMaxBytes: 1 << 20,
		Instances: []StarryInstance{{
			ID:                 "starry-1",
			Name:               "Primary Starry instance",
			Enabled:            true,
			BaseURL:            "https://control.internal.example.test",
			ExpectedInstanceID: "0191f6a0-0000-7000-8000-000000000001",
			TLSServerName:      "control.internal.example.test",
			CAFile:             "/run/secrets/control-ca.pem",
			ClientCertFile:     "/run/secrets/control-client.pem",
			ClientKeyFile:      "/run/secrets/control-client-key.pem",
			ControlKeyFile:     "/run/secrets/control-signing-key",
			ControlKeyID:       "control-2026-08",
			ControlIssuer:      "https://api.internal.example.test",
			AuthorizedParty:    "spiffe://example.test/kessoku/production",
		}},
	}
}

func TestStarryControlValidationAcceptsFixedSecureInstance(t *testing.T) {
	candidate := validStarryControlConfig()
	if err := candidate.Validate(); err != nil {
		t.Fatalf("valid server-control configuration rejected: %v", err)
	}
}

func TestStarryControlValidationRejectsUnsafeConfiguration(t *testing.T) {
	valid := validStarryControlConfig()
	tests := []struct {
		name   string
		mutate func(*ServerControl)
	}{
		{name: "negative timeout", mutate: func(c *ServerControl) { c.RequestTimeout = -time.Second }},
		{name: "excessive timeout", mutate: func(c *ServerControl) { c.RequestTimeout = 31 * time.Second }},
		{name: "excessive response", mutate: func(c *ServerControl) { c.ResponseMaxBytes = MaxStarryControlPayloadBytes + 1 }},
		{name: "duplicate id", mutate: func(c *ServerControl) { c.Instances = append(c.Instances, c.Instances[0]) }},
		{name: "invalid id", mutate: func(c *ServerControl) { c.Instances[0].ID = "../starry" }},
		{name: "missing name", mutate: func(c *ServerControl) { c.Instances[0].Name = "" }},
		{name: "plain HTTP", mutate: func(c *ServerControl) { c.Instances[0].BaseURL = "http://control.example.test" }},
		{name: "base URL path", mutate: func(c *ServerControl) { c.Instances[0].BaseURL += "/api" }},
		{name: "base URL credentials", mutate: func(c *ServerControl) { c.Instances[0].BaseURL = "https://user:pass@control.example.test" }},
		{name: "invalid expected instance", mutate: func(c *ServerControl) { c.Instances[0].ExpectedInstanceID = "not-a-uuid" }},
		{name: "invalid TLS name", mutate: func(c *ServerControl) { c.Instances[0].TLSServerName = "https://control.example.test" }},
		{name: "missing CA file", mutate: func(c *ServerControl) { c.Instances[0].CAFile = "" }},
		{name: "padded key file", mutate: func(c *ServerControl) { c.Instances[0].ControlKeyFile += " " }},
		{name: "invalid key id", mutate: func(c *ServerControl) { c.Instances[0].ControlKeyID = "control\nkey" }},
		{name: "insecure issuer", mutate: func(c *ServerControl) { c.Instances[0].ControlIssuer = "http://api.example.test" }},
		{name: "invalid authorized party", mutate: func(c *ServerControl) { c.Instances[0].AuthorizedParty = strings.Repeat("a", 129) }},
		{name: "non URI authorized party", mutate: func(c *ServerControl) { c.Instances[0].AuthorizedParty = "kessoku-production" }},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			candidate := valid
			candidate.Instances = append([]StarryInstance(nil), valid.Instances...)
			test.mutate(&candidate)
			if err := candidate.Validate(); err == nil {
				t.Fatal("unsafe server-control configuration accepted")
			}
		})
	}
}

func TestDisabledStarryInstanceStillRequiresStableIdentity(t *testing.T) {
	candidate := ServerControl{Instances: []StarryInstance{{
		ID:      "future-starry",
		Name:    "Future Starry instance",
		Enabled: false,
	}}}
	if err := candidate.Validate(); err != nil {
		t.Fatalf("disabled instance with stable identity rejected: %v", err)
	}
	candidate.Instances[0].ID = ""
	if err := candidate.Validate(); err == nil {
		t.Fatal("disabled instance without an id accepted")
	}
}
