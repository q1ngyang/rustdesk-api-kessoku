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
	RelayInventory                  int `json:"relay_inventory"`
	AllocationSimulation            int `json:"allocation_simulation"`
	ConfigTransaction               int `json:"config_transaction"`
	ConfigRollback                  int `json:"config_rollback"`
	ConnectionAuth                  int `json:"connection_auth"`
	RelayQuality                    int `json:"relay_quality"`
	RelayActiveProbe                int `json:"relay_active_probe"`
	RelayProbeProtocol              int `json:"relay_probe_protocol"`
	RelayLoadProtocol               int `json:"relay_load_protocol"`
	RelayTelemetrySchema            int `json:"relay_telemetry_schema"`
	FastRelayAuthorization          int `json:"fast_relay_authorization"`
	FastMediaRelayUDP               int `json:"fast_media_relay_udp"`
	StarryPairing                   int `json:"starry_pairing"`
	ConfigDowngradePreview          int `json:"config_downgrade_preview"`
	RelayEnrollment                 int `json:"relay_enrollment"`
	RelayEnrollmentWrite            int `json:"relay_enrollment_write"`
	RelayEnrollmentHealthActivation int `json:"relay_enrollment_health_activation"`
	ProfileActivationLease          int `json:"profile_activation_lease"`
	PeerRegistry                    int `json:"peer_registry"`
	ConfigSchema                    int `json:"config_schema"`
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
	ConfigGeneration  uint64                    `json:"config_generation"`
	HealthSnapshotID  string                    `json:"health_snapshot_id"`
	Relays            []Relay                   `json:"relays"`
	Quality           *RelayQualityRuntime      `json:"quality,omitempty"`
	FastRelay         *FastRelayRuntime         `json:"fast_relay,omitempty"`
	ProfileActivation *ProfileActivationRuntime `json:"profile_activation,omitempty"`
	Warning           string                    `json:"warning"`
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
	FastMediaUDP      *FastMediaUDPRuntime `json:"fast_media_udp,omitempty"`
	EligibleFor       []Transport          `json:"eligible_for"`
	ReferencedByRules []string             `json:"referenced_by_rules"`

	presentFields map[string]struct{}
}

func (relay *Relay) UnmarshalJSON(data []byte) error {
	type wireRelay Relay
	var decoded wireRelay
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return err
	}
	*relay = Relay(decoded)
	relay.presentFields = make(map[string]struct{}, len(fields))
	for field := range fields {
		relay.presentFields[field] = struct{}{}
	}
	return nil
}

// HasWireField distinguishes an omitted additive field from an explicit null.
// Values assembled directly in Go are treated as complete test/internal data.
func (relay Relay) HasWireField(field string) bool {
	if relay.presentFields == nil {
		return true
	}
	_, present := relay.presentFields[field]
	return present
}

type RelayCapabilities struct {
	RelayProbeProtocol *int `json:"relay_probe_protocol"`
	RelayLoadProtocol  *int `json:"relay_load_protocol"`
	FastMediaRelayUDP  *int `json:"fast_media_relay_udp"`

	presentFields map[string]struct{}
}

func (capabilities *RelayCapabilities) UnmarshalJSON(data []byte) error {
	type wireRelayCapabilities RelayCapabilities
	var decoded wireRelayCapabilities
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return err
	}
	*capabilities = RelayCapabilities(decoded)
	capabilities.presentFields = make(map[string]struct{}, len(fields))
	for field := range fields {
		capabilities.presentFields[field] = struct{}{}
	}
	return nil
}

func (capabilities RelayCapabilities) HasWireField(field string) bool {
	if capabilities.presentFields == nil {
		return true
	}
	_, present := capabilities.presentFields[field]
	return present
}

