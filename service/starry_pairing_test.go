package service

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/q1ngyang/rustdesk-api-kessoku/v3/config"
	"github.com/q1ngyang/rustdesk-api-kessoku/v3/internal/servercontrolregistry"
	"github.com/q1ngyang/rustdesk-api-kessoku/v3/internal/starrycontrol"
)

func TestControlPairingClaimRecoveryAndManagedProviderRestart(t *testing.T) {
	securityAuditDatabase(t, true)
	root := filepath.Join(t.TempDir(), "server-control")
	cfg := pairingTestConfig(t, root)
	control := NewStarryControlService(cfg, nil, nil)
	if status := control.PairingStatusLocal(); status.Available || status.ErrorCode != "PAIRING_REGISTRY_NOT_INITIALIZED" {
		t.Fatalf("fresh service silently initialized its registry: %+v", status)
	}
	if _, err := os.Lstat(root); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("service startup created replacement registry state: %v", err)
	}
	ctx := pairingServiceContext()
	created, err := control.CreateControlPairing(ctx, ControlPairingCreateRequest{
		ManagedID: "starry-managed", Name: "Managed Starry", AgentOriginID: "primary", Action: "pair",
		Confirmation: "confirm:pair:starry-managed:primary",
	})
	if err != nil {
		t.Fatal(err)
	}
	payload := decodePairingCode(t, created.Code)
	instanceID := uuid.NewString()
	csr, fingerprint := pairingCSR(t, "starry.internal.test")
	claim := servercontrolregistry.ClaimRequest{
		Version: 1, Purpose: "control-agent", Action: "pair", EnrollmentID: payload.EnrollmentID,
		ConfigurationDigest: payload.ConfigurationDigest, Secret: payload.Secret,
		KeyFingerprint: fingerprint, CSRPEM: csr, InstanceID: &instanceID,
	}
	claim.RequestDigest = servercontrolregistry.ExpectedRequestDigest(claim)
	first, err := control.ClaimPairing(pairingServiceContext(), claim)
	if err != nil {
		t.Fatal(err)
	}
	bundle, ok := first.Bundle.(servercontrolregistry.ControlAgentBundle)
	if !ok || bundle.InstanceID != instanceID || bundle.AgentOrigin != "https://starry.internal.test:21120" || len(bundle.AllowedClientURISANs) != 1 {
		t.Fatalf("unexpected control bundle: %#v", first.Bundle)
	}
	instances := control.Instances()
	if len(instances) != 1 || !instances[0].Managed || !instances[0].ReadOnly || !instances[0].Available {
		t.Fatalf("managed provider was not loaded read-only: %+v", instances)
	}
	if err := control.Close(); err != nil {
		t.Fatal(err)
	}
	staticExport := filepath.Join(root, "exports", "starry-managed.static-instance.yaml")
	if err := os.Remove(staticExport); err != nil {
		t.Fatalf("remove static export to exercise restart repair: %v", err)
	}

	// Recreate the Kessoku service with the same data root. The registry,
	// generation and credentials must be reused without another pairing, and
	// the derived v3.0.7-compatible export must be repaired.
	restarted := NewStarryControlService(cfg, nil, nil)
	defer restarted.Close()
	if _, err := os.Stat(staticExport); err != nil {
		t.Fatalf("restart did not repair managed static export: %v", err)
	}
	restartedInstances := restarted.Instances()
	if len(restartedInstances) != 1 || !restartedInstances[0].Available || !restartedInstances[0].ReadOnly {
		t.Fatalf("restart did not recover managed provider: %+v", restartedInstances)
	}
	recovered, err := restarted.ClaimPairing(pairingServiceContext(), claim)
	if err != nil {
		t.Fatal(err)
	}
	recoveredBundle, ok := recovered.Bundle.(servercontrolregistry.ControlAgentBundle)
	if !ok || recoveredBundle.ServerCertificatePEM != bundle.ServerCertificatePEM {
		t.Fatal("lost response recovery did not return the exact stored certificate")
	}
	beforeRotation, err := restarted.registry.ManagedInstance(context.Background(), "starry-managed")
	if err != nil {
		t.Fatal(err)
	}
	rotated, err := restarted.CreateControlPairing(ctx, ControlPairingCreateRequest{
		ManagedID: "starry-managed", Name: "Managed Starry", AgentOriginID: "primary", Action: "rotate",
		TargetInstanceID: instanceID, Confirmation: "confirm:rotate:starry-managed:primary",
	})
	if err != nil {
		t.Fatal(err)
	}
	rotatedPayload := decodePairingCode(t, rotated.Code)
	rotatedCSR, rotatedFingerprint := pairingCSR(t, "starry.internal.test")
	rotatedClaim := servercontrolregistry.ClaimRequest{
		Version: 1, Purpose: "control-agent", Action: "rotate", EnrollmentID: rotatedPayload.EnrollmentID,
		ConfigurationDigest: rotatedPayload.ConfigurationDigest, Secret: rotatedPayload.Secret,
		KeyFingerprint: rotatedFingerprint, CSRPEM: rotatedCSR, InstanceID: &instanceID,
	}
	rotatedClaim.RequestDigest = servercontrolregistry.ExpectedRequestDigest(rotatedClaim)
	rotatedResponse, err := restarted.ClaimPairing(pairingServiceContext(), rotatedClaim)
	if err != nil {
		t.Fatal(err)
	}
	rotatedBundle, ok := rotatedResponse.Bundle.(servercontrolregistry.ControlAgentBundle)
	if !ok || rotatedBundle.InstanceID != instanceID || rotatedBundle.ClientCAPEM == bundle.ClientCAPEM {
		t.Fatalf("rotation did not preserve the instance UUID while replacing credentials: %#v", rotatedResponse.Bundle)
	}
	afterRotation, err := restarted.registry.ManagedInstance(context.Background(), "starry-managed")
	if err != nil {
		t.Fatal(err)
	}
	if afterRotation.InstanceUUID != beforeRotation.InstanceUUID || afterRotation.ClientKeyFile == beforeRotation.ClientKeyFile || !afterRotation.ReadOnly {
		t.Fatalf("unexpected rotated managed instance: before=%+v after=%+v", beforeRotation, afterRotation)
	}
	for _, path := range []string{beforeRotation.CAFile, beforeRotation.ClientCertFile, beforeRotation.ClientKeyFile, beforeRotation.ControlKeyFile} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("rotation removed rollback credential %s: %v", path, err)
		}
	}
	if _, err := restarted.CreateControlPairing(ctx, ControlPairingCreateRequest{
		ManagedID: "arbitrary", Name: "Arbitrary", AgentOriginID: "https://attacker.invalid", Action: "pair",
		Confirmation: "confirm:pair:arbitrary:https://attacker.invalid",
	}); err == nil {
		t.Fatal("browser-style arbitrary Agent URL bypassed the deployment allowlist")
	}
}

