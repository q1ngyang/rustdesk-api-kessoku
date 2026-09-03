package starry

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/q1ngyang/rustdesk-api-kessoku/v3/internal/starrycontrol"
	"github.com/q1ngyang/rustdesk-api-kessoku/v3/internal/starrycontrol/clientgen"
)

func TestRelayQualityCapabilityVersionsRemainBackwardCompatible(t *testing.T) {
	legacy := validRelayQualityCapabilities()
	legacy.Capabilities.RelayQuality = 0
	legacy.Capabilities.RelayActiveProbe = 0
	legacy.Capabilities.RelayProbeProtocol = 0
	legacy.Capabilities.RelayLoadProtocol = 0
	legacy.Capabilities.RelayTelemetrySchema = 0
	legacy.Config.SupportedSchemaVersions = []int{1, 2, 3}
	legacy.Config.ActiveSchemaVersion = 3
	if err := validateCapabilitiesResponse(legacy); err != nil {
		t.Fatalf("patch-v1.2 capabilities rejected: %v", err)
	}

	current := validRelayQualityCapabilities()
	if err := validateCapabilitiesResponse(current); err != nil {
		t.Fatalf("adaptive capabilities rejected: %v", err)
	}
	if current.Capabilities.RelayQuality != 1 || current.Capabilities.RelayActiveProbe != 1 ||
		current.Capabilities.RelayProbeProtocol != 1 || current.Capabilities.RelayLoadProtocol != 1 {
		t.Fatalf("adaptive capability versions were not decoded: %#v", current.Capabilities)
	}

	encoded, err := json.Marshal(current)
	if err != nil {
		t.Fatal(err)
	}
	var object map[string]any
	if err := json.Unmarshal(encoded, &object); err != nil {
		t.Fatal(err)
	}
	object["capabilities"].(map[string]any)["future_adaptive_extension"] = float64(9)
	encoded, err = json.Marshal(object)
	if err != nil {
		t.Fatal(err)
	}
	decoded := starrycontrol.Capabilities{}
	if err := json.Unmarshal(encoded, &decoded); err != nil || validateCapabilitiesResponse(decoded) != nil {
		t.Fatalf("unknown additive capability was not ignored safely: decoded=%#v err=%v", decoded, err)
	}

	current.Capabilities.RelayQuality = 2
	if err := validateCapabilitiesResponse(current); err == nil {
		t.Fatal("unsupported adaptive capability version was accepted")
	}

	incomplete := validRelayQualityCapabilities()
	incomplete.Capabilities.RelayLoadProtocol = 0
	if err := validateCapabilitiesResponse(incomplete); err == nil {
		t.Fatal("incomplete adaptive capability suite was accepted")
	}
}

