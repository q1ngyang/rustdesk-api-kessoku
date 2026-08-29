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
	PeerRegistry         int `json:"peer_registry"`
}

type PeerIdentityInput struct {
	ID   string `json:"id"`
	UUID string `json:"uuid"`
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
	ConfigGeneration uint64  `json:"config_generation"`
	HealthSnapshotID string  `json:"health_snapshot_id"`
	Relays           []Relay `json:"relays"`
	Warning          string  `json:"warning"`
}

type Relay struct {
	ID string `json:"id"`
	// Version is optional in Control v1. Current Starry versions may omit it;
	// Kessoku reports that explicitly instead of guessing from the center.
	Version           string               `json:"version,omitempty"`
	ConfiguredOrder   int                  `json:"configured_order"`
	Native            NativeRelayStatus    `json:"native"`
	WebSocket         WebSocketRelayStatus `json:"websocket"`
	EligibleFor       []Transport          `json:"eligible_for"`
	ReferencedByRules []string             `json:"referenced_by_rules"`
}

type NativeRelayStatus struct {
	State      string    `json:"state"`
	ObservedAt time.Time `json:"observed_at"`
}

type WebSocketRelayStatus struct {
	Configured  bool       `json:"configured"`
	URL         *string    `json:"url"`
	State       string     `json:"state"`
	LastProbeAt *time.Time `json:"last_probe_at"`
	LatencyMS   *int64     `json:"latency_ms"`
	ErrorCode   *string    `json:"error_code"`
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
