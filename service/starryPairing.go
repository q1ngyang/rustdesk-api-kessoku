package service

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/q1ngyang/rustdesk-api-kessoku/v3/config"
	"github.com/q1ngyang/rustdesk-api-kessoku/v3/internal/servercontrolregistry"
	"github.com/q1ngyang/rustdesk-api-kessoku/v3/internal/starrycontrol"
)

type PairingOrigin struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Origin        string `json:"origin"`
	TLSServerName string `json:"tls_server_name"`
}

type PairingBrokerStatus struct {
	ProtocolVersion    int             `json:"protocol_version"`
	Enabled            bool            `json:"enabled"`
	Available          bool            `json:"available"`
	RegistrySchema     int             `json:"registry_schema,omitempty"`
	RegistryGeneration uint64          `json:"registry_generation,omitempty"`
	InstallationID     string          `json:"installation_id,omitempty"`
	BrokerOrigin       string          `json:"broker_origin,omitempty"`
	BrokerSPKISHA256   string          `json:"broker_spki_sha256,omitempty"`
	AgentOrigins       []PairingOrigin `json:"agent_origins"`
	ErrorCode          string          `json:"error_code,omitempty"`
}

type ControlPairingCreateRequest struct {
	ManagedID        string `json:"managed_id"`
	Name             string `json:"name"`
	AgentOriginID    string `json:"agent_origin_id"`
	Action           string `json:"action"`
	TargetInstanceID string `json:"target_instance_id,omitempty"`
	Confirmation     string `json:"-"`
}

type RelayPairingCreateRequest struct {
	InstanceID     string                                      `json:"instance_id"`
	Enrollment     starrycontrol.RelayEnrollmentPrepareRequest `json:"enrollment"`
	IdempotencyKey string                                      `json:"-"`
	Confirmation   string                                      `json:"-"`
}

type PairingCodeResult struct {
	Version             int    `json:"version"`
	Purpose             string `json:"purpose"`
	EnrollmentID        string `json:"enrollment_id"`
	ConfigurationDigest string `json:"configuration_digest"`
	ExpiresAtUnix       int64  `json:"expires_at_unix"`
	State               string `json:"state"`
	Code                string `json:"code"`
}

type ControlPairingRevokeRequest struct {
	EnrollmentID string `json:"enrollment_id"`
	Confirmation string `json:"-"`
}

type PairingRevokeResult struct {
	Version      int    `json:"version"`
	Purpose      string `json:"purpose"`
	EnrollmentID string `json:"enrollment_id"`
	State        string `json:"state"`
}

type ManagedWriteRequest struct {
	WriteEnabled bool   `json:"write_enabled"`
	Confirmation string `json:"-"`
}

func (s *StarryControlService) PairingStatus(ctx context.Context) (PairingBrokerStatus, error) {
	return auditedControlCall(s, ctx, "server_control.pairing.status", "*", nil, func() (PairingBrokerStatus, error) {
		result := s.pairingStatus()
		return result, nil
	})
}

func (s *StarryControlService) PairingStatusLocal() PairingBrokerStatus {
	return s.pairingStatus()
}

func (s *StarryControlService) pairingStatus() PairingBrokerStatus {
	result := PairingBrokerStatus{
		ProtocolVersion:  1,
		Enabled:          s.config.Pairing.Enabled,
		BrokerOrigin:     s.config.Pairing.BrokerOrigin,
		BrokerSPKISHA256: s.config.Pairing.BrokerSPKISHA256,
		AgentOrigins:     make([]PairingOrigin, 0, len(s.config.Pairing.AgentOrigins)),
	}
	for _, item := range s.config.Pairing.AgentOrigins {
		result.AgentOrigins = append(result.AgentOrigins, PairingOrigin{ID: item.ID, Name: item.Name, Origin: item.Origin, TLSServerName: item.TLSServerName})
	}
	if !result.Enabled {
		result.ErrorCode = "PAIRING_DISABLED"
		return result
	}
	// A separate Cobra process may have performed the exact confirmed first
	// pairing after this long-running service started. Re-open only a complete
	// existing registry so the public claim endpoint and managed providers can
	// observe that generation without a service restart. This path never
	// initializes replacement identity state.
	_ = s.refreshManagedInstances(context.Background())
	registry, registryErr := s.registryState()
	if registry == nil {
		if errors.Is(registryErr, servercontrolregistry.ErrNotFound) {
			result.ErrorCode = "PAIRING_REGISTRY_NOT_INITIALIZED"
		} else {
			result.ErrorCode = "PAIRING_REGISTRY_UNAVAILABLE"
		}
		return result
	}
	metadata, err := registry.Metadata(context.Background())
	if err != nil {
		result.ErrorCode = "PAIRING_REGISTRY_UNAVAILABLE"
		return result
	}
	result.Available = true
	result.RegistrySchema = metadata.SchemaVersion
	result.RegistryGeneration = metadata.Generation
	result.InstallationID = metadata.InstallationID
	return result
}