func TestRunningServiceHotLoadsRegistryCreatedBySeparateCLIProcess(t *testing.T) {
	securityAuditDatabase(t, true)
	root := filepath.Join(t.TempDir(), "server-control")
	cfg := pairingTestConfig(t, root)
	running := NewStarryControlService(cfg, nil, nil)
	defer running.Close()
	if running.PairingEnabled() {
		t.Fatal("missing registry unexpectedly enabled public pairing")
	}

	// A second service instance exercises the same independent-registry path as
	// the Cobra CLI. It performs the only action allowed to initialize state and
	// exits before the already-running service observes the new generation.
	cliProcess := NewStarryControlService(cfg, nil, nil)
	created, err := cliProcess.CreateControlPairingLocal(pairingServiceContext(), ControlPairingCreateRequest{
		ManagedID: "starry-cli", Name: "CLI-created Starry", AgentOriginID: "primary", Action: "pair",
		Confirmation: "confirm:pair:starry-cli:primary",
	})
	if err != nil {
		_ = cliProcess.Close()
		t.Fatal(err)
	}
	if err := cliProcess.Close(); err != nil {
		t.Fatal(err)
	}

	status := running.PairingStatusLocal()
	if !status.Available || status.RegistryGeneration == 0 || !running.PairingEnabled() {
		t.Fatalf("running service did not hot-load externally created registry: %+v", status)
	}
	payload := decodePairingCode(t, created.Code)
	instanceID := uuid.NewString()
	csr, fingerprint := pairingCSR(t, "starry.internal.test")
	claim := servercontrolregistry.ClaimRequest{
		Version: 1, Purpose: servercontrolregistry.PurposeControlAgent, Action: servercontrolregistry.ActionPair,
		EnrollmentID: payload.EnrollmentID, ConfigurationDigest: payload.ConfigurationDigest,
		Secret: payload.Secret, KeyFingerprint: fingerprint, CSRPEM: csr, InstanceID: &instanceID,
	}
	claim.RequestDigest = servercontrolregistry.ExpectedRequestDigest(claim)
	if _, err := running.ClaimPairing(pairingServiceContext(), claim); err != nil {
		t.Fatalf("running service could not claim CLI-created enrollment without restart: %v", err)
	}
	instances := running.Instances()
	if len(instances) != 1 || instances[0].ID != "starry-cli" || !instances[0].Managed || !instances[0].ReadOnly {
		t.Fatalf("CLI-created managed provider was not hot-loaded: %+v", instances)
	}
}