// FastMediaUDPRuntime is Starry's redacted HBBR AKR1 aggregate. It contains
// no allocation/session UUIDs, client addresses, grants, tokens, or media.
type FastMediaUDPRuntime struct {
	ConfiguredPort     *int    `json:"configured_port"`
	ReportedPort       *int    `json:"reported_port"`
	Enabled            *bool   `json:"enabled"`
	Healthy            *bool   `json:"healthy"`
	ActiveAllocations  *uint64 `json:"active_allocations"`
	ActiveStreams      *uint64 `json:"active_streams"`
	HelloAccepted      *uint64 `json:"hello_accepted"`
	CookieRejected     *uint64 `json:"cookie_rejected"`
	BindSucceeded      *uint64 `json:"bind_succeeded"`
	BindRejected       *uint64 `json:"bind_rejected"`
	GrantRejected      *uint64 `json:"grant_rejected"`
	RoleMismatch       *uint64 `json:"role_mismatch"`
	SessionMismatch    *uint64 `json:"session_mismatch"`
	AllocationMismatch *uint64 `json:"allocation_mismatch"`
	Rebinds            *uint64 `json:"rebinds"`
	ForwardedPackets   *uint64 `json:"forwarded_packets"`
	ForwardedBytes     *uint64 `json:"forwarded_bytes"`
	DroppedPackets     *uint64 `json:"dropped_packets"`
	RateLimited        *uint64 `json:"rate_limited"`
	ReplayRejected     *uint64 `json:"replay_rejected"`
	ExpiredAllocations *uint64 `json:"expired_allocations"`
	ListenerFailures   *uint64 `json:"listener_failures"`

	// missingRequiredFields preserves the distinction made by the frozen
	// Starry schema between an explicitly-null aggregate and an absent key.
	// Every field in fast_media_udp is required on the wire, although each
	// value may be null while telemetry is unavailable.
	missingRequiredFields []string
}

func (runtime *FastMediaUDPRuntime) UnmarshalJSON(data []byte) error {
	type wireFastMediaUDPRuntime FastMediaUDPRuntime
	var decoded wireFastMediaUDPRuntime
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return err
	}
	required := [...]string{
		"configured_port", "reported_port", "enabled", "healthy",
		"active_allocations", "active_streams", "hello_accepted", "cookie_rejected",
		"bind_succeeded", "bind_rejected", "grant_rejected", "role_mismatch",
		"session_mismatch", "allocation_mismatch", "rebinds", "forwarded_packets",
		"forwarded_bytes", "dropped_packets", "rate_limited", "replay_rejected",
		"expired_allocations", "listener_failures",
	}
	missing := make([]string, 0)
	for _, field := range required {
		if _, present := fields[field]; !present {
			missing = append(missing, field)
		}
	}
	*runtime = FastMediaUDPRuntime(decoded)
	runtime.missingRequiredFields = missing
	return nil
}

// MissingRequiredFields returns frozen schema-v5 keys omitted by a decoded
// Starry response. It returns a copy so callers cannot alter validation state.
func (runtime FastMediaUDPRuntime) MissingRequiredFields() []string {
	return append([]string(nil), runtime.missingRequiredFields...)
}

// FastRelayRuntime contains server-side authorization and fallback counters.
// These counters prove only that Starry processed an event; they do not prove
// that any particular client entered FastCompat or FastMedia.
type FastRelayRuntime struct {
	ProtocolVersion int `json:"protocol_version"`

	// Enabled and Issued are the exact patch-v1.3.0 FastCompat wire names.
	// They remain typed solely so an older Starry can keep its existing Relay
	// page; v1.3.1 responses use the fields below.
	Enabled *bool   `json:"enabled,omitempty"`
	Issued  *uint64 `json:"issued,omitempty"`

	FastCompatEnabled                       *bool   `json:"fast_compat_enabled,omitempty"`
	FastMediaV1Enabled                      *bool   `json:"fast_media_v1_enabled,omitempty"`
	ActiveAuthorizations                    *uint64 `json:"active_authorizations,omitempty"`
	ActiveFastMediaAuthorizations           *uint64 `json:"active_fast_media_authorizations,omitempty"`
	LastFastMediaAuthorizationExpiresAtUnix *uint64 `json:"last_fast_media_authorization_expires_at_unix,omitempty"`
	IssuedSessions                          *uint64 `json:"issued_sessions,omitempty"`
	TargetGrantsIssued                      *uint64 `json:"target_grants_issued,omitempty"`
	ControllerGrantsIssued                  *uint64 `json:"controller_grants_issued,omitempty"`
	FastCompatSessions                      *uint64 `json:"fast_compat_sessions,omitempty"`
	FastMediaSessions                       *uint64 `json:"fast_media_sessions,omitempty"`
	Reused                                  *uint64 `json:"reused,omitempty"`
	Delivered                               *uint64 `json:"delivered,omitempty"`
	Disabled                                *uint64 `json:"disabled,omitempty"`
	InsecureRequests                        *uint64 `json:"insecure_requests,omitempty"`
	InvalidConfiguration                    *uint64 `json:"invalid_configuration,omitempty"`
	InvalidUUIDs                            *uint64 `json:"invalid_uuids,omitempty"`
	InvalidServerSelection                  *uint64 `json:"invalid_server_selection,omitempty"`
	MissingSigningKeys                      *uint64 `json:"missing_signing_keys,omitempty"`
	SigningFailures                         *uint64 `json:"signing_failures,omitempty"`
	QualitySelectionFailures                *uint64 `json:"quality_selection_failures,omitempty"`
	RateLimited                             *uint64 `json:"rate_limited,omitempty"`
	ResponseMisses                          *uint64 `json:"response_misses,omitempty"`
	ExpiredCacheEvictions                   *uint64 `json:"expired_cache_evictions,omitempty"`
	FastMediaUnavailable                    *uint64 `json:"fast_media_unavailable,omitempty"`
	ReliableFallbacks                       *uint64 `json:"reliable_fallbacks,omitempty"`
}