func (s *StarryControlService) PairingEnabled() bool {
	if s == nil || !s.config.Pairing.Enabled {
		return false
	}
	_ = s.refreshManagedInstances(context.Background())
	registry, _ := s.registryState()
	return registry != nil
}

func (s *StarryControlService) CreateControlPairing(ctx context.Context, input ControlPairingCreateRequest) (PairingCodeResult, error) {
	return auditedControlCall(s, ctx, "server_control.pairing.create", input.ManagedID, map[string]interface{}{
		"purpose": servercontrolregistry.PurposeControlAgent, "action": input.Action,
		"managed_id": input.ManagedID, "agent_origin_id": input.AgentOriginID,
	}, func() (PairingCodeResult, error) {
		return s.createControlPairing(ctx, input)
	})
}

// CreateControlPairingLocal is the local Cobra seam. It intentionally writes
// only the independent registry; the command never starts a listener or opens
// the Kessoku business database.
func (s *StarryControlService) CreateControlPairingLocal(ctx context.Context, input ControlPairingCreateRequest) (PairingCodeResult, error) {
	return s.createControlPairing(ctx, input)
}

func (s *StarryControlService) createControlPairing(ctx context.Context, input ControlPairingCreateRequest) (PairingCodeResult, error) {
	if s == nil || !s.config.Pairing.Enabled {
		return PairingCodeResult{}, pairingProblem(404, "PAIRING_DISABLED")
	}
	if !managedIDPattern.MatchString(input.ManagedID) || strings.TrimSpace(input.Name) == "" || len(input.Name) > 256 {
		return PairingCodeResult{}, starrycontrol.ErrRequestInvalid
	}
	if input.Action == "" {
		input.Action = servercontrolregistry.ActionPair
	}
	if input.Action != servercontrolregistry.ActionPair && input.Action != servercontrolregistry.ActionAdopt && input.Action != servercontrolregistry.ActionRotate {
		return PairingCodeResult{}, starrycontrol.ErrRequestInvalid
	}
	if input.Action == servercontrolregistry.ActionPair && input.TargetInstanceID != "" {
		return PairingCodeResult{}, starrycontrol.ErrRequestInvalid
	}
	if input.Action != servercontrolregistry.ActionPair {
		if _, err := uuid.Parse(input.TargetInstanceID); err != nil {
			return PairingCodeResult{}, starrycontrol.ErrRequestInvalid
		}
	}
	if input.Confirmation != "confirm:"+input.Action+":"+input.ManagedID+":"+input.AgentOriginID {
		return PairingCodeResult{}, pairingProblem(428, "PAIRING_CONFIRMATION_REQUIRED")
	}
	agentOrigin, ok := s.approvedAgentOrigin(input.AgentOriginID)
	if !ok {
		return PairingCodeResult{}, pairingProblem(422, "AGENT_ORIGIN_NOT_ALLOWLISTED")
	}
	s.mu.RLock()
	_, staticCollision := s.staticIDs[input.ManagedID]
	s.mu.RUnlock()
	if staticCollision && input.Action != servercontrolregistry.ActionRotate {
		return PairingCodeResult{}, pairingProblem(409, "MANAGED_INSTANCE_ID_CONFLICT")
	}
	centerKey := s.centerPublicKey
	if !validCenterPublicKey(centerKey) {
		return PairingCodeResult{}, pairingProblem(503, "CENTER_PUBLIC_KEY_UNAVAILABLE")
	}
	registry, err := s.ensurePairingRegistry(input.Action == servercontrolregistry.ActionPair)
	if err != nil {
		return PairingCodeResult{}, err
	}
	enrollmentUUID, uuidErr := uuid.NewV7()
	if uuidErr != nil {
		enrollmentUUID = uuid.New()
	}
	prepared, err := servercontrolregistry.PrepareControlIdentity(registry.Root(), servercontrolregistry.ControlIdentityOptions{
		ManagedID: input.ManagedID, EnrollmentID: enrollmentUUID.String(), Name: strings.TrimSpace(input.Name),
		AgentOrigin: agentOrigin.Origin, TLSServerName: agentOrigin.TLSServerName,
		BrokerOrigin: s.config.Pairing.BrokerOrigin, CenterPublicKey: centerKey,
	})
	if err != nil {
		return PairingCodeResult{}, pairingProblemWithCause(503, "PAIRING_IDENTITY_UNAVAILABLE", err)
	}
	secret, err := randomPairingSecret()
	if err != nil {
		return PairingCodeResult{}, pairingProblemWithCause(503, "PAIRING_RANDOM_UNAVAILABLE", err)
	}
	secretDigest, err := servercontrolregistry.SecretDigest(secret)
	if err != nil {
		return PairingCodeResult{}, err
	}
	expiresAt := time.Now().UTC().Add(s.config.Pairing.EffectiveCodeTTL())
	enrollment, err := registry.CreateEnrollment(ctx, servercontrolregistry.EnrollmentCreate{
		EnrollmentID: enrollmentUUID.String(), Purpose: servercontrolregistry.PurposeControlAgent,
		Action: input.Action, ManagedID: input.ManagedID, Name: strings.TrimSpace(input.Name),
		AgentOriginID: agentOrigin.ID, AgentOrigin: agentOrigin.Origin, TLSServerName: agentOrigin.TLSServerName,
		TargetInstanceID: input.TargetInstanceID, ConfigurationDigest: prepared.ConfigurationDigest,
		SecretDigest: secretDigest, ExpiresAt: expiresAt, RecoveryTTL: s.config.Pairing.EffectiveRecoveryTTL(),
	})
	if err != nil {
		return PairingCodeResult{}, registryProblem(err)
	}
	return s.encodePairingCode(enrollment, secret)
}

