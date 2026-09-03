package config

import (
	"path/filepath"
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

func TestStarryControlValidationAcceptsAllowlistedLogFiles(t *testing.T) {
	candidate := validStarryControlConfig()
	candidate.LogDirectory = "/var/log/kessoku-control"
	candidate.LogSources = []ControlLogSource{{ID: "center", Label: "Starry center", Component: "starry", InstanceID: "starry-1", File: "starry.log"}}
	if err := candidate.Validate(); err != nil {
		t.Fatalf("valid log allowlist rejected: %v", err)
	}
}

func TestStarryControlValidationRejectsUnsafeLogAllowlist(t *testing.T) {
	tests := []ControlLogSource{
		{ID: "../center", Label: "Center", Component: "starry", File: "starry.log"},
		{ID: "center", Label: "", Component: "starry", File: "starry.log"},
		{ID: "center", Label: "Center", Component: "database", File: "starry.log"},
		{ID: "center", Label: "Center", Component: "starry", File: "../starry.log"},
		{ID: "center", Label: "Center", Component: "starry", InstanceID: "unknown", File: "starry.log"},
	}
	for index, source := range tests {
		candidate := validStarryControlConfig()
		candidate.LogDirectory = "/var/log/kessoku-control"
		candidate.LogSources = []ControlLogSource{source}
		if err := candidate.Validate(); err == nil {
			t.Fatalf("unsafe log source %d accepted", index)
		}
	}
	candidate := validStarryControlConfig()
	candidate.LogDirectory = "relative/logs"
	candidate.LogSources = []ControlLogSource{{ID: "center", Label: "Center", Component: "starry", File: "starry.log"}}
	if err := candidate.Validate(); err == nil {
		t.Fatal("relative log directory accepted")
	}
}

func TestPairingBrokerRequiresExactHTTPSPinAndAgentAllowlist(t *testing.T) {
	candidate := validStarryControlConfig()
	candidate.RegistryDirectory = "/app/data/server-control"
	candidate.HostIdentityFile = "/etc/machine-id"
	candidate.Pairing = PairingBroker{
		Enabled: true, BrokerOrigin: "https://kessoku.example.test",
		BrokerSPKISHA256: "sha256:" + strings.Repeat("a", 64),
		CodeTTL:          10 * time.Minute, RecoveryTTL: 5 * time.Minute,
		AgentOrigins: []PairingAgentOrigin{{
			ID: "primary", Name: "Primary Agent", Origin: "https://starry.internal.test:21120",
			TLSServerName: "starry.internal.test",
		}},
	}
	if err := candidate.Validate(); err != nil {
		t.Fatalf("valid pairing broker rejected: %v", err)
	}

	tests := []struct {
		name   string
		mutate func(*ServerControl)
	}{
		{name: "plain broker", mutate: func(c *ServerControl) { c.Pairing.BrokerOrigin = "http://kessoku.example.test" }},
		{name: "broker path", mutate: func(c *ServerControl) { c.Pairing.BrokerOrigin += "/pair" }},
		{name: "short pin", mutate: func(c *ServerControl) { c.Pairing.BrokerSPKISHA256 = "sha256:aa" }},
		{name: "uppercase pin", mutate: func(c *ServerControl) { c.Pairing.BrokerSPKISHA256 = "sha256:" + strings.Repeat("A", 64) }},
		{name: "no allowlist", mutate: func(c *ServerControl) { c.Pairing.AgentOrigins = nil }},
		{name: "arbitrary path", mutate: func(c *ServerControl) { c.Pairing.AgentOrigins[0].Origin += "/callback" }},
		{name: "duplicate origin", mutate: func(c *ServerControl) {
			c.Pairing.AgentOrigins = append(c.Pairing.AgentOrigins, PairingAgentOrigin{ID: "other", Name: "Other", Origin: c.Pairing.AgentOrigins[0].Origin, TLSServerName: "other.test"})
		}},
		{name: "long recovery", mutate: func(c *ServerControl) { c.Pairing.RecoveryTTL = 11 * time.Minute }},
		{name: "long code", mutate: func(c *ServerControl) { c.Pairing.CodeTTL = 61 * time.Minute }},
		{name: "unsafe registry whitespace", mutate: func(c *ServerControl) { c.RegistryDirectory = " /app/data/server-control" }},
		{name: "missing host identity", mutate: func(c *ServerControl) { c.HostIdentityFile = "" }},
		{name: "relative host identity", mutate: func(c *ServerControl) { c.HostIdentityFile = "machine-id" }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			copy := candidate
			copy.Pairing.AgentOrigins = append([]PairingAgentOrigin(nil), candidate.Pairing.AgentOrigins...)
			test.mutate(&copy)
			if err := copy.Validate(); err == nil {
				t.Fatal("unsafe pairing configuration accepted")
			}
		})
	}
}

func TestRegistryDeploymentDefaultsMatchContainerAndSystemdWorkingDirectories(t *testing.T) {
	candidate := ServerControl{}
	if got := candidate.EffectiveRegistryDirectory(); got != filepath.Clean("./data/server-control") {
		t.Fatalf("default registry directory = %q", got)
	}
	for _, test := range []struct{ working, want string }{
		{"/app", "/app/data/server-control"},
		{"/var/lib/kessoku-api", "/var/lib/kessoku-api/data/server-control"},
	} {
		got := filepath.Clean(filepath.Join(test.working, candidate.EffectiveRegistryDirectory()))
		if got != test.want {
			t.Fatalf("working directory %s resolves registry to %s", test.working, got)
		}
	}
}