type ProfileActivationRuntime struct {
	ProtocolVersion     int     `json:"protocol_version"`
	ActiveLeases        *uint64 `json:"active_leases"`
	LastRouteGeneration *uint64 `json:"last_route_generation"`
	LeasesIssued        *uint64 `json:"leases_issued"`
	LeasesReused        *uint64 `json:"leases_reused"`
	ReadyAcks           *uint64 `json:"ready_acks"`
	FastReregistrations *uint64 `json:"fast_reregistrations"`
	Renewals            *uint64 `json:"renewals"`
	RouteReplacements   *uint64 `json:"route_replacements"`
	Deactivations       *uint64 `json:"deactivations"`
	DisconnectCleanups  *uint64 `json:"disconnect_cleanups"`
	TTLExpirations      *uint64 `json:"ttl_expirations"`
	InvalidRequests     *uint64 `json:"invalid_requests"`
	StaleRejections     *uint64 `json:"stale_rejections"`
	RateLimited         *uint64 `json:"rate_limited"`
	CapacityRejections  *uint64 `json:"capacity_rejections"`
	LeaseTTLSeconds     *uint64 `json:"lease_ttl_seconds"`
	BurstWindowSeconds  *uint64 `json:"burst_window_seconds"`
	BurstLimit          *uint64 `json:"burst_limit"`
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
	ErrorMessage                 *string    `json:"error_message"`

	presentFields map[string]struct{}
}

func (status *WebSocketRelayStatus) UnmarshalJSON(data []byte) error {
	type wireWebSocketRelayStatus WebSocketRelayStatus
	var decoded wireWebSocketRelayStatus
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return err
	}
	*status = WebSocketRelayStatus(decoded)
	status.presentFields = make(map[string]struct{}, len(fields))
	for field := range fields {
		status.presentFields[field] = struct{}{}
	}
	return nil
}

func (status WebSocketRelayStatus) HasWireField(field string) bool {
	if status.presentFields == nil {
		return true
	}
	_, present := status.presentFields[field]
	return present
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
	ReviewToken     string            `json:"review_token,omitempty"`
}

type PlanImpact struct {
	Risk            string `json:"risk"`
	RestartRequired bool   `json:"restart_required"`
}

type ApplyRequest struct {
	PlanID           string `json:"plan_id"`
	CandidateDigest  string `json:"candidate_digest"`
	IfMatch          string `json:"-"`
	IdempotencyKey   string `json:"-"`
	Comment          string `json:"comment,omitempty"`
	ReviewToken      string `json:"-"`
	RiskConfirmation string `json:"-"`
}