func TestRelayInventorySupportsLegacyAdaptiveAndUnknownFieldsWithoutLeaks(t *testing.T) {
	legacy := legacyRelayInventory()
	if err := validateRelaysResponse(legacy, starrycontrol.CapabilityVersions{RelayInventory: 1}); err != nil {
		t.Fatalf("patch-v1.2 Relay inventory rejected: %v", err)
	}
	if legacy.Quality != nil || legacy.Relays[0].Capabilities != nil || legacy.Relays[0].QualityCandidate != nil || legacy.Relays[0].WebSocket.Stale != nil {
		t.Fatalf("legacy optional state should remain unsupported: %#v", legacy)
	}

	current := currentRelayInventory()
	versions := validRelayQualityCapabilities().Capabilities
	if err := validateRelaysResponse(current, versions); err != nil {
		t.Fatalf("adaptive Relay inventory rejected: %v", err)
	}
	if current.Quality == nil || current.Quality.ProtocolVersion != versions.RelayQuality || current.Quality.Strategy != "adaptive" {
		t.Fatalf("adaptive runtime not decoded: %#v", current.Quality)
	}

	encoded, err := json.Marshal(current)
	if err != nil {
		t.Fatal(err)
	}
	var object map[string]any
	if err := json.Unmarshal(encoded, &object); err != nil {
		t.Fatal(err)
	}
	object["allocation_id"] = "secret-allocation"
	object["session_uuid"] = "secret-session"
	quality := object["quality"].(map[string]any)
	quality["nonce"] = "secret-nonce"
	quality["raw_report"] = map[string]any{"client_ip": "203.0.113.99", "connection_token": "secret-token"}
	relays := object["relays"].([]any)
	relays[0].(map[string]any)["future_relay_field"] = true
	relays[0].(map[string]any)["websocket"].(map[string]any)["process_instance_id"] = "0198f3a0-5c11-7cb2-9b64-9cf25ab8cd10"
	encoded, err = json.Marshal(object)
	if err != nil {
		t.Fatal(err)
	}
	redacted := starrycontrol.RelayInventory{}
	if err := json.Unmarshal(encoded, &redacted); err != nil {
		t.Fatal(err)
	}
	if err := validateRelaysResponse(redacted, versions); err != nil {
		t.Fatalf("unknown additive inventory fields were not tolerated: %v", err)
	}
	forwarded, err := json.Marshal(redacted)
	if err != nil {
		t.Fatal(err)
	}
	lower := strings.ToLower(string(forwarded))
	for _, forbidden := range []string{"allocation_id", "session_uuid", "process_instance_id", "nonce", "raw_report", "client_ip", "connection_token", "secret-allocation", "secret-token"} {
		if strings.Contains(lower, forbidden) {
			t.Fatalf("typed Relay inventory leaked %q: %s", forbidden, forwarded)
		}
	}
}

func TestRelayInventoryValidationBoundaries(t *testing.T) {
	versions := validRelayQualityCapabilities().Capabilities
	tests := []struct {
		name   string
		mutate func(*starrycontrol.RelayInventory)
	}{
		{"missing aggregate", func(value *starrycontrol.RelayInventory) { value.Quality = nil }},
		{"missing Relay capabilities", func(value *starrycontrol.RelayInventory) { value.Relays[0].Capabilities = nil }},
		{"missing candidate flag", func(value *starrycontrol.RelayInventory) { value.Relays[0].QualityCandidate = nil }},
		{"missing freshness flag", func(value *starrycontrol.RelayInventory) { value.Relays[0].WebSocket.Stale = nil }},
		{"unknown quality protocol", func(value *starrycontrol.RelayInventory) { value.Quality.ProtocolVersion = 2 }},
		{"unknown strategy", func(value *starrycontrol.RelayInventory) { value.Quality.Strategy = "future" }},
		{"invalid load basis points", func(value *starrycontrol.RelayInventory) {
			value.Relays[0].WebSocket.LoadBasisPoints = intPointer(10001)
		}},
		{"invalid telemetry sequence", func(value *starrycontrol.RelayInventory) {
			value.Relays[0].WebSocket.TelemetrySequence = uint64Pointer(0)
		}},
		{"int64 counter overflow", func(value *starrycontrol.RelayInventory) {
			value.Quality.ReportsLate = uint64Pointer(uint64(1) << 63)
		}},
		{"Relay selection int64 overflow", func(value *starrycontrol.RelayInventory) {
			value.Quality.RelaySelections["relay.example.test:21117"] = uint64(1) << 63
		}},
		{"stale quality candidate", func(value *starrycontrol.RelayInventory) { value.Relays[0].WebSocket.Stale = boolPointer(true) }},
		{"missing fallback aggregate", func(value *starrycontrol.RelayInventory) { value.Quality.FallbackReasons.ReportLate = nil }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			value := cloneRelayInventory(t, currentRelayInventory())
			test.mutate(&value)
			if err := validateRelaysResponse(value, versions); err == nil {
				t.Fatal("invalid adaptive Relay inventory was accepted")
			}
		})
	}

	legacy := legacyRelayInventory()
	if err := validateRelaysResponse(legacy, starrycontrol.CapabilityVersions{RelayInventory: 1}); err != nil {
		t.Fatalf("missing adaptive fields must remain valid without negotiated capability: %v", err)
	}
}

