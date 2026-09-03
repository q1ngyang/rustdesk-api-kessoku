package starry

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/q1ngyang/rustdesk-api-kessoku/v3/internal/starrycontrol"
)

func TestFastMediaCapabilitiesFailClosedAndRemainBackwardCompatible(t *testing.T) {
	legacy := validRelayQualityCapabilities()
	if err := validateCapabilitiesResponse(legacy); err != nil {
		t.Fatalf("v1.3.0 capabilities rejected: %v", err)
	}

	current := validFastMediaCapabilities()
	if err := validateCapabilitiesResponse(current); err != nil {
		t.Fatalf("v1.3.1 capabilities rejected: %v", err)
	}
	if current.Capabilities.FastRelayAuthorization != 1 || current.Capabilities.FastMediaRelayUDP != 1 ||
		current.Capabilities.ConfigSchema != 5 || current.Capabilities.RelayTelemetrySchema != 2 {
		t.Fatalf("frozen capability versions not decoded: %#v", current.Capabilities)
	}

	tests := []struct {
		name   string
		mutate func(*starrycontrol.Capabilities)
	}{
		{"unknown fast authorization", func(value *starrycontrol.Capabilities) { value.Capabilities.FastRelayAuthorization = 2 }},
		{"unknown FastMedia UDP", func(value *starrycontrol.Capabilities) { value.Capabilities.FastMediaRelayUDP = 2 }},
		{"unknown config schema capability", func(value *starrycontrol.Capabilities) { value.Capabilities.ConfigSchema = 6 }},
		{"missing authorization dependency", func(value *starrycontrol.Capabilities) { value.Capabilities.FastRelayAuthorization = 0 }},
		{"missing telemetry v2 dependency", func(value *starrycontrol.Capabilities) { value.Capabilities.RelayTelemetrySchema = 1 }},
		{"schema v5 inferred from active version", func(value *starrycontrol.Capabilities) { value.Capabilities.ConfigSchema = 0 }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			value := current
			test.mutate(&value)
			if err := validateCapabilitiesResponse(value); err == nil {
				t.Fatal("unsupported or incomplete capability set was accepted")
			}
		})
	}
}

func TestFastMediaRelayInventoryValidationAndRedaction(t *testing.T) {
	versions := validFastMediaCapabilities().Capabilities
	current := currentFastMediaInventory()
	if err := validateRelaysResponse(current, versions); err != nil {
		t.Fatalf("frozen v1.3.1 Relay inventory rejected: %v", err)
	}
	if !hasHealthyFastMediaCandidate(current) {
		t.Fatal("healthy FastMedia candidate was not recognized")
	}

	tests := []struct {
		name   string
		mutate func(*starrycontrol.RelayInventory)
	}{
		{"missing FastRelay aggregate", func(value *starrycontrol.RelayInventory) { value.FastRelay = nil }},
		{"missing per-Relay UDP aggregate", func(value *starrycontrol.RelayInventory) { value.Relays[0].FastMediaUDP = nil }},
		{"missing required counter", func(value *starrycontrol.RelayInventory) { value.FastRelay.ReliableFallbacks = nil }},
		{"unknown Relay capability", func(value *starrycontrol.RelayInventory) {
			value.Relays[0].Capabilities.FastMediaRelayUDP = intPointer(2)
		}},
		{"invalid configured port", func(value *starrycontrol.RelayInventory) { value.Relays[0].FastMediaUDP.ConfiguredPort = intPointer(0) }},
		{"healthy but disabled", func(value *starrycontrol.RelayInventory) { value.Relays[0].FastMediaUDP.Enabled = boolPointer(false) }},
		{"counter overflow", func(value *starrycontrol.RelayInventory) {
			value.Relays[0].FastMediaUDP.ForwardedBytes = uint64Pointer(uint64(1) << 63)
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			value := cloneRelayInventory(t, current)
			test.mutate(&value)
			if err := validateRelaysResponse(value, versions); err == nil {
				t.Fatal("invalid FastMedia response was accepted")
			}
		})
	}

	encoded, err := json.Marshal(current)
	if err != nil {
		t.Fatal(err)
	}
	var raw map[string]any
	if err := json.Unmarshal(encoded, &raw); err != nil {
		t.Fatal(err)
	}
	raw["session_uuid"] = "forbidden-session"
	raw["allocation_id"] = "forbidden-allocation"
	raw["client_ip"] = "203.0.113.10"
	raw["connection_token"] = "forbidden-token"
	raw["relays"].([]any)[0].(map[string]any)["signed_grant"] = "forbidden-grant"
	encoded, err = json.Marshal(raw)
	if err != nil {
		t.Fatal(err)
	}
	decoded := starrycontrol.RelayInventory{}
	if err := json.Unmarshal(encoded, &decoded); err != nil || validateRelaysResponse(decoded, versions) != nil {
		t.Fatalf("additive unknown fields broke typed response: %v", err)
	}
	forwarded, err := json.Marshal(decoded)
	if err != nil {
		t.Fatal(err)
	}
	lower := strings.ToLower(string(forwarded))
	for _, forbidden := range []string{"session_uuid", "allocation_id", "client_ip", "connection_token", "signed_grant", "forbidden-"} {
		if strings.Contains(lower, forbidden) {
			t.Fatalf("typed FastMedia response leaked %q: %s", forbidden, forwarded)
		}
	}
}

