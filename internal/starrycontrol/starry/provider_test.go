package starry

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/q1ngyang/rustdesk-api-kessoku/v3/internal/starrycontrol"
	"github.com/q1ngyang/rustdesk-api-kessoku/v3/internal/starrycontrol/clientgen"
)

func TestTypedProviderSimulationAndConfigTransactions(t *testing.T) {
	const (
		instanceID  = "0191f6a0-0000-7000-8000-000000000001"
		configETag  = "\"sha256:7b02700a93eb8af7192724bd1b5ff49fd340f0ef81cfee1fb0d8cdc1467fea8c\""
		candidate   = "sha256:7b02700a93eb8af7192724bd1b5ff49fd340f0ef81cfee1fb0d8cdc1467fea8c"
		planID      = "0191f6a0-0000-7000-8000-000000000002"
		operationID = "0191f6a0-0000-7000-8000-000000000003"
	)
	var mu sync.Mutex
	var calls []string
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		calls = append(calls, r.Method+" "+r.URL.Path)
		mu.Unlock()
		if r.Header.Get("Authorization") != "Bearer control-token" || r.Header.Get("X-Request-ID") == "" {
			t.Errorf("missing control authentication headers")
		}
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/control/v1/capabilities":
			_, _ = w.Write([]byte(`{"protocol":{"name":"starry-control","version":"1.0.0","major":1},"instance":{"id":"` + instanceID + `","role":"hbbs","starry_version":"1.1.16-patch-v1.2.2","upstream_version":"1.1.16"},"capabilities":{"relay_inventory":1,"allocation_simulation":1,"config_transaction":1,"config_rollback":1,"connection_auth":1},"config":{"supported_schema_versions":[1,2,3],"active_schema_version":3,"schema_digest":"sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"},"limits":{"max_config_bytes":1048576,"max_plan_lifetime_seconds":600,"operation_retention_seconds":86400},"future_extension":{"ignored":true}}`))
		case "/control/v1/allocations:simulate":
			var input starrycontrol.SimulationInput
			if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
				t.Error(err)
			}
			if input.ClientA.IP != "192.0.2.10" || input.ClientB.IP != "2001:db8::20" || input.Transport != starrycontrol.TransportMixed {
				t.Errorf("unexpected simulation input: %+v", input)
			}
			_, _ = w.Write([]byte(`{"config_generation":42,"health_snapshot_id":"snapshot","matched_rule":null,"candidates":[],"selection":{"kind":"no_eligible_relay","relay_id":null,"predicted_index":null,"non_binding":true},"warnings":["simulation has no side effects"]}`))
		case "/control/v1/peers:verify":
			var input starrycontrol.PeerIdentityInput
			if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
				t.Error(err)
			}
			if input.ID != "301132036" || input.UUID != "MDEyMzQ1Njc4OWFiY2RlZg==" {
				t.Errorf("unexpected peer identity: %+v", input)
			}
			_, _ = w.Write([]byte(`{"instance_id":"` + instanceID + `","registered":true}`))
		case "/control/v1/config":
			w.Header().Set("ETag", configETag)
			_, _ = w.Write([]byte(fmt.Sprintf(`{"status":"active","generation":42,"schema_version":3,"source_digest":"%s","effective_digest":"sha256:dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd","activated_at":"2026-08-18T10:00:00Z","subsystem_acks":[{"subsystem":"config_core","accepted":true,"detail":"active"}],"last_error":null,"etag":%q,"drift":false,"document":"version: 3\n","format":"yaml"}`, candidate, configETag)))
		case "/control/v1/config:plan":
			if r.Header.Get("If-Match") != configETag {
				t.Errorf("plan missing If-Match")
			}
			var body map[string]interface{}
			_ = json.NewDecoder(r.Body).Decode(&body)
			if _, leaked := body["base_etag"]; leaked {
				t.Error("base_etag leaked into transport body")
			}
			_, _ = w.Write([]byte(fmt.Sprintf(`{"plan_id":"%s","instance_id":"%s","base_etag":%q,"base_generation":42,"candidate_digest":"%s","changes":[],"impact":{"risk":"low","restart_required":false},"expires_at":"2026-08-18T10:00:00Z"}`, planID, instanceID, configETag, candidate)))
		case "/control/v1/config:apply":
			if r.Header.Get("If-Match") != configETag || r.Header.Get("Idempotency-Key") != "control-apply-0001" {
				t.Errorf("apply transaction headers missing: %+v", r.Header)
			}
			var body map[string]interface{}
			_ = json.NewDecoder(r.Body).Decode(&body)
			if body["candidate_digest"] != candidate {
				t.Errorf("candidate digest missing from apply: %+v", body)
			}
			_, _ = w.Write([]byte(`{"id":"` + operationID + `","audit_id":null,"kind":"config_apply","state":"pending","created_at":"2026-08-18T10:00:00Z","updated_at":"2026-08-18T10:00:00Z","activation_ack":null,"error":null}`))
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
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
		ActorUserID: 42,
		RequestID:   "0191f6a0-0000-7000-8000-000000000001",
	})

	if _, err := provider.Capabilities(ctx); err != nil {
		t.Fatal(err)
	}
	verification, err := provider.VerifyPeer(ctx, starrycontrol.PeerIdentityInput{ID: "301132036", UUID: "MDEyMzQ1Njc4OWFiY2RlZg=="})
	if err != nil || !verification.Registered || verification.InstanceID != instanceID {
		t.Fatalf("peer verification = %+v, err=%v", verification, err)
	}
	expectedGeneration := uint64(42)
	simulation, err := provider.SimulateAllocation(ctx, starrycontrol.SimulationInput{
		ClientA:                  starrycontrol.SimulationClient{IP: "192.0.2.10"},
		ClientB:                  starrycontrol.SimulationClient{IP: "2001:db8::20"},
		Transport:                starrycontrol.TransportMixed,
		Explain:                  true,
		ExpectedConfigGeneration: &expectedGeneration,
	})
	if err != nil || simulation.ConfigGeneration != 42 || !simulation.Selection.NonBinding {
		t.Fatalf("simulation = %+v, err=%v", simulation, err)
	}
	staleGeneration := uint64(41)
	if _, err := provider.SimulateAllocation(ctx, starrycontrol.SimulationInput{
		ClientA:                  starrycontrol.SimulationClient{IP: "192.0.2.10"},
		ClientB:                  starrycontrol.SimulationClient{IP: "2001:db8::20"},
		Transport:                starrycontrol.TransportMixed,
		ExpectedConfigGeneration: &staleGeneration,
	}); err == nil {
		t.Fatal("simulation response from a different config generation was accepted")
	}
	beforeInvalid := len(calls)
	if _, err := provider.SimulateAllocation(ctx, starrycontrol.SimulationInput{
		ClientA: starrycontrol.SimulationClient{IP: "not-an-ip"}, ClientB: starrycontrol.SimulationClient{IP: "192.0.2.2"}, Transport: starrycontrol.TransportNative,
	}); err == nil {
		t.Fatal("invalid IP accepted")
	}
	if len(calls) != beforeInvalid {
		t.Fatal("invalid simulation reached Agent")
	}

	document, err := provider.GetConfig(ctx)
	if err != nil || document.ETag != configETag || document.Document != "version: 3\n" {
		t.Fatalf("config = %+v, err=%v", document, err)
	}
	plan, err := provider.PlanConfig(ctx, starrycontrol.ConfigCandidate{Document: "version: 3\n", Format: "yaml", BaseETag: configETag})
	if err != nil || plan.PlanID != planID {
		t.Fatalf("plan = %+v, err=%v", plan, err)
	}
	apply, err := provider.ApplyConfig(ctx, starrycontrol.ApplyRequest{PlanID: plan.PlanID, CandidateDigest: plan.CandidateDigest, IfMatch: configETag, IdempotencyKey: "control-apply-0001"})
	if err != nil || apply.ID != operationID {
		t.Fatalf("apply = %+v, err=%v", apply, err)
	}
}

