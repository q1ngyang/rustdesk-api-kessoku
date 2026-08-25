package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/q1ngyang/rustdesk-api-kessoku/v3/config"
	internalAuth "github.com/q1ngyang/rustdesk-api-kessoku/v3/internal/auth"
	"github.com/q1ngyang/rustdesk-api-kessoku/v3/internal/controlauth"
	"github.com/q1ngyang/rustdesk-api-kessoku/v3/internal/starrycontrol"
	starryProvider "github.com/q1ngyang/rustdesk-api-kessoku/v3/internal/starrycontrol/starry"
	"github.com/q1ngyang/rustdesk-api-kessoku/v3/model"
	"github.com/q1ngyang/rustdesk-api-kessoku/v3/model/custom_types"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

type ServerControlInstance struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Enabled   bool   `json:"enabled"`
	ReadOnly  bool   `json:"read_only"`
	Available bool   `json:"available"`
	ErrorCode string `json:"error_code,omitempty"`
}

type StarryControlService struct {
	config       config.ServerControl
	instances    map[string]ServerControlInstance
	providers    map[string]starrycontrol.ServerControlProvider
	providerErrs map[string]error
	logger       *logrus.Logger
}

func NewStarryControlService(cfg *config.Config, logger *logrus.Logger, authManager *internalAuth.Manager) *StarryControlService {
	service := &StarryControlService{
		config:       cfg.ServerControl,
		instances:    make(map[string]ServerControlInstance, len(cfg.ServerControl.Instances)),
		providers:    make(map[string]starrycontrol.ServerControlProvider, len(cfg.ServerControl.Instances)),
		providerErrs: make(map[string]error),
		logger:       logger,
	}
	for _, instance := range cfg.ServerControl.Instances {
		summary := ServerControlInstance{ID: instance.ID, Name: instance.Name, Enabled: instance.Enabled, ReadOnly: cfg.ServerControl.ReadOnly}
		if instance.ID == "" {
			continue
		}
		if _, duplicate := service.instances[instance.ID]; duplicate {
			service.providerErrs[instance.ID] = errors.New("duplicate Starry instance id")
			summary.ErrorCode = "INSTANCE_CONFIG_INVALID"
			service.instances[instance.ID] = summary
			continue
		}
		if !instance.Enabled {
			service.instances[instance.ID] = summary
			continue
		}
		if authManager != nil {
			fingerprint, fingerprintErr := controlauth.PrivateKeyPublicFingerprint(instance.ControlKeyFile)
			if fingerprintErr != nil {
				service.providerErrs[instance.ID] = fmt.Errorf("read control signing key for isolation check: %w", fingerprintErr)
				summary.ErrorCode = "INSTANCE_CONFIG_INVALID"
				service.instances[instance.ID] = summary
				continue
			}
			if authManager.UsesPublicKeyFingerprint(fingerprint) {
				service.providerErrs[instance.ID] = errors.New("control and access-token keyrings must use different Ed25519 keys")
				summary.ErrorCode = "CONTROL_KEYRING_NOT_ISOLATED"
				service.instances[instance.ID] = summary
				continue
			}
		}
		provider, err := starryProvider.NewProvider(instance, cfg.ServerControl)
		if err != nil {
			service.providerErrs[instance.ID] = err
			summary.ErrorCode = "INSTANCE_CONFIG_INVALID"
			if logger != nil {
				logger.Errorf("configure Starry instance %q: %v", instance.ID, err)
			}
		} else {
			service.providers[instance.ID] = provider
			summary.Available = true
		}
		service.instances[instance.ID] = summary
	}
	return service
}

func (s *StarryControlService) Instances() []ServerControlInstance {
	result := make([]ServerControlInstance, 0, len(s.instances))
	for _, instance := range s.instances {
		result = append(result, instance)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ID < result[j].ID })
	return result
}

func (s *StarryControlService) InstancesContext(ctx context.Context) ([]ServerControlInstance, error) {
	return auditedControlCall(s, ctx, "server_control.instances.read", "*", nil, func() ([]ServerControlInstance, error) {
		return s.Instances(), nil
	})
}

