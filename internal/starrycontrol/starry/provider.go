package starry

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/q1ngyang/rustdesk-api-kessoku/v3/config"
	"github.com/q1ngyang/rustdesk-api-kessoku/v3/internal/controlauth"
	"github.com/q1ngyang/rustdesk-api-kessoku/v3/internal/starrycontrol"
	"github.com/q1ngyang/rustdesk-api-kessoku/v3/internal/starrycontrol/clientgen"
	"gopkg.in/yaml.v3"
)

type Provider struct {
	instanceID string
	client     *clientgen.Client
}

func NewProvider(instance config.StarryInstance, control config.ServerControl) (*Provider, error) {
	if !instance.Enabled {
		return nil, errors.New("Starry instance is disabled")
	}
	if instance.ID == "" || instance.ExpectedInstanceID == "" || instance.TLSServerName == "" {
		return nil, errors.New("Starry instance id, expected instance id, and TLS server name are required")
	}
	if instance.CAFile == "" || instance.ClientCertFile == "" || instance.ClientKeyFile == "" {
		return nil, errors.New("Starry mTLS CA, client certificate, and key files are required")
	}
	certificate, err := tls.LoadX509KeyPair(instance.ClientCertFile, instance.ClientKeyFile)
	if err != nil {
		return nil, fmt.Errorf("load Starry client certificate: %w", err)
	}
	if len(certificate.Certificate) == 0 {
		return nil, errors.New("Starry client certificate chain is empty")
	}
	leaf, err := x509.ParseCertificate(certificate.Certificate[0])
	if err != nil {
		return nil, fmt.Errorf("parse Starry client certificate: %w", err)
	}
	uriSANMatches := 0
	for _, uri := range leaf.URIs {
		if uri.String() == instance.AuthorizedParty {
			uriSANMatches++
		}
	}
	if uriSANMatches != 1 {
		return nil, errors.New("Starry authorized party must match exactly one client certificate URI SAN")
	}
	caPEM, err := os.ReadFile(instance.CAFile)
	if err != nil {
		return nil, fmt.Errorf("read Starry CA file: %w", err)
	}
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(caPEM) {
		return nil, errors.New("Starry CA file contains no certificates")
	}
	signer, err := controlauth.NewSigner(
		instance.ControlIssuer,
		instance.AuthorizedParty,
		instance.ControlKeyID,
		instance.ControlKeyFile,
		2*time.Minute,
	)
	if err != nil {
		return nil, err
	}
	dialer := &net.Dialer{Timeout: 3 * time.Second, KeepAlive: 30 * time.Second}
	transport := &http.Transport{
		// Control-plane credentials must never be delegated to an ambient
		// HTTP(S)_PROXY. Instances are fixed deployment endpoints and use mTLS.
		Proxy:                 nil,
		DialContext:           dialer.DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          20,
		MaxIdleConnsPerHost:   10,
		IdleConnTimeout:       60 * time.Second,
		TLSHandshakeTimeout:   3 * time.Second,
		ResponseHeaderTimeout: control.Timeout(),
		TLSClientConfig: &tls.Config{
			Certificates: []tls.Certificate{certificate},
			RootCAs:      roots,
			ServerName:   instance.TLSServerName,
			MinVersion:   tls.VersionTLS13,
		},
	}
	httpClient := &http.Client{Transport: transport, Timeout: control.Timeout()}
	client, err := clientgen.New(instance.BaseURL, httpClient, func(ctx context.Context, scope string) (string, error) {
		metadata, ok := starrycontrol.MetadataFromContext(ctx)
		if !ok {
			return "", errors.New("missing authenticated request metadata")
		}
		if metadata.Service {
			return signer.SignService(instance.ExpectedInstanceID, scope)
		}
		return signer.Sign(instance.ExpectedInstanceID, scope, metadata.ActorUserID)
	}, control.MaxResponseBytes())
	if err != nil {
		return nil, err
	}
	return &Provider{instanceID: instance.ExpectedInstanceID, client: client}, nil
}

func (p *Provider) Capabilities(ctx context.Context) (starrycontrol.Capabilities, error) {
	return p.capabilities(ctx)
}