func (s *StarryControlService) RevokeControlPairing(ctx context.Context, input ControlPairingRevokeRequest) (PairingRevokeResult, error) {
	return auditedControlCall(s, ctx, "server_control.pairing.revoke", input.EnrollmentID, map[string]interface{}{
		"purpose": servercontrolregistry.PurposeControlAgent, "enrollment_id": input.EnrollmentID,
	}, func() (PairingRevokeResult, error) {
		return s.revokeControlPairing(ctx, input)
	})
}

func (s *StarryControlService) RevokeControlPairingLocal(ctx context.Context, input ControlPairingRevokeRequest) (PairingRevokeResult, error) {
	return s.revokeControlPairing(ctx, input)
}

func (s *StarryControlService) revokeControlPairing(ctx context.Context, input ControlPairingRevokeRequest) (PairingRevokeResult, error) {
	if _, err := uuid.Parse(input.EnrollmentID); err != nil {
		return PairingRevokeResult{}, starrycontrol.ErrRequestInvalid
	}
	if input.Confirmation != "confirm:revoke-pairing:"+input.EnrollmentID {
		return PairingRevokeResult{}, pairingProblem(428, "PAIRING_CONFIRMATION_REQUIRED")
	}
	registry, err := s.pairingRegistry()
	if err != nil {
		return PairingRevokeResult{}, err
	}
	enrollment, err := registry.Enrollment(ctx, input.EnrollmentID)
	if err != nil {
		return PairingRevokeResult{}, registryProblem(err)
	}
	if enrollment.Purpose != servercontrolregistry.PurposeControlAgent {
		return PairingRevokeResult{}, pairingProblem(409, "PAIRING_PURPOSE_MISMATCH")
	}
	if enrollment.State != servercontrolregistry.StatePending && enrollment.State != servercontrolregistry.StateBound {
		return PairingRevokeResult{}, pairingProblem(409, "PAIRING_CODE_NOT_REVOCABLE")
	}
	if err := registry.RevokeEnrollment(ctx, input.EnrollmentID); err != nil {
		return PairingRevokeResult{}, registryProblem(err)
	}
	return PairingRevokeResult{
		Version: 1, Purpose: servercontrolregistry.PurposeControlAgent,
		EnrollmentID: input.EnrollmentID, State: servercontrolregistry.StateRevoked,
	}, nil
}

func (s *StarryControlService) CreateRelayPairing(ctx context.Context, input RelayPairingCreateRequest) (PairingCodeResult, error) {
	return auditedControlCall(s, ctx, "server_control.relay_enrollment.prepare", input.InstanceID, map[string]interface{}{
		"node_id": input.Enrollment.NodeID, "profile": input.Enrollment.Profile,
		"activate_after_health": input.Enrollment.ActivateAfterHealth,
	}, func() (PairingCodeResult, error) {
		return s.createRelayPairing(ctx, input)
	})
}