func (s *StarryControlService) Capabilities(ctx context.Context, instanceID string) (starrycontrol.Capabilities, error) {
	return auditedControlCall(s, ctx, "server_control.capabilities.read", instanceID, nil, func() (starrycontrol.Capabilities, error) {
		provider, err := s.provider(instanceID, false)
		if err != nil {
			return starrycontrol.Capabilities{}, err
		}
		return provider.Capabilities(ctx)
	})
}

func (s *StarryControlService) Status(ctx context.Context, instanceID string) (starrycontrol.Status, error) {
	return auditedControlCall(s, ctx, "server_control.status.read", instanceID, nil, func() (starrycontrol.Status, error) {
		provider, err := s.provider(instanceID, false)
		if err != nil {
			return starrycontrol.Status{}, err
		}
		return provider.Status(ctx)
	})
}

func (s *StarryControlService) Relays(ctx context.Context, instanceID string) (starrycontrol.RelayInventory, error) {
	return auditedControlCall(s, ctx, "server_control.relays.read", instanceID, nil, func() (starrycontrol.RelayInventory, error) {
		provider, err := s.provider(instanceID, false)
		if err != nil {
			return starrycontrol.RelayInventory{}, err
		}
		return provider.Relays(ctx)
	})
}

func (s *StarryControlService) SimulateAllocation(ctx context.Context, instanceID string, input starrycontrol.SimulationInput) (starrycontrol.SimulationResult, error) {
	return auditedControlCall(s, ctx, "server_control.simulate", instanceID, map[string]interface{}{
		"transport": input.Transport,
	}, func() (starrycontrol.SimulationResult, error) {
		provider, err := s.provider(instanceID, false)
		if err != nil {
			return starrycontrol.SimulationResult{}, err
		}
		return provider.SimulateAllocation(ctx, input)
	})
}

func (s *StarryControlService) GetConfig(ctx context.Context, instanceID string) (starrycontrol.ConfigDocument, error) {
	return auditedControlCall(s, ctx, "server_control.config.read", instanceID, nil, func() (starrycontrol.ConfigDocument, error) {
		provider, err := s.provider(instanceID, false)
		if err != nil {
			return starrycontrol.ConfigDocument{}, err
		}
		return provider.GetConfig(ctx)
	})
}

func (s *StarryControlService) GetConfigSchema(ctx context.Context, instanceID string) (starrycontrol.SchemaBundle, error) {
	return auditedControlCall(s, ctx, "server_control.config_schema.read", instanceID, nil, func() (starrycontrol.SchemaBundle, error) {
		provider, err := s.provider(instanceID, false)
		if err != nil {
			return starrycontrol.SchemaBundle{}, err
		}
		return provider.GetConfigSchema(ctx)
	})
}

func (s *StarryControlService) ValidateConfig(ctx context.Context, instanceID string, input starrycontrol.ConfigCandidate) (starrycontrol.ValidationResult, error) {
	return auditedControlCall(s, ctx, "server_control.config.validate", instanceID, candidateMetadata(input), func() (starrycontrol.ValidationResult, error) {
		provider, err := s.provider(instanceID, false)
		if err != nil {
			return starrycontrol.ValidationResult{}, err
		}
		return provider.ValidateConfig(ctx, input)
	})
}

func (s *StarryControlService) PlanConfig(ctx context.Context, instanceID string, input starrycontrol.ConfigCandidate) (starrycontrol.ConfigPlan, error) {
	return auditedControlCall(s, ctx, "server_control.config.plan", instanceID, candidateMetadata(input), func() (starrycontrol.ConfigPlan, error) {
		provider, err := s.provider(instanceID, false)
		if err != nil {
			return starrycontrol.ConfigPlan{}, err
		}
		return provider.PlanConfig(ctx, input)
	})
}

func (s *StarryControlService) ApplyConfig(ctx context.Context, instanceID string, input starrycontrol.ApplyRequest) (starrycontrol.Operation, error) {
	return s.startControlOperation(ctx, "server_control.config.apply", instanceID, "config_apply", map[string]interface{}{
		"plan_id":          input.PlanID,
		"etag":             input.IfMatch,
		"candidate_digest": input.CandidateDigest,
	}, func() (starrycontrol.Operation, string, error) {
		provider, err := s.provider(instanceID, true)
		if err != nil {
			return starrycontrol.Operation{}, "", err
		}
		operation, err := provider.ApplyConfig(ctx, input)
		return operation, input.CandidateDigest, err
	})
}

