package starry

import (
	"context"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/pem"
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode"

	"github.com/google/uuid"
	"github.com/q1ngyang/rustdesk-api-kessoku/v3/internal/starrycontrol"
	"github.com/q1ngyang/rustdesk-api-kessoku/v3/internal/starrycontrol/clientgen"
)

func (p *Provider) ListRelayEnrollments(ctx context.Context) (starrycontrol.RelayEnrollmentList, error) {
	result := starrycontrol.RelayEnrollmentList{}
	capabilities, err := p.ensureIdentity(ctx)
	if err != nil {
		return result, err
	}
	if err := requireCapability(capabilities.Capabilities.RelayEnrollment, "relay_enrollment"); err != nil {
		return result, err
	}
	if err := p.call(ctx, clientgen.Request{Method: http.MethodGet, Path: "/control/v1/relay-enrollments", Scope: "starry.relay.read"}, &result); err != nil {
		return result, err
	}
	if err := validateRelayEnrollmentList(result); err != nil {
		return starrycontrol.RelayEnrollmentList{}, err
	}
	return result, nil
}

func (p *Provider) GetRelayEnrollment(ctx context.Context, enrollmentID string) (starrycontrol.RelayEnrollmentSummary, error) {
	result := starrycontrol.RelayEnrollmentSummary{}
	if _, err := uuid.Parse(enrollmentID); err != nil {
		return result, starrycontrol.ErrRequestInvalid
	}
	capabilities, err := p.ensureIdentity(ctx)
	if err != nil {
		return result, err
	}
	if err := requireCapability(capabilities.Capabilities.RelayEnrollment, "relay_enrollment"); err != nil {
		return result, err
	}
	if err := p.call(ctx, clientgen.Request{Method: http.MethodGet, Path: "/control/v1/relay-enrollments/" + enrollmentID, Scope: "starry.relay.read"}, &result); err != nil {
		return result, err
	}
	if err := validateRelayEnrollmentSummary(result); err != nil || result.EnrollmentID != enrollmentID {
		return starrycontrol.RelayEnrollmentSummary{}, contractResponseError()
	}
	return result, nil
}

func (p *Provider) PrepareRelayEnrollment(ctx context.Context, input starrycontrol.RelayEnrollmentPrepareRequest, idempotencyKey string) (starrycontrol.RelayEnrollmentPrepareResponse, error) {
	result := starrycontrol.RelayEnrollmentPrepareResponse{}
	if !validRelayEnrollmentPrepareRequest(input) || !validIdempotencyKey(idempotencyKey) {
		return result, starrycontrol.ErrRequestInvalid
	}
	capabilities, err := p.ensureIdentity(ctx)
	if err != nil {
		return result, err
	}
	if err := requireCapability(capabilities.Capabilities.RelayEnrollmentWrite, "relay_enrollment_write"); err != nil {
		return result, err
	}
	if err := p.call(ctx, clientgen.Request{
		Method: http.MethodPost, Path: "/control/v1/relay-enrollments:prepare", Scope: "starry.relay.enroll",
		Body: input, IdempotencyKey: idempotencyKey,
	}, &result); err != nil {
		return result, err
	}
	if err := validateRelayEnrollmentPrepareResponse(result); err != nil {
		return starrycontrol.RelayEnrollmentPrepareResponse{}, err
	}
	expectedDigest, err := starrycontrol.RelayEnrollmentConfigurationDigest(input)
	if err != nil || result.ConfigurationDigest != expectedDigest {
		return starrycontrol.RelayEnrollmentPrepareResponse{}, contractResponseError()
	}
	return result, nil
}

func (p *Provider) CompleteRelayEnrollment(ctx context.Context, input starrycontrol.RelayEnrollmentCompleteRequest) (starrycontrol.RelayEnrollmentCompleteResponse, error) {
	result := starrycontrol.RelayEnrollmentCompleteResponse{}
	if !validRelayEnrollmentCompleteRequest(input) {
		return result, starrycontrol.ErrRequestInvalid
	}
	capabilities, err := p.ensureIdentity(ctx)
	if err != nil {
		return result, err
	}
	if err := requireCapability(capabilities.Capabilities.RelayEnrollmentWrite, "relay_enrollment_write"); err != nil {
		return result, err
	}
	if err := p.call(ctx, clientgen.Request{Method: http.MethodPost, Path: "/control/v1/relay-enrollments:complete", Scope: "starry.relay.enroll", Body: input}, &result); err != nil {
		return result, err
	}
	if err := validateRelayEnrollmentCompleteResponse(result); err != nil ||
		result.EnrollmentID != input.EnrollmentID || result.ConfigurationDigest != input.ConfigurationDigest ||
		result.RequestDigest != input.RequestDigest || result.KeyFingerprint != input.KeyFingerprint {
		return starrycontrol.RelayEnrollmentCompleteResponse{}, contractResponseError()
	}
	if err := validateRelayEnrollmentCredentials(result.Bundle, input.KeyFingerprint, time.Now().UTC()); err != nil {
		return starrycontrol.RelayEnrollmentCompleteResponse{}, contractResponseError()
	}
	return result, nil
}

