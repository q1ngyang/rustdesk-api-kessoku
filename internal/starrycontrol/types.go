package starrycontrol

import (
	"encoding/json"
	"time"
)

type Capabilities struct {
	Protocol     ProtocolInfo       `json:"protocol"`
	Instance     InstanceInfo       `json:"instance"`
	Capabilities CapabilityVersions `json:"capabilities"`
	Config       ConfigCapabilities `json:"config"`
	Limits       AgentLimits        `json:"limits"`
}

type ProtocolInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	Major   int    `json:"major"`
}

type InstanceInfo struct {
	ID              string `json:"id"`
	Role            string `json:"role"`
	StarryVersion   string `json:"starry_version"`
	UpstreamVersion string `json:"upstream_version"`
}

type CapabilityVersions struct {
	RelayInventory       int `json:"relay_inventory"`
	AllocationSimulation int `json:"allocation_simulation"`
	ConfigTransaction    int `json:"config_transaction"`
	ConfigRollback       int `json:"config_rollback"`
	ConnectionAuth       int `json:"connection_auth"`
	RelayQuality         int `json:"relay_quality"`
	RelayActiveProbe     int `json:"relay_active_probe"`
	RelayProbeProtocol   int `json:"relay_probe_protocol"`
	RelayLoadProtocol    int `json:"relay_load_protocol"`
	RelayTelemetrySchema int `json:"relay_telemetry_schema"`
	PeerRegistry         int `json:"peer_registry"`
}

type PeerIdentityInput struct {
	ID              string   `json:"id"`
	UUID            string   `json:"uuid"`
	ActivationEpoch uint64   `json:"activation_epoch,omitempty"`
	ActivationID    string   `json:"activation_id,omitempty"`
	RouteLeases     []string `json:"route_leases,omitempty"`
}

type PeerVerification struct {
	InstanceID string `json:"instance_id"`
	Registered bool   `json:"registered"`
}

type ConfigCapabilities struct {
	SupportedSchemaVersions []int  `json:"supported_schema_versions"`
	ActiveSchemaVersion     int    `json:"active_schema_version"`
	SchemaDigest            string `json:"schema_digest"`
}

type AgentLimits struct {
	MaxConfigBytes            int64 `json:"max_config_bytes"`
	MaxPlanLifetimeSeconds    int64 `json:"max_plan_lifetime_seconds"`
	OperationRetentionSeconds int64 `json:"operation_retention_seconds"`
}

// Status mirrors Starry's /control/v1/status response. Instance identity is
// negotiated separately through Capabilities before every operation.
type Status struct {
	Ready  bool               `json:"ready"`
	Config RuntimeConfigState `json:"config"`
	Auth   AuthStatus         `json:"auth"`
}

type RuntimeConfigState struct {
	Status          string         `json:"status"`
	Generation      uint64         `json:"generation"`
	SchemaVersion   *int           `json:"schema_version"`
	SourceDigest    *string        `json:"source_digest"`
	EffectiveDigest *string        `json:"effective_digest"`
	ActivatedAt     *time.Time     `json:"activated_at"`
	SubsystemAcks   []SubsystemAck `json:"subsystem_acks"`
	LastError       *string        `json:"last_error"`
}

type AuthStatus struct {
	ConfiguredMode string      `json:"configured_mode"`
	EffectiveMode  string      `json:"effective_mode"`
	VerifierState  string      `json:"verifier_state"`
	KeyCount       uint64      `json:"key_count"`
	KeyAgeSeconds  *uint64     `json:"key_age_seconds"`
	Metrics        AuthMetrics `json:"metrics"`
}

type AuthMetrics struct {
	Attempts              uint64 `json:"attempts"`
	Allowed               uint64 `json:"allowed"`
	Denied                uint64 `json:"denied"`
	AuditWouldDeny        uint64 `json:"audit_would_deny"`
	CacheHits             uint64 `json:"cache_hits"`
	IntrospectionRequests uint64 `json:"introspection_requests"`
	IntrospectionFailures uint64 `json:"introspection_failures"`
}

type RelayInventory struct {
	ConfigGeneration uint64               `json:"config_generation"`
	HealthSnapshotID string               `json:"health_snapshot_id"`
	Relays           []Relay              `json:"relays"`
	Quality          *RelayQualityRuntime `json:"quality,omitempty"`
	Warning          string               `json:"warning"`
}