func (s *StarryControlService) Operation(ctx context.Context, instanceID, operationID string) (starrycontrol.Operation, error) {
	return auditedControlCall(s, ctx, "server_control.operation.read", instanceID, map[string]interface{}{
		"operation_id": operationID,
	}, func() (starrycontrol.Operation, error) {
		expectations, err := s.operationExpectations(instanceID, operationID)
		if err != nil {
			return starrycontrol.Operation{}, err
		}
		provider, err := s.provider(instanceID, false)
		if err != nil {
			return starrycontrol.Operation{}, err
		}
		operation, err := provider.Operation(ctx, operationID)
		if err != nil {
			return starrycontrol.Operation{}, err
		}
		if err := validateExpectedControlOperation(operation, expectations[0].Kind, expectations[0].ExpectedSourceDigest); err != nil {
			return starrycontrol.Operation{}, err
		}
		if err := s.finalizeControlOperation(expectations, operation); err != nil {
			return starrycontrol.Operation{}, err
		}
		return operation, nil
	})
}

func (s *StarryControlService) ConfigHistory(ctx context.Context, instanceID string) ([]starrycontrol.ConfigRevision, error) {
	return auditedControlCall(s, ctx, "server_control.config_history.read", instanceID, nil, func() ([]starrycontrol.ConfigRevision, error) {
		provider, err := s.provider(instanceID, false)
		if err != nil {
			return nil, err
		}
		return provider.ConfigHistory(ctx)
	})
}

func (s *StarryControlService) RollbackConfig(ctx context.Context, instanceID string, input starrycontrol.RollbackRequest) (starrycontrol.Operation, error) {
	return s.startControlOperation(ctx, "server_control.config.rollback", instanceID, "config_rollback", map[string]interface{}{
		"revision_id": input.RevisionID,
		"etag":        input.IfMatch,
	}, func() (starrycontrol.Operation, string, error) {
		provider, err := s.provider(instanceID, true)
		if err != nil {
			return starrycontrol.Operation{}, "", err
		}
		revisions, err := provider.ConfigHistory(ctx)
		if err != nil {
			return starrycontrol.Operation{}, "", err
		}
		expectedDigest := ""
		for _, revision := range revisions {
			if revision.ID == input.RevisionID {
				expectedDigest = revision.CandidateDigest
				break
			}
		}
		if expectedDigest == "" {
			return starrycontrol.Operation{}, "", starrycontrol.ErrRequestInvalid
		}
		operation, err := provider.RollbackConfig(ctx, input)
		return operation, expectedDigest, err
	})
}

func (s *StarryControlService) ReloadRuntime(ctx context.Context, instanceID string, input starrycontrol.RuntimeReloadRequest) (starrycontrol.ActivationAck, error) {
	return auditedControlCall(s, ctx, "server_control.runtime.reload", instanceID, map[string]interface{}{
		"expected_source_digest": input.ExpectedSourceDigest,
	}, func() (starrycontrol.ActivationAck, error) {
		provider, err := s.provider(instanceID, true)
		if err != nil {
			return starrycontrol.ActivationAck{}, err
		}
		return provider.ReloadRuntime(ctx, input)
	})
}

func (s *StarryControlService) AuditEvents(ctx context.Context, page, pageSize uint) (*model.AdminAuditEventList, error) {
	return auditedControlCall(s, ctx, "server_control.audit_events.read", "*", map[string]interface{}{
		"page":      page,
		"page_size": pageSize,
	}, func() (*model.AdminAuditEventList, error) {
		result := &model.AdminAuditEventList{Pagination: model.Pagination{Page: int64(page), PageSize: int64(pageSize)}}
		tx := DB.Model(&model.AdminAuditEvent{}).Where("target_type = ?", "starry_instance")
		if err := tx.Count(&result.Total).Error; err != nil {
			return nil, err
		}
		if err := tx.Order("id DESC").Scopes(Paginate(page, pageSize)).Find(&result.Events).Error; err != nil {
			return nil, err
		}
		return result, nil
	})
}