func (s *StarryControlService) CreateRelayPairingLocal(ctx context.Context, input RelayPairingCreateRequest) (PairingCodeResult, error) {
	return s.createRelayPairing(ctx, input)
}

func (s *StarryControlService) createRelayPairing(ctx context.Context, input RelayPairingCreateRequest) (PairingCodeResult, error) {
	registry, err := s.pairingRegistry()
	if err != nil {
		return PairingCodeResult{}, err
	}
	if input.Enrollment.ActivateAfterHealth && input.Confirmation != "confirm:activate-after-health:"+input.InstanceID+":"+input.Enrollment.NodeID {
		return PairingCodeResult{}, pairingProblem(428, "HIGH_RISK_CONFIRMATION_REQUIRED")
	}
	provider, err := s.provider(input.InstanceID, true)
	if err != nil {
		return PairingCodeResult{}, err
	}
	prepared, err := provider.PrepareRelayEnrollment(ctx, input.Enrollment, input.IdempotencyKey)
	if err != nil {
		return PairingCodeResult{}, err
	}
	expectedConfigurationDigest, digestErr := starrycontrol.RelayEnrollmentConfigurationDigest(input.Enrollment)
	if digestErr != nil || prepared.ConfigurationDigest != expectedConfigurationDigest {
		_, _ = provider.RevokeRelayEnrollment(ctx, starrycontrol.RelayEnrollmentRevokeRequest{
			Version: 1, EnrollmentID: prepared.EnrollmentID, ConfigurationDigest: prepared.ConfigurationDigest,
		})
		return PairingCodeResult{}, pairingProblemWithCause(502, "RELAY_CONFIGURATION_DRIFT", digestErr)
	}
	expiresAt := time.Unix(int64(prepared.ExpiresAtUnix), 0).UTC()
	now := time.Now().UTC()
	if !expiresAt.After(now) || expiresAt.After(now.Add(time.Hour)) {
		_, _ = provider.RevokeRelayEnrollment(ctx, starrycontrol.RelayEnrollmentRevokeRequest{
			Version: 1, EnrollmentID: prepared.EnrollmentID, ConfigurationDigest: prepared.ConfigurationDigest,
		})
		return PairingCodeResult{}, pairingProblem(502, "RELAY_ENROLLMENT_EXPIRY_INVALID")
	}
	secret, err := randomPairingSecret()
	if err != nil {
		return PairingCodeResult{}, pairingProblemWithCause(503, "PAIRING_RANDOM_UNAVAILABLE", err)
	}
	secretDigest, err := servercontrolregistry.SecretDigest(secret)
	if err != nil {
		return PairingCodeResult{}, pairingProblemWithCause(503, "PAIRING_RANDOM_UNAVAILABLE", err)
	}
	relaySpec, err := json.Marshal(input.Enrollment)
	if err != nil {
		return PairingCodeResult{}, err
	}
	enrollment, err := registry.CreateEnrollment(ctx, servercontrolregistry.EnrollmentCreate{
		EnrollmentID: prepared.EnrollmentID, Purpose: servercontrolregistry.PurposeRelay,
		Action: servercontrolregistry.ActionEnroll, ManagedID: input.InstanceID,
		Name: input.Enrollment.NodeID, ConfigurationDigest: prepared.ConfigurationDigest,
		SecretDigest: secretDigest, ExpiresAt: expiresAt,
		RecoveryTTL: s.config.Pairing.EffectiveRecoveryTTL(), RelaySpecJSON: string(relaySpec),
	})
	if err != nil {
		_, _ = provider.RevokeRelayEnrollment(ctx, starrycontrol.RelayEnrollmentRevokeRequest{
			Version: 1, EnrollmentID: prepared.EnrollmentID, ConfigurationDigest: prepared.ConfigurationDigest,
		})
		return PairingCodeResult{}, registryProblem(err)
	}
	return s.encodePairingCode(enrollment, secret)
}

