package starry

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/q1ngyang/rustdesk-api-kessoku/v3/internal/starrycontrol"
)

func validateCapabilitiesResponse(result starrycontrol.Capabilities) error {
	if result.Protocol.Name != "starry-control" || result.Protocol.Major != 1 || !validIdentifier(result.Protocol.Version, 64) {
		return contractResponseError()
	}
	if _, err := uuid.Parse(result.Instance.ID); err != nil || result.Instance.Role != "hbbs" ||
		!validIdentifier(result.Instance.StarryVersion, 128) || !validIdentifier(result.Instance.UpstreamVersion, 128) {
		return contractResponseError()
	}
	versions := []int{
		result.Capabilities.RelayInventory,
		result.Capabilities.AllocationSimulation,
		result.Capabilities.ConfigTransaction,
		result.Capabilities.ConfigRollback,
		result.Capabilities.ConnectionAuth,
	}
	for _, version := range versions {
		if version < 0 || version > 1 {
			return contractResponseError()
		}
	}
	if result.Config.ActiveSchemaVersion < 1 || result.Config.ActiveSchemaVersion > 3 || !validDigest(result.Config.SchemaDigest) {
		return contractResponseError()
	}
	foundActive := false
	seen := make(map[int]struct{}, len(result.Config.SupportedSchemaVersions))
	for _, version := range result.Config.SupportedSchemaVersions {
		if version < 1 || version > 3 {
			return contractResponseError()
		}
		if _, duplicate := seen[version]; duplicate {
			return contractResponseError()
		}
		seen[version] = struct{}{}
		if version == result.Config.ActiveSchemaVersion {
			foundActive = true
		}
	}
	if !foundActive || result.Limits.MaxConfigBytes <= 0 || result.Limits.MaxConfigBytes > 1<<20 ||
		result.Limits.MaxPlanLifetimeSeconds <= 0 || result.Limits.MaxPlanLifetimeSeconds > 600 ||
		result.Limits.OperationRetentionSeconds < 86400 {
		return contractResponseError()
	}
	return nil
}

func validateStatusResponse(result starrycontrol.Status) error {
	if err := validateRuntimeConfigState(result.Config); err != nil {
		return contractResponseError()
	}
	if !oneOf(result.Auth.ConfiguredMode, "off", "audit", "enforce") ||
		!oneOf(result.Auth.EffectiveMode, "off", "audit", "enforce") ||
		!oneOf(result.Auth.VerifierState, "disabled", "ready", "degraded", "unavailable") {
		return contractResponseError()
	}
	return nil
}

func validateRuntimeConfigState(result starrycontrol.RuntimeConfigState) error {
	if !oneOf(result.Status, "disabled_no_config", "active", "unavailable") || result.SchemaVersion != nil && (*result.SchemaVersion < 1 || *result.SchemaVersion > 3) {
		return contractResponseError()
	}
	for _, digest := range []*string{result.SourceDigest, result.EffectiveDigest} {
		if digest != nil && !validDigest(*digest) {
			return contractResponseError()
		}
	}
	if result.LastError != nil && !validResponseText(*result.LastError, 2048) {
		return contractResponseError()
	}
	if result.Status == "active" && (result.Generation == 0 || result.SchemaVersion == nil || result.SourceDigest == nil || result.EffectiveDigest == nil || result.ActivatedAt == nil || result.ActivatedAt.IsZero()) {
		return contractResponseError()
	}
	if result.SubsystemAcks == nil {
		return contractResponseError()
	}
	for _, ack := range result.SubsystemAcks {
		if err := validateSubsystemAck(ack); err != nil {
			return err
		}
	}
	return nil
}