func (p *Provider) ActivateRelayEnrollment(ctx context.Context, input starrycontrol.RelayEnrollmentActivateRequest) (starrycontrol.RelayEnrollmentSummary, error) {
	result := starrycontrol.RelayEnrollmentSummary{}
	if !validRelayEnrollmentActivateRequest(input) {
		return result, starrycontrol.ErrRequestInvalid
	}
	capabilities, err := p.ensureIdentity(ctx)
	if err != nil {
		return result, err
	}
	if err := requireCapability(capabilities.Capabilities.RelayEnrollmentHealthActivation, "relay_enrollment_health_activation"); err != nil {
		return result, err
	}
	if err := p.call(ctx, clientgen.Request{Method: http.MethodPost, Path: "/control/v1/relay-enrollments:activate", Scope: "starry.relay.enroll", Body: input}, &result); err != nil {
		return result, err
	}
	if err := validateRelayEnrollmentSummary(result); err != nil || result.EnrollmentID != input.EnrollmentID ||
		result.ConfigurationDigest != input.ConfigurationDigest || result.State != "active" {
		return starrycontrol.RelayEnrollmentSummary{}, contractResponseError()
	}
	return result, nil
}

func (p *Provider) RevokeRelayEnrollment(ctx context.Context, input starrycontrol.RelayEnrollmentRevokeRequest) (starrycontrol.RelayEnrollmentSummary, error) {
	result := starrycontrol.RelayEnrollmentSummary{}
	if input.Version != 1 || !validEnrollmentBinding(input.EnrollmentID, input.ConfigurationDigest) {
		return result, starrycontrol.ErrRequestInvalid
	}
	capabilities, err := p.ensureIdentity(ctx)
	if err != nil {
		return result, err
	}
	if err := requireCapability(capabilities.Capabilities.RelayEnrollmentWrite, "relay_enrollment_write"); err != nil {
		return result, err
	}
	if err := p.call(ctx, clientgen.Request{Method: http.MethodPost, Path: "/control/v1/relay-enrollments:revoke", Scope: "starry.relay.enroll", Body: input}, &result); err != nil {
		return result, err
	}
	if err := validateRelayEnrollmentSummary(result); err != nil || result.EnrollmentID != input.EnrollmentID ||
		result.ConfigurationDigest != input.ConfigurationDigest || result.State != "revoked" {
		return starrycontrol.RelayEnrollmentSummary{}, contractResponseError()
	}
	return result, nil
}

func validRelayEnrollmentPrepareRequest(input starrycontrol.RelayEnrollmentPrepareRequest) bool {
	if input.Version != 1 || !validEnrollmentIdentifier(input.NodeID) || !validEnrollmentIdentifier(input.RelayPool) ||
		input.RelayServer != input.PublicEndpoint || !validRelayEndpoint(input.RelayServer) ||
		!oneOf(input.Profile, "native", "native-wss", "native-wss-fastmedia") || input.MaxSessions < 1 ||
		input.MaxSessions > 1_000_000 || input.CapacityBandwidthBPS == 0 || input.CapacityBandwidthBPS > 1<<63-1 ||
		(input.ExpiresInSeconds != 0 && (input.ExpiresInSeconds < 1 || input.ExpiresInSeconds > 3600)) {
		return false
	}
	if input.Profile == "native" && input.WSSEndpoint != nil || input.Profile != "native" && (input.WSSEndpoint == nil || !validTelemetryWSSURL(*input.WSSEndpoint)) {
		return false
	}
	return (input.Profile == "native-wss-fastmedia") == (input.FastMediaUDPPort != nil) &&
		(input.FastMediaUDPPort == nil || *input.FastMediaUDPPort >= 1 && *input.FastMediaUDPPort <= 65535)
}

func validRelayEnrollmentCompleteRequest(input starrycontrol.RelayEnrollmentCompleteRequest) bool {
	return input.Version == 1 && validEnrollmentBinding(input.EnrollmentID, input.ConfigurationDigest) &&
		validDigest(input.RequestDigest) && validDigest(input.KeyFingerprint) && len(input.CSRPEM) >= 1 &&
		len(input.CSRPEM) <= 32768 && strings.TrimSpace(input.CSRPEM) == input.CSRPEM
}