func (s *StarryControlService) ClaimPairing(ctx context.Context, request servercontrolregistry.ClaimRequest) (servercontrolregistry.ClaimResponse, error) {
	registry, err := s.pairingRegistry()
	if err != nil {
		return servercontrolregistry.ClaimResponse{}, err
	}
	return auditedControlCall(s, ctx, "server_control.pairing.claim", request.EnrollmentID, map[string]interface{}{
		"purpose": request.Purpose, "enrollment_id": request.EnrollmentID,
	}, func() (servercontrolregistry.ClaimResponse, error) {
		binding, err := registry.BeginClaim(ctx, request)
		if err != nil {
			return servercontrolregistry.ClaimResponse{}, registryProblem(err)
		}
		response := servercontrolregistry.ClaimResponse{
			Version: 1, Purpose: request.Purpose, EnrollmentID: request.EnrollmentID,
			ConfigurationDigest: request.ConfigurationDigest, RequestDigest: request.RequestDigest,
			KeyFingerprint: request.KeyFingerprint,
		}
		switch request.Purpose {
		case servercontrolregistry.PurposeControlAgent:
			prepared, err := servercontrolregistry.PrepareControlIdentity(registry.Root(), servercontrolregistry.ControlIdentityOptions{
				ManagedID: binding.Enrollment.ManagedID, EnrollmentID: binding.Enrollment.EnrollmentID,
				Name: binding.Enrollment.Name, AgentOrigin: binding.Enrollment.AgentOrigin,
				TLSServerName: binding.Enrollment.TLSServerName, BrokerOrigin: s.config.Pairing.BrokerOrigin,
				CenterPublicKey: s.centerPublicKey,
			})
			if err != nil || prepared.ConfigurationDigest != binding.Enrollment.ConfigurationDigest {
				return servercontrolregistry.ClaimResponse{}, pairingProblemWithCause(409, "PAIRING_CONFIGURATION_DRIFT", err)
			}
			certificate, err := prepared.IssueAgentCertificate(ctx, request)
			if err != nil {
				return servercontrolregistry.ClaimResponse{}, registryProblem(err)
			}
			managed, err := prepared.ManagedInstance(binding.Enrollment.InstanceUUID)
			if err != nil {
				return servercontrolregistry.ClaimResponse{}, registryProblem(err)
			}
			managed, err = registry.CompleteControlClaim(ctx, request.EnrollmentID, managed)
			if err != nil {
				return servercontrolregistry.ClaimResponse{}, registryProblem(err)
			}
			if _, err := servercontrolregistry.WriteStaticExport(registry.Root(), managed); err != nil {
				return servercontrolregistry.ClaimResponse{}, pairingProblemWithCause(503, "STATIC_EXPORT_FAILED", err)
			}
			if err := s.refreshManagedInstances(ctx); err != nil {
				return servercontrolregistry.ClaimResponse{}, pairingProblemWithCause(503, "MANAGED_PROVIDER_RELOAD_FAILED", err)
			}
			response.Bundle = prepared.Bundle(binding.Enrollment.InstanceUUID, certificate)
		case servercontrolregistry.PurposeRelay:
			provider, err := s.provider(binding.Enrollment.ManagedID, true)
			if err != nil {
				return servercontrolregistry.ClaimResponse{}, err
			}
			completed, err := provider.CompleteRelayEnrollment(ctx, starrycontrol.RelayEnrollmentCompleteRequest{
				Version: 1, EnrollmentID: request.EnrollmentID, ConfigurationDigest: request.ConfigurationDigest,
				RequestDigest: request.RequestDigest, KeyFingerprint: request.KeyFingerprint, CSRPEM: request.CSRPEM,
			})
			if err != nil {
				return servercontrolregistry.ClaimResponse{}, err
			}
			if err := validateRelayPairingBundle(binding.Enrollment.RelaySpecJSON, completed.Bundle); err != nil {
				return servercontrolregistry.ClaimResponse{}, pairingProblemWithCause(502, "RELAY_CONFIGURATION_DRIFT", err)
			}
			if err := registry.CompleteRelayClaim(ctx, request.EnrollmentID); err != nil {
				return servercontrolregistry.ClaimResponse{}, registryProblem(err)
			}
			response.Bundle = completed.Bundle
		default:
			return servercontrolregistry.ClaimResponse{}, starrycontrol.ErrRequestInvalid
		}
		return response, nil
	})
}