type RollbackRequest struct {
	RevisionID       string `json:"revision_id"`
	IfMatch          string `json:"-"`
	IdempotencyKey   string `json:"-"`
	Comment          string `json:"comment,omitempty"`
	RiskConfirmation string `json:"-"`
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

type RelayEnrollmentPrepareRequest struct {
	Version              int     `json:"version"`
	NodeID               string  `json:"node_id"`
	RelayServer          string  `json:"relay_server"`
	PublicEndpoint       string  `json:"public_endpoint"`
	RelayPool            string  `json:"relay_pool"`
	Profile              string  `json:"profile"`
	WSSEndpoint          *string `json:"wss_endpoint"`
	ActivateAfterHealth  bool    `json:"activate_after_health"`
	MaxSessions          int     `json:"max_sessions"`
	CapacityBandwidthBPS uint64  `json:"capacity_bandwidth_bps"`
	Draining             bool    `json:"draining"`
	FastMediaUDPPort     *int    `json:"fast_media_udp_port"`
	ExpiresInSeconds     int     `json:"expires_in_seconds,omitempty"`
}

type RelayEnrollmentPrepareResponse struct {
	Version             int    `json:"version"`
	EnrollmentID        string `json:"enrollment_id"`
	ConfigurationDigest string `json:"configuration_digest"`
	ExpiresAtUnix       uint64 `json:"expires_at_unix"`
	State               string `json:"state"`
	Reused              bool   `json:"reused"`
}

type RelayEnrollmentCompleteRequest struct {
	Version             int    `json:"version"`
	EnrollmentID        string `json:"enrollment_id"`
	ConfigurationDigest string `json:"configuration_digest"`
	RequestDigest       string `json:"request_digest"`
	KeyFingerprint      string `json:"key_fingerprint"`
	CSRPEM              string `json:"csr_pem"`
}

type RelayEnrollmentCompleteResponse struct {
	Version             int                   `json:"version"`
	EnrollmentID        string                `json:"enrollment_id"`
	ConfigurationDigest string                `json:"configuration_digest"`
	RequestDigest       string                `json:"request_digest"`
	KeyFingerprint      string                `json:"key_fingerprint"`
	State               string                `json:"state"`
	Bundle              RelayEnrollmentBundle `json:"bundle"`
	Reused              bool                  `json:"reused"`
}

// RelayEnrollmentBundle is a one-time pass-through payload. Callers must
// install and discard TelemetrySecret; it must never enter the registry,
// logs, audit metadata, browser storage, or a read API.
type RelayEnrollmentBundle struct {
	NodeID               string  `json:"node_id"`
	RelayServer          string  `json:"relay_server"`
	PublicEndpoint       string  `json:"public_endpoint"`
	NodeCertificatePEM   string  `json:"node_certificate_pem"`
	RelayCAPEM           string  `json:"relay_ca_pem"`
	CenterPublicKey      string  `json:"center_public_key"`
	TelemetrySecret      string  `json:"telemetry_secret"`
	MaxSessions          int     `json:"max_sessions"`
	CapacityBandwidthBPS uint64  `json:"capacity_bandwidth_bps"`
	Draining             bool    `json:"draining"`
	RelayPool            string  `json:"relay_pool"`
	Profile              string  `json:"profile"`
	WSSEndpoint          *string `json:"wss_endpoint"`
	ActivateAfterHealth  bool    `json:"activate_after_health"`
	FastMediaUDPPort     *int    `json:"fast_media_udp_port"`
}

type RelayEnrollmentActivateRequest struct {
	Version             int    `json:"version"`
	EnrollmentID        string `json:"enrollment_id"`
	ConfigurationDigest string `json:"configuration_digest"`
	OperationID         string `json:"operation_id"`
	ConfigGeneration    uint64 `json:"config_generation"`
	HealthSnapshotID    string `json:"health_snapshot_id"`
}

type RelayEnrollmentRevokeRequest struct {
	Version             int    `json:"version"`
	EnrollmentID        string `json:"enrollment_id"`
	ConfigurationDigest string `json:"configuration_digest"`
}

type RelayEnrollmentList struct {
	Version int                      `json:"version"`
	Items   []RelayEnrollmentSummary `json:"items"`
}

type RelayEnrollmentSummary struct {
	Version                    int     `json:"version"`
	EnrollmentID               string  `json:"enrollment_id"`
	NodeID                     string  `json:"node_id"`
	RelayServer                string  `json:"relay_server"`
	RelayPool                  string  `json:"relay_pool"`
	Profile                    string  `json:"profile"`
	ConfigurationDigest        string  `json:"configuration_digest"`
	ExpiresAtUnix              uint64  `json:"expires_at_unix"`
	State                      string  `json:"state"`
	ActivateAfterHealth        bool    `json:"activate_after_health"`
	KeyFingerprint             *string `json:"key_fingerprint"`
	ActivationOperationID      *string `json:"activation_operation_id"`
	ActivationConfigGeneration *uint64 `json:"activation_config_generation"`
	ActivationHealthSnapshotID *string `json:"activation_health_snapshot_id"`
	ActivatedAtUnix            *uint64 `json:"activated_at_unix"`
}