func TestPatchV130FastRelayRuntimeRemainsCompatible(t *testing.T) {
	versions := validRelayQualityCapabilities().Capabilities
	legacy := currentRelayInventory()
	if err := validateRelaysResponse(legacy, versions); err != nil {
		t.Fatalf("patch-v1.3.0 FastRelay runtime rejected: %v", err)
	}

	missing := cloneRelayInventory(t, legacy)
	missing.FastRelay.Issued = nil
	if err := validateRelaysResponse(missing, versions); err == nil {
		t.Fatal("incomplete patch-v1.3.0 FastRelay runtime was accepted")
	}

	mixed := currentFastMediaInventory()
	mixed.FastRelay = legacy.FastRelay
	if err := validateRelaysResponse(mixed, validFastMediaCapabilities().Capabilities); err == nil {
		t.Fatal("legacy FastRelay shape was accepted with the v1.3.1 FastMedia capability")
	}
}

func TestRelayInventoryRejectsMissingFrozenAdaptiveWireFields(t *testing.T) {
	versions := validRelayQualityCapabilities().Capabilities
	tests := []struct {
		name   string
		mutate func(map[string]any)
	}{
		{"Relay version", func(raw map[string]any) { delete(raw["relays"].([]any)[0].(map[string]any), "version") }},
		{"probe capability", func(raw map[string]any) {
			delete(raw["relays"].([]any)[0].(map[string]any)["capabilities"].(map[string]any), "relay_probe_protocol")
		}},
		{"telemetry authentication aggregate", func(raw map[string]any) {
			delete(raw["relays"].([]any)[0].(map[string]any)["websocket"].(map[string]any), "telemetry_auth_failures")
		}},
		{"nullable error message", func(raw map[string]any) {
			delete(raw["relays"].([]any)[0].(map[string]any)["websocket"].(map[string]any), "error_message")
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			encoded, err := json.Marshal(currentRelayInventory())
			if err != nil {
				t.Fatal(err)
			}
			var raw map[string]any
			if err := json.Unmarshal(encoded, &raw); err != nil {
				t.Fatal(err)
			}
			test.mutate(raw)
			encoded, err = json.Marshal(raw)
			if err != nil {
				t.Fatal(err)
			}
			var decoded starrycontrol.RelayInventory
			if err := json.Unmarshal(encoded, &decoded); err != nil {
				t.Fatal(err)
			}
			if err := validateRelaysResponse(decoded, versions); err == nil {
				t.Fatal("missing frozen adaptive field was accepted")
			}
		})
	}

	encoded, err := json.Marshal(currentRelayInventory())
	if err != nil {
		t.Fatal(err)
	}
	var raw map[string]any
	if err := json.Unmarshal(encoded, &raw); err != nil {
		t.Fatal(err)
	}
	raw["relays"].([]any)[0].(map[string]any)["websocket"].(map[string]any)["telemetry_auth_failures"] = nil
	encoded, err = json.Marshal(raw)
	if err != nil {
		t.Fatal(err)
	}
	var explicitlyNull starrycontrol.RelayInventory
	if err := json.Unmarshal(encoded, &explicitlyNull); err != nil {
		t.Fatal(err)
	}
	if err := validateRelaysResponse(explicitlyNull, versions); err != nil {
		t.Fatalf("explicitly-null frozen adaptive field was rejected: %v", err)
	}
}