func TestProviderRefusesUnsupportedCapabilityVersion(t *testing.T) {
	const instanceID = "0191f6a0-0000-7000-8000-000000000010"
	relayCalls := 0
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/control/v1/capabilities" {
			_, _ = w.Write([]byte(providerCapabilitiesJSON(instanceID, 2)))
			return
		}
		relayCalls++
		_, _ = w.Write([]byte(`{"relays":[]}`))
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
		ActorUserID: 42,
		RequestID:   "0191f6a0-0000-7000-8000-000000000001",
	})
	if _, err := provider.Relays(ctx); err == nil {
		t.Fatal("unsupported relay capability version was used")
	}
	if relayCalls != 0 {
		t.Fatal("relay endpoint was called despite incompatible capability")
	}
}

func TestProviderVerifiesInstanceIdentityBeforeEveryOperation(t *testing.T) {
	const (
		expectedID    = "0191f6a0-0000-7000-8000-000000000020"
		replacementID = "0191f6a0-0000-7000-8000-000000000021"
	)
	capabilityCalls := 0
	statusCalls := 0
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/control/v1/capabilities":
			capabilityCalls++
			instanceID := expectedID
			if capabilityCalls == 2 {
				instanceID = replacementID
			}
			_, _ = w.Write([]byte(providerCapabilitiesJSON(instanceID, 1)))
		case "/control/v1/status":
			statusCalls++
			_, _ = w.Write([]byte(`{"ready":true,"config":{"status":"active","generation":1,"schema_version":3,"source_digest":"sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","effective_digest":"sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb","activated_at":"2026-08-18T10:00:00Z","subsystem_acks":[],"last_error":null},"auth":{"configured_mode":"off","effective_mode":"off","verifier_state":"disabled","key_count":0,"key_age_seconds":null,"metrics":{"attempts":0,"allowed":0,"denied":0,"audit_would_deny":0,"cache_hits":0,"introspection_requests":0,"introspection_failures":0}}}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	client, err := clientgen.New(server.URL, server.Client(), func(context.Context, string) (string, error) {
		return "control-token", nil
	}, 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	provider := &Provider{instanceID: expectedID, client: client}
	ctx := starrycontrol.WithRequestMetadata(context.Background(), starrycontrol.RequestMetadata{
		ActorUserID: 42,
		RequestID:   "0191f6a0-0000-7000-8000-000000000001",
	})

	if _, err := provider.Status(ctx); err != nil {
		t.Fatalf("first health request failed: %v", err)
	}
	if _, err := provider.Status(ctx); err == nil {
		t.Fatal("replacement instance passed identity handshake")
	}
	if capabilityCalls != 2 || statusCalls != 1 {
		t.Fatalf("capabilities=%d status=%d", capabilityCalls, statusCalls)
	}
}

func TestProviderRejectsResponseMissingRequiredField(t *testing.T) {
	const instanceID = "0191f6a0-0000-7000-8000-000000000030"
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/control/v1/capabilities":
			_, _ = w.Write([]byte(providerCapabilitiesJSON(instanceID, 1)))
		case "/control/v1/status":
			// auth is a required contract field. Incremental fields remain
			// allowed, but deleting a required field must fail closed.
			_, _ = w.Write([]byte(`{"ready":true,"config":{"status":"disabled_no_config","generation":0,"schema_version":null,"source_digest":null,"effective_digest":null,"activated_at":null,"subsystem_acks":[],"last_error":null},"future_extension":true}`))
		default:
			w.WriteHeader(http.StatusNotFound)
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
		ActorUserID: 42,
		RequestID:   "0191f6a0-0000-7000-8000-000000000001",
	})
	_, err = provider.Status(ctx)
	var providerError *starrycontrol.ProviderError
	if !errors.As(err, &providerError) || providerError.Code != "CONTRACT_RESPONSE_INVALID" || providerError.Status != http.StatusBadGateway {
		t.Fatalf("missing required field error = %#v, err=%v", providerError, err)
	}
}

func TestProviderRejectsBindingSimulationAndUnsafeComments(t *testing.T) {
	selection := "relay-1"
	priority := 1
	unsafeSimulation := starrycontrol.SimulationResult{
		ConfigGeneration: 1,
		HealthSnapshotID: "snapshot-1",
		Candidates: []starrycontrol.AllocationCandidate{{
			RelayID:         "relay-1",
			ConfiguredOrder: 0,
			Priority:        &priority,
			Eligible:        true,
		}},
		Selection: starrycontrol.AllocationSelection{
			Kind:       "geo_rule",
			RelayID:    &selection,
			NonBinding: false,
		},
	}
	if err := validateSimulationResponse(unsafeSimulation); err == nil {
		t.Fatal("binding allocation simulation response was accepted")
	}
	unsafeSimulation.Selection.NonBinding = true
	unsafeSimulation.ConfigGeneration = 0
	if err := validateSimulationResponse(unsafeSimulation); err == nil {
		t.Fatal("zero-generation allocation simulation response was accepted")
	}

	if validComment("approved\nforged-log-line") {
		t.Fatal("control character in configuration comment was accepted")
	}
}

func TestProviderRejectsRollbackKindAndReloadDigestMismatch(t *testing.T) {
	const (
		instanceID  = "0191f6a0-0000-7000-8000-000000000040"
		revisionID  = "0191f6a0-0000-7000-8000-000000000041"
		operationID = "0191f6a0-0000-7000-8000-000000000042"
	)
	expectedDigest := "sha256:" + strings.Repeat("a", 64)
	otherDigest := "sha256:" + strings.Repeat("b", 64)
	etag := `"` + expectedDigest + `"`
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/control/v1/capabilities":
			_, _ = w.Write([]byte(providerCapabilitiesJSON(instanceID, 1)))
		case "/control/v1/config:rollback":
			_, _ = w.Write([]byte(`{"id":"` + operationID + `","audit_id":null,"kind":"config_apply","state":"pending","created_at":"2026-08-18T10:00:00Z","updated_at":"2026-08-18T10:00:00Z","activation_ack":null,"error":null}`))
		case "/control/v1/runtime:reload":
			_, _ = w.Write([]byte(`{"generation":42,"schema_version":3,"source_digest":"` + otherDigest + `","effective_digest":"` + otherDigest + `","activated_at":"2026-08-18T10:00:00Z","audit_id":null,"subsystem_acks":[{"subsystem":"config_core","accepted":true,"detail":"active"}]}`))
		default:
			w.WriteHeader(http.StatusNotFound)
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
		ActorUserID: 42,
		RequestID:   "0191f6a0-0000-7000-8000-000000000043",
	})
	if _, err := provider.RollbackConfig(ctx, starrycontrol.RollbackRequest{
		RevisionID: revisionID, IfMatch: etag, IdempotencyKey: "rollback-request-0001",
	}); err == nil {
		t.Fatal("rollback response with apply kind was accepted")
	}
	if _, err := provider.ReloadRuntime(ctx, starrycontrol.RuntimeReloadRequest{
		ExpectedSourceDigest: expectedDigest, IdempotencyKey: "runtime-reload-0001",
	}); err == nil {
		t.Fatal("runtime reload response with a different source digest was accepted")
	}
}

func TestConfigResponseDriftMustMatchTheExactManagedDocument(t *testing.T) {
	document := "version: 3\n"
	digest := documentDigest(document)
	otherDigest := "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	schemaVersion := 3
	activatedAt := time.Date(2026, 8, 18, 10, 0, 0, 0, time.UTC)
	activeState := starrycontrol.RuntimeConfigState{
		Status:          "active",
		Generation:      1,
		SchemaVersion:   &schemaVersion,
		SourceDigest:    &digest,
		EffectiveDigest: &digest,
		ActivatedAt:     &activatedAt,
		SubsystemAcks:   []starrycontrol.SubsystemAck{},
	}
	valid := starrycontrol.ConfigDocument{
		RuntimeConfigState: activeState,
		ETag:               `"` + digest + `"`,
		Drift:              false,
		Document:           document,
		Format:             "yaml",
	}
	if err := validateConfigResponse(valid); err != nil {
		t.Fatalf("valid response rejected: %v", err)
	}

	incorrect := valid
	incorrect.SourceDigest = &otherDigest
	if err := validateConfigResponse(incorrect); err == nil {
		t.Fatal("a changed managed document reported as drift=false was accepted")
	}

	incorrect = valid
	incorrect.Drift = true
	if err := validateConfigResponse(incorrect); err == nil {
		t.Fatal("an unchanged managed document reported as drift=true was accepted")
	}

	emptyDigest := documentDigest("")
	disabled := starrycontrol.ConfigDocument{
		RuntimeConfigState: starrycontrol.RuntimeConfigState{
			Status:        "disabled_no_config",
			SubsystemAcks: []starrycontrol.SubsystemAck{},
		},
		ETag:     `"` + emptyDigest + `"`,
		Drift:    false,
		Document: "",
		Format:   "yaml",
	}
	if err := validateConfigResponse(disabled); err != nil {
		t.Fatalf("empty unmanaged response rejected: %v", err)
	}
	disabled.Drift = true
	if err := validateConfigResponse(disabled); err == nil {
		t.Fatal("empty unmanaged document reported as drift=true was accepted")
	}
}

func providerCapabilitiesJSON(instanceID string, relayVersion int) string {
	return fmt.Sprintf(`{"protocol":{"name":"starry-control","version":"1.0.0","major":1},"instance":{"id":"%s","role":"hbbs","starry_version":"1.1.16-patch-v1.2.2","upstream_version":"1.1.16"},"capabilities":{"relay_inventory":%d,"allocation_simulation":1,"config_transaction":1,"config_rollback":1,"connection_auth":1,"peer_registry":1},"config":{"supported_schema_versions":[1,2,3],"active_schema_version":3,"schema_digest":"sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"},"limits":{"max_config_bytes":1048576,"max_plan_lifetime_seconds":600,"operation_retention_seconds":86400}}`, instanceID, relayVersion)
}
