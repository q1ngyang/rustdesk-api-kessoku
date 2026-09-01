package starry

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/q1ngyang/rustdesk-api-kessoku/v3/config"
	"github.com/q1ngyang/rustdesk-api-kessoku/v3/internal/starrycontrol"
)

const (
	realAgentLegacyCandidate   = "version: 3\nrelay_servers:\n  - relay-kessoku-e2e.example.test:21117\n"
	realAgentAdaptiveCandidate = "version: 4\nrelay_servers:\n  - relay-kessoku-e2e.example.test:21117\nrelay_quality:\n  enabled: false\n  strategy: adaptive\n"
	realAgentEagerCandidate    = "version: 4\nrelay_servers:\n  - relay-kessoku-e2e.example.test:21117\nrelay_quality:\n  enabled: false\n  strategy: eager\n"
)

// TestRealStarryAgentProviderE2E is intentionally environment-gated. Starry's
// integration harness starts real HBBS and Control Agent processes, provisions
// ephemeral mTLS/JWT identities, and invokes this prebuilt Go test binary.
func TestRealStarryAgentProviderE2E(t *testing.T) {
	baseURL := os.Getenv("STARRY_E2E_BASE_URL")
	if baseURL == "" {
		t.Skip("real Starry Agent harness is not active")
	}
	required := func(name string) string {
		t.Helper()
		value := os.Getenv(name)
		if value == "" {
			t.Fatalf("%s is required by the real Agent harness", name)
		}
		return value
	}

	instanceID := required("STARRY_E2E_INSTANCE_ID")
	provider, err := NewProvider(config.StarryInstance{
		ID:                 "starry-e2e",
		Name:               "Starry cross-repository E2E",
		Enabled:            true,
		BaseURL:            baseURL,
		ExpectedInstanceID: instanceID,
		TLSServerName:      required("STARRY_E2E_TLS_SERVER_NAME"),
		CAFile:             required("STARRY_E2E_CA_FILE"),
		ClientCertFile:     required("STARRY_E2E_CLIENT_CERT_FILE"),
		ClientKeyFile:      required("STARRY_E2E_CLIENT_KEY_FILE"),
		ControlKeyFile:     required("STARRY_E2E_CONTROL_KEY_FILE"),
		ControlKeyID:       required("STARRY_E2E_CONTROL_KEY_ID"),
		ControlIssuer:      required("STARRY_E2E_CONTROL_ISSUER"),
		AuthorizedParty:    required("STARRY_E2E_AUTHORIZED_PARTY"),
	}, config.ServerControl{RequestTimeout: 5 * time.Second, ResponseMaxBytes: 1 << 20})
	if err != nil {
		t.Fatal(err)
	}
	ctx := starrycontrol.WithRequestMetadata(context.Background(), starrycontrol.RequestMetadata{
		ActorUserID: 42,
		RequestID:   "0191f6a0-0002-7000-8000-000000000001",
	})

	capabilities, err := provider.Capabilities(ctx)
	if err != nil || capabilities.Instance.ID != instanceID || capabilities.Capabilities.ConfigTransaction != 1 || capabilities.Capabilities.PeerRegistry < 1 {
		t.Fatalf("capabilities = %+v, err=%v", capabilities, err)
	}
	if capabilities.Capabilities.RelayQuality == 1 && capabilities.Capabilities.PeerRegistry != 2 {
		t.Fatalf("Relay Quality-capable server must expose peer_registry=2: %+v", capabilities.Capabilities)
	}
	if capabilities.Capabilities.PeerRegistry >= 2 {
		verification, err := provider.VerifyPeer(ctx, starrycontrol.PeerIdentityInput{
			ID:              "301132036",
			UUID:            "AQEBAQEBAQEBAQEBAQEBAQ==",
			ActivationEpoch: 1,
			ActivationID:    "AgICAgICAgICAgICAgICAg==",
			RouteLeases:     []string{"AwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwM="},
		})
		if err != nil || verification.InstanceID != instanceID || verification.Registered {
			t.Fatalf("unknown peer verification = %+v, err=%v", verification, err)
		}
	}
	status, err := provider.Status(ctx)
	if err != nil || !status.Ready || status.Config.Status != "active" {
		t.Fatalf("status = %+v, err=%v", status, err)
	}
	inventory, err := provider.Relays(ctx)
	if err != nil || inventory.Relays == nil {
		t.Fatalf("relays = %+v, err=%v", inventory, err)
	}
	simulation, err := provider.SimulateAllocation(ctx, starrycontrol.SimulationInput{
		ClientA:                  starrycontrol.SimulationClient{IP: "192.0.2.10"},
		ClientB:                  starrycontrol.SimulationClient{IP: "2001:db8::20"},
		Transport:                starrycontrol.TransportNative,
		Explain:                  true,
		ExpectedConfigGeneration: &inventory.ConfigGeneration,
	})
	if err != nil || !simulation.Selection.NonBinding || simulation.ConfigGeneration != inventory.ConfigGeneration {
		t.Fatalf("simulation = %+v, err=%v", simulation, err)
	}

	initial, err := provider.GetConfig(ctx)
	if err != nil || initial.Document == "" || initial.SourceDigest == nil {
		t.Fatalf("initial config = %+v, err=%v", initial, err)
	}
	schema, err := provider.GetConfigSchema(ctx)
	if err != nil || schema.Digest != capabilities.Config.SchemaDigest {
		t.Fatalf("schema = %+v, err=%v", schema, err)
	}
	invalid, err := provider.ValidateConfig(ctx, starrycontrol.ConfigCandidate{Document: "version: nope\n", Format: "yaml"})
	if err != nil || invalid.Valid || len(invalid.Diagnostics) == 0 {
		t.Fatalf("invalid validation = %+v, err=%v", invalid, err)
	}
	candidates := []struct {
		document       string
		idempotencyKey string
		strategy       string
	}{
		{realAgentLegacyCandidate, "kessoku-e2e-apply-0001", ""}, // gitleaks:allow -- deterministic non-secret test key
	}
	if capabilities.Capabilities.RelayQuality == 1 {
		candidates = []struct {
			document       string
			idempotencyKey string
			strategy       string
		}{
			{realAgentAdaptiveCandidate, "kessoku-e2e-apply-adaptive-0001", "adaptive"}, // gitleaks:allow -- deterministic non-secret test key
			{realAgentEagerCandidate, "kessoku-e2e-apply-eager-0001", "eager"},          // gitleaks:allow -- deterministic non-secret test key
		}
	}
	current := initial
	for _, candidate := range candidates {
		valid, err := provider.ValidateConfig(ctx, starrycontrol.ConfigCandidate{Document: candidate.document, Format: "yaml"})
		if err != nil || !valid.Valid || valid.SourceDigest == nil {
			t.Fatalf("valid validation = %+v, err=%v", valid, err)
		}
		plan, err := provider.PlanConfig(ctx, starrycontrol.ConfigCandidate{
			Document: candidate.document,
			Format:   "yaml",
			BaseETag: current.ETag,
		})
		if err != nil {
			t.Fatal(err)
		}
		if candidate.strategy != "" && plan.Impact.Risk != "medium" {
			t.Fatalf("Relay Quality plan risk = %q, want medium", plan.Impact.Risk)
		}
		accepted, err := provider.ApplyConfig(ctx, starrycontrol.ApplyRequest{
			PlanID:          plan.PlanID,
			CandidateDigest: plan.CandidateDigest,
			IfMatch:         current.ETag,
			IdempotencyKey:  candidate.idempotencyKey,
			Comment:         "Kessoku provider cross-repository E2E",
		})
		if err != nil || accepted.State != "pending" {
			t.Fatalf("accepted apply = %+v, err=%v", accepted, err)
		}
		applied := waitForRealAgentOperation(t, ctx, provider, accepted.ID)
		if applied.State != "succeeded" || applied.ActivationAck == nil || applied.ActivationAck.SourceDigest != plan.CandidateDigest {
			t.Fatalf("completed apply = %+v", applied)
		}
		if candidate.strategy != "" && (applied.ActivationAck.SchemaVersion != 4 ||
			applied.ActivationAck.Generation <= current.Generation) {
			t.Fatalf("Relay Quality activation ACK = %+v", applied.ActivationAck)
		}
		for _, subsystem := range applied.ActivationAck.SubsystemAcks {
			if !subsystem.Accepted {
				t.Fatalf("activation subsystem rejected = %+v", subsystem)
			}
		}
		current, err = provider.GetConfig(ctx)
		if err != nil || current.Document != candidate.document {
			t.Fatalf("applied config = %+v, err=%v", current, err)
		}
		if candidate.strategy != "" {
			inventory, err := provider.Relays(ctx)
			if err != nil || inventory.Quality == nil || inventory.Quality.Strategy != candidate.strategy {
				t.Fatalf("applied Relay Quality strategy = %+v, err=%v", inventory.Quality, err)
			}
		}
	}

	history, err := provider.ConfigHistory(ctx)
	if err != nil {
		t.Fatal(err)
	}
	baselineID := ""
	for _, revision := range history {
		if revision.Result == "baseline" && revision.AfterETag == initial.ETag {
			baselineID = revision.ID
			break
		}
	}
	if baselineID == "" {
		t.Fatalf("rollback baseline missing from history: %+v", history)
	}
	rollback, err := provider.RollbackConfig(ctx, starrycontrol.RollbackRequest{
		RevisionID:     baselineID,
		IfMatch:        current.ETag,
		IdempotencyKey: "kessoku-e2e-rollback-0001",
		Comment:        "Restore the Starry harness baseline",
	})
	if err != nil || rollback.State != "pending" {
		t.Fatalf("accepted rollback = %+v, err=%v", rollback, err)
	}
	rolledBack := waitForRealAgentOperation(t, ctx, provider, rollback.ID)
	if rolledBack.State != "succeeded" || rolledBack.Kind != "config_rollback" {
		t.Fatalf("completed rollback = %+v", rolledBack)
	}
	restored, err := provider.GetConfig(ctx)
	if err != nil || restored.Document != initial.Document || restored.SourceDigest == nil {
		t.Fatalf("restored config = %+v, err=%v", restored, err)
	}
	reloaded, err := provider.ReloadRuntime(ctx, starrycontrol.RuntimeReloadRequest{
		ExpectedSourceDigest: *restored.SourceDigest,
		IdempotencyKey:       "kessoku-e2e-reload-0001",
	})
	if err != nil || reloaded.SourceDigest != *restored.SourceDigest {
		t.Fatalf("runtime reload = %+v, err=%v", reloaded, err)
	}
}

func waitForRealAgentOperation(t *testing.T, ctx context.Context, provider *Provider, operationID string) starrycontrol.Operation {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		operation, err := provider.Operation(ctx, operationID)
		if err != nil {
			t.Fatal(err)
		}
		switch operation.State {
		case "pending", "running":
			time.Sleep(50 * time.Millisecond)
		default:
			return operation
		}
	}
	t.Fatalf("operation %s did not complete", operationID)
	return starrycontrol.Operation{}
}
