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

	"github.com/q1ngyang/rustdesk-api-kessoku/v2/internal/starrycontrol"
)

func TestLocalStarryContractCandidate(t *testing.T) {
	metadata := readCandidateMetadata(t)
	if metadata["control_contract"] != "control/v1" || metadata["config_schema"] != "config/v3" ||
		!validContractHexDigest(metadata["candidate_sha256"]) {
		t.Fatalf("invalid contract provenance: %#v", metadata)
	}
	expectedDigest := metadata["candidate_sha256"]
	switch metadata["status"] {
	case "LOCAL_CANDIDATE_VALIDATED":
		if metadata["starry_tag"] != "UNPUBLISHED" || metadata["release_sha256"] != "UNAVAILABLE" {
			t.Fatalf("local candidate must remain release-blocked: %#v", metadata)
		}
	case "PINNED":
		if metadata["starry_tag"] == "" || metadata["starry_tag"] == "UNPUBLISHED" ||
			metadata["release_sha256"] != metadata["candidate_sha256"] {
			t.Fatalf("published contract provenance is incomplete: %#v", metadata)
		}
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
	var status starrycontrol.Status
	readExample(t, root, "status.json", &status)
	if err := validateStatusResponse(status); err != nil {
		t.Fatalf("status fixture: %v", err)
	}
	var relays starrycontrol.RelayInventory
	readExample(t, root, "relays.json", &relays)
	if err := validateRelaysResponse(relays); err != nil {
		t.Fatalf("relay fixture: %v", err)
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

	schema := readContractFile(t, root, "contracts/config/v3/config.schema.json")
	uiSchema := readContractFile(t, root, "contracts/config/v3/config.ui-schema.json")
	schemaHash := sha256.Sum256(schema)
	schemaDigest := "sha256:" + hex.EncodeToString(schemaHash[:])
	if schemaDigest != capabilities.Config.SchemaDigest {
		t.Fatalf("capabilities schema digest = %s, actual schema digest = %s", capabilities.Config.SchemaDigest, schemaDigest)
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