func validateRelaysResponse(inventory starrycontrol.RelayInventory) error {
	if inventory.ConfigGeneration == 0 || !validIdentifier(inventory.HealthSnapshotID, 256) || inventory.Relays == nil || !validResponseText(inventory.Warning, 2048) {
		return contractResponseError()
	}
	seen := make(map[string]struct{}, len(inventory.Relays))
	for _, relay := range inventory.Relays {
		if !validIdentifier(relay.ID, 256) || relay.ConfiguredOrder < 0 || !validIdentifier(relay.Native.State, 96) || relay.Native.ObservedAt.IsZero() || !validIdentifier(relay.WebSocket.State, 96) {
			return contractResponseError()
		}
		if !oneOf(relay.Native.State, "online", "offline", "unknown") || !oneOf(relay.WebSocket.State, "unknown", "healthy", "unhealthy", "disabled") {
			return contractResponseError()
		}
		if _, duplicate := seen[relay.ID]; duplicate {
			return contractResponseError()
		}
		seen[relay.ID] = struct{}{}
		if relay.WebSocket.Configured && (relay.WebSocket.URL == nil || !validWSSURL(*relay.WebSocket.URL)) || !relay.WebSocket.Configured && relay.WebSocket.URL != nil {
			return contractResponseError()
		}
		if relay.WebSocket.LatencyMS != nil && *relay.WebSocket.LatencyMS < 0 || relay.WebSocket.ErrorCode != nil && !validIdentifier(*relay.WebSocket.ErrorCode, 96) {
			return contractResponseError()
		}
		eligible := make(map[starrycontrol.Transport]struct{}, len(relay.EligibleFor))
		for _, transport := range relay.EligibleFor {
			if !transport.Valid() {
				return contractResponseError()
			}
			if _, duplicate := eligible[transport]; duplicate {
				return contractResponseError()
			}
			eligible[transport] = struct{}{}
		}
		for _, rule := range relay.ReferencedByRules {
			if !validResponseText(rule, 256) {
				return contractResponseError()
			}
		}
	}
	return nil
}

func validateSimulationResponse(result starrycontrol.SimulationResult) error {
	if result.ConfigGeneration == 0 || !validIdentifier(result.HealthSnapshotID, 256) || result.Candidates == nil || result.Warnings == nil ||
		!oneOf(result.Selection.Kind, "geo_rule", "single_candidate", "rotation_prediction", "no_eligible_relay") || !result.Selection.NonBinding {
		return contractResponseError()
	}
	if result.MatchedRule != nil {
		if !validResponseText(result.MatchedRule.Name, 256) || result.MatchedRule.Index < 0 || !oneOf(result.MatchedRule.Direction, "direct", "reverse") {
			return contractResponseError()
		}
	}
	seen := make(map[string]struct{}, len(result.Candidates))
	for _, candidate := range result.Candidates {
		if !validIdentifier(candidate.RelayID, 256) || candidate.ConfiguredOrder < 0 || candidate.Priority != nil && *candidate.Priority < 1 {
			return contractResponseError()
		}
		if _, duplicate := seen[candidate.RelayID]; duplicate {
			return contractResponseError()
		}
		seen[candidate.RelayID] = struct{}{}
		if candidate.Eligible && candidate.ExclusionReason != nil || !candidate.Eligible && (candidate.ExclusionReason == nil || !validIdentifier(*candidate.ExclusionReason, 96)) {
			return contractResponseError()
		}
	}
	if result.Selection.RelayID != nil {
		if !validIdentifier(*result.Selection.RelayID, 256) {
			return contractResponseError()
		}
		if _, exists := seen[*result.Selection.RelayID]; !exists {
			return contractResponseError()
		}
	}
	if result.Selection.PredictedIndex != nil && (*result.Selection.PredictedIndex < 0 || *result.Selection.PredictedIndex >= len(result.Candidates)) {
		return contractResponseError()
	}
	for _, warning := range result.Warnings {
		if !validResponseText(warning, 2048) {
			return contractResponseError()
		}
	}
	return nil
}