func TestControlPairingLearnsNewUUIDButPrebindsAdoptAndRotate(t *testing.T) {
	securityAuditDatabase(t, true)
	control := NewStarryControlService(pairingTestConfig(t, filepath.Join(t.TempDir(), "server-control")), nil, nil)
	defer control.Close()
	instanceID := uuid.NewString()
	if _, err := control.CreateControlPairing(pairingServiceContext(), ControlPairingCreateRequest{
		ManagedID: "targeted-pair", Name: "Targeted pair", AgentOriginID: "primary", Action: "pair",
		TargetInstanceID: instanceID, Confirmation: "confirm:pair:targeted-pair:primary",
	}); !errors.Is(err, starrycontrol.ErrRequestInvalid) {
		t.Fatalf("new pair accepted a caller-supplied instance UUID: %v", err)
	}
	for _, action := range []string{"adopt", "rotate"} {
		if _, err := control.CreateControlPairing(pairingServiceContext(), ControlPairingCreateRequest{
			ManagedID: action + "-missing-target", Name: "Missing target", AgentOriginID: "primary", Action: action,
			Confirmation: "confirm:" + action + ":" + action + "-missing-target:primary",
		}); !errors.Is(err, starrycontrol.ErrRequestInvalid) {
			t.Fatalf("%s accepted without exact instance UUID: %v", action, err)
		}
	}
}

