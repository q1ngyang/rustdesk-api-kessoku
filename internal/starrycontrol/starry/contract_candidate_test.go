package starry

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/q1ngyang/rustdesk-api-kessoku/v3/internal/starrycontrol"
)

func TestLocalStarryContractCandidate(t *testing.T) {
	metadata := readCandidateMetadata(t)
	if metadata["control_contract"] != "control/v1" || metadata["config_schema"] != "config/v4" ||
		!validContractHexDigest(metadata["candidate_sha256"]) ||
		!validContractHexDigest(metadata["config_schema_sha256"]) ||
		!validContractHexDigest(metadata["config_ui_schema_sha256"]) {
		t.Fatalf("invalid contract provenance: %#v", metadata)
	}
	expectedDigest := metadata["candidate_sha256"]
	switch metadata["status"] {
	case "LOCAL_CANDIDATE_VALIDATED":
		if metadata["starry_tag"] != "UNPUBLISHED" || metadata["release_sha256"] != "UNAVAILABLE" ||
			metadata["image_index_sha256"] != "UNAVAILABLE" || metadata["image_linux_amd64_sha256"] != "UNAVAILABLE" {
			t.Fatalf("local candidate must remain release-blocked: %#v", metadata)
		}
	case "PINNED":
		if metadata["starry_tag"] == "" || metadata["starry_tag"] == "UNPUBLISHED" ||
			metadata["release_sha256"] != metadata["candidate_sha256"] ||
			!validContractHexDigest(metadata["image_index_sha256"]) ||
			!validContractHexDigest(metadata["image_linux_amd64_sha256"]) {
			t.Fatalf("published contract provenance is incomplete: %#v", metadata)
		}
		verifyPublishedSummary(t, metadata)
		expectedDigest = metadata["release_sha256"]
	default:
		t.Fatalf("unsupported contract provenance status: %#v", metadata)
	}

	root := os.Getenv("STARRY_CONTRACT_ROOT")
	if root == "" {
		t.Skip("set STARRY_CONTRACT_ROOT to a Starry checkout to run the cross-repository candidate test")
	}
	openAPI := readContractFile(t, root, "contracts/control/v1/openapi.yaml")
	digest := sha256.Sum256(openAPI)
	if actual := hex.EncodeToString(digest[:]); actual != expectedDigest {
		t.Fatalf("Starry OpenAPI candidate digest = %s, want %s", actual, expectedDigest)
	}

	var capabilities starrycontrol.Capabilities
	readExample(t, root, "capabilities.json", &capabilities)
	if err := validateCapabilitiesResponse(capabilities); err != nil {
		t.Fatalf("capabilities fixture: %v", err)
	}
	if capabilities.Capabilities.RelayQuality != 1 || capabilities.Capabilities.RelayActiveProbe != 1 ||
		capabilities.Capabilities.RelayProbeProtocol != 1 || capabilities.Capabilities.RelayLoadProtocol != 1 {
		t.Fatalf("adaptive Relay Quality capabilities: %#v", capabilities.Capabilities)
	}
	var status starrycontrol.Status
	readExample(t, root, "status.json", &status)
	if err := validateStatusResponse(status); err != nil {
		t.Fatalf("status fixture: %v", err)
	}
	var relays starrycontrol.RelayInventory
	readExample(t, root, "relays.json", &relays)
	if err := validateRelaysResponse(relays, capabilities.Capabilities); err != nil {
		t.Fatalf("relay fixture: %v", err)
	}
	if relays.Quality == nil || relays.Quality.ProtocolVersion != capabilities.Capabilities.RelayQuality {
		t.Fatalf("adaptive Relay Quality version mismatch: %#v", relays.Quality)
	}
	var peerVerification starrycontrol.PeerVerification
	readExample(t, root, "peer-verification.json", &peerVerification)
	if peerVerification.InstanceID != capabilities.Instance.ID || !peerVerification.Registered {
		t.Fatalf("peer-verification fixture: %#v", peerVerification)
	}
	var simulation starrycontrol.SimulationResult
	readExample(t, root, "allocation-simulation.json", &simulation)
	if err := validateSimulationResponse(simulation); err != nil {
		t.Fatalf("allocation fixture: %v", err)
	}
	var configDocument starrycontrol.ConfigDocument
	readExample(t, root, "config.json", &configDocument)
	if err := validateConfigResponse(configDocument); err != nil {
		t.Fatalf("config fixture: %v", err)
	}
	var validation starrycontrol.ValidationResult
	readExample(t, root, "validation.json", &validation)
	if err := validateValidationResponse(validation); err != nil {
		t.Fatalf("validation fixture: %v", err)
	}
	var plan starrycontrol.ConfigPlan
	readExample(t, root, "plan.json", &plan)
	if err := validatePlanResponse(plan, capabilities.Instance.ID, plan.BaseETag, plan.CandidateDigest); err != nil {
		t.Fatalf("plan fixture: %v", err)
	}
	var operation starrycontrol.Operation
	readExample(t, root, "operation.json", &operation)
	if err := validateOperationResponse(operation, operation.ID); err != nil {
		t.Fatalf("operation fixture: %v", err)
	}
	var history struct {
		Revisions []starrycontrol.ConfigRevision `json:"revisions"`
	}
	readExample(t, root, "history.json", &history)
	if err := validateHistoryResponse(history.Revisions); err != nil {
		t.Fatalf("history fixture: %v", err)
	}

	schema := readContractFile(t, root, "contracts", metadata["config_schema"], "config.schema.json")
	uiSchema := readContractFile(t, root, "contracts", metadata["config_schema"], "config.ui-schema.json")
	schemaHash := sha256.Sum256(schema)
	schemaDigest := "sha256:" + hex.EncodeToString(schemaHash[:])
	if schemaDigest != capabilities.Config.SchemaDigest {
		t.Fatalf("capabilities schema digest = %s, actual schema digest = %s", capabilities.Config.SchemaDigest, schemaDigest)
	}
	if actual := hex.EncodeToString(schemaHash[:]); actual != metadata["config_schema_sha256"] {
		t.Fatalf("Starry config schema candidate digest = %s, want %s", actual, metadata["config_schema_sha256"])
	}
	uiSchemaHash := sha256.Sum256(uiSchema)
	if actual := hex.EncodeToString(uiSchemaHash[:]); actual != metadata["config_ui_schema_sha256"] {
		t.Fatalf("Starry UI schema candidate digest = %s, want %s", actual, metadata["config_ui_schema_sha256"])
	}
	bundle := starrycontrol.SchemaBundle{
		ETag:     fmt.Sprintf("%q", schemaDigest),
		Digest:   schemaDigest,
		Schema:   json.RawMessage(schema),
		UISchema: json.RawMessage(uiSchema),
	}
	if err := validateSchemaResponse(bundle); err != nil {
		t.Fatalf("schema bundle: %v", err)
	}
}