func (s *StarryControlService) provider(instanceID string, mutatesActiveConfig bool) (starrycontrol.ServerControlProvider, error) {
	instance, exists := s.instances[instanceID]
	if !exists {
		return nil, starrycontrol.ErrInstanceNotFound
	}
	if !instance.Enabled {
		return nil, starrycontrol.ErrUnavailable
	}
	if mutatesActiveConfig && s.config.ReadOnly {
		return nil, starrycontrol.ErrReadOnly
	}
	provider, ok := s.providers[instanceID]
	if !ok {
		return nil, starrycontrol.ErrUnavailable
	}
	return provider, nil
}

func candidateMetadata(input starrycontrol.ConfigCandidate) map[string]interface{} {
	metadata := map[string]interface{}{"etag": input.BaseETag}
	hash := sha256.New()
	hash.Write([]byte(input.Document))
	metadata["format"] = input.Format
	metadata["candidate_digest"] = "sha256:" + hex.EncodeToString(hash.Sum(nil))
	return metadata
}

func (s *StarryControlService) auditIntent(ctx context.Context, action, instanceID string, metadata map[string]interface{}) (*model.AdminAuditEvent, error) {
	request, ok := starrycontrol.MetadataFromContext(ctx)
	if !ok || DB == nil {
		return nil, errors.New("missing audit context or database")
	}
	if metadata == nil {
		metadata = map[string]interface{}{}
	}
	encoded, err := json.Marshal(metadata)
	if err != nil {
		return nil, err
	}
	event := &model.AdminAuditEvent{
		ActorUserID: request.ActorUserID,
		Action:      action,
		TargetType:  "starry_instance",
		TargetID:    instanceID,
		RequestID:   request.RequestID,
		Result:      "intent",
		Metadata:    custom_types.AutoJson(encoded),
	}
	if err := DB.Create(event).Error; err != nil {
		return nil, err
	}
	return event, nil
}

func (s *StarryControlService) finishAudit(event *model.AdminAuditEvent, operationErr error) error {
	if event == nil || DB == nil {
		return errors.New("missing audit event or database")
	}
	updates := map[string]interface{}{"result": "success", "error_code": ""}
	if operationErr != nil {
		updates["result"] = "failure"
		updates["error_code"] = stableControlErrorCode(operationErr)
	}
	if err := DB.Model(event).Updates(updates).Error; err != nil {
		if s.logger != nil {
			s.logger.Errorf("finish server-control audit event %d: %v", event.Id, err)
		}
		return err
	}
	return nil
}

func (s *StarryControlService) startControlOperation(
	ctx context.Context,
	action string,
	instanceID string,
	expectedKind string,
	metadata map[string]interface{},
	start func() (starrycontrol.Operation, string, error),
) (starrycontrol.Operation, error) {
	event, err := s.auditIntent(ctx, action, instanceID, metadata)
	if err != nil {
		return starrycontrol.Operation{}, fmt.Errorf("persist server-control audit intent: %w", err)
	}
	operation, expectedDigest, operationErr := start()
	if operationErr == nil {
		operationErr = validateExpectedControlOperation(operation, expectedKind, expectedDigest)
	}
	if operationErr != nil {
		if auditErr := s.finishAudit(event, operationErr); auditErr != nil {
			return starrycontrol.Operation{}, errors.Join(operationErr, fmt.Errorf("finish server-control audit event: %w", auditErr))
		}
		return starrycontrol.Operation{}, operationErr
	}
	expectation := &model.ControlOperationExpectation{
		OperationID:          operation.ID,
		InstanceID:           instanceID,
		Kind:                 expectedKind,
		ExpectedSourceDigest: expectedDigest,
		AuditEventID:         event.Id,
		ExpiresAt:            time.Now().Add(7 * 24 * time.Hour).Unix(),
	}
	result, errorCode := controlOperationAuditResult(operation)
	persistErr := DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(expectation).Error; err != nil {
			return err
		}
		return tx.Model(&model.AdminAuditEvent{}).Where("id = ?", event.Id).
			Updates(map[string]interface{}{"result": result, "error_code": errorCode}).Error
	})
	if persistErr != nil {
		persistErr = fmt.Errorf("persist Starry operation expectation: %w", persistErr)
		_ = s.finishAudit(event, persistErr)
		return starrycontrol.Operation{}, persistErr
	}
	return operation, nil
}