type Relay struct {
	ID string `json:"id"`
	// Version is optional in Control v1. Current Starry versions may omit it;
	// Kessoku reports that explicitly instead of guessing from the center.
	Version           string               `json:"version,omitempty"`
	Capabilities      *RelayCapabilities   `json:"capabilities,omitempty"`
	QualityCandidate  *bool                `json:"quality_candidate,omitempty"`
	ConfiguredOrder   int                  `json:"configured_order"`
	Native            NativeRelayStatus    `json:"native"`
	WebSocket         WebSocketRelayStatus `json:"websocket"`
	EligibleFor       []Transport          `json:"eligible_for"`
	ReferencedByRules []string             `json:"referenced_by_rules"`
}

type RelayCapabilities struct {
	RelayProbeProtocol *int `json:"relay_probe_protocol"`
	RelayLoadProtocol  *int `json:"relay_load_protocol"`
}

type NativeRelayStatus struct {
	State      string    `json:"state"`
	ObservedAt time.Time `json:"observed_at"`
}

type WebSocketRelayStatus struct {
	Configured                   bool       `json:"configured"`
	URL                          *string    `json:"url"`
	State                        string     `json:"state"`
	LastProbeAt                  *time.Time `json:"last_probe_at"`
	ObservedAt                   *time.Time `json:"observed_at,omitempty"`
	ObservedAtUnixMS             *uint64    `json:"observed_at_unix_ms,omitempty"`
	AgeSeconds                   *uint64    `json:"age_seconds,omitempty"`
	Stale                        *bool      `json:"stale,omitempty"`
	LatencyMS                    *int64     `json:"latency_ms"`
	TelemetrySchema              *int       `json:"telemetry_schema,omitempty"`
	ProcessInstanceID            *string    `json:"process_instance_id,omitempty"`
	TelemetrySequence            *uint64    `json:"telemetry_sequence,omitempty"`
	UptimeSeconds                *uint64    `json:"uptime_seconds,omitempty"`
	TelemetryRestarts            *uint64    `json:"telemetry_restarts,omitempty"`
	LastRestartAt                *time.Time `json:"last_restart_at,omitempty"`
	LoadBasisPoints              *int       `json:"load_basis_points,omitempty"`
	ActiveSessions               *int       `json:"active_sessions,omitempty"`
	PendingPairs                 *int       `json:"pending_pairs,omitempty"`
	CapacitySessions             *int       `json:"capacity_sessions,omitempty"`
	BandwidthBPS                 *uint64    `json:"bandwidth_bps,omitempty"`
	BandwidthEMAAlphaBasisPoints *int       `json:"bandwidth_ema_alpha_basis_points,omitempty"`
	CapacityBandwidthBPS         *uint64    `json:"capacity_bandwidth_bps,omitempty"`
	Draining                     *bool      `json:"draining,omitempty"`
	AdmissionOpen                *bool      `json:"admission_open,omitempty"`
	AdmissionRejections          *uint64    `json:"admission_rejections,omitempty"`
	ProbeMalformed               *uint64    `json:"probe_malformed,omitempty"`
	ProbeUnsupported             *uint64    `json:"probe_unsupported,omitempty"`
	ProbeRateLimited             *uint64    `json:"probe_rate_limited,omitempty"`
	ProbeSuccessful              *uint64    `json:"probe_successful,omitempty"`
	TelemetryAuthFailures        *uint64    `json:"telemetry_auth_failures,omitempty"`
	ErrorCode                    *string    `json:"error_code"`
	ErrorMessage                 *string    `json:"error_message,omitempty"`
}