func TestRelayQualityPlanRiskAndGenerationAcknowledgement(t *testing.T) {
	const instanceID = "0191f6a0-0000-7000-8000-000000000001"
	document := "version: 4\nrelay_quality:\n  enabled: true\n  strategy: adaptive\n"
	digest := documentDigest(document)
	etag := fmt.Sprintf("%q", digest)
	planID := "0191f6a0-0001-7000-8000-000000000010"
	operationID := "0191f6a0-0001-7000-8000-000000000011"
	planRisk := "medium"

	server := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		switch request.URL.Path {
		case "/control/v1/capabilities":
			capabilities := validRelayQualityCapabilities()
			capabilities.Instance.ID = instanceID
			_ = json.NewEncoder(writer).Encode(capabilities)
		case "/control/v1/config:validate":
			_ = json.NewEncoder(writer).Encode(starrycontrol.ValidationResult{
				Valid: true, SourceDigest: stringPointer(digest), EffectiveDigest: stringPointer(digest), Diagnostics: []starrycontrol.Diagnostic{},
			})
		case "/control/v1/config:plan":
			if request.Header.Get("If-Match") != etag {
				t.Errorf("plan If-Match = %q", request.Header.Get("If-Match"))
			}
			_, _ = fmt.Fprintf(writer, `{"plan_id":%q,"instance_id":%q,"base_etag":%q,"base_generation":42,"candidate_digest":%q,"changes":[{"pointer":"/relay_quality/strategy","kind":"replace"}],"impact":{"risk":%q,"restart_required":false},"expires_at":"2030-08-18T10:10:00Z"}`, planID, instanceID, etag, digest, planRisk)
		case "/control/v1/config:apply":
			if request.Header.Get("If-Match") != etag || request.Header.Get("Idempotency-Key") != "relay-quality-apply-0001" {
				t.Errorf("apply transaction headers = %#v", request.Header)
			}
			_, _ = fmt.Fprintf(writer, `{"id":%q,"audit_id":null,"kind":"config_apply","state":"succeeded","created_at":"2026-08-18T10:00:00Z","updated_at":"2026-08-18T10:00:01Z","activation_ack":{"generation":43,"schema_version":4,"source_digest":%q,"effective_digest":%q,"activated_at":"2026-08-18T10:00:01Z","audit_id":null,"subsystem_acks":[{"subsystem":"relay_quality","accepted":true,"detail":"generation 43 active"}]},"error":null}`, operationID, digest, digest)
		default:
			t.Errorf("unexpected path %s", request.URL.Path)
			writer.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	client, err := clientgen.New(server.URL, server.Client(), func(context.Context, string) (string, error) {
		return "control-token", nil
	}, 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	provider := &Provider{instanceID: instanceID, client: client}
	ctx := starrycontrol.WithRequestMetadata(context.Background(), starrycontrol.RequestMetadata{
		ActorUserID: 42, RequestID: "0191f6a0-0000-7000-8000-000000000099",
	})

	validation, err := provider.ValidateConfig(ctx, starrycontrol.ConfigCandidate{Document: document, Format: "yaml"})
	if err != nil || !validation.Valid || validation.SourceDigest == nil || *validation.SourceDigest != digest {
		t.Fatalf("validate = %#v, err=%v", validation, err)
	}
	plan, err := provider.PlanConfig(ctx, starrycontrol.ConfigCandidate{Document: document, Format: "yaml", BaseETag: etag})
	if err != nil || plan.Impact.Risk != "medium" || plan.CandidateDigest != digest {
		t.Fatalf("plan = %#v, err=%v", plan, err)
	}
	operation, err := provider.ApplyConfig(ctx, starrycontrol.ApplyRequest{
		PlanID: plan.PlanID, CandidateDigest: plan.CandidateDigest, IfMatch: etag,
		IdempotencyKey: "relay-quality-apply-0001",
	})
	if err != nil || operation.ActivationAck == nil || operation.ActivationAck.Generation != 43 ||
		operation.ActivationAck.SourceDigest != digest || operation.ActivationAck.SchemaVersion != 4 {
		t.Fatalf("apply = %#v, err=%v", operation, err)
	}

	planRisk = "low"
	plan, err = provider.PlanConfig(ctx, starrycontrol.ConfigCandidate{Document: document, Format: "yaml", BaseETag: etag})
	if err != nil || plan.Impact.Risk != "medium" {
		t.Fatalf("Relay Quality risk floor = %#v, err=%v", plan.Impact, err)
	}
}

func TestRelayQualityRiskFloorCoversWholeDocumentPlan(t *testing.T) {
	changes := []json.RawMessage{json.RawMessage(`{"pointer":"","kind":"add"}`)}
	if !planTouchesRelayQuality(changes) {
		t.Fatal("whole-document plan must conservatively receive the Relay Quality risk floor")
	}
	if planTouchesRelayQuality([]json.RawMessage{json.RawMessage(`{"pointer":"/log_level","kind":"replace"}`)}) {
		t.Fatal("unrelated precise change unexpectedly received the Relay Quality risk floor")
	}
}

func validRelayQualityCapabilities() starrycontrol.Capabilities {
	return starrycontrol.Capabilities{
		Protocol: starrycontrol.ProtocolInfo{Name: "starry-control", Version: "1.0.0", Major: 1},
		Instance: starrycontrol.InstanceInfo{
			ID: "0191f6a0-0000-7000-8000-000000000001", Role: "hbbs",
			StarryVersion: "1.1.16-patch-v1.3.0", UpstreamVersion: "1.1.16",
		},
		Capabilities: starrycontrol.CapabilityVersions{
			RelayInventory: 1, AllocationSimulation: 1, ConfigTransaction: 1, ConfigRollback: 1,
			ConnectionAuth: 1, RelayQuality: 1, RelayActiveProbe: 1, RelayProbeProtocol: 1,
			RelayLoadProtocol: 1, RelayTelemetrySchema: 1, FastRelayAuthorization: 1, PeerRegistry: 2,
		},
		Config: starrycontrol.ConfigCapabilities{
			SupportedSchemaVersions: []int{1, 2, 3, 4}, ActiveSchemaVersion: 4,
			SchemaDigest: "sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc",
		},
		Limits: starrycontrol.AgentLimits{MaxConfigBytes: 1 << 20, MaxPlanLifetimeSeconds: 600, OperationRetentionSeconds: 86400},
	}
}

func legacyRelayInventory() starrycontrol.RelayInventory {
	url := "wss://relay.example.test/ws/relay"
	lastProbe := time.Date(2026, 8, 18, 9, 59, 30, 0, time.UTC)
	latency := int64(83)
	return starrycontrol.RelayInventory{
		ConfigGeneration: 42, HealthSnapshotID: "health-17", Warning: "Relay probes do not prove a complete session.",
		Relays: []starrycontrol.Relay{{
			ID: "relay.example.test:21117", ConfiguredOrder: 0,
			Native:      starrycontrol.NativeRelayStatus{State: "online", ObservedAt: time.Date(2026, 8, 18, 10, 0, 0, 0, time.UTC)},
			WebSocket:   starrycontrol.WebSocketRelayStatus{Configured: true, URL: &url, State: "healthy", LastProbeAt: &lastProbe, LatencyMS: &latency},
			EligibleFor: []starrycontrol.Transport{starrycontrol.TransportNative, starrycontrol.TransportWSS}, ReferencedByRules: []string{"Asia preference"},
		}},
	}
}

func currentRelayInventory() starrycontrol.RelayInventory {
	value := legacyRelayInventory()
	value.Relays[0].Version = "1.1.16-patch-v1.3.0"
	value.Relays[0].Capabilities = &starrycontrol.RelayCapabilities{RelayProbeProtocol: intPointer(1), RelayLoadProtocol: intPointer(1)}
	value.Relays[0].QualityCandidate = boolPointer(true)
	websocket := &value.Relays[0].WebSocket
	observed := time.Date(2026, 8, 18, 9, 59, 30, 0, time.UTC)
	restarted := time.Date(2026, 8, 17, 10, 0, 0, 0, time.UTC)
	websocket.ObservedAt = &observed
	websocket.ObservedAtUnixMS = uint64Pointer(1787047170000)
	websocket.AgeSeconds = uint64Pointer(30)
	websocket.Stale = boolPointer(false)
	websocket.TelemetrySchema = intPointer(1)
	websocket.TelemetrySequence = uint64Pointer(3812)
	websocket.UptimeSeconds = uint64Pointer(86400)
	websocket.TelemetryRestarts = uint64Pointer(1)
	websocket.LastRestartAt = &restarted
	websocket.LoadBasisPoints = intPointer(2375)
	websocket.ActiveSessions = intPointer(19)
	websocket.PendingPairs = intPointer(3)
	websocket.CapacitySessions = intPointer(200)
	websocket.BandwidthBPS = uint64Pointer(123456700)
	websocket.BandwidthEMAAlphaBasisPoints = intPointer(2500)
	websocket.CapacityBandwidthBPS = uint64Pointer(1000000000)
	websocket.Draining = boolPointer(false)
	websocket.AdmissionOpen = boolPointer(true)
	websocket.AdmissionRejections = uint64Pointer(4)
	websocket.ProbeMalformed = uint64Pointer(2)
	websocket.ProbeUnsupported = uint64Pointer(1)
	websocket.ProbeRateLimited = uint64Pointer(8)
	websocket.ProbeSuccessful = uint64Pointer(9200)
	websocket.TelemetryAuthFailures = uint64Pointer(0)
	value.Quality = &starrycontrol.RelayQualityRuntime{
		ProtocolVersion: 1, Strategy: "adaptive", Enabled: boolPointer(true), ActiveAllocations: uint64Pointer(7),
		CachedNetworkPairs: uint64Pointer(42), PendingDecisions: uint64Pointer(2), OffersCreated: uint64Pointer(1200), OffersSkipped: uint64Pointer(40),
		OfferSkipReasons: &starrycontrol.RelayQualityOfferSkipReason{
			Disabled: uint64Pointer(0), UnsupportedClient: uint64Pointer(25), InvalidFallback: uint64Pointer(0),
			InconsistentSnapshot: uint64Pointer(1), InsufficientCandidates: uint64Pointer(13), PrimaryNotProbeable: uint64Pointer(1),
		},
		PeerReportsAccepted: uint64Pointer(1168), ControllerReportsAccepted: uint64Pointer(1160), ReportsAccepted: uint64Pointer(2328),
		ReportsDuplicate: uint64Pointer(9), ReportsStageMismatch: uint64Pointer(2), ReportsLate: uint64Pointer(3),
		ReportsInvalid: uint64Pointer(5), ReportsBindingMismatch: uint64Pointer(2), DecisionsCreated: uint64Pointer(1180),
		FallbackDecisions: uint64Pointer(12),
		FallbackReasons: &starrycontrol.RelayQualityFallbackReason{
			LegacyFallback: uint64Pointer(0), ProbeFailure: uint64Pointer(7), ManualOverride: uint64Pointer(0), InvalidReport: uint64Pointer(3), ReportLate: uint64Pointer(2),
		},
		CacheHits: uint64Pointer(730), HysteresisDecisions: uint64Pointer(142), PrimaryProbes: uint64Pointer(2100),
		PrimaryAccepted: uint64Pointer(920), ExpansionsTriggered: uint64Pointer(260), P2PCancellations: uint64Pointer(18),
		EstimatedProbeAttemptsSaved: uint64Pointer(17440), ExpandedDecisions: uint64Pointer(248), StageTimeouts: uint64Pointer(4),
		RelaySelections: map[string]uint64{"relay.example.test:21117": 1168}, RelaySelectionOverflow: uint64Pointer(0),
	}
	value.FastRelay = &starrycontrol.FastRelayRuntime{
		ProtocolVersion: 1, Enabled: boolPointer(false), ActiveAuthorizations: uint64Pointer(0), Issued: uint64Pointer(0),
		Reused: uint64Pointer(0), Delivered: uint64Pointer(0), Disabled: uint64Pointer(0), InsecureRequests: uint64Pointer(0),
		InvalidConfiguration: uint64Pointer(0), InvalidUUIDs: uint64Pointer(0), MissingSigningKeys: uint64Pointer(0),
		SigningFailures: uint64Pointer(0), QualitySelectionFailures: uint64Pointer(0), RateLimited: uint64Pointer(0),
		ResponseMisses: uint64Pointer(0), ExpiredCacheEvictions: uint64Pointer(0),
	}
	return value
}

func cloneRelayInventory(t *testing.T, input starrycontrol.RelayInventory) starrycontrol.RelayInventory {
	t.Helper()
	encoded, err := json.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	result := starrycontrol.RelayInventory{}
	if err := json.Unmarshal(encoded, &result); err != nil {
		t.Fatal(err)
	}
	return result
}

func boolPointer(value bool) *bool       { return &value }
func intPointer(value int) *int          { return &value }
func uint64Pointer(value uint64) *uint64 { return &value }
func stringPointer(value string) *string { return &value }