func TestUnclaimedControlPairingCanBeRevokedAndRecreated(t *testing.T) {
	securityAuditDatabase(t, true)
	control := NewStarryControlService(pairingTestConfig(t, filepath.Join(t.TempDir(), "server-control")), nil, nil)
	defer control.Close()
	created, err := control.CreateControlPairing(pairingServiceContext(), ControlPairingCreateRequest{
		ManagedID: "revocable", Name: "Revocable", AgentOriginID: "primary", Action: "pair",
		Confirmation: "confirm:pair:revocable:primary",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := control.RevokeControlPairing(pairingServiceContext(), ControlPairingRevokeRequest{
		EnrollmentID: created.EnrollmentID, Confirmation: "confirm:revoke-pairing:wrong",
	}); err == nil {
		t.Fatal("pairing code was revoked without its exact enrollment confirmation")
	}
	revoked, err := control.RevokeControlPairing(pairingServiceContext(), ControlPairingRevokeRequest{
		EnrollmentID: created.EnrollmentID, Confirmation: "confirm:revoke-pairing:" + created.EnrollmentID,
	})
	if err != nil || revoked.Purpose != servercontrolregistry.PurposeControlAgent || revoked.State != servercontrolregistry.StateRevoked {
		t.Fatalf("revoke result=%+v err=%v", revoked, err)
	}
	payload := decodePairingCode(t, created.Code)
	instanceID := uuid.NewString()
	csr, fingerprint := pairingCSR(t, "starry.internal.test")
	claim := servercontrolregistry.ClaimRequest{
		Version: 1, Purpose: servercontrolregistry.PurposeControlAgent, Action: servercontrolregistry.ActionPair,
		EnrollmentID: payload.EnrollmentID, ConfigurationDigest: payload.ConfigurationDigest, Secret: payload.Secret,
		KeyFingerprint: fingerprint, CSRPEM: csr, InstanceID: &instanceID,
	}
	claim.RequestDigest = servercontrolregistry.ExpectedRequestDigest(claim)
	if _, err := control.ClaimPairing(pairingServiceContext(), claim); err == nil {
		t.Fatal("revoked SP1 code was accepted")
	}
	if _, err := control.CreateControlPairing(pairingServiceContext(), ControlPairingCreateRequest{
		ManagedID: "revocable", Name: "Replacement", AgentOriginID: "primary", Action: "pair",
		Confirmation: "confirm:pair:revocable:primary",
	}); err != nil {
		t.Fatalf("revoked unclaimed code could not be recreated: %v", err)
	}
}

type relayPairingProvider struct {
	starrycontrol.ServerControlProvider
	prepareCalls  int
	completeCalls int
	revokeCalls   int
	prepared      starrycontrol.RelayEnrollmentPrepareResponse
	bundle        starrycontrol.RelayEnrollmentBundle
}

func (p *relayPairingProvider) PrepareRelayEnrollment(_ context.Context, _ starrycontrol.RelayEnrollmentPrepareRequest, key string) (starrycontrol.RelayEnrollmentPrepareResponse, error) {
	p.prepareCalls++
	if key == "" {
		return starrycontrol.RelayEnrollmentPrepareResponse{}, errors.New("missing idempotency key")
	}
	return p.prepared, nil
}

func (p *relayPairingProvider) CompleteRelayEnrollment(_ context.Context, request starrycontrol.RelayEnrollmentCompleteRequest) (starrycontrol.RelayEnrollmentCompleteResponse, error) {
	p.completeCalls++
	return starrycontrol.RelayEnrollmentCompleteResponse{
		Version: 1, EnrollmentID: request.EnrollmentID, ConfigurationDigest: request.ConfigurationDigest,
		RequestDigest: request.RequestDigest, KeyFingerprint: request.KeyFingerprint,
		State: "claimed_pending_health", Bundle: p.bundle,
	}, nil
}

func (p *relayPairingProvider) RevokeRelayEnrollment(_ context.Context, request starrycontrol.RelayEnrollmentRevokeRequest) (starrycontrol.RelayEnrollmentSummary, error) {
	p.revokeCalls++
	return starrycontrol.RelayEnrollmentSummary{
		Version: 1, EnrollmentID: request.EnrollmentID, ConfigurationDigest: request.ConfigurationDigest, State: "revoked",
	}, nil
}

func TestRelayPairingRequiresAgentPrepareAndExactHighRiskPreauthorization(t *testing.T) {
	securityAuditDatabase(t, true)
	root := filepath.Join(t.TempDir(), "server-control")
	registry, err := servercontrolregistry.Open(root, servercontrolregistry.OpenOptions{HostIdentity: "relay-host"})
	if err != nil {
		t.Fatal(err)
	}
	defer registry.Close()
	enrollmentID := uuid.NewString()
	wss := "wss://relay.example.test/ws/telemetry"
	udp := 22119
	request := starrycontrol.RelayEnrollmentPrepareRequest{
		Version: 1, NodeID: "relay-sin-01", RelayServer: "relay.example.test:21117", PublicEndpoint: "relay.example.test:21117",
		RelayPool: "asia", Profile: "native-wss-fastmedia", WSSEndpoint: &wss, ActivateAfterHealth: true,
		MaxSessions: 10000, CapacityBandwidthBPS: 1_000_000_000, FastMediaUDPPort: &udp, ExpiresInSeconds: 600,
	}
	digest, err := starrycontrol.RelayEnrollmentConfigurationDigest(request)
	if err != nil {
		t.Fatal(err)
	}
	provider := &relayPairingProvider{
		prepared: starrycontrol.RelayEnrollmentPrepareResponse{
			Version: 1, EnrollmentID: enrollmentID, ConfigurationDigest: digest,
			ExpiresAtUnix: uint64(time.Now().Add(10 * time.Minute).Unix()), State: "pending_claim",
		},
		bundle: starrycontrol.RelayEnrollmentBundle{
			NodeID: "relay-sin-01", RelayServer: "relay.example.test:21117", PublicEndpoint: "relay.example.test:21117",
			NodeCertificatePEM: "certificate", RelayCAPEM: "ca", CenterPublicKey: base64.StdEncoding.EncodeToString(make([]byte, 32)),
			TelemetrySecret: base64.RawURLEncoding.EncodeToString(make([]byte, 32)), MaxSessions: 10000,
			CapacityBandwidthBPS: 1_000_000_000, RelayPool: "asia", Profile: "native-wss-fastmedia",
			WSSEndpoint: &wss, ActivateAfterHealth: true, FastMediaUDPPort: &udp,
		},
	}
	control := &StarryControlService{
		config: config.ServerControl{Pairing: config.PairingBroker{
			Enabled: true, BrokerOrigin: "https://kessoku.example.test", BrokerSPKISHA256: "sha256:" + strings.Repeat("a", 64),
		}},
		instances:    map[string]ServerControlInstance{"starry-1": {ID: "starry-1", Enabled: true, Available: true}},
		providers:    map[string]starrycontrol.ServerControlProvider{"starry-1": provider},
		providerErrs: map[string]error{}, staticIDs: map[string]struct{}{"starry-1": {}}, managedIDs: map[string]struct{}{},
		registry: registry,
	}
	if _, err := control.CreateRelayPairing(pairingServiceContext(), RelayPairingCreateRequest{
		InstanceID: "starry-1", Enrollment: request, IdempotencyKey: "prepare-1",
	}); err == nil || provider.prepareCalls != 0 {
		t.Fatalf("activate-after-health proceeded without exact second confirmation: calls=%d err=%v", provider.prepareCalls, err)
	}
	created, err := control.CreateRelayPairing(pairingServiceContext(), RelayPairingCreateRequest{
		InstanceID: "starry-1", Enrollment: request, IdempotencyKey: "prepare-1",
		Confirmation: "confirm:activate-after-health:starry-1:relay-sin-01",
	})
	if err != nil || provider.prepareCalls != 1 {
		t.Fatalf("Agent-authorized prepare: result=%+v calls=%d err=%v", created, provider.prepareCalls, err)
	}
	payload := decodePairingCode(t, created.Code)
	csr, fingerprint := pairingCSR(t, "relay.example.test")
	claim := servercontrolregistry.ClaimRequest{
		Version: 1, Purpose: "relay", Action: "enroll", EnrollmentID: payload.EnrollmentID,
		ConfigurationDigest: payload.ConfigurationDigest, Secret: payload.Secret,
		KeyFingerprint: fingerprint, CSRPEM: csr,
	}
	claim.RequestDigest = servercontrolregistry.ExpectedRequestDigest(claim)
	response, err := control.ClaimPairing(pairingServiceContext(), claim)
	if err != nil || provider.completeCalls != 1 {
		t.Fatalf("Relay claim: response=%+v calls=%d err=%v", response, provider.completeCalls, err)
	}
	returned, ok := response.Bundle.(starrycontrol.RelayEnrollmentBundle)
	if !ok || returned.NodeID != request.NodeID || returned.TelemetrySecret == "" {
		t.Fatalf("Relay bundle not passed through once: %#v", response.Bundle)
	}
	record, err := registry.Enrollment(context.Background(), enrollmentID)
	if err != nil || record.RelaySpecJSON == "" || strings.Contains(record.RelaySpecJSON, returned.TelemetrySecret) {
		t.Fatalf("registry retained sensitive Relay bundle: %+v err=%v", record, err)
	}

	provider.prepared.EnrollmentID = uuid.NewString()
	provider.prepared.ConfigurationDigest = "sha256:" + strings.Repeat("f", 64)
	if _, err := control.CreateRelayPairing(pairingServiceContext(), RelayPairingCreateRequest{
		InstanceID: "starry-1", Enrollment: request, IdempotencyKey: "prepare-drift-0001",
		Confirmation: "confirm:activate-after-health:starry-1:relay-sin-01",
	}); err == nil || provider.revokeCalls != 1 {
		t.Fatalf("Agent configuration digest drift was not revoked: revokes=%d err=%v", provider.revokeCalls, err)
	}

	pendingID := uuid.NewString()
	provider.prepared.EnrollmentID = pendingID
	provider.prepared.ConfigurationDigest = digest
	if _, err := control.CreateRelayPairing(pairingServiceContext(), RelayPairingCreateRequest{
		InstanceID: "starry-1", Enrollment: request, IdempotencyKey: "prepare-revoke-0001",
		Confirmation: "confirm:activate-after-health:starry-1:relay-sin-01",
	}); err != nil {
		t.Fatalf("prepare revocable Relay enrollment: %v", err)
	}
	if _, err := control.RevokeRelayEnrollment(pairingServiceContext(), "starry-1", starrycontrol.RelayEnrollmentRevokeRequest{
		Version: 1, EnrollmentID: pendingID, ConfigurationDigest: digest,
	}); err != nil {
		t.Fatalf("Agent-authoritative Relay revoke: %v", err)
	}
	revokedRecord, err := registry.Enrollment(context.Background(), pendingID)
	if err != nil || revokedRecord.State != servercontrolregistry.StateRevoked {
		t.Fatalf("local Relay code state was not synchronized: record=%+v err=%v", revokedRecord, err)
	}
}

func pairingTestConfig(t *testing.T, root string) *config.Config {
	t.Helper()
	hostIdentityFile := filepath.Join(filepath.Dir(root), "host-machine-id")
	if err := os.WriteFile(hostIdentityFile, []byte("kessoku-sp1-test-host\n"), 0o600); err != nil {
		t.Fatalf("write test host identity: %v", err)
	}
	return &config.Config{
		Rustdesk: config.Rustdesk{Key: base64.StdEncoding.EncodeToString(make([]byte, 32))},
		ServerControl: config.ServerControl{
			ReadOnly: true, RegistryDirectory: root, HostIdentityFile: hostIdentityFile,
			Pairing: config.PairingBroker{
				Enabled: true, BrokerOrigin: "https://kessoku.example.test",
				BrokerSPKISHA256: "sha256:" + strings.Repeat("a", 64),
				AgentOrigins: []config.PairingAgentOrigin{{
					ID: "primary", Name: "Primary", Origin: "https://starry.internal.test:21120", TLSServerName: "starry.internal.test",
				}},
			},
		},
	}
}

func pairingServiceContext() context.Context {
	return starrycontrol.WithRequestMetadata(context.Background(), starrycontrol.RequestMetadata{RequestID: uuid.NewString(), Service: true})
}

func decodePairingCode(t *testing.T, code string) servercontrolregistry.PairingCodePayload {
	t.Helper()
	if !strings.HasPrefix(code, "SP1.") {
		t.Fatalf("not an SP1 code: %q", code)
	}
	raw, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(code, "SP1."))
	if err != nil {
		t.Fatal(err)
	}
	var payload servercontrolregistry.PairingCodePayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatal(err)
	}
	return payload
}

func pairingCSR(t *testing.T, host string) (string, string) {
	t.Helper()
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	csrDER, err := x509.CreateCertificateRequest(rand.Reader, &x509.CertificateRequest{
		Subject: pkix.Name{CommonName: host}, DNSNames: []string{host},
	}, privateKey)
	if err != nil {
		t.Fatal(err)
	}
	publicDER, err := x509.MarshalPKIXPublicKey(publicKey)
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(publicDER)
	return string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE REQUEST", Bytes: csrDER})), "sha256:" + hex.EncodeToString(digest[:])
}