// RelayQualityRuntime contains only the redacted aggregate counters published
// by Starry Control v1. It intentionally has no client report, allocation,
// session, nonce, IP, or token fields.
type RelayQualityRuntime struct {
	ProtocolVersion             int                          `json:"protocol_version"`
	Strategy                    string                       `json:"strategy"`
	Enabled                     *bool                        `json:"enabled"`
	ActiveAllocations           *uint64                      `json:"active_allocations"`
	CachedNetworkPairs          *uint64                      `json:"cached_network_pairs"`
	PendingDecisions            *uint64                      `json:"pending_decisions"`
	OffersCreated               *uint64                      `json:"offers_created"`
	OffersSkipped               *uint64                      `json:"offers_skipped"`
	OfferSkipReasons            *RelayQualityOfferSkipReason `json:"offer_skip_reasons"`
	PeerReportsAccepted         *uint64                      `json:"peer_reports_accepted"`
	ControllerReportsAccepted   *uint64                      `json:"controller_reports_accepted"`
	ReportsAccepted             *uint64                      `json:"reports_accepted"`
	ReportsDuplicate            *uint64                      `json:"reports_duplicate"`
	ReportsStageMismatch        *uint64                      `json:"reports_stage_mismatch"`
	ReportsLate                 *uint64                      `json:"reports_late"`
	ReportsInvalid              *uint64                      `json:"reports_invalid"`
	ReportsBindingMismatch      *uint64                      `json:"reports_binding_mismatch"`
	DecisionsCreated            *uint64                      `json:"decisions_created"`
	FallbackDecisions           *uint64                      `json:"fallback_decisions"`
	FallbackReasons             *RelayQualityFallbackReason  `json:"fallback_reasons"`
	CacheHits                   *uint64                      `json:"cache_hits"`
	HysteresisDecisions         *uint64                      `json:"hysteresis_decisions"`
	PrimaryProbes               *uint64                      `json:"primary_probes"`
	PrimaryAccepted             *uint64                      `json:"primary_accepted"`
	ExpansionsTriggered         *uint64                      `json:"expansions_triggered"`
	P2PCancellations            *uint64                      `json:"p2p_cancellations"`
	EstimatedProbeAttemptsSaved *uint64                      `json:"estimated_probe_attempts_saved"`
	ExpandedDecisions           *uint64                      `json:"expanded_decisions"`
	StageTimeouts               *uint64                      `json:"stage_timeouts"`
	RelaySelections             map[string]uint64            `json:"relay_selections"`
	RelaySelectionOverflow      *uint64                      `json:"relay_selection_overflow"`
}

type RelayQualityOfferSkipReason struct {
	Disabled               *uint64 `json:"disabled"`
	UnsupportedClient      *uint64 `json:"unsupported_client"`
	InvalidFallback        *uint64 `json:"invalid_fallback"`
	InconsistentSnapshot   *uint64 `json:"inconsistent_snapshot"`
	InsufficientCandidates *uint64 `json:"insufficient_candidates"`
	PrimaryNotProbeable    *uint64 `json:"primary_not_probeable"`
}

type RelayQualityFallbackReason struct {
	LegacyFallback *uint64 `json:"legacy_fallback"`
	ProbeFailure   *uint64 `json:"probe_failure"`
	ManualOverride *uint64 `json:"manual_override"`
	InvalidReport  *uint64 `json:"invalid_report"`
	ReportLate     *uint64 `json:"report_late"`
}

type Transport string

const (
	TransportNative Transport = "native"
	TransportWSS    Transport = "wss"
	TransportMixed  Transport = "mixed"
)

func (t Transport) Valid() bool {
	return t == TransportNative || t == TransportWSS || t == TransportMixed
}

type SimulationInput struct {
	ClientA                  SimulationClient `json:"client_a"`
	ClientB                  SimulationClient `json:"client_b"`
	Transport                Transport        `json:"transport"`
	Explain                  bool             `json:"explain"`
	ExpectedConfigGeneration *uint64          `json:"expected_config_generation,omitempty"`
}

type SimulationClient struct {
	IP string `json:"ip"`
}

type SimulationResult struct {
	ConfigGeneration uint64                `json:"config_generation"`
	HealthSnapshotID string                `json:"health_snapshot_id"`
	MatchedRule      *MatchedRule          `json:"matched_rule,omitempty"`
	Candidates       []AllocationCandidate `json:"candidates"`
	Selection        AllocationSelection   `json:"selection"`
	Warnings         []string              `json:"warnings"`
}

type MatchedRule struct {
	Name      string `json:"name"`
	Index     int    `json:"index"`
	Direction string `json:"direction"`
}

type AllocationCandidate struct {
	RelayID         string  `json:"relay_id"`
	ConfiguredOrder int     `json:"configured_order"`
	Priority        *int    `json:"priority"`
	Eligible        bool    `json:"eligible"`
	ExclusionReason *string `json:"exclusion_reason"`
}

type AllocationSelection struct {
	Kind           string  `json:"kind"`
	RelayID        *string `json:"relay_id"`
	PredictedIndex *int    `json:"predicted_index"`
	NonBinding     bool    `json:"non_binding"`
}