func validateConfigResponse(result starrycontrol.ConfigDocument) error {
	if !validETag(result.ETag) || result.Format != "yaml" || len(result.Document) > 1<<20 || validateRuntimeConfigState(result.RuntimeConfigState) != nil {
		return contractResponseError()
	}
	digest := documentDigest(result.Document)
	if strings.Trim(result.ETag, "\"") != digest {
		return contractResponseError()
	}
	expectedDrift := result.SourceDigest != nil
	if result.Document != "" {
		expectedDrift = result.SourceDigest == nil || *result.SourceDigest != digest
	}
	if result.Drift != expectedDrift {
		return contractResponseError()
	}
	return nil
}

func validateSchemaResponse(result starrycontrol.SchemaBundle) error {
	if !validETag(result.ETag) || !validDigest(result.Digest) || strings.Trim(result.ETag, "\"") != result.Digest ||
		!validJSONObject(result.Schema) || !validJSONObject(result.UISchema) {
		return contractResponseError()
	}
	return nil
}

func validateValidationResponse(result starrycontrol.ValidationResult) error {
	if result.Diagnostics == nil {
		return contractResponseError()
	}
	if result.Valid && (result.SourceDigest == nil || !validDigest(*result.SourceDigest) || result.EffectiveDigest == nil || !validDigest(*result.EffectiveDigest) || len(result.Diagnostics) != 0) {
		return contractResponseError()
	}
	if !result.Valid && (result.SourceDigest != nil || result.EffectiveDigest != nil || len(result.Diagnostics) == 0) {
		return contractResponseError()
	}
	for _, diagnostic := range result.Diagnostics {
		if !validIdentifier(diagnostic.Code, 96) || !oneOf(diagnostic.Severity, "error", "warning") ||
			!validResponseText(diagnostic.Message, 2048) || len(diagnostic.Pointer) > 1024 ||
			diagnostic.Line != nil && *diagnostic.Line < 1 || diagnostic.Column != nil && *diagnostic.Column < 1 {
			return contractResponseError()
		}
	}
	return nil
}

func validatePlanResponse(result starrycontrol.ConfigPlan, expectedInstanceID, expectedBaseETag, expectedCandidateDigest string) error {
	if _, err := uuid.Parse(result.PlanID); err != nil {
		return contractResponseError()
	}
	if result.InstanceID != expectedInstanceID || result.BaseETag != expectedBaseETag || result.CandidateDigest != expectedCandidateDigest ||
		!validETag(result.BaseETag) || !validDigest(result.CandidateDigest) ||
		result.Changes == nil || !oneOf(result.Impact.Risk, "low", "medium", "high", "critical") || result.ExpiresAt.IsZero() {
		return contractResponseError()
	}
	for _, change := range result.Changes {
		if !validJSONObject(change) {
			return contractResponseError()
		}
	}
	return nil
}

func validateActivationAck(result starrycontrol.ActivationAck) error {
	if result.Generation == 0 || result.SchemaVersion < 1 || result.SchemaVersion > 3 ||
		!validDigest(result.SourceDigest) || !validDigest(result.EffectiveDigest) || result.ActivatedAt.IsZero() || len(result.SubsystemAcks) == 0 {
		return contractResponseError()
	}
	if result.AuditID != nil {
		if _, err := uuid.Parse(*result.AuditID); err != nil {
			return contractResponseError()
		}
	}
	for _, ack := range result.SubsystemAcks {
		if err := validateSubsystemAck(ack); err != nil {
			return err
		}
		if !ack.Accepted {
			return contractResponseError()
		}
	}
	return nil
}

func validateOperationResponse(result starrycontrol.Operation, expectedID string) error {
	if _, err := uuid.Parse(result.ID); err != nil {
		return contractResponseError()
	}
	if expectedID != "" && result.ID != expectedID || !oneOf(result.Kind, "config_apply", "config_rollback") ||
		!oneOf(result.State, "pending", "running", "succeeded", "rolled_back", "failed", "manual_intervention_required") ||
		result.CreatedAt.IsZero() || result.UpdatedAt.IsZero() || result.UpdatedAt.Before(result.CreatedAt) {
		return contractResponseError()
	}
	if result.AuditID != nil {
		if _, err := uuid.Parse(*result.AuditID); err != nil {
			return contractResponseError()
		}
	}
	if result.ActivationAck != nil && validateActivationAck(*result.ActivationAck) != nil {
		return contractResponseError()
	}
	if result.Error != nil && validateOperationProblem(*result.Error) != nil {
		return contractResponseError()
	}
	switch result.State {
	case "pending", "running":
		if result.ActivationAck != nil || result.Error != nil {
			return contractResponseError()
		}
	case "succeeded":
		if result.ActivationAck == nil || result.Error != nil {
			return contractResponseError()
		}
	case "rolled_back", "failed", "manual_intervention_required":
		if result.Error == nil {
			return contractResponseError()
		}
	}
	return nil
}