func TestFastMediaUDPRequiredNullableFields(t *testing.T) {
	versions := validFastMediaCapabilities().Capabilities
	encoded, err := json.Marshal(currentFastMediaInventory())
	if err != nil {
		t.Fatal(err)
	}
	var raw map[string]any
	if err := json.Unmarshal(encoded, &raw); err != nil {
		t.Fatal(err)
	}
	udp := raw["relays"].([]any)[0].(map[string]any)["fast_media_udp"].(map[string]any)
	delete(udp, "bind_succeeded")
	encoded, err = json.Marshal(raw)
	if err != nil {
		t.Fatal(err)
	}
	var missing starrycontrol.RelayInventory
	if err := json.Unmarshal(encoded, &missing); err != nil {
		t.Fatal(err)
	}
	if err := validateRelaysResponse(missing, versions); err == nil {
		t.Fatal("missing required nullable FastMedia UDP field was accepted")
	}

	udp["bind_succeeded"] = nil
	encoded, err = json.Marshal(raw)
	if err != nil {
		t.Fatal(err)
	}
	var explicitlyNull starrycontrol.RelayInventory
	if err := json.Unmarshal(encoded, &explicitlyNull); err != nil {
		t.Fatal(err)
	}
	if err := validateRelaysResponse(explicitlyNull, versions); err != nil {
		t.Fatalf("explicitly-null frozen FastMedia UDP field was rejected: %v", err)
	}
}

func TestFastMediaRelayCapabilityWireFieldIsRequired(t *testing.T) {
	versions := validFastMediaCapabilities().Capabilities
	encoded, err := json.Marshal(currentFastMediaInventory())
	if err != nil {
		t.Fatal(err)
	}
	var raw map[string]any
	if err := json.Unmarshal(encoded, &raw); err != nil {
		t.Fatal(err)
	}
	capabilities := raw["relays"].([]any)[0].(map[string]any)["capabilities"].(map[string]any)
	delete(capabilities, "fast_media_relay_udp")
	encoded, err = json.Marshal(raw)
	if err != nil {
		t.Fatal(err)
	}
	var missing starrycontrol.RelayInventory
	if err := json.Unmarshal(encoded, &missing); err != nil {
		t.Fatal(err)
	}
	if err := validateRelaysResponse(missing, versions); err == nil {
		t.Fatal("missing frozen per-Relay FastMedia capability field was accepted")
	}

	capabilities["fast_media_relay_udp"] = nil
	raw["relays"].([]any)[0].(map[string]any)["quality_candidate"] = false
	for field := range raw["relays"].([]any)[0].(map[string]any)["fast_media_udp"].(map[string]any) {
		raw["relays"].([]any)[0].(map[string]any)["fast_media_udp"].(map[string]any)[field] = nil
	}
	encoded, err = json.Marshal(raw)
	if err != nil {
		t.Fatal(err)
	}
	var explicitlyNull starrycontrol.RelayInventory
	if err := json.Unmarshal(encoded, &explicitlyNull); err != nil {
		t.Fatal(err)
	}
	if err := validateRelaysResponse(explicitlyNull, versions); err != nil {
		t.Fatalf("explicitly unsupported per-Relay FastMedia capability was rejected: %v", err)
	}
}

