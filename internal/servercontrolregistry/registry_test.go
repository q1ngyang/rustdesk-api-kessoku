package servercontrolregistry

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"gopkg.in/yaml.v3"
)

func TestControlPairingRegistryRecoveryAndStaticExport(t *testing.T) {
	now := time.Date(2026, 9, 2, 1, 2, 3, 0, time.UTC)
	root := filepath.Join(t.TempDir(), "server-control")
	store, err := Open(root, OpenOptions{HostIdentity: "host-a", Now: func() time.Time { return now }})
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	enrollmentID := uuid.NewString()
	prepared, err := PrepareControlIdentity(root, ControlIdentityOptions{
		ManagedID: "starry-main", EnrollmentID: enrollmentID, Name: "Starry main",
		AgentOrigin: "https://starry.internal:21120", TLSServerName: "starry.internal",
		BrokerOrigin: "https://kessoku.example", CenterPublicKey: strings.Repeat("A", 43),
		Now: func() time.Time { return now },
	})
	if err != nil {
		t.Fatal(err)
	}
	secret := base64.RawURLEncoding.EncodeToString(randomBytes(t, 32))
	secretDigest, err := SecretDigest(secret)
	if err != nil {
		t.Fatal(err)
	}
	created, err := store.CreateEnrollment(context.Background(), EnrollmentCreate{
		EnrollmentID: enrollmentID, Purpose: PurposeControlAgent, Action: ActionPair,
		ManagedID: "starry-main", Name: "Starry main", AgentOriginID: "main",
		AgentOrigin: "https://starry.internal:21120", TLSServerName: "starry.internal",
		ConfigurationDigest: prepared.ConfigurationDigest, SecretDigest: secretDigest,
		ExpiresAt: now.Add(10 * time.Minute), RecoveryTTL: 10 * time.Minute,
	})
	if err != nil || created.State != StatePending {
		t.Fatalf("create enrollment: %+v %v", created, err)
	}

	instanceID := uuid.NewString()
	csrPEM, fingerprint := testCSR(t, "starry.internal")
	request := ClaimRequest{
		Version: 1, Purpose: PurposeControlAgent, Action: ActionPair,
		EnrollmentID: enrollmentID, ConfigurationDigest: prepared.ConfigurationDigest,
		Secret: secret, KeyFingerprint: fingerprint, CSRPEM: csrPEM, InstanceID: &instanceID,
	}
	request.RequestDigest = ExpectedRequestDigest(request)
	bound, err := store.BeginClaim(context.Background(), request)
	if err != nil || bound.Reused {
		t.Fatalf("begin claim: %+v %v", bound, err)
	}
	certificate, err := prepared.IssueAgentCertificate(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	managed, err := prepared.ManagedInstance(instanceID)
	if err != nil {
		t.Fatal(err)
	}
	managed, err = store.CompleteControlClaim(context.Background(), enrollmentID, managed)
	if err != nil || !managed.ReadOnly {
		t.Fatalf("complete claim: %+v %v", managed, err)
	}
	metadataAfterCompletion, err := store.Metadata(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	exportPath, err := WriteStaticExport(root, managed)
	if err != nil {
		t.Fatal(err)
	}
	export, err := os.ReadFile(exportPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"server-control:", "starry-main", instanceID, "client-key.pem", "service-jwt-key.pem"} {
		if !strings.Contains(string(export), expected) {
			t.Fatalf("static export missing %q: %s", expected, export)
		}
	}

	// A lost response is recoverable only with the exact same request and must
	// return the byte-identical certificate rather than issuing another one.
	recovered, err := store.BeginClaim(context.Background(), request)
	if err != nil || !recovered.Reused {
		t.Fatalf("recover claim: %+v %v", recovered, err)
	}
	recoveredCertificate, err := prepared.IssueAgentCertificate(context.Background(), request)
	if err != nil || recoveredCertificate != certificate {
		t.Fatalf("certificate recovery changed bytes: %v", err)
	}
	recoveredManaged, err := prepared.ManagedInstance(instanceID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.CompleteControlClaim(context.Background(), enrollmentID, recoveredManaged); err != nil {
		t.Fatalf("complete recovered control claim: %v", err)
	}
	metadataAfterRecovery, err := store.Metadata(context.Background())
	if err != nil || metadataAfterRecovery.Generation != metadataAfterCompletion.Generation {
		t.Fatalf("idempotent recovery changed registry generation: before=%+v after=%+v err=%v", metadataAfterCompletion, metadataAfterRecovery, err)
	}

	changed := request
	changed.CSRPEM += "\n"
	changed.RequestDigest = ExpectedRequestDigest(changed)
	if _, err := store.BeginClaim(context.Background(), changed); !errors.Is(err, ErrBinding) {
		t.Fatalf("changed CSR accepted: %v", err)
	}
	wrongSecret := request
	wrongSecret.Secret = base64.RawURLEncoding.EncodeToString(randomBytes(t, 32))
	if _, err := store.BeginClaim(context.Background(), wrongSecret); !errors.Is(err, ErrSecret) {
		t.Fatalf("wrong secret accepted: %v", err)
	}

	metadata, err := store.Metadata(context.Background())
	if err != nil || metadata.SchemaVersion != 1 || metadata.Generation < 4 {
		t.Fatalf("metadata: %+v %v", metadata, err)
	}
	if err := assertTreeDoesNotContain(root, []string{secret, csrPEM}); err != nil {
		t.Fatal(err)
	}

	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(root, OpenOptions{HostIdentity: "host-a", Now: func() time.Time { return now }})
	if err != nil {
		t.Fatal(err)
	}
	reopened.Close()
	if _, err := Open(root, OpenOptions{HostIdentity: "host-b", Now: func() time.Time { return now }}); !errors.Is(err, ErrIdentityClone) {
		t.Fatalf("cloned identity accepted: %v", err)
	}
}

func TestRelayClaimRecoveryDoesNotAdvanceGeneration(t *testing.T) {
	now := time.Date(2026, 9, 2, 1, 30, 0, 0, time.UTC)
	store, err := Open(filepath.Join(t.TempDir(), "registry"), OpenOptions{HostIdentity: "host-a", Now: func() time.Time { return now }})
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	secret := base64.RawURLEncoding.EncodeToString(randomBytes(t, 32))
	secretDigest, err := SecretDigest(secret)
	if err != nil {
		t.Fatal(err)
	}
	enrollmentID := uuid.NewString()
	configurationDigest := "sha256:" + strings.Repeat("4", 64)
	if _, err := store.CreateEnrollment(context.Background(), EnrollmentCreate{
		EnrollmentID: enrollmentID, Purpose: PurposeRelay, Action: ActionEnroll, ManagedID: "starry-main",
		ConfigurationDigest: configurationDigest, SecretDigest: secretDigest, ExpiresAt: now.Add(10 * time.Minute),
		RelaySpecJSON: `{"node_id":"relay-a"}`,
	}); err != nil {
		t.Fatal(err)
	}
	csr, fingerprint := testCSR(t, "relay.example")
	claim := ClaimRequest{
		Version: 1, Purpose: PurposeRelay, Action: ActionEnroll, EnrollmentID: enrollmentID,
		ConfigurationDigest: configurationDigest, Secret: secret, KeyFingerprint: fingerprint, CSRPEM: csr,
	}
	claim.RequestDigest = ExpectedRequestDigest(claim)
	if binding, err := store.BeginClaim(context.Background(), claim); err != nil || binding.Reused {
		t.Fatalf("first Relay claim: %+v, %v", binding, err)
	}
	if err := store.CompleteRelayClaim(context.Background(), enrollmentID); err != nil {
		t.Fatal(err)
	}
	before, err := store.Metadata(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if binding, err := store.BeginClaim(context.Background(), claim); err != nil || !binding.Reused {
		t.Fatalf("recovered Relay claim: %+v, %v", binding, err)
	}
	if err := store.CompleteRelayClaim(context.Background(), enrollmentID); err != nil {
		t.Fatal(err)
	}
	after, err := store.Metadata(context.Background())
	if err != nil || after.Generation != before.Generation {
		t.Fatalf("Relay recovery changed registry generation: before=%+v after=%+v err=%v", before, after, err)
	}
}

func TestConcurrentFirstClaimBindsExactlyOneKey(t *testing.T) {
	now := time.Date(2026, 9, 2, 2, 0, 0, 0, time.UTC)
	root := filepath.Join(t.TempDir(), "registry")
	store, err := Open(root, OpenOptions{HostIdentity: "host-a", Now: func() time.Time { return now }})
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	secret := base64.RawURLEncoding.EncodeToString(randomBytes(t, 32))
	digest, _ := SecretDigest(secret)
	enrollmentID := uuid.NewString()
	if _, err := store.CreateEnrollment(context.Background(), EnrollmentCreate{
		EnrollmentID: enrollmentID, Purpose: PurposeRelay, Action: ActionEnroll,
		ManagedID: "starry-main", ConfigurationDigest: "sha256:" + strings.Repeat("1", 64),
		SecretDigest: digest, ExpiresAt: now.Add(time.Minute), RelaySpecJSON: `{"node_id":"relay-a"}`,
	}); err != nil {
		t.Fatal(err)
	}
	requests := make([]ClaimRequest, 2)
	for index := range requests {
		csr, fingerprint := testCSR(t, "relay.example")
		requests[index] = ClaimRequest{
			Version: 1, Purpose: PurposeRelay, Action: ActionEnroll, EnrollmentID: enrollmentID,
			ConfigurationDigest: "sha256:" + strings.Repeat("1", 64), Secret: secret,
			KeyFingerprint: fingerprint, CSRPEM: csr,
		}
		requests[index].RequestDigest = ExpectedRequestDigest(requests[index])
	}
	start := make(chan struct{})
	errorsSeen := make(chan error, 2)
	var group sync.WaitGroup
	for _, request := range requests {
		request := request
		group.Add(1)
		go func() {
			defer group.Done()
			<-start
			_, err := store.BeginClaim(context.Background(), request)
			errorsSeen <- err
		}()
	}
	close(start)
	group.Wait()
	close(errorsSeen)
	successes := 0
	bindings := 0
	for err := range errorsSeen {
		if err == nil {
			successes++
		} else if errors.Is(err, ErrBinding) {
			bindings++
		} else {
			t.Fatalf("unexpected concurrent result: %v", err)
		}
	}
	if successes != 1 || bindings != 1 {
		t.Fatalf("success=%d binding rejects=%d", successes, bindings)
	}
}

func TestRegistryRejectsUnsafeRootPermissions(t *testing.T) {
	root := filepath.Join(t.TempDir(), "registry")
	if err := os.Mkdir(root, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(root, OpenOptions{HostIdentity: "host-a"}); !errors.Is(err, ErrUnsafePermissions) {
		t.Fatalf("unsafe permissions accepted: %v", err)
	}
}

func TestHostIdentityFileDetectsCloneAndRejectsSymlink(t *testing.T) {
	directory := t.TempDir()
	root := filepath.Join(directory, "registry")
	hostA := filepath.Join(directory, "host-a")
	hostB := filepath.Join(directory, "host-b")
	if err := os.WriteFile(hostA, []byte("host-a\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(hostB, []byte("host-b\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	store, err := Open(root, OpenOptions{HostIdentityFile: hostA})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := OpenExisting(root, OpenOptions{HostIdentityFile: hostB}); !errors.Is(err, ErrIdentityClone) {
		t.Fatalf("registry clone opened with another host identity: %v", err)
	}
	symlink := filepath.Join(directory, "host-link")
	if err := os.Symlink(hostA, symlink); err != nil {
		t.Fatal(err)
	}
	if _, err := OpenExisting(root, OpenOptions{HostIdentityFile: symlink}); err == nil || !strings.Contains(err.Error(), "non-symlink") {
		t.Fatalf("symlinked host identity accepted: %v", err)
	}
	if err := assertTreeDoesNotContain(root, []string{"host-a"}); err != nil {
		t.Fatal(err)
	}
}

func TestOpenExistingNeverInitializesMissingRegistry(t *testing.T) {
	root := filepath.Join(t.TempDir(), "registry")
	if _, err := OpenExisting(root, OpenOptions{HostIdentity: "host-a"}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing existing registry = %v", err)
	}
	if _, err := os.Lstat(root); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("existing-only preflight created a registry root: %v", err)
	}

	store, err := Open(root, OpenOptions{HostIdentity: "host-a"})
	if err != nil {
		t.Fatal(err)
	}
	metadataBefore, err := store.Metadata(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := OpenExisting(root, OpenOptions{HostIdentity: "host-a"})
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	metadataAfter, err := reopened.Metadata(context.Background())
	if err != nil || metadataAfter.InstallationID != metadataBefore.InstallationID || metadataAfter.Generation != metadataBefore.Generation {
		t.Fatalf("existing-only open changed identity/generation: before=%+v after=%+v err=%v", metadataBefore, metadataAfter, err)
	}
}

func TestOpenExistingRejectsIncompleteRegistryWithoutRepair(t *testing.T) {
	root := filepath.Join(t.TempDir(), "registry")
	store, err := Open(root, OpenOptions{HostIdentity: "host-a"})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	exports := filepath.Join(root, "exports")
	if err := os.Remove(exports); err != nil {
		t.Fatal(err)
	}
	if _, err := OpenExisting(root, OpenOptions{HostIdentity: "host-a"}); !errors.Is(err, ErrUnsafePermissions) {
		t.Fatalf("incomplete registry was accepted or repaired: %v", err)
	}
	if _, err := os.Lstat(exports); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("existing-only preflight recreated a missing credential directory: %v", err)
	}
}

func TestRegistryRejectsUnsafeExistingLockWithoutRepair(t *testing.T) {
	root := filepath.Join(t.TempDir(), "registry")
	store, err := Open(root, OpenOptions{HostIdentity: "host-a"})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	lockPath := filepath.Join(root, "registry-v1.lock")
	if err := os.Chmod(lockPath, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := OpenExisting(root, OpenOptions{HostIdentity: "host-a"}); !errors.Is(err, ErrUnsafePermissions) {
		t.Fatalf("unsafe registry lock was accepted: %v", err)
	}
	info, err := os.Stat(lockPath)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o644 {
		t.Fatalf("failed lock preflight silently repaired permissions: mode=%v", info.Mode().Perm())
	}
}

func TestRegistryRejectsUnsafeExistingDatabaseBeforeOpening(t *testing.T) {
	root := filepath.Join(t.TempDir(), "registry")
	store, err := Open(root, OpenOptions{HostIdentity: "host-a"})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	databasePath := filepath.Join(root, "registry-v1.sqlite")
	if err := os.Chmod(databasePath, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(root, OpenOptions{HostIdentity: "host-a"}); !errors.Is(err, ErrUnsafePermissions) {
		t.Fatalf("unsafe existing database permissions were accepted: %v", err)
	}
	info, err := os.Stat(databasePath)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o644 {
		t.Fatalf("failed preflight silently changed database permissions to %04o", info.Mode().Perm())
	}
}

func TestRegistryRejectsSymlinkDatabaseBeforeOpening(t *testing.T) {
	root := filepath.Join(t.TempDir(), "registry")
	if err := os.Mkdir(root, 0o700); err != nil {
		t.Fatal(err)
	}
	for _, child := range []string{"pki", "instances", "exports"} {
		if err := os.Mkdir(filepath.Join(root, child), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	target := filepath.Join(t.TempDir(), "unrelated.sqlite")
	if err := os.WriteFile(target, []byte("must remain unchanged"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(root, "registry-v1.sqlite")); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(root, OpenOptions{HostIdentity: "host-a"}); !errors.Is(err, ErrUnsafePermissions) {
		t.Fatalf("symlink registry database was accepted: %v", err)
	}
	raw, err := os.ReadFile(target)
	if err != nil || string(raw) != "must remain unchanged" {
		t.Fatalf("symlink target was changed: %q, %v", raw, err)
	}
}

func testCSR(t *testing.T, host string) (string, string) {
	t.Helper()
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	der, err := x509.CreateCertificateRequest(rand.Reader, &x509.CertificateRequest{
		Subject: pkix.Name{CommonName: host}, DNSNames: []string{host},
	}, privateKey)
	if err != nil {
		t.Fatal(err)
	}
	fingerprint, err := publicKeyFingerprint(publicKey)
	if err != nil {
		t.Fatal(err)
	}
	return string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE REQUEST", Bytes: der})), fingerprint
}

func randomBytes(t *testing.T, size int) []byte {
	t.Helper()
	value := make([]byte, size)
	if _, err := rand.Read(value); err != nil {
		t.Fatal(err)
	}
	return value
}

func assertTreeDoesNotContain(root string, values []string) error {
	return filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for _, value := range values {
			if strings.Contains(string(raw), value) {
				return errors.New("sensitive claim material found in " + path)
			}
		}
		return nil
	})
}

func TestPairingCodeCanonicalRoundTrip(t *testing.T) {
	payload := PairingCodePayload{
		Version: 1, Purpose: PurposeRelay, BrokerOrigin: "https://kessoku.example",
		BrokerSPKISHA256: "sha256:" + strings.Repeat("a", 64), EnrollmentID: uuid.NewString(),
		ConfigurationDigest: "sha256:" + strings.Repeat("b", 64), ExpiresAtUnix: 1790000600,
		Secret: base64.RawURLEncoding.EncodeToString(make([]byte, 32)),
	}
	code, err := payload.Encode()
	if err != nil {
		t.Fatal(err)
	}
	raw, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(code, "SP1."))
	if err != nil {
		t.Fatal(err)
	}
	var decoded PairingCodePayload
	if err := json.Unmarshal(raw, &decoded); err != nil || decoded != payload {
		t.Fatalf("round trip: %+v %v", decoded, err)
	}
}

func TestRegistryPurgeRequiresExactIdentityAndTwoConfirmations(t *testing.T) {
	root := filepath.Join(t.TempDir(), "server-control")
	store, err := Open(root, OpenOptions{HostIdentity: "purge-host"})
	if err != nil {
		t.Fatal(err)
	}
	metadata, err := store.Metadata(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}

	confirmation := "confirm:purge:" + metadata.InstallationID
	if err := Purge(context.Background(), root, metadata.InstallationID, confirmation, true, false); err == nil {
		t.Fatal("purge accepted without the independent data-loss confirmation")
	}
	if _, err := os.Stat(filepath.Join(root, "registry-v1.sqlite")); err != nil {
		t.Fatalf("failed purge changed the registry: %v", err)
	}
	wrongInstallationID := uuid.NewString()
	if err := Purge(context.Background(), root, wrongInstallationID, "confirm:purge:"+wrongInstallationID, true, true); !errors.Is(err, ErrBinding) {
		t.Fatalf("purge with a different installation id = %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "registry-v1.sqlite")); err != nil {
		t.Fatalf("identity mismatch changed the registry: %v", err)
	}
	if err := Purge(context.Background(), root, metadata.InstallationID, confirmation, true, true); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(root); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("purged registry still exists: %v", err)
	}
}

func TestFrozenStarryClaimDigestVector(t *testing.T) {
	request := ClaimRequest{
		Purpose: PurposeRelay, Action: ActionEnroll,
		EnrollmentID:        "019b1234-5678-7000-8000-000000000001",
		ConfigurationDigest: "sha256:" + strings.Repeat("1", 64),
		KeyFingerprint:      "sha256:" + strings.Repeat("2", 64),
		CSRPEM:              "csr-a",
	}
	const expected = "sha256:199384643ccb7bc28fa2be66d78407d88f2ae1000e0cb80c5f8b0614b806206c"
	if actual := ExpectedRequestDigest(request); actual != expected {
		t.Fatalf("claim request digest = %s, want frozen Starry vector %s", actual, expected)
	}
}

func TestClaimExpiryPurposeReplayRevocationAndRecoveryWindow(t *testing.T) {
	now := time.Date(2026, 9, 2, 3, 0, 0, 0, time.UTC)
	store, err := Open(filepath.Join(t.TempDir(), "registry"), OpenOptions{HostIdentity: "host-a", Now: func() time.Time { return now }})
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	newRelay := func(ttl, recovery time.Duration) (ClaimRequest, string) {
		t.Helper()
		secret := base64.RawURLEncoding.EncodeToString(randomBytes(t, 32))
		secretDigest, digestErr := SecretDigest(secret)
		if digestErr != nil {
			t.Fatal(digestErr)
		}
		enrollmentID := uuid.NewString()
		configurationDigest := "sha256:" + strings.Repeat("3", 64)
		if _, createErr := store.CreateEnrollment(context.Background(), EnrollmentCreate{
			EnrollmentID: enrollmentID, Purpose: PurposeRelay, Action: ActionEnroll,
			ManagedID: "starry-main", ConfigurationDigest: configurationDigest,
			SecretDigest: secretDigest, ExpiresAt: now.Add(ttl), RecoveryTTL: recovery,
			RelaySpecJSON: `{"node_id":"relay-a"}`,
		}); createErr != nil {
			t.Fatal(createErr)
		}
		csr, fingerprint := testCSR(t, "relay.example")
		request := ClaimRequest{
			Version: 1, Purpose: PurposeRelay, Action: ActionEnroll, EnrollmentID: enrollmentID,
			ConfigurationDigest: configurationDigest, Secret: secret, KeyFingerprint: fingerprint, CSRPEM: csr,
		}
		request.RequestDigest = ExpectedRequestDigest(request)
		return request, enrollmentID
	}

	expired, _ := newRelay(time.Minute, time.Minute)
	now = now.Add(time.Minute)
	if _, err := store.BeginClaim(context.Background(), expired); !errors.Is(err, ErrExpired) {
		t.Fatalf("expired first claim = %v", err)
	}

	now = now.Add(time.Minute)
	bound, _ := newRelay(10*time.Minute, time.Minute)
	wrongPurpose := bound
	wrongPurpose.Purpose = PurposeControlAgent
	wrongPurpose.Action = ActionPair
	instanceID := uuid.NewString()
	wrongPurpose.InstanceID = &instanceID
	wrongPurpose.RequestDigest = ExpectedRequestDigest(wrongPurpose)
	if _, err := store.BeginClaim(context.Background(), wrongPurpose); !errors.Is(err, ErrBinding) {
		t.Fatalf("purpose exchange = %v", err)
	}
	if _, err := store.BeginClaim(context.Background(), bound); err != nil {
		t.Fatal(err)
	}
	now = now.Add(time.Minute)
	if _, err := store.BeginClaim(context.Background(), bound); !errors.Is(err, ErrRecoveryWindow) {
		t.Fatalf("late exact recovery = %v", err)
	}

	now = now.Add(time.Minute)
	revoked, revokedID := newRelay(10*time.Minute, time.Minute)
	if err := store.RevokeEnrollment(context.Background(), revokedID); err != nil {
		t.Fatal(err)
	}
	if _, err := store.BeginClaim(context.Background(), revoked); !errors.Is(err, ErrRevoked) {
		t.Fatalf("revoked claim = %v", err)
	}
}

func TestExpiredControlPairingReleasesManagedIDForRecreation(t *testing.T) {
	now := time.Date(2026, 9, 2, 3, 30, 0, 0, time.UTC)
	store, err := Open(filepath.Join(t.TempDir(), "registry"), OpenOptions{HostIdentity: "host-a", Now: func() time.Time { return now }})
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	create := func(managedID string, ttl, recoveryTTL time.Duration) (ClaimRequest, string) {
		t.Helper()
		secret := base64.RawURLEncoding.EncodeToString(randomBytes(t, 32))
		secretDigest, digestErr := SecretDigest(secret)
		if digestErr != nil {
			t.Fatal(digestErr)
		}
		enrollmentID := uuid.NewString()
		configurationDigest := "sha256:" + strings.Repeat("5", 64)
		if _, createErr := store.CreateEnrollment(context.Background(), EnrollmentCreate{
			EnrollmentID: enrollmentID, Purpose: PurposeControlAgent, Action: ActionPair,
			ManagedID: managedID, Name: managedID, AgentOriginID: managedID,
			AgentOrigin: "https://starry.internal:21120", TLSServerName: "starry.internal",
			ConfigurationDigest: configurationDigest, SecretDigest: secretDigest,
			ExpiresAt: now.Add(ttl), RecoveryTTL: recoveryTTL,
		}); createErr != nil {
			t.Fatal(createErr)
		}
		instanceID := uuid.NewString()
		csr, fingerprint := testCSR(t, "starry.internal")
		request := ClaimRequest{
			Version: 1, Purpose: PurposeControlAgent, Action: ActionPair,
			EnrollmentID: enrollmentID, ConfigurationDigest: configurationDigest,
			Secret: secret, KeyFingerprint: fingerprint, CSRPEM: csr, InstanceID: &instanceID,
		}
		request.RequestDigest = ExpectedRequestDigest(request)
		return request, enrollmentID
	}

	_, expiredPendingID := create("expired-pending", time.Minute, time.Minute)
	now = now.Add(time.Minute)
	if _, replacementID := create("expired-pending", time.Minute, time.Minute); replacementID == expiredPendingID {
		t.Fatal("replacement reused the expired enrollment ID")
	}
	expiredPending, err := store.Enrollment(context.Background(), expiredPendingID)
	if err != nil || expiredPending.State != StateExpired {
		t.Fatalf("expired pending enrollment = %+v, %v", expiredPending, err)
	}

	boundRequest, expiredBoundID := create("expired-bound", 10*time.Minute, time.Minute)
	if _, err := store.BeginClaim(context.Background(), boundRequest); err != nil {
		t.Fatal(err)
	}
	now = now.Add(time.Minute)
	create("expired-bound", time.Minute, time.Minute)
	expiredBound, err := store.Enrollment(context.Background(), expiredBoundID)
	if err != nil || expiredBound.State != StateExpired {
		t.Fatalf("expired bound enrollment = %+v, %v", expiredBound, err)
	}
	if _, err := store.BeginClaim(context.Background(), boundRequest); !errors.Is(err, ErrExpired) {
		t.Fatalf("expired bound claim = %v", err)
	}
}

func TestRegistryBackupRestoreStaticTakeoverAndExplicitHostAdoption(t *testing.T) {
	now := time.Date(2026, 9, 2, 4, 0, 0, 0, time.UTC)
	sourceRoot := filepath.Join(t.TempDir(), "source")
	store, err := Open(sourceRoot, OpenOptions{HostIdentity: "host-a", Now: func() time.Time { return now }})
	if err != nil {
		t.Fatal(err)
	}
	enrollmentID := uuid.NewString()
	prepared, err := PrepareControlIdentity(sourceRoot, ControlIdentityOptions{
		ManagedID: "starry-main", EnrollmentID: enrollmentID, Name: "Starry main",
		AgentOrigin: "https://starry.internal:21120", TLSServerName: "starry.internal",
		BrokerOrigin: "https://kessoku.example", CenterPublicKey: strings.Repeat("A", 43),
		Now: func() time.Time { return now },
	})
	if err != nil {
		t.Fatal(err)
	}
	secret := base64.RawURLEncoding.EncodeToString(randomBytes(t, 32))
	secretDigest, _ := SecretDigest(secret)
	if _, err := store.CreateEnrollment(context.Background(), EnrollmentCreate{
		EnrollmentID: enrollmentID, Purpose: PurposeControlAgent, Action: ActionPair,
		ManagedID: "starry-main", Name: "Starry main", AgentOriginID: "main",
		AgentOrigin: "https://starry.internal:21120", TLSServerName: "starry.internal",
		ConfigurationDigest: prepared.ConfigurationDigest, SecretDigest: secretDigest,
		ExpiresAt: now.Add(10 * time.Minute), RecoveryTTL: 10 * time.Minute,
	}); err != nil {
		t.Fatal(err)
	}
	instanceID := uuid.NewString()
	csr, fingerprint := testCSR(t, "starry.internal")
	claim := ClaimRequest{
		Version: 1, Purpose: PurposeControlAgent, Action: ActionPair, EnrollmentID: enrollmentID,
		ConfigurationDigest: prepared.ConfigurationDigest, Secret: secret, KeyFingerprint: fingerprint,
		CSRPEM: csr, InstanceID: &instanceID,
	}
	claim.RequestDigest = ExpectedRequestDigest(claim)
	if _, err := store.BeginClaim(context.Background(), claim); err != nil {
		t.Fatal(err)
	}
	certificate, err := prepared.IssueAgentCertificate(context.Background(), claim)
	if err != nil {
		t.Fatal(err)
	}
	managed, err := prepared.ManagedInstance(instanceID)
	if err != nil {
		t.Fatal(err)
	}
	managed, err = store.CompleteControlClaim(context.Background(), enrollmentID, managed)
	if err != nil {
		t.Fatal(err)
	}
	exportPath, err := WriteStaticExport(sourceRoot, managed)
	if err != nil {
		t.Fatal(err)
	}
	metadataBefore, err := store.Metadata(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}

	staticRaw, err := os.ReadFile(exportPath)
	if err != nil {
		t.Fatal(err)
	}
	var staticDocument struct {
		ServerControl struct {
			Instances []map[string]interface{} `yaml:"instances"`
		} `yaml:"server-control"`
	}
	if err := yaml.Unmarshal(staticRaw, &staticDocument); err != nil || len(staticDocument.ServerControl.Instances) != 1 ||
		staticDocument.ServerControl.Instances[0]["expected-instance-id"] != instanceID {
		t.Fatalf("v3.0.7-compatible static export = %#v err=%v", staticDocument, err)
	}

	restoreRoot := filepath.Join(t.TempDir(), "restored")
	if err := copyPrivateTree(sourceRoot, restoreRoot); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(restoreRoot, OpenOptions{HostIdentity: "host-b", Now: func() time.Time { return now }}); !errors.Is(err, ErrIdentityClone) {
		t.Fatalf("restored clone opened without adoption: %v", err)
	}
	if err := AdoptHost(context.Background(), restoreRoot, metadataBefore.InstallationID, OpenOptions{HostIdentity: "host-b"}); err != nil {
		t.Fatal(err)
	}
	restored, err := Open(restoreRoot, OpenOptions{HostIdentity: "host-b", Now: func() time.Time { return now }})
	if err != nil {
		t.Fatal(err)
	}
	defer restored.Close()
	metadataAfter, err := restored.Metadata(context.Background())
	if err != nil || metadataAfter.InstallationID != metadataBefore.InstallationID || metadataAfter.Generation != metadataBefore.Generation+1 {
		t.Fatalf("restored metadata = %+v err=%v", metadataAfter, err)
	}
	restoredManaged, err := restored.ManagedInstance(context.Background(), "starry-main")
	if err != nil || restoredManaged.InstanceUUID != instanceID || restoredManaged.RegistryGeneration != managed.RegistryGeneration {
		t.Fatalf("restored managed identity = %+v err=%v", restoredManaged, err)
	}
	restoredCertificate, err := os.ReadFile(filepath.Join(restoreRoot, "instances", "starry-main", enrollmentID, "agent-server-cert.pem"))
	if err != nil || string(restoredCertificate) != certificate {
		t.Fatal("restored credential bytes changed")
	}
}

func TestRegistryRejectsFutureIndependentSchema(t *testing.T) {
	root := filepath.Join(t.TempDir(), "registry")
	store, err := Open(root, OpenOptions{HostIdentity: "host-a"})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	database, err := openSQLite(filepath.Join(root, "registry-v1.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`UPDATE registry_meta SET schema_version = ? WHERE singleton = ?`, SchemaVersion+1, metadataRow); err != nil {
		database.Close()
		t.Fatal(err)
	}
	if err := database.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(root, OpenOptions{HostIdentity: "host-a"}); !errors.Is(err, ErrFutureSchema) {
		t.Fatalf("future independent schema accepted: %v", err)
	}
}

func copyPrivateTree(source, destination string) error {
	if err := os.Mkdir(destination, 0o700); err != nil {
		return err
	}
	entries, err := os.ReadDir(source)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		sourcePath := filepath.Join(source, entry.Name())
		destinationPath := filepath.Join(destination, entry.Name())
		if entry.IsDir() {
			if err := copyPrivateTree(sourcePath, destinationPath); err != nil {
				return err
			}
			continue
		}
		raw, err := os.ReadFile(sourcePath)
		if err != nil {
			return err
		}
		if err := os.WriteFile(destinationPath, raw, 0o600); err != nil {
			return err
		}
	}
	return nil
}