func (s *StarryControlService) SetManagedWriteEnabled(ctx context.Context, managedID string, input ManagedWriteRequest) (ServerControlInstance, error) {
	metadata := map[string]interface{}{
		"before": map[string]interface{}{"read_only": "unknown"},
		"after":  map[string]interface{}{"read_only": !input.WriteEnabled},
	}
	if registry, _ := s.registryState(); registry != nil {
		if current, err := registry.ManagedInstance(ctx, managedID); err == nil {
			metadata["before"] = map[string]interface{}{"read_only": current.ReadOnly}
		}
	}
	return auditedControlCall(s, ctx, "server_control.managed.write_policy", managedID, metadata, func() (ServerControlInstance, error) {
		registry, err := s.pairingRegistry()
		if err != nil {
			return ServerControlInstance{}, err
		}
		expected := fmt.Sprintf("confirm:managed-write:%s:%t", managedID, input.WriteEnabled)
		if input.Confirmation != expected {
			return ServerControlInstance{}, pairingProblem(428, "HIGH_RISK_CONFIRMATION_REQUIRED")
		}
		if _, err := registry.SetManagedReadOnly(ctx, managedID, !input.WriteEnabled); err != nil {
			return ServerControlInstance{}, registryProblem(err)
		}
		if err := s.refreshManagedInstances(ctx); err != nil {
			return ServerControlInstance{}, err
		}
		s.mu.RLock()
		defer s.mu.RUnlock()
		return s.instances[managedID], nil
	})
}

func (s *StarryControlService) ListRelayEnrollments(ctx context.Context, instanceID string) (starrycontrol.RelayEnrollmentList, error) {
	return auditedControlCall(s, ctx, "server_control.relay_enrollment.list", instanceID, nil, func() (starrycontrol.RelayEnrollmentList, error) {
		provider, err := s.provider(instanceID, false)
		if err != nil {
			return starrycontrol.RelayEnrollmentList{}, err
		}
		return provider.ListRelayEnrollments(ctx)
	})
}

func (s *StarryControlService) ListRelayEnrollmentsLocal(ctx context.Context, instanceID string) (starrycontrol.RelayEnrollmentList, error) {
	provider, err := s.provider(instanceID, false)
	if err != nil {
		return starrycontrol.RelayEnrollmentList{}, err
	}
	return provider.ListRelayEnrollments(ctx)
}

func (s *StarryControlService) ActivateRelayEnrollment(ctx context.Context, instanceID string, input starrycontrol.RelayEnrollmentActivateRequest, confirmation string) (starrycontrol.RelayEnrollmentSummary, error) {
	return auditedControlCall(s, ctx, "server_control.relay_enrollment.activate", instanceID, map[string]interface{}{
		"enrollment_id": input.EnrollmentID, "operation_id": input.OperationID,
		"config_generation": input.ConfigGeneration, "health_snapshot_id": input.HealthSnapshotID,
	}, func() (starrycontrol.RelayEnrollmentSummary, error) {
		expected := fmt.Sprintf("confirm:relay-activate:%s:%s:%d", input.EnrollmentID, input.OperationID, input.ConfigGeneration)
		if confirmation != expected {
			return starrycontrol.RelayEnrollmentSummary{}, pairingProblem(428, "HIGH_RISK_CONFIRMATION_REQUIRED")
		}
		provider, err := s.provider(instanceID, true)
		if err != nil {
			return starrycontrol.RelayEnrollmentSummary{}, err
		}
		return provider.ActivateRelayEnrollment(ctx, input)
	})
}

func (s *StarryControlService) ActivateRelayEnrollmentLocal(ctx context.Context, instanceID string, input starrycontrol.RelayEnrollmentActivateRequest, confirmation string) (starrycontrol.RelayEnrollmentSummary, error) {
	expected := fmt.Sprintf("confirm:relay-activate:%s:%s:%d", input.EnrollmentID, input.OperationID, input.ConfigGeneration)
	if confirmation != expected {
		return starrycontrol.RelayEnrollmentSummary{}, pairingProblem(428, "HIGH_RISK_CONFIRMATION_REQUIRED")
	}
	provider, err := s.provider(instanceID, true)
	if err != nil {
		return starrycontrol.RelayEnrollmentSummary{}, err
	}
	return provider.ActivateRelayEnrollment(ctx, input)
}

func (s *StarryControlService) RevokeRelayEnrollment(ctx context.Context, instanceID string, input starrycontrol.RelayEnrollmentRevokeRequest) (starrycontrol.RelayEnrollmentSummary, error) {
	return auditedControlCall(s, ctx, "server_control.relay_enrollment.revoke", instanceID, map[string]interface{}{
		"enrollment_id": input.EnrollmentID,
	}, func() (starrycontrol.RelayEnrollmentSummary, error) {
		provider, err := s.provider(instanceID, true)
		if err != nil {
			return starrycontrol.RelayEnrollmentSummary{}, err
		}
		result, err := provider.RevokeRelayEnrollment(ctx, input)
		if err != nil {
			return starrycontrol.RelayEnrollmentSummary{}, err
		}
		if err := s.syncLocalRelayRevocation(ctx, instanceID, input); err != nil {
			return starrycontrol.RelayEnrollmentSummary{}, err
		}
		return result, nil
	})
}