func verifyPublishedSummary(t *testing.T, metadata map[string]string) {
	t.Helper()
	const expectedPath = "docs/releases/v3.0.7/STARRY-RELEASE-SUMMARY.json"
	if metadata["release_summary_path"] != expectedPath ||
		!validContractHexDigest(metadata["release_summary_sha256"]) ||
		!validContractHexDigest(metadata["relay_quality_protocol_sha256"]) ||
		!validContractHexDigest(metadata["relay_telemetry_schema_sha256"]) {
		t.Fatalf("published summary provenance is incomplete: %#v", metadata)
	}
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate candidate test source")
	}
	repositoryRoot := filepath.Clean(filepath.Join(filepath.Dir(filename), "..", "..", ".."))
	encoded, err := os.ReadFile(filepath.Join(repositoryRoot, filepath.FromSlash(expectedPath)))
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(encoded)
	if actual := hex.EncodeToString(digest[:]); actual != metadata["release_summary_sha256"] {
		t.Fatalf("published Starry summary digest = %s, want %s", actual, metadata["release_summary_sha256"])
	}

	var summary struct {
		SchemaVersion int `json:"schema_version"`
		Release       struct {
			Tag          string `json:"tag"`
			SourceCommit string `json:"source_commit"`
		} `json:"release"`
		Image struct {
			Reference   string            `json:"reference"`
			IndexDigest string            `json:"index_digest"`
			Platforms   map[string]string `json:"platforms"`
		} `json:"image"`
		Contracts map[string]struct {
			ID     string `json:"id"`
			Status string `json:"status"`
			Path   string `json:"path"`
			Digest string `json:"digest"`
		} `json:"contracts"`
	}
	if err := json.Unmarshal(encoded, &summary); err != nil {
		t.Fatal(err)
	}
	trimDigest := func(value string) string { return strings.TrimPrefix(value, "sha256:") }
	if summary.SchemaVersion != 1 || summary.Release.Tag != metadata["starry_tag"] ||
		summary.Release.SourceCommit != metadata["starry_source_commit"] ||
		summary.Image.Reference != "ghcr.io/q1ngyang/rustdesk-server-starry:"+metadata["starry_tag"] ||
		trimDigest(summary.Image.IndexDigest) != metadata["image_index_sha256"] ||
		trimDigest(summary.Image.Platforms["linux/amd64"]) != metadata["image_linux_amd64_sha256"] {
		t.Fatalf("published Starry release identity mismatch: %#v", summary)
	}
	expectedContracts := map[string]struct {
		id, status, path, digest string
	}{
		"control_openapi":        {"control/v1", "", "contracts/control/v1/openapi.yaml", metadata["release_sha256"]},
		"config_schema":          {"config/v4", "", "contracts/config/v4/config.schema.json", metadata["config_schema_sha256"]},
		"config_ui_schema":       {"config/v4-ui", "", "contracts/config/v4/config.ui-schema.json", metadata["config_ui_schema_sha256"]},
		"relay_quality_protocol": {"relay-quality/v1", "FROZEN", "contracts/relay-quality/v1/rendezvous-extension.proto", metadata["relay_quality_protocol_sha256"]},
		"relay_telemetry_schema": {"relay-telemetry/v1", "", "contracts/relay-telemetry/v1/telemetry.schema.json", metadata["relay_telemetry_schema_sha256"]},
	}
	if len(summary.Contracts) != len(expectedContracts) {
		t.Fatalf("published Starry contract inventory = %#v", summary.Contracts)
	}
	for name, expected := range expectedContracts {
		actual, found := summary.Contracts[name]
		if !found || actual.ID != expected.id || actual.Status != expected.status ||
			actual.Path != expected.path || trimDigest(actual.Digest) != expected.digest {
			t.Fatalf("published Starry contract %s = %#v, want %#v", name, actual, expected)
		}
	}
	lower := strings.ToLower(string(encoded))
	for _, forbidden := range []string{"allocation_id", "session_uuid", "nonce", "client_ip", "target_ip", "connection_token", "secret"} {
		if strings.Contains(lower, forbidden) {
			t.Fatalf("published Starry summary contains sensitive field %q", forbidden)
		}
	}
}

