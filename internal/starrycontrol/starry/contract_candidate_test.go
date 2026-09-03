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
	if metadata["control_contract"] != "control/v1" ||
		(metadata["config_schema"] != "config/v4" && metadata["config_schema"] != "config/v5") ||
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
	case "FROZEN_CONTRACT_CANDIDATE":
		if metadata["starry_tag"] != "UNPUBLISHED" || metadata["release_sha256"] != "UNAVAILABLE" ||
			metadata["image_index_sha256"] != "UNAVAILABLE" || metadata["image_linux_amd64_sha256"] != "UNAVAILABLE" ||
			!validContractCommit(metadata["starry_source_commit"]) || !validContractHexDigest(metadata["release_summary_sha256"]) {
			t.Fatalf("frozen contract candidate provenance is incomplete: %#v", metadata)
		}
		verifyFrozenCandidateSummary(t, metadata, "")
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
	if metadata["status"] == "FROZEN_CONTRACT_CANDIDATE" {
		verifyFrozenCandidateSummary(t, metadata, root)
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
		capabilities.Capabilities.RelayProbeProtocol != 1 || capabilities.Capabilities.RelayLoadProtocol != 1 ||
		capabilities.Capabilities.RelayTelemetrySchema != 2 || capabilities.Capabilities.FastRelayAuthorization != 1 ||
		capabilities.Capabilities.FastMediaRelayUDP != 1 || capabilities.Capabilities.ConfigSchema != 5 {
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
	if relays.FastRelay == nil || relays.FastRelay.ProtocolVersion != capabilities.Capabilities.FastRelayAuthorization ||
		len(relays.Relays) == 0 || relays.Relays[0].FastMediaUDP == nil {
		t.Fatalf("FastRelay runtime version mismatch: %#v", relays.FastRelay)
	}
	var relayEnrollmentPrepare starrycontrol.RelayEnrollmentPrepareResponse
	readExample(t, root, "relay-enrollment-prepare.json", &relayEnrollmentPrepare)
	if err := validateRelayEnrollmentPrepareResponse(relayEnrollmentPrepare); err != nil {
		t.Fatalf("Relay enrollment prepare fixture: %v", err)
	}
	var relayEnrollmentComplete starrycontrol.RelayEnrollmentCompleteResponse
	readExample(t, root, "relay-enrollment-complete.json", &relayEnrollmentComplete)
	if err := validateRelayEnrollmentCompleteResponse(relayEnrollmentComplete); err != nil {
		t.Fatalf("Relay enrollment complete fixture: %v", err)
	}
	var relayEnrollmentActivate starrycontrol.RelayEnrollmentActivateRequest
	readExample(t, root, "relay-enrollment-activate.json", &relayEnrollmentActivate)
	if !validRelayEnrollmentActivateRequest(relayEnrollmentActivate) {
		t.Fatalf("Relay enrollment activate fixture: %#v", relayEnrollmentActivate)
	}
	var relayEnrollments starrycontrol.RelayEnrollmentList
	readExample(t, root, "relay-enrollments.json", &relayEnrollments)
	if err := validateRelayEnrollmentList(relayEnrollments); err != nil {
		t.Fatalf("Relay enrollment inventory fixture: %v", err)
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

func verifyFrozenCandidateSummary(t *testing.T, metadata map[string]string, starryRoot string) {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate candidate test source")
	}
	repositoryRoot := filepath.Clean(filepath.Join(filepath.Dir(filename), "..", "..", ".."))
	encoded, err := os.ReadFile(filepath.Join(repositoryRoot, filepath.FromSlash(metadata["release_summary_path"])))
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(encoded)
	if actual := hex.EncodeToString(digest[:]); actual != metadata["release_summary_sha256"] {
		t.Fatalf("frozen Starry summary digest = %s, want %s", actual, metadata["release_summary_sha256"])
	}
	var summary struct {
		Status               string `json:"status"`
		CandidateKind        string `json:"candidate_kind"`
		RuntimeReleaseStatus string `json:"runtime_release_status"`
		SourceBinding        struct {
			Branch string `json:"branch"`
		} `json:"source_binding"`
		Files []struct {
			ID     string `json:"id"`
			Path   string `json:"path"`
			SHA256 string `json:"sha256"`
		} `json:"files"`
		Inherited []struct {
			ID     string `json:"id"`
			Path   string `json:"path"`
			SHA256 string `json:"sha256"`
		} `json:"inherited_frozen_contracts"`
	}
	if err := json.Unmarshal(encoded, &summary); err != nil {
		t.Fatal(err)
	}
	if summary.Status != "FROZEN" || summary.CandidateKind != "CONTRACT_ONLY" || summary.RuntimeReleaseStatus != "BLOCKED" ||
		summary.SourceBinding.Branch != "codex/patch-v1.3.1-fastmedia-pairing" || len(summary.Files) != 16 || len(summary.Inherited) != 1 {
		t.Fatalf("unexpected frozen Starry contract summary: %#v", summary)
	}
	if starryRoot == "" {
		return
	}
	for _, file := range summary.Files {
		contents := readContractFile(t, starryRoot, filepath.FromSlash(file.Path))
		actual := sha256.Sum256(contents)
		if "sha256:"+hex.EncodeToString(actual[:]) != file.SHA256 {
			t.Fatalf("Starry contract %s digest does not match frozen summary", file.ID)
		}
	}
	for _, file := range summary.Inherited {
		contents := readContractFile(t, starryRoot, filepath.FromSlash(file.Path))
		actual := sha256.Sum256(contents)
		if "sha256:"+hex.EncodeToString(actual[:]) != file.SHA256 {
			t.Fatalf("inherited Starry contract %s digest does not match frozen summary", file.ID)
		}
	}
}

func verifyPublishedSummary(t *testing.T, metadata map[string]string) {
	t.Helper()
	expectedPath := metadata["release_summary_path"]
	if expectedPath != "docs/releases/v3.0.8/STARRY-RELEASE-SUMMARY.json" ||
		!validContractHexDigest(metadata["release_summary_sha256"]) ||
		!validContractHexDigest(metadata["relay_quality_protocol_sha256"]) ||
		!validContractHexDigest(metadata["relay_telemetry_schema_sha256"]) ||
		!validContractHexDigest(metadata["contract_candidate_sha256"]) ||
		!validContractHexDigest(metadata["fast_relay_protocol_sha256"]) ||
		!validContractHexDigest(metadata["fast_media_relay_udp_sha256"]) ||
		!validContractHexDigest(metadata["starry_pairing_protocol_sha256"]) ||
		!validContractHexDigest(metadata["downgrade_drain_state_sha256"]) {
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
			Channel      string `json:"channel"`
			SourceCommit string `json:"source_commit"`
		} `json:"release"`
		Image struct {
			Reference   string            `json:"reference"`
			IndexDigest string            `json:"index_digest"`
			Platforms   map[string]string `json:"platforms"`
		} `json:"image"`
		Contracts map[string]struct {
			ID                   string `json:"id"`
			Status               string `json:"status"`
			Path                 string `json:"path"`
			Digest               string `json:"digest"`
			RuntimeReleaseStatus string `json:"runtime_release_status"`
		} `json:"contracts"`
	}
	if err := json.Unmarshal(encoded, &summary); err != nil {
		t.Fatal(err)
	}
	trimDigest := func(value string) string { return strings.TrimPrefix(value, "sha256:") }
	if summary.SchemaVersion != 1 || summary.Release.Tag != metadata["starry_tag"] ||
		summary.Release.Channel != metadata["starry_release_channel"] ||
		summary.Release.SourceCommit != metadata["starry_source_commit"] ||
		summary.Image.Reference != "ghcr.io/q1ngyang/rustdesk-server-starry:"+metadata["starry_tag"] ||
		trimDigest(summary.Image.IndexDigest) != metadata["image_index_sha256"] ||
		trimDigest(summary.Image.Platforms["linux/amd64"]) != metadata["image_linux_amd64_sha256"] {
		t.Fatalf("published Starry release identity mismatch: %#v", summary)
	}
	expectedContracts := map[string]struct {
		id, status, runtimeStatus, path, digest string
	}{
		"contract_candidate":      {"patch-v1.3.1-contract-candidate", "FROZEN", "BLOCKED", "contracts/patch-v1.3.1/CONTRACT-RELEASE-SUMMARY.json", metadata["contract_candidate_sha256"]},
		"control_openapi":         {"control/v1", "", "", "contracts/control/v1/openapi.yaml", metadata["release_sha256"]},
		"config_schema":           {"config/v5", "", "", "contracts/config/v5/config.schema.json", metadata["config_schema_sha256"]},
		"config_ui_schema":        {"config/v5-ui", "", "", "contracts/config/v5/config.ui-schema.json", metadata["config_ui_schema_sha256"]},
		"relay_quality_protocol":  {"relay-quality/v1", "FROZEN", "", "contracts/relay-quality/v1/rendezvous-extension.proto", metadata["relay_quality_protocol_sha256"]},
		"relay_telemetry_schema":  {"relay-telemetry/v2", "", "", "contracts/relay-telemetry/v2/telemetry.schema.json", metadata["relay_telemetry_schema_sha256"]},
		"fast_relay_protocol":     {"fast-relay/v1", "", "", "contracts/fast-relay/v1/rendezvous-extension.proto", metadata["fast_relay_protocol_sha256"]},
		"fast_media_relay_udp":    {"fast-media/v1", "FROZEN", "PREVIEW", "contracts/fast-media/v1/akr1-wire.json", metadata["fast_media_relay_udp_sha256"]},
		"starry_pairing_protocol": {"starry-pairing/v1", "", "", "contracts/starry-pairing/v1/pairing.schema.json", metadata["starry_pairing_protocol_sha256"]},
		"downgrade_drain_state":   {"config/v5-downgrade-drain-state/v1", "", "", "contracts/config/v5/downgrade-drain-state.schema.json", metadata["downgrade_drain_state_sha256"]},
	}
	if len(summary.Contracts) != len(expectedContracts) {
		t.Fatalf("published Starry contract inventory = %#v", summary.Contracts)
	}
	for name, expected := range expectedContracts {
		actual, found := summary.Contracts[name]
		if !found || actual.ID != expected.id || actual.Status != expected.status ||
			actual.RuntimeReleaseStatus != expected.runtimeStatus ||
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

func validContractCommit(value string) bool {
	if len(value) != 40 {
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