type ConfigDocument struct {
	RuntimeConfigState
	ETag     string `json:"etag"`
	Drift    bool   `json:"drift"`
	Document string `json:"document"`
	Format   string `json:"format"`
}

type SchemaBundle struct {
	ETag     string          `json:"etag"`
	Digest   string          `json:"digest"`
	Schema   json.RawMessage `json:"schema" swaggertype:"object"`
	UISchema json.RawMessage `json:"ui_schema" swaggertype:"object"`
}

type ConfigCandidate struct {
	Document string `json:"document"`
	Format   string `json:"format"`
	BaseETag string `json:"-"`
}

type Diagnostic struct {
	Code     string `json:"code"`
	Pointer  string `json:"pointer,omitempty"`
	Line     *int   `json:"line,omitempty"`
	Column   *int   `json:"column,omitempty"`
	Severity string `json:"severity"`
	Message  string `json:"message"`
}

type ValidationResult struct {
	Valid           bool         `json:"valid"`
	SourceDigest    *string      `json:"source_digest"`
	EffectiveDigest *string      `json:"effective_digest"`
	Diagnostics     []Diagnostic `json:"diagnostics"`
}

type ConfigPlan struct {
	PlanID          string            `json:"plan_id"`
	InstanceID      string            `json:"instance_id"`
	BaseETag        string            `json:"base_etag"`
	BaseGeneration  uint64            `json:"base_generation"`
	CandidateDigest string            `json:"candidate_digest"`
	Changes         []json.RawMessage `json:"changes" swaggertype:"array,object"`
	Impact          PlanImpact        `json:"impact"`
	ExpiresAt       time.Time         `json:"expires_at"`
}

type PlanImpact struct {
	Risk            string `json:"risk"`
	RestartRequired bool   `json:"restart_required"`
}

type ApplyRequest struct {
	PlanID          string `json:"plan_id"`
	CandidateDigest string `json:"candidate_digest"`
	IfMatch         string `json:"-"`
	IdempotencyKey  string `json:"-"`
	Comment         string `json:"comment,omitempty"`
}

type RollbackRequest struct {
	RevisionID     string `json:"revision_id"`
	IfMatch        string `json:"-"`
	IdempotencyKey string `json:"-"`
	Comment        string `json:"comment,omitempty"`
}

type RuntimeReloadRequest struct {
	ExpectedSourceDigest string `json:"expected_source_digest"`
	IdempotencyKey       string `json:"-"`
}

type SubsystemAck struct {
	Subsystem string `json:"subsystem"`
	Accepted  bool   `json:"accepted"`
	Detail    string `json:"detail"`
}

type ActivationAck struct {
	Generation      uint64         `json:"generation"`
	SchemaVersion   int            `json:"schema_version"`
	SourceDigest    string         `json:"source_digest"`
	EffectiveDigest string         `json:"effective_digest"`
	ActivatedAt     time.Time      `json:"activated_at"`
	AuditID         *string        `json:"audit_id"`
	SubsystemAcks   []SubsystemAck `json:"subsystem_acks"`
}

type OperationProblem struct {
	Type      string       `json:"type"`
	Title     string       `json:"title"`
	Status    int          `json:"status"`
	Code      string       `json:"code"`
	Detail    string       `json:"detail"`
	RequestID string       `json:"request_id"`
	Retryable bool         `json:"retryable"`
	Errors    []Diagnostic `json:"errors"`
}

type Operation struct {
	ID            string            `json:"id"`
	AuditID       *string           `json:"audit_id"`
	Kind          string            `json:"kind"`
	State         string            `json:"state"`
	CreatedAt     time.Time         `json:"created_at"`
	UpdatedAt     time.Time         `json:"updated_at"`
	ActivationAck *ActivationAck    `json:"activation_ack"`
	Error         *OperationProblem `json:"error"`
}

type ConfigRevision struct {
	ID              string    `json:"id"`
	Generation      uint64    `json:"generation"`
	BeforeETag      string    `json:"before_etag"`
	AfterETag       string    `json:"after_etag"`
	CandidateDigest string    `json:"candidate_digest"`
	Actor           string    `json:"actor"`
	Comment         string    `json:"comment"`
	CreatedAt       time.Time `json:"created_at"`
	Result          string    `json:"result"`
}