func (s *StarryControlService) operationExpectations(instanceID, operationID string) ([]model.ControlOperationExpectation, error) {
	expectations := []model.ControlOperationExpectation{}
	if err := DB.Where("instance_id = ? AND operation_id = ? AND expires_at > ?", instanceID, operationID, time.Now().Unix()).
		Order("id ASC").Find(&expectations).Error; err != nil {
		return nil, err
	}
	if len(expectations) == 0 {
		return nil, starrycontrol.ErrRequestInvalid
	}
	for _, expectation := range expectations[1:] {
		if expectation.Kind != expectations[0].Kind || expectation.ExpectedSourceDigest != expectations[0].ExpectedSourceDigest {
			return nil, errors.New("conflicting Starry operation expectations")
		}
	}
	return expectations, nil
}

func validateExpectedControlOperation(operation starrycontrol.Operation, expectedKind, expectedDigest string) error {
	if operation.ID == "" || operation.Kind != expectedKind || expectedDigest == "" {
		return errors.New("Starry operation did not match the authorized request")
	}
	if operation.ActivationAck != nil && operation.ActivationAck.SourceDigest != expectedDigest {
		return errors.New("Starry operation source digest did not match the authorized request")
	}
	return nil
}

func (s *StarryControlService) finalizeControlOperation(expectations []model.ControlOperationExpectation, operation starrycontrol.Operation) error {
	result, errorCode := controlOperationAuditResult(operation)
	if result == "pending" {
		return nil
	}
	ids := make([]uint, 0, len(expectations))
	for _, expectation := range expectations {
		ids = append(ids, expectation.AuditEventID)
	}
	return DB.Model(&model.AdminAuditEvent{}).Where("id IN ?", ids).
		Updates(map[string]interface{}{"result": result, "error_code": errorCode}).Error
}

func controlOperationAuditResult(operation starrycontrol.Operation) (string, string) {
	switch operation.State {
	case "pending", "running":
		return "pending", ""
	case "succeeded":
		return "success", ""
	case "rolled_back", "failed", "manual_intervention_required":
		if operation.Error != nil && operation.Error.Code != "" {
			return "failure", operation.Error.Code
		}
		return "failure", "STARRY_OPERATION_FAILED"
	default:
		return "failure", "STARRY_OPERATION_INVALID"
	}
}

func auditedControlCall[T any](
	s *StarryControlService,
	ctx context.Context,
	action string,
	instanceID string,
	metadata map[string]interface{},
	operation func() (T, error),
) (T, error) {
	var zero T
	event, err := s.auditIntent(ctx, action, instanceID, metadata)
	if err != nil {
		return zero, fmt.Errorf("persist server-control audit intent: %w", err)
	}
	result, operationErr := operation()
	if auditErr := s.finishAudit(event, operationErr); auditErr != nil {
		auditErr = fmt.Errorf("finish server-control audit event: %w", auditErr)
		if operationErr != nil {
			return result, errors.Join(operationErr, auditErr)
		}
		return zero, auditErr
	}
	return result, operationErr
}

func stableControlErrorCode(err error) string {
	if err == nil {
		return ""
	}
	var providerErr *starrycontrol.ProviderError
	if errors.As(err, &providerErr) && providerErr.Code != "" {
		return providerErr.Code
	}
	switch {
	case errors.Is(err, starrycontrol.ErrReadOnly):
		return "CONTROL_READ_ONLY"
	case errors.Is(err, starrycontrol.ErrInstanceNotFound):
		return "INSTANCE_NOT_FOUND"
	case errors.Is(err, starrycontrol.ErrRequestInvalid):
		return "REQUEST_INVALID"
	default:
		return "STARRY_CONTROL_UNAVAILABLE"
	}
}