func (p *Provider) capabilities(ctx context.Context) (starrycontrol.Capabilities, error) {
	result := starrycontrol.Capabilities{}
	if err := p.call(ctx, clientgen.Request{Method: http.MethodGet, Path: "/control/v1/capabilities", Scope: "starry.control.read"}, &result); err != nil {
		return result, err
	}
	if result.Protocol.Name != "starry-control" || result.Protocol.Major != 1 {
		return starrycontrol.Capabilities{}, &starrycontrol.ProviderError{Status: http.StatusBadGateway, Code: "CONTRACT_VERSION_INCOMPATIBLE", Message: "Starry contract is incompatible"}
	}
	if result.Instance.ID != p.instanceID {
		return starrycontrol.Capabilities{}, &starrycontrol.ProviderError{Status: http.StatusBadGateway, Code: "INSTANCE_ID_MISMATCH", Message: "Starry instance identity did not match deployment configuration"}
	}
	if err := validateCapabilitiesResponse(result); err != nil {
		return starrycontrol.Capabilities{}, err
	}
	return result, nil
}

func (p *Provider) Status(ctx context.Context) (starrycontrol.Status, error) {
	result := starrycontrol.Status{}
	if _, err := p.ensureIdentity(ctx); err != nil {
		return result, err
	}
	if err := p.call(ctx, clientgen.Request{Method: http.MethodGet, Path: "/control/v1/status", Scope: "starry.control.read"}, &result); err != nil {
		return result, err
	}
	if err := validateStatusResponse(result); err != nil {
		return starrycontrol.Status{}, err
	}
	return result, nil
}

func (p *Provider) Relays(ctx context.Context) (starrycontrol.RelayInventory, error) {
	result := starrycontrol.RelayInventory{}
	capabilities, err := p.ensureIdentity(ctx)
	if err != nil {
		return result, err
	}
	if err := requireCapability(capabilities.Capabilities.RelayInventory, "relay_inventory"); err != nil {
		return result, err
	}
	if err := p.call(ctx, clientgen.Request{Method: http.MethodGet, Path: "/control/v1/relays", Scope: "starry.relay.read"}, &result); err != nil {
		return result, err
	}
	if err := validateRelaysResponse(result, capabilities.Capabilities); err != nil {
		return starrycontrol.RelayInventory{}, err
	}
	return result, nil
}

func (p *Provider) VerifyPeer(ctx context.Context, input starrycontrol.PeerIdentityInput) (starrycontrol.PeerVerification, error) {
	result := starrycontrol.PeerVerification{}
	if !validIdentifier(input.ID, 128) || !validIdentifier(input.UUID, 256) {
		return result, starrycontrol.ErrRequestInvalid
	}
	requiredVersion := 1
	if input.ActivationEpoch != 0 || input.ActivationID != "" || len(input.RouteLeases) != 0 {
		requiredVersion = 2
		if input.ActivationEpoch == 0 || !validStandardBase64(input.ActivationID, 16) || len(input.RouteLeases) == 0 || len(input.RouteLeases) > 16 {
			return result, starrycontrol.ErrRequestInvalid
		}
		for _, routeLease := range input.RouteLeases {
			if !validStandardBase64(routeLease, 32) {
				return result, starrycontrol.ErrRequestInvalid
			}
		}
	}
	if requiredVersion > 1 {
		capabilities, err := p.ensureIdentity(ctx)
		if err != nil {
			return result, err
		}
		if err := requireCapabilityVersion(capabilities.Capabilities.PeerRegistry, requiredVersion, "peer_registry"); err != nil {
			return result, err
		}
	}
	if err := p.call(ctx, clientgen.Request{Method: http.MethodPost, Path: "/control/v1/peers:verify", Scope: "starry.peer.verify", Body: input}, &result); err != nil {
		return result, err
	}
	if result.InstanceID != p.instanceID {
		return starrycontrol.PeerVerification{}, contractResponseError()
	}
	return result, nil
}