func validRelayEnrollmentActivateRequest(input starrycontrol.RelayEnrollmentActivateRequest) bool {
	if input.Version != 1 || !validEnrollmentBinding(input.EnrollmentID, input.ConfigurationDigest) || input.ConfigGeneration == 0 ||
		!validHealthSnapshotID(input.HealthSnapshotID) {
		return false
	}
	_, err := uuid.Parse(input.OperationID)
	return err == nil
}

func validEnrollmentBinding(enrollmentID, configurationDigest string) bool {
	_, err := uuid.Parse(enrollmentID)
	return err == nil && validDigest(configurationDigest)
}

func validateRelayEnrollmentPrepareResponse(result starrycontrol.RelayEnrollmentPrepareResponse) error {
	if result.Version != 1 || !validEnrollmentBinding(result.EnrollmentID, result.ConfigurationDigest) ||
		result.ExpiresAtUnix == 0 || result.ExpiresAtUnix > 1<<63-1 || result.State != "pending_claim" {
		return contractResponseError()
	}
	return nil
}

func validateRelayEnrollmentCompleteResponse(result starrycontrol.RelayEnrollmentCompleteResponse) error {
	if result.Version != 1 || !validEnrollmentBinding(result.EnrollmentID, result.ConfigurationDigest) ||
		!validDigest(result.RequestDigest) || !validDigest(result.KeyFingerprint) ||
		!oneOf(result.State, "claimed_pending_health", "pending_approval", "active") || !validRelayEnrollmentBundle(result.Bundle) {
		return contractResponseError()
	}
	return nil
}

func validRelayEnrollmentBundle(bundle starrycontrol.RelayEnrollmentBundle) bool {
	if !validEnrollmentIdentifier(bundle.NodeID) || !validEnrollmentIdentifier(bundle.RelayPool) ||
		bundle.RelayServer != bundle.PublicEndpoint || !validRelayEndpoint(bundle.RelayServer) ||
		len(bundle.NodeCertificatePEM) < 1 || len(bundle.NodeCertificatePEM) > 65536 ||
		len(bundle.RelayCAPEM) < 1 || len(bundle.RelayCAPEM) > 65536 || len(bundle.CenterPublicKey) < 43 || len(bundle.CenterPublicKey) > 128 ||
		bundle.MaxSessions < 1 || bundle.MaxSessions > 1_000_000 || bundle.CapacityBandwidthBPS == 0 || bundle.CapacityBandwidthBPS > 1<<63-1 ||
		!oneOf(bundle.Profile, "native", "native-wss", "native-wss-fastmedia") {
		return false
	}
	secret, err := base64.RawURLEncoding.DecodeString(bundle.TelemetrySecret)
	if err != nil || len(secret) != 32 || base64.RawURLEncoding.EncodeToString(secret) != bundle.TelemetrySecret {
		return false
	}
	centerKey, err := base64.StdEncoding.DecodeString(bundle.CenterPublicKey)
	if err != nil || len(centerKey) != 32 || base64.StdEncoding.EncodeToString(centerKey) != bundle.CenterPublicKey {
		return false
	}
	if bundle.Profile == "native" && bundle.WSSEndpoint != nil || bundle.Profile != "native" && (bundle.WSSEndpoint == nil || !validTelemetryWSSURL(*bundle.WSSEndpoint)) {
		return false
	}
	return (bundle.Profile == "native-wss-fastmedia") == (bundle.FastMediaUDPPort != nil) &&
		(bundle.FastMediaUDPPort == nil || *bundle.FastMediaUDPPort >= 1 && *bundle.FastMediaUDPPort <= 65535)
}

func validateRelayEnrollmentCredentials(bundle starrycontrol.RelayEnrollmentBundle, expectedFingerprint string, now time.Time) error {
	leaf, err := parseSingleCertificate(bundle.NodeCertificatePEM)
	if err != nil {
		return err
	}
	ca, err := parseSingleCertificate(bundle.RelayCAPEM)
	if err != nil {
		return err
	}
	for _, certificate := range []*x509.Certificate{leaf, ca} {
		if now.Before(certificate.NotBefore) || !now.Before(certificate.NotAfter) {
			return contractResponseError()
		}
	}
	publicKey, err := x509.MarshalPKIXPublicKey(leaf.PublicKey)
	if err != nil {
		return err
	}
	digest := sha256.Sum256(publicKey)
	if "sha256:"+hex.EncodeToString(digest[:]) != expectedFingerprint || !ca.IsCA || leaf.CheckSignatureFrom(ca) != nil || ca.CheckSignatureFrom(ca) != nil {
		return contractResponseError()
	}
	return nil
}

