package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/q1ngyang/rustdesk-api-kessoku/v3/internal/servercontrolregistry"
	"github.com/q1ngyang/rustdesk-api-kessoku/v3/service"
)

func TestServerControlCLIUsesIndependentRegistryAndOneTimeCode(t *testing.T) {
	directory := t.TempDir()
	withWorkingDirectory(t, directory)
	configPath, registryRoot := writeServerControlCLIConfig(t, directory)
	statusRaw, statusErr, err := executeRoot("server-control", "registry", "status", "--config", configPath, "--json")
	if err == nil || commandExitCode(err) != exitServerControl || !bytes.Contains(statusRaw, []byte(`"code":"REGISTRY_NOT_INITIALIZED"`)) {
		t.Fatalf("missing registry preflight: exit=%d err=%v stdout=%s stderr=%s", commandExitCode(err), err, statusRaw, statusErr)
	}
	if _, err := os.Lstat(registryRoot); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("status preflight silently initialized registry: %v", err)
	}

	stdout, stderr, err := executeRoot(
		"server-control", "pair", "create", "--config", configPath, "--json",
		"--id", "starry-main", "--name", "Starry Main", "--agent-origin", "primary",
		"--confirm", "confirm:pair:starry-main:primary",
	)
	if err != nil {
		t.Fatalf("pair create: %v stdout=%s stderr=%s", err, stdout, stderr)
	}
	var result service.PairingCodeResult
	if err := json.Unmarshal(stdout, &result); err != nil || !strings.HasPrefix(result.Code, "SP1.") || result.EnrollmentID == "" {
		t.Fatalf("pair result=%+v err=%v", result, err)
	}
	payloadRaw, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(result.Code, "SP1."))
	if err != nil {
		t.Fatal(err)
	}
	var payload servercontrolregistry.PairingCodePayload
	if err := json.Unmarshal(payloadRaw, &payload); err != nil || payload.EnrollmentID != result.EnrollmentID || payload.Secret == "" {
		t.Fatalf("pair payload=%+v err=%v", payload, err)
	}
	if err := assertCommandTreeDoesNotContain(registryRoot, []string{result.Code, payload.Secret}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(directory, "data", "rustdeskapi.db")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("server-control CLI opened the business database: %v", err)
	}
	_, _, err = executeRoot(
		"server-control", "pair", "revoke", "--config", configPath, "--json",
		"--enrollment-id", result.EnrollmentID, "--confirm", "confirm:revoke-pairing:wrong",
	)
	if err == nil || commandExitCode(err) != exitServerControl {
		t.Fatalf("pair revoke accepted an inexact confirmation: exit=%d err=%v", commandExitCode(err), err)
	}
	stdout, stderr, err = executeRoot(
		"server-control", "pair", "revoke", "--config", configPath, "--json",
		"--enrollment-id", result.EnrollmentID, "--confirm", "confirm:revoke-pairing:"+result.EnrollmentID,
	)
	if err != nil {
		t.Fatalf("pair revoke: %v stdout=%s stderr=%s", err, stdout, stderr)
	}
	var revokeResult service.PairingRevokeResult
	if err := json.Unmarshal(stdout, &revokeResult); err != nil || revokeResult.State != servercontrolregistry.StateRevoked {
		t.Fatalf("pair revoke result=%+v err=%v", revokeResult, err)
	}

	statusRaw, statusErr, err = executeRoot("server-control", "registry", "status", "--config", configPath, "--json")
	if err != nil {
		t.Fatalf("registry status: %v stdout=%s stderr=%s", err, statusRaw, statusErr)
	}
	var metadata servercontrolregistry.Metadata
	if err := json.Unmarshal(statusRaw, &metadata); err != nil || metadata.SchemaVersion != 1 || metadata.Generation < 2 || metadata.InstallationID == "" {
		t.Fatalf("registry status=%+v err=%v", metadata, err)
	}

	_, _, err = executeRoot(
		"server-control", "registry", "adopt-host", "--config", configPath, "--json",
		"--installation-id", metadata.InstallationID, "--confirm", "confirm:adopt-host:"+metadata.InstallationID,
	)
	if err == nil || commandExitCode(err) != exitUsage {
		t.Fatalf("host adoption omitted old-host assertion: exit=%d err=%v", commandExitCode(err), err)
	}

	_, _, err = executeRoot(
		"server-control", "registry", "purge", "--config", configPath, "--json",
		"--installation-id", metadata.InstallationID, "--service-stopped",
		"--confirm", "confirm:purge:"+metadata.InstallationID,
	)
	if err == nil || commandExitCode(err) != exitUsage {
		t.Fatalf("purge omitted data-loss confirmation: exit=%d err=%v", commandExitCode(err), err)
	}
	if _, err := os.Stat(filepath.Join(registryRoot, "registry-v1.sqlite")); err != nil {
		t.Fatalf("rejected purge changed the registry: %v", err)
	}
	stdout, stderr, err = executeRoot(
		"server-control", "registry", "purge", "--config", configPath, "--json",
		"--installation-id", metadata.InstallationID, "--service-stopped", "--data-loss-understood",
		"--confirm", "confirm:purge:"+metadata.InstallationID,
	)
	if err != nil {
		t.Fatalf("confirmed purge: %v stdout=%s stderr=%s", err, stdout, stderr)
	}
	if _, err := os.Stat(registryRoot); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("purged registry still exists: %v", err)
	}
}