func (p *Provider) SimulateAllocation(ctx context.Context, input starrycontrol.SimulationInput) (starrycontrol.SimulationResult, error) {
	result := starrycontrol.SimulationResult{}
	if !input.Transport.Valid() {
		return result, fmt.Errorf("%w: transport", starrycontrol.ErrRequestInvalid)
	}
	if _, err := netip.ParseAddr(input.ClientA.IP); err != nil {
		return result, fmt.Errorf("%w: client_a.ip", starrycontrol.ErrRequestInvalid)
	}
	if _, err := netip.ParseAddr(input.ClientB.IP); err != nil {
		return result, fmt.Errorf("%w: client_b.ip", starrycontrol.ErrRequestInvalid)
	}
	capabilities, err := p.ensureIdentity(ctx)
	if err != nil {
		return result, err
	}
	if err := requireCapability(capabilities.Capabilities.AllocationSimulation, "allocation_simulation"); err != nil {
		return result, err
	}
	if err := p.call(ctx, clientgen.Request{Method: http.MethodPost, Path: "/control/v1/allocations:simulate", Scope: "starry.relay.simulate", Body: input}, &result); err != nil {
		return result, err
	}
	if err := validateSimulationResponse(result); err != nil {
		return starrycontrol.SimulationResult{}, err
	}
	if input.ExpectedConfigGeneration != nil && result.ConfigGeneration != *input.ExpectedConfigGeneration {
		return starrycontrol.SimulationResult{}, contractResponseError()
	}
	return result, nil
}

func (p *Provider) GetConfig(ctx context.Context) (starrycontrol.ConfigDocument, error) {
	result := starrycontrol.ConfigDocument{}
	if _, err := p.ensureIdentity(ctx); err != nil {
		return result, err
	}
	headers, err := p.callWithHeaders(ctx, clientgen.Request{Method: http.MethodGet, Path: "/control/v1/config", Scope: "starry.config.read"}, &result)
	if err != nil {
		return result, err
	}
	headerETag := headers.Get("ETag")
	if result.ETag == "" {
		result.ETag = headerETag
	} else if result.ETag != headerETag {
		return starrycontrol.ConfigDocument{}, contractResponseError()
	}
	if err := validateConfigResponse(result); err != nil {
		return starrycontrol.ConfigDocument{}, err
	}
	return result, nil
}

func (p *Provider) GetConfigSchema(ctx context.Context) (starrycontrol.SchemaBundle, error) {
	result := starrycontrol.SchemaBundle{}
	capabilities, err := p.ensureIdentity(ctx)
	if err != nil {
		return result, err
	}
	headers, err := p.callWithHeaders(ctx, clientgen.Request{Method: http.MethodGet, Path: "/control/v1/config/schema", Scope: "starry.config.read"}, &result)
	if err != nil {
		return result, err
	}
	result.ETag = headers.Get("ETag")
	result.Digest = strings.Trim(result.ETag, "\"")
	if result.Digest != capabilities.Config.SchemaDigest {
		return starrycontrol.SchemaBundle{}, contractResponseError()
	}
	if err := validateSchemaResponse(result); err != nil {
		return starrycontrol.SchemaBundle{}, err
	}
	return result, nil
}

func (p *Provider) ValidateConfig(ctx context.Context, input starrycontrol.ConfigCandidate) (starrycontrol.ValidationResult, error) {
	result := starrycontrol.ValidationResult{}
	if err := validateCandidate(input, false); err != nil {
		return result, err
	}
	if _, err := p.ensureIdentity(ctx); err != nil {
		return result, err
	}
	if err := p.call(ctx, clientgen.Request{Method: http.MethodPost, Path: "/control/v1/config:validate", Scope: "starry.config.validate", Body: input}, &result); err != nil {
		return result, err
	}
	if err := validateValidationResponse(result); err != nil {
		return starrycontrol.ValidationResult{}, err
	}
	if result.Valid && (result.SourceDigest == nil || *result.SourceDigest != documentDigest(input.Document)) {
		return starrycontrol.ValidationResult{}, contractResponseError()
	}
	return result, nil
}