func parseSingleCertificate(value string) (*x509.Certificate, error) {
	block, rest := pem.Decode([]byte(value))
	if block == nil || block.Type != "CERTIFICATE" || len(strings.TrimSpace(string(rest))) != 0 {
		return nil, contractResponseError()
	}
	certificate, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, contractResponseError()
	}
	return certificate, nil
}

func validateRelayEnrollmentList(result starrycontrol.RelayEnrollmentList) error {
	if result.Version != 1 || result.Items == nil || len(result.Items) > 2048 {
		return contractResponseError()
	}
	seen := make(map[string]struct{}, len(result.Items))
	for _, item := range result.Items {
		if err := validateRelayEnrollmentSummary(item); err != nil {
			return err
		}
		if _, duplicate := seen[item.EnrollmentID]; duplicate {
			return contractResponseError()
		}
		seen[item.EnrollmentID] = struct{}{}
	}
	return nil
}

func validateRelayEnrollmentSummary(result starrycontrol.RelayEnrollmentSummary) error {
	if result.Version != 1 || !validEnrollmentBinding(result.EnrollmentID, result.ConfigurationDigest) ||
		!validEnrollmentIdentifier(result.NodeID) || !validRelayEndpoint(result.RelayServer) || !validEnrollmentIdentifier(result.RelayPool) ||
		!oneOf(result.Profile, "native", "native-wss", "native-wss-fastmedia") || result.ExpiresAtUnix == 0 || result.ExpiresAtUnix > 1<<63-1 ||
		!oneOf(result.State, "pending_claim", "claimed_pending_health", "pending_approval", "active", "revoked", "expired") {
		return contractResponseError()
	}
	if result.KeyFingerprint != nil && !validDigest(*result.KeyFingerprint) ||
		result.ActivationOperationID != nil && !validUUID(*result.ActivationOperationID) ||
		result.ActivationConfigGeneration != nil && (*result.ActivationConfigGeneration == 0 || *result.ActivationConfigGeneration > 1<<63-1) ||
		result.ActivationHealthSnapshotID != nil && !validHealthSnapshotID(*result.ActivationHealthSnapshotID) ||
		result.ActivatedAtUnix != nil && (*result.ActivatedAtUnix == 0 || *result.ActivatedAtUnix > 1<<63-1) {
		return contractResponseError()
	}
	return nil
}

func validUUID(value string) bool {
	_, err := uuid.Parse(value)
	return err == nil
}

func validHealthSnapshotID(value string) bool {
	if len(value) < 8 || len(value) > 128 || !strings.HasPrefix(value, "health-") {
		return false
	}
	for _, character := range strings.TrimPrefix(value, "health-") {
		if character >= 'a' && character <= 'z' || character >= 'A' && character <= 'Z' || character >= '0' && character <= '9' || strings.ContainsRune("_.-", character) {
			continue
		}
		return false
	}
	return true
}

func validTelemetryWSSURL(value string) bool {
	if len(value) > 512 || strings.IndexFunc(value, unicode.IsSpace) >= 0 {
		return false
	}
	parsed, err := url.Parse(value)
	return err == nil && parsed.Scheme == "wss" && parsed.Hostname() != "" && parsed.User == nil &&
		parsed.Path == "/ws/telemetry" && parsed.RawPath == "" && parsed.RawQuery == "" && !parsed.ForceQuery && parsed.Fragment == ""
}

func validRelayEndpoint(value string) bool {
	if len(value) > 256 || strings.IndexFunc(value, unicode.IsSpace) >= 0 {
		return false
	}
	parsed, err := url.Parse("tcp://" + value)
	return err == nil && parsed.Scheme == "tcp" && parsed.Hostname() != "" && parsed.Port() != "" && parsed.User == nil &&
		parsed.Path == "" && parsed.RawPath == "" && parsed.RawQuery == "" && !parsed.ForceQuery && parsed.Fragment == ""
}

func validEnrollmentIdentifier(value string) bool {
	if value == "" || len(value) > 128 || strings.HasPrefix(value, ".") {
		return false
	}
	for _, character := range []byte(value) {
		if character >= 'a' && character <= 'z' || character >= 'A' && character <= 'Z' || character >= '0' && character <= '9' || strings.ContainsRune("_.-", rune(character)) {
			continue
		}
		return false
	}
	return true
}
