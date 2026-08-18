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

type Health struct {
	InstanceID   string          `json:"instance_id"`
	Status       string          `json:"status"`
	ObservedAt   time.Time       `json:"observed_at"`
	Config       ComponentHealth `json:"config"`
	HBBS         ComponentHealth `json:"hbbs"`
	Auth         ComponentHealth `json:"auth"`
	NativeRelay  ComponentHealth `json:"native_relay"`
	WebSocket    ComponentHealth `json:"websocket"`
	RecentErrors []AgentError    `json:"recent_errors"`
}

type ComponentHealth struct {
	Status     string    `json:"status"`
	ObservedAt time.Time `json:"observed_at,omitempty"`
	Message    string    `json:"message,omitempty"`
}

type AgentError struct {
	Code       string    `json:"code"`
	ObservedAt time.Time `json:"observed_at"`
	Retryable  bool      `json:"retryable"`
	Message    string    `json:"message,omitempty"`
}

type Relay struct {
	ID                string               `json:"id"`
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
	URL         string     `json:"url,omitempty"`
	State       string     `json:"state"`
	LastProbeAt *time.Time `json:"last_probe_at,omitempty"`
	LatencyMS   *int64     `json:"latency_ms,omitempty"`
	ErrorCode   *string    `json:"error_code,omitempty"`
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
	RelayID     string   `json:"relay_id"`
	Priority    int      `json:"priority"`
	Eligible    bool     `json:"eligible"`
	ReasonCodes []string `json:"reason_codes"`
}

type AllocationSelection struct {
	Kind       string  `json:"kind"`
	RelayID    *string `json:"relay_id,omitempty"`
	NonBinding bool    `json:"non_binding"`
}

type ConfigDocument struct {
	ETag            string                 `json:"etag"`
	Generation      uint64                 `json:"generation"`
	SchemaVersion   int                    `json:"schema_version"`
	SourceDigest    string                 `json:"source_digest"`
	EffectiveDigest string                 `json:"effective_digest"`
	YAML            string                 `json:"yaml"`
	Values          map[string]interface{} `json:"values"`
	RuntimeInSync   bool                   `json:"runtime_in_sync"`
}

type SchemaBundle struct {
	ETag     string          `json:"etag"`
	Digest   string          `json:"digest"`
	Schema   json.RawMessage `json:"schema"`
	UISchema json.RawMessage `json:"ui_schema"`
}

type ConfigCandidate struct {
	YAML     *string                `json:"yaml,omitempty"`
	Values   map[string]interface{} `json:"values,omitempty"`
	BaseETag string                 `json:"-"`
	Comment  string                 `json:"comment,omitempty"`
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
	CandidateDigest string       `json:"candidate_digest,omitempty"`
	SchemaVersion   int          `json:"schema_version,omitempty"`
	Diagnostics     []Diagnostic `json:"diagnostics"`
}

type ConfigPlan struct {
	PlanID           string          `json:"plan_id"`
	BaseETag         string          `json:"base_etag"`
	BaseGeneration   uint64          `json:"base_generation"`
	TargetGeneration uint64          `json:"target_generation"`
	CandidateDigest  string          `json:"candidate_digest"`
	SchemaVersion    int             `json:"schema_version"`
	Changes          json.RawMessage `json:"changes"`
	Warnings         []string        `json:"warnings"`
	RestartRequired  bool            `json:"restart_required"`
	ExpiresAt        time.Time       `json:"expires_at"`
}

type ApplyRequest struct {
	PlanID         string `json:"plan_id"`
	IfMatch        string `json:"-"`
	IdempotencyKey string `json:"-"`
	Comment        string `json:"comment,omitempty"`
}

type RollbackRequest struct {
	Generation     uint64 `json:"generation"`
	IfMatch        string `json:"-"`
	IdempotencyKey string `json:"-"`
	Comment        string `json:"comment,omitempty"`
}

type ApplyResult struct {
	OperationID string `json:"operation_id"`
	Status      string `json:"status"`
}

type Operation struct {
	ID                 string     `json:"id"`
	Kind               string     `json:"kind"`
	Status             string     `json:"status"`
	StartedAt          time.Time  `json:"started_at"`
	FinishedAt         *time.Time `json:"finished_at,omitempty"`
	TargetGeneration   uint64     `json:"target_generation,omitempty"`
	ActiveGeneration   uint64     `json:"active_generation,omitempty"`
	RollbackGeneration *uint64    `json:"rollback_generation,omitempty"`
	ErrorCode          string     `json:"error_code,omitempty"`
}

type ConfigRevision struct {
	Generation  uint64    `json:"generation"`
	ETag        string    `json:"etag"`
	Digest      string    `json:"digest"`
	Actor       string    `json:"actor"`
	Comment     string    `json:"comment,omitempty"`
	AppliedAt   time.Time `json:"applied_at"`
	ApplyResult string    `json:"apply_result"`
}