func TestControlPairCLIRejectsArbitraryOriginAndPairTarget(t *testing.T) {
	directory := t.TempDir()
	withWorkingDirectory(t, directory)
	configPath, _ := writeServerControlCLIConfig(t, directory)
	tests := [][]string{
		{"--id", "ssrf", "--name", "SSRF", "--agent-origin", "https://attacker.invalid", "--confirm", "confirm:pair:ssrf:https://attacker.invalid"},
		{"--id", "targeted", "--name", "Targeted", "--agent-origin", "primary", "--target-instance-id", "019b1234-5678-7000-8000-000000000001", "--confirm", "confirm:pair:targeted:primary"},
	}
	for _, arguments := range tests {
		base := []string{"server-control", "pair", "create", "--config", configPath, "--json"}
		_, _, err := executeRoot(append(base, arguments...)...)
		if err == nil || commandExitCode(err) != exitServerControl {
			t.Fatalf("unsafe pairing accepted: args=%v exit=%d err=%v", arguments, commandExitCode(err), err)
		}
	}
}

func writeServerControlCLIConfig(t *testing.T, directory string) (string, string) {
	t.Helper()
	path := writeCLIConfig(t, directory, false)
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	registryRoot := filepath.Join(directory, "server-control")
	hostIdentityFile := filepath.Join(directory, "host-machine-id")
	if err := os.WriteFile(hostIdentityFile, []byte("test-host-machine-id\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	replacement := `server-control:
  read-only: true
  registry-directory: "` + registryRoot + `"
  host-identity-file: "` + hostIdentityFile + `"
  instances: []
  pairing:
    enabled: true
    broker-origin: "https://kessoku.example.test"
    broker-spki-sha256: "sha256:` + strings.Repeat("a", 64) + `"
    code-ttl: 10m
    recovery-ttl: 10m
    agent-origins:
      - id: primary
        name: Primary
        origin: "https://starry.internal.test:21120"
        tls-server-name: "starry.internal.test"
`
	contents := strings.Replace(string(raw), `key: "public-rustdesk-key"`, `key: "`+base64.StdEncoding.EncodeToString(make([]byte, 32))+`"`, 1)
	contents = strings.Replace(contents, "server-control:\n  read-only: true\n  instances: []\n", replacement, 1)
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
	return path, registryRoot
}

func assertCommandTreeDoesNotContain(root string, forbidden []string) error {
	return filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for _, value := range forbidden {
			if strings.Contains(string(raw), value) {
				return errors.New("sensitive pairing material found in " + path)
			}
		}
		return nil
	})
}