func (p *Provider) PlanConfig(ctx context.Context, input starrycontrol.ConfigCandidate) (starrycontrol.ConfigPlan, error) {
	result := starrycontrol.ConfigPlan{}
	if err := validateCandidate(input, true); err != nil {
		return result, err
	}
	capabilities, err := p.ensureIdentity(ctx)
	if err != nil {
		return result, err
	}
	if err := requireCapability(capabilities.Capabilities.ConfigTransaction, "config_transaction"); err != nil {
		return result, err
	}
	inspection, err := inspectFastModeCandidate(input.Document)
	if err != nil {
		return result, starrycontrol.ErrRequestInvalid
	}
	if inspection.SchemaVersion == 5 && capabilities.Capabilities.ConfigSchema != 5 {
		return result, capabilityUnavailable("config_schema", 5)
	}
	if inspection.HasFastMode && capabilities.Capabilities.FastRelayAuthorization != 1 {
		return result, capabilityUnavailable("fast_relay_authorization", 1)
	}
	if inspection.FastMediaEnabled {
		if capabilities.Capabilities.FastMediaRelayUDP != 1 {
			return result, capabilityUnavailable("fast_media_relay_udp", 1)
		}
		inventory, inventoryErr := p.Relays(ctx)
		if inventoryErr != nil {
			return result, inventoryErr
		}
		if !hasHealthyFastMediaCandidate(inventory) {
			return result, &starrycontrol.ProviderError{
				Status:  http.StatusConflict,
				Code:    "FAST_MEDIA_DEPENDENCY_UNMET",
				Message: "FastMedia requires a selectable Relay with fresh authenticated telemetry schema 2, capability version 1, and a healthy configured UDP endpoint",
			}
		}
	}
	if err := p.call(ctx, clientgen.Request{Method: http.MethodPost, Path: "/control/v1/config:plan", Scope: "starry.config.plan", Body: input, IfMatch: input.BaseETag}, &result); err != nil {
		return result, err
	}
	if err := validatePlanResponse(result, p.instanceID, input.BaseETag, documentDigest(input.Document)); err != nil {
		return starrycontrol.ConfigPlan{}, err
	}
	// Starry patch-v1.3 guarantees at least medium risk for every
	// /relay_quality change. Keep a defensive client-side floor for older or
	// non-conforming agents without changing the server-issued plan binding.
	minimumRisk := planMinimumRisk(result.Changes, inspection.FastMediaEnabled)
	if riskRank(result.Impact.Risk) < riskRank(minimumRisk) {
		result.Impact.Risk = minimumRisk
	}
	return result, nil
}

type fastModeInspection struct {
	SchemaVersion    int
	HasFastMode      bool
	FastMediaEnabled bool
}

func inspectFastModeCandidate(document string) (fastModeInspection, error) {
	var candidate struct {
		Version  int `yaml:"version"`
		FastMode *struct {
			Relay *struct {
				FastMediaV1Enabled bool `yaml:"fast_media_v1_enabled"`
			} `yaml:"relay"`
		} `yaml:"fast_mode"`
	}
	if err := yaml.Unmarshal([]byte(document), &candidate); err != nil {
		return fastModeInspection{}, err
	}
	result := fastModeInspection{SchemaVersion: candidate.Version, HasFastMode: candidate.FastMode != nil}
	if candidate.FastMode != nil && candidate.FastMode.Relay != nil {
		result.FastMediaEnabled = candidate.FastMode.Relay.FastMediaV1Enabled
	}
	return result, nil
}

func hasHealthyFastMediaCandidate(inventory starrycontrol.RelayInventory) bool {
	for _, relay := range inventory.Relays {
		udp := relay.FastMediaUDP
		if len(relay.EligibleFor) == 0 || relay.Capabilities == nil || relay.Capabilities.FastMediaRelayUDP == nil ||
			*relay.Capabilities.FastMediaRelayUDP != 1 || relay.WebSocket.TelemetrySchema == nil || *relay.WebSocket.TelemetrySchema != 2 ||
			relay.WebSocket.State != "healthy" || relay.WebSocket.Stale == nil || *relay.WebSocket.Stale || udp == nil ||
			udp.ConfiguredPort == nil || udp.ReportedPort == nil || *udp.ConfiguredPort != *udp.ReportedPort ||
			udp.Enabled == nil || !*udp.Enabled || udp.Healthy == nil || !*udp.Healthy {
			continue
		}
		return true
	}
	return false
}

func capabilityUnavailable(name string, version int) error {
	return &starrycontrol.ProviderError{
		Status:  http.StatusBadGateway,
		Code:    "CAPABILITY_UNAVAILABLE",
		Message: fmt.Sprintf("required Starry capability %s version %d is unavailable", name, version),
	}
}