func (s *StarryControlService) RevokeRelayEnrollmentLocal(ctx context.Context, instanceID string, input starrycontrol.RelayEnrollmentRevokeRequest) (starrycontrol.RelayEnrollmentSummary, error) {
	provider, err := s.provider(instanceID, true)
	if err != nil {
		return starrycontrol.RelayEnrollmentSummary{}, err
	}
	result, err := provider.RevokeRelayEnrollment(ctx, input)
	if err != nil {
		return starrycontrol.RelayEnrollmentSummary{}, err
	}
	if err := s.syncLocalRelayRevocation(ctx, instanceID, input); err != nil {
		return starrycontrol.RelayEnrollmentSummary{}, err
	}
	return result, nil
}

func (s *StarryControlService) syncLocalRelayRevocation(ctx context.Context, instanceID string, input starrycontrol.RelayEnrollmentRevokeRequest) error {
	registry, _ := s.registryState()
	if registry == nil {
		return nil
	}
	enrollment, err := registry.Enrollment(ctx, input.EnrollmentID)
	if errors.Is(err, servercontrolregistry.ErrNotFound) {
		return nil
	}
	if err != nil {
		return registryProblem(err)
	}
	if enrollment.Purpose != servercontrolregistry.PurposeRelay || enrollment.ManagedID != instanceID ||
		enrollment.ConfigurationDigest != input.ConfigurationDigest {
		return pairingProblem(409, "PAIRING_RELAY_BINDING_MISMATCH")
	}
	if enrollment.State != servercontrolregistry.StatePending && enrollment.State != servercontrolregistry.StateBound {
		return nil
	}
	if err := registry.RevokeEnrollment(ctx, input.EnrollmentID); err != nil {
		return registryProblem(err)
	}
	return nil
}

func (s *StarryControlService) pairingRegistry() (*servercontrolregistry.Store, error) {
	if s == nil || !s.config.Pairing.Enabled {
		return nil, pairingProblem(404, "PAIRING_DISABLED")
	}
	registry, registryErr := s.registryState()
	if registry == nil {
		return nil, pairingProblemWithCause(503, "PAIRING_REGISTRY_UNAVAILABLE", registryErr)
	}
	return registry, nil
}

// ensurePairingRegistry may initialize an absent registry only for an exact,
// already-validated new-pair action. Startup, status, rotate, adopt, and Relay
// operations never replace state lost through down -v or a changed data path.
func (s *StarryControlService) ensurePairingRegistry(allowInitialize bool) (*servercontrolregistry.Store, error) {
	if s == nil || !s.config.Pairing.Enabled {
		return nil, pairingProblem(404, "PAIRING_DISABLED")
	}
	s.registryInitMu.Lock()
	defer s.registryInitMu.Unlock()
	registry, registryErr := s.registryState()
	if registry != nil {
		return registry, nil
	}
	if !allowInitialize || !errors.Is(registryErr, servercontrolregistry.ErrNotFound) {
		return nil, pairingProblemWithCause(503, "PAIRING_REGISTRY_UNAVAILABLE", registryErr)
	}
	s.mu.RLock()
	closed := s.closed
	s.mu.RUnlock()
	if closed {
		return nil, pairingProblem(503, "PAIRING_REGISTRY_UNAVAILABLE")
	}
	registry, err := servercontrolregistry.Open(s.config.EffectiveRegistryDirectory(), servercontrolregistry.OpenOptions{HostIdentityFile: s.config.EffectiveHostIdentityFile()})
	if err != nil {
		s.mu.Lock()
		s.registryErr = err
		s.mu.Unlock()
		return nil, pairingProblemWithCause(503, "PAIRING_REGISTRY_UNAVAILABLE", err)
	}
	s.mu.Lock()
	s.registry = registry
	s.registryErr = nil
	s.mu.Unlock()
	if err := s.refreshManagedInstances(context.Background()); err != nil {
		s.mu.Lock()
		s.registryErr = err
		s.mu.Unlock()
		return nil, pairingProblemWithCause(503, "PAIRING_REGISTRY_UNAVAILABLE", err)
	}
	s.logManagedRegistryPreflight(context.Background())
	return registry, nil
}

func (s *StarryControlService) approvedAgentOrigin(id string) (config.PairingAgentOrigin, bool) {
	for _, item := range s.config.Pairing.AgentOrigins {
		if item.ID == id {
			return item, true
		}
	}
	return config.PairingAgentOrigin{}, false
}