func TestFastModeInspectionRiskAndDependencyGates(t *testing.T) {
	for _, test := range []struct {
		name             string
		document         string
		schemaVersion    int
		fastMediaEnabled bool
	}{
		{
			name: "schema v4 FastCompat only",
			document: "version: 4\nfast_mode:\n  relay:\n" +
				"    fast_compat_enabled: true\n    fast_media_v1_enabled: false\n",
			schemaVersion: 4,
		},
		{
			name: "schema v5 FastMedia only",
			document: "version: 5\nfast_mode:\n  relay:\n" +
				"    fast_compat_enabled: false\n    fast_media_v1_enabled: true\n",
			schemaVersion:    5,
			fastMediaEnabled: true,
		},
		{
			name: "schema v5 both disabled",
			document: "version: 5\nfast_mode:\n  relay:\n" +
				"    fast_compat_enabled: false\n    fast_media_v1_enabled: false\n",
			schemaVersion: 5,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			inspection, err := inspectFastModeCandidate(test.document)
			if err != nil || inspection.SchemaVersion != test.schemaVersion || !inspection.HasFastMode ||
				inspection.FastMediaEnabled != test.fastMediaEnabled {
				t.Fatalf("candidate inspection = %#v, %v", inspection, err)
			}
		})
	}
	if risk := planMinimumRisk([]json.RawMessage{json.RawMessage(`{"pointer":"/fast_mode/relay/fast_compat_enabled","kind":"replace"}`)}, false); risk != "medium" {
		t.Fatalf("FastCompat risk floor = %s", risk)
	}
	for _, pointer := range []string{
		"/fast_mode/relay/fast_media_v1_enabled",
		"/fast_mode/relay/relay_max_datagram",
		"/fast_mode/relay/max_bitrate_kbps",
		"/relay_servers/0/endpoints/0/fast_media_udp_port",
	} {
		changes := []json.RawMessage{json.RawMessage(`{"pointer":"` + pointer + `","kind":"replace"}`)}
		if risk := planMinimumRisk(changes, true); risk != "high" {
			t.Fatalf("%s risk floor = %s", pointer, risk)
		}
	}

	inventory := currentFastMediaInventory()
	*inventory.Relays[0].WebSocket.Stale = true
	if hasHealthyFastMediaCandidate(inventory) {
		t.Fatal("stale telemetry was treated as a healthy FastMedia candidate")
	}
	inventory = currentFastMediaInventory()
	*inventory.Relays[0].FastMediaUDP.ReportedPort = 22120
	if hasHealthyFastMediaCandidate(inventory) {
		t.Fatal("UDP port drift was treated as a healthy FastMedia candidate")
	}
}

func validFastMediaCapabilities() starrycontrol.Capabilities {
	value := validRelayQualityCapabilities()
	value.Instance.StarryVersion = "1.1.16-patch-v1.3.1"
	value.Capabilities.RelayTelemetrySchema = 2
	value.Capabilities.FastRelayAuthorization = 1
	value.Capabilities.FastMediaRelayUDP = 1
	value.Capabilities.ConfigSchema = 5
	value.Config.SupportedSchemaVersions = []int{1, 2, 3, 4, 5}
	value.Config.ActiveSchemaVersion = 5
	return value
}

func currentFastMediaInventory() starrycontrol.RelayInventory {
	value := currentRelayInventory()
	value.Relays[0].Version = "1.1.16-patch-v1.3.1"
	value.Relays[0].Capabilities.FastMediaRelayUDP = intPointer(1)
	value.Relays[0].WebSocket.TelemetrySchema = intPointer(2)
	value.Relays[0].FastMediaUDP = &starrycontrol.FastMediaUDPRuntime{
		ConfiguredPort: intPointer(22119), ReportedPort: intPointer(22119), Enabled: boolPointer(true), Healthy: boolPointer(true),
		ActiveAllocations: uint64Pointer(2), ActiveStreams: uint64Pointer(1), HelloAccepted: uint64Pointer(9200),
		CookieRejected: uint64Pointer(3), BindSucceeded: uint64Pointer(1800), BindRejected: uint64Pointer(5), GrantRejected: uint64Pointer(2),
		RoleMismatch: uint64Pointer(1), SessionMismatch: uint64Pointer(0), AllocationMismatch: uint64Pointer(1), Rebinds: uint64Pointer(14),
		ForwardedPackets: uint64Pointer(150000), ForwardedBytes: uint64Pointer(120000000), DroppedPackets: uint64Pointer(42),
		RateLimited: uint64Pointer(7), ReplayRejected: uint64Pointer(2), ExpiredAllocations: uint64Pointer(12), ListenerFailures: uint64Pointer(0),
	}
	value.FastRelay = &starrycontrol.FastRelayRuntime{
		ProtocolVersion: 1, FastCompatEnabled: boolPointer(false), FastMediaV1Enabled: boolPointer(false),
		ActiveAuthorizations: uint64Pointer(0), ActiveFastMediaAuthorizations: uint64Pointer(0), LastFastMediaAuthorizationExpiresAtUnix: uint64Pointer(0),
		IssuedSessions: uint64Pointer(0), TargetGrantsIssued: uint64Pointer(0), ControllerGrantsIssued: uint64Pointer(0),
		FastCompatSessions: uint64Pointer(0), FastMediaSessions: uint64Pointer(0), Reused: uint64Pointer(0), Delivered: uint64Pointer(0),
		Disabled: uint64Pointer(0), InsecureRequests: uint64Pointer(0), InvalidConfiguration: uint64Pointer(0), InvalidUUIDs: uint64Pointer(0),
		InvalidServerSelection: uint64Pointer(0), MissingSigningKeys: uint64Pointer(0), SigningFailures: uint64Pointer(0), QualitySelectionFailures: uint64Pointer(0),
		RateLimited: uint64Pointer(0), ResponseMisses: uint64Pointer(0), ExpiredCacheEvictions: uint64Pointer(0), FastMediaUnavailable: uint64Pointer(0), ReliableFallbacks: uint64Pointer(0),
	}
	return value
}