func (p *Provider) ApplyConfig(ctx context.Context, input starrycontrol.ApplyRequest) (starrycontrol.Operation, error) {
	result := starrycontrol.Operation{}
	if _, err := uuid.Parse(input.PlanID); err != nil || !validDigest(input.CandidateDigest) || !validETag(input.IfMatch) || !validIdempotencyKey(input.IdempotencyKey) || !validComment(input.Comment) {
		return result, starrycontrol.ErrRequestInvalid
	}
	capabilities, err := p.ensureIdentity(ctx)
	if err != nil {
		return result, err
	}
	if err := requireCapability(capabilities.Capabilities.ConfigTransaction, "config_transaction"); err != nil {
		return result, err
	}
	if err := p.call(ctx, clientgen.Request{Method: http.MethodPost, Path: "/control/v1/config:apply", Scope: "starry.config.apply", Body: input, IfMatch: input.IfMatch, IdempotencyKey: input.IdempotencyKey}, &result); err != nil {
		return result, err
	}
	if err := validateOperationResponse(result, result.ID); err != nil {
		return starrycontrol.Operation{}, err
	}
	if err := validateOperationBinding(result, "config_apply", input.CandidateDigest); err != nil {
		return starrycontrol.Operation{}, err
	}
	return result, nil
}

func (p *Provider) Operation(ctx context.Context, operationID string) (starrycontrol.Operation, error) {
	result := starrycontrol.Operation{}
	if _, err := uuid.Parse(operationID); err != nil {
		return result, starrycontrol.ErrRequestInvalid
	}
	capabilities, err := p.ensureIdentity(ctx)
	if err != nil {
		return result, err
	}
	if err := requireCapability(capabilities.Capabilities.ConfigTransaction, "config_transaction"); err != nil {
		return result, err
	}
	if err := p.call(ctx, clientgen.Request{Method: http.MethodGet, Path: "/control/v1/operations/" + operationID, Scope: "starry.control.read"}, &result); err != nil {
		return result, err
	}
	if err := validateOperationResponse(result, operationID); err != nil {
		return starrycontrol.Operation{}, err
	}
	return result, nil
}

func (p *Provider) ConfigHistory(ctx context.Context) ([]starrycontrol.ConfigRevision, error) {
	response := struct {
		Revisions []starrycontrol.ConfigRevision `json:"revisions"`
	}{}
	if _, err := p.ensureIdentity(ctx); err != nil {
		return nil, err
	}
	if err := p.call(ctx, clientgen.Request{Method: http.MethodGet, Path: "/control/v1/config/history", Scope: "starry.config.read"}, &response); err != nil {
		return nil, err
	}
	if err := validateHistoryResponse(response.Revisions); err != nil {
		return nil, err
	}
	return response.Revisions, nil
}

func (p *Provider) RollbackConfig(ctx context.Context, input starrycontrol.RollbackRequest) (starrycontrol.Operation, error) {
	result := starrycontrol.Operation{}
	if _, err := uuid.Parse(input.RevisionID); err != nil || !validETag(input.IfMatch) || !validIdempotencyKey(input.IdempotencyKey) || !validComment(input.Comment) {
		return result, starrycontrol.ErrRequestInvalid
	}
	capabilities, err := p.ensureIdentity(ctx)
	if err != nil {
		return result, err
	}
	if err := requireCapability(capabilities.Capabilities.ConfigRollback, "config_rollback"); err != nil {
		return result, err
	}
	if err := p.call(ctx, clientgen.Request{Method: http.MethodPost, Path: "/control/v1/config:rollback", Scope: "starry.config.rollback", Body: input, IfMatch: input.IfMatch, IdempotencyKey: input.IdempotencyKey}, &result); err != nil {
		return result, err
	}
	if err := validateOperationResponse(result, result.ID); err != nil {
		return starrycontrol.Operation{}, err
	}
	if err := validateOperationBinding(result, "config_rollback", ""); err != nil {
		return starrycontrol.Operation{}, err
	}
	return result, nil
}

func (p *Provider) ReloadRuntime(ctx context.Context, input starrycontrol.RuntimeReloadRequest) (starrycontrol.ActivationAck, error) {
	result := starrycontrol.ActivationAck{}
	if !validDigest(input.ExpectedSourceDigest) || !validIdempotencyKey(input.IdempotencyKey) {
		return result, starrycontrol.ErrRequestInvalid
	}
	capabilities, err := p.ensureIdentity(ctx)
	if err != nil {
		return result, err
	}
	if err := requireCapability(capabilities.Capabilities.ConfigTransaction, "config_transaction"); err != nil {
		return result, err
	}
	if err := p.call(ctx, clientgen.Request{Method: http.MethodPost, Path: "/control/v1/runtime:reload", Scope: "starry.runtime.reload", Body: input, IdempotencyKey: input.IdempotencyKey}, &result); err != nil {
		return result, err
	}
	if err := validateActivationAck(result); err != nil {
		return starrycontrol.ActivationAck{}, err
	}
	if result.SourceDigest != input.ExpectedSourceDigest {
		return starrycontrol.ActivationAck{}, contractResponseError()
	}
	return result, nil
}