func (s *StarryControlService) encodePairingCode(enrollment servercontrolregistry.Enrollment, secret string) (PairingCodeResult, error) {
	code, err := (servercontrolregistry.PairingCodePayload{
		Version: 1, Purpose: enrollment.Purpose, BrokerOrigin: s.config.Pairing.BrokerOrigin,
		BrokerSPKISHA256: s.config.Pairing.BrokerSPKISHA256, EnrollmentID: enrollment.EnrollmentID,
		ConfigurationDigest: enrollment.ConfigurationDigest, ExpiresAtUnix: enrollment.ExpiresAtUnix, Secret: secret,
	}).Encode()
	if err != nil {
		return PairingCodeResult{}, err
	}
	return PairingCodeResult{
		Version: 1, Purpose: enrollment.Purpose, EnrollmentID: enrollment.EnrollmentID,
		ConfigurationDigest: enrollment.ConfigurationDigest, ExpiresAtUnix: enrollment.ExpiresAtUnix,
		State: enrollment.State, Code: code,
	}, nil
}

func randomPairingSecret() (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

func validCenterPublicKey(value string) bool {
	decoded, err := base64.StdEncoding.DecodeString(value)
	if err != nil {
		decoded, err = base64.RawStdEncoding.DecodeString(value)
	}
	return err == nil && len(decoded) == 32
}

func validateRelayPairingBundle(rawSpec string, bundle starrycontrol.RelayEnrollmentBundle) error {
	var approved starrycontrol.RelayEnrollmentPrepareRequest
	decoder := json.NewDecoder(strings.NewReader(rawSpec))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&approved); err != nil {
		return err
	}
	if approved.NodeID != bundle.NodeID || approved.RelayServer != bundle.RelayServer ||
		approved.PublicEndpoint != bundle.PublicEndpoint || approved.RelayPool != bundle.RelayPool ||
		approved.Profile != bundle.Profile || approved.MaxSessions != bundle.MaxSessions ||
		approved.CapacityBandwidthBPS != bundle.CapacityBandwidthBPS || approved.Draining != bundle.Draining ||
		approved.ActivateAfterHealth != bundle.ActivateAfterHealth || !sameOptionalString(approved.WSSEndpoint, bundle.WSSEndpoint) ||
		!sameOptionalInt(approved.FastMediaUDPPort, bundle.FastMediaUDPPort) {
		return errors.New("Relay claim bundle differs from the Agent-authorized prepare request")
	}
	return nil
}

func sameOptionalString(left, right *string) bool {
	return left == nil && right == nil || left != nil && right != nil && *left == *right
}

func sameOptionalInt(left, right *int) bool {
	return left == nil && right == nil || left != nil && right != nil && *left == *right
}

func pairingProblem(status int, code string) error {
	return &starrycontrol.ProviderError{Status: status, Code: code, Message: "pairing request rejected"}
}

func pairingProblemWithCause(status int, code string, cause error) error {
	if cause == nil {
		return pairingProblem(status, code)
	}
	return fmt.Errorf("%w: %v", pairingProblem(status, code), cause)
}

func registryProblem(err error) error {
	switch {
	case errors.Is(err, servercontrolregistry.ErrNotFound), errors.Is(err, servercontrolregistry.ErrExpired):
		return pairingProblemWithCause(410, "PAIRING_ENROLLMENT_EXPIRED_OR_UNKNOWN", err)
	case errors.Is(err, servercontrolregistry.ErrRevoked):
		return pairingProblemWithCause(410, "PAIRING_ENROLLMENT_REVOKED", err)
	case errors.Is(err, servercontrolregistry.ErrSecret), errors.Is(err, servercontrolregistry.ErrBinding),
		errors.Is(err, servercontrolregistry.ErrConflict), errors.Is(err, servercontrolregistry.ErrRecoveryWindow):
		return pairingProblemWithCause(409, "PAIRING_REPLAY_OR_BINDING_REJECTED", err)
	case errors.Is(err, servercontrolregistry.ErrUnsafePermissions), errors.Is(err, servercontrolregistry.ErrIdentityClone),
		errors.Is(err, servercontrolregistry.ErrFutureSchema):
		return pairingProblemWithCause(503, "PAIRING_REGISTRY_UNAVAILABLE", err)
	default:
		return pairingProblemWithCause(503, "PAIRING_REGISTRY_UNAVAILABLE", err)
	}
}

var managedIDPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]{0,127}$`)