func validContractHexDigest(value string) bool {
	if len(value) != sha256.Size*2 {
		return false
	}
	decoded, err := hex.DecodeString(value)
	return err == nil && hex.EncodeToString(decoded) == value
}

func readCandidateMetadata(t *testing.T) map[string]string {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate candidate test source")
	}
	encoded, err := os.ReadFile(filepath.Join(filepath.Dir(filename), "..", "CONTRACT_VERSION"))
	if err != nil {
		t.Fatal(err)
	}
	result := make(map[string]string)
	for _, line := range strings.Split(string(encoded), "\n") {
		key, value, found := strings.Cut(line, ":")
		if found && !strings.HasPrefix(strings.TrimSpace(line), "#") {
			result[strings.TrimSpace(key)] = strings.TrimSpace(value)
		}
	}
	return result
}

func readExample(t *testing.T, root, name string, destination interface{}) {
	t.Helper()
	encoded := readContractFile(t, root, filepath.Join("contracts/control/v1/examples", name))
	if err := json.Unmarshal(encoded, destination); err != nil {
		t.Fatalf("decode %s: %v", name, err)
	}
}

func readContractFile(t *testing.T, root string, elements ...string) []byte {
	t.Helper()
	path := filepath.Join(append([]string{root}, elements...)...)
	encoded, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return encoded
}