func validateOperationBinding(result starrycontrol.Operation, expectedKind, expectedSourceDigest string) error {
	if result.Kind != expectedKind {
		return contractResponseError()
	}
	if expectedSourceDigest != "" && result.ActivationAck != nil && result.ActivationAck.SourceDigest != expectedSourceDigest {
		return contractResponseError()
	}
	return nil
}

func validateCandidate(input starrycontrol.ConfigCandidate, requireETag bool) error {
	if input.Format != "yaml" || len(input.Document) > 1<<20 || requireETag && !validETag(input.BaseETag) || !requireETag && input.BaseETag != "" {
		return starrycontrol.ErrRequestInvalid
	}
	return nil
}

func validComment(value string) bool {
	return value == "" || len(value) <= 500 && validResponseText(value, 500)
}

func validETag(value string) bool {
	return len(value) == 73 && strings.HasPrefix(value, "\"sha256:") && strings.HasSuffix(value, "\"") && validLowerHex(value[8:72])
}

func validDigest(value string) bool {
	return len(value) == 71 && strings.HasPrefix(value, "sha256:") && validLowerHex(value[7:])
}

func validLowerHex(value string) bool {
	if value == "" {
		return false
	}
	for _, character := range value {
		if character < '0' || character > '9' && character < 'a' || character > 'f' {
			return false
		}
	}
	return true
}

func validIdempotencyKey(value string) bool {
	if len(value) < 16 || !validIdentifier(value, 128) {
		return false
	}
	for _, character := range value {
		if character >= 'a' && character <= 'z' || character >= 'A' && character <= 'Z' || character >= '0' && character <= '9' || strings.ContainsRune("._:-", character) {
			continue
		}
		return false
	}
	return true
}

func validIdentifier(value string, maximum int) bool {
	if value == "" || len(value) > maximum || strings.TrimSpace(value) != value {
		return false
	}
	for _, character := range value {
		if character < 0x21 || character > 0x7e {
			return false
		}
	}
	return true
}

// ensureIdentity performs the capabilities handshake immediately before each
// non-capabilities operation. It is intentionally not cached: a deployment
// endpoint can be re-targeted while this process is running, and mTLS proves
// possession of a trusted certificate rather than the configured instance ID.
func (p *Provider) ensureIdentity(ctx context.Context) (starrycontrol.Capabilities, error) {
	return p.capabilities(ctx)
}

func requireCapability(version int, name string) error {
	return requireCapabilityVersion(version, 1, name)
}

func requireCapabilityVersion(version, required int, name string) error {
	if version == required {
		return nil
	}
	return &starrycontrol.ProviderError{
		Status:  http.StatusBadGateway,
		Code:    "CAPABILITY_UNAVAILABLE",
		Message: fmt.Sprintf("required Starry capability %s version %d is unavailable", name, required),
	}
}

func validStandardBase64(value string, expectedBytes int) bool {
	decoded, err := base64.StdEncoding.DecodeString(value)
	return err == nil && len(decoded) == expectedBytes && base64.StdEncoding.EncodeToString(decoded) == value
}

func (p *Provider) call(ctx context.Context, request clientgen.Request, destination interface{}) error {
	_, err := p.callWithHeaders(ctx, request, destination)
	return err
}

func (p *Provider) callWithHeaders(ctx context.Context, request clientgen.Request, destination interface{}) (http.Header, error) {
	metadata, ok := starrycontrol.MetadataFromContext(ctx)
	if !ok {
		return nil, starrycontrol.ErrRequestInvalid
	}
	request.RequestID = metadata.RequestID
	headers, err := p.client.Do(ctx, request, destination)
	if err == nil {
		return headers, nil
	}
	var httpError *clientgen.HTTPError
	if errors.As(err, &httpError) {
		problem := httpError.Problem
		return headers, &starrycontrol.ProviderError{
			Status:    problem.Status,
			Code:      problem.Code,
			Message:   "Starry control request failed",
			RequestID: problem.RequestID,
			Retryable: problem.Retryable,
		}
	}
	return nil, fmt.Errorf("%w: %v", starrycontrol.ErrUnavailable, err)
}