func validateHistoryResponse(revisions []starrycontrol.ConfigRevision) error {
	if revisions == nil {
		return contractResponseError()
	}
	seen := make(map[string]struct{}, len(revisions))
	for _, revision := range revisions {
		if _, err := uuid.Parse(revision.ID); err != nil {
			return contractResponseError()
		}
		if revision.Generation == 0 || !validETag(revision.BeforeETag) || !validETag(revision.AfterETag) || !validDigest(revision.CandidateDigest) ||
			!validResponseText(revision.Actor, 256) || revision.CreatedAt.IsZero() || !validIdentifier(revision.Result, 96) {
			return contractResponseError()
		}
		if _, duplicate := seen[revision.ID]; duplicate {
			return contractResponseError()
		}
		seen[revision.ID] = struct{}{}
		if revision.Comment != "" && !validResponseText(revision.Comment, 1024) {
			return contractResponseError()
		}
	}
	return nil
}

func validateSubsystemAck(ack starrycontrol.SubsystemAck) error {
	if !validIdentifier(ack.Subsystem, 96) || !validResponseText(ack.Detail, 2048) {
		return contractResponseError()
	}
	return nil
}

func validateOperationProblem(problem starrycontrol.OperationProblem) error {
	parsed, err := url.Parse(problem.Type)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || !validResponseText(problem.Title, 256) ||
		problem.Status < 400 || problem.Status > 599 || !validIdentifier(problem.Code, 96) ||
		!validResponseText(problem.Detail, 2048) || problem.Errors == nil {
		return contractResponseError()
	}
	if _, err := uuid.Parse(problem.RequestID); err != nil {
		return contractResponseError()
	}
	for _, diagnostic := range problem.Errors {
		if !validIdentifier(diagnostic.Code, 96) || !oneOf(diagnostic.Severity, "error", "warning") ||
			!validResponseText(diagnostic.Message, 2048) || len(diagnostic.Pointer) > 1024 ||
			diagnostic.Line != nil && *diagnostic.Line < 1 || diagnostic.Column != nil && *diagnostic.Column < 1 {
			return contractResponseError()
		}
	}
	return nil
}

func documentDigest(document string) string {
	digest := sha256.Sum256([]byte(document))
	return "sha256:" + hex.EncodeToString(digest[:])
}

func oneOf(value string, allowed ...string) bool {
	for _, candidate := range allowed {
		if value == candidate {
			return true
		}
	}
	return false
}

func contractResponseError() error {
	return &starrycontrol.ProviderError{
		Status:  http.StatusBadGateway,
		Code:    "CONTRACT_RESPONSE_INVALID",
		Message: "Starry contract response is invalid",
	}
}

func validJSONObject(value json.RawMessage) bool {
	if len(value) == 0 || !json.Valid(value) {
		return false
	}
	var object map[string]json.RawMessage
	return json.Unmarshal(value, &object) == nil && object != nil
}

func validWSSURL(value string) bool {
	parsed, err := url.Parse(value)
	return err == nil && parsed.Scheme == "wss" && parsed.Host != "" && parsed.User == nil && parsed.Fragment == ""
}

func validResponseText(value string, maximum int) bool {
	if value == "" || len(value) > maximum || strings.TrimSpace(value) != value || !utf8.ValidString(value) {
		return false
	}
	for _, character := range value {
		if unicode.IsControl(character) {
			return false
		}
	}
	return true
}
