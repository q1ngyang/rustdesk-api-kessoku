package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"

	"github.com/q1ngyang/rustdesk-api-kessoku/v2/config"
	"github.com/q1ngyang/rustdesk-api-kessoku/v2/internal/starrycontrol"
	starryProvider "github.com/q1ngyang/rustdesk-api-kessoku/v2/internal/starrycontrol/starry"
	"github.com/q1ngyang/rustdesk-api-kessoku/v2/model"
	"github.com/q1ngyang/rustdesk-api-kessoku/v2/model/custom_types"
	"github.com/sirupsen/logrus"
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

func NewStarryControlService(cfg *config.Config, logger *logrus.Logger) *StarryControlService {
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
		if cfg.Auth.CurrentKey.PrivateKeyFile != "" && instance.ControlKeyFile == cfg.Auth.CurrentKey.PrivateKeyFile {
			service.providerErrs[instance.ID] = errors.New("control and access-token key files must be isolated")
			summary.ErrorCode = "CONTROL_KEYRING_NOT_ISOLATED"
			service.instances[instance.ID] = summary
			continue
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

func (s *StarryControlService) Capabilities(ctx context.Context, instanceID string) (starrycontrol.Capabilities, error) {
	provider, err := s.provider(instanceID, false)
	if err != nil {
		return starrycontrol.Capabilities{}, err
	}
	return provider.Capabilities(ctx)
}

func (s *StarryControlService) Health(ctx context.Context, instanceID string) (starrycontrol.Health, error) {
	provider, err := s.provider(instanceID, false)
	if err != nil {
		return starrycontrol.Health{}, err
	}
	return provider.Health(ctx)
}

func (s *StarryControlService) Relays(ctx context.Context, instanceID string) ([]starrycontrol.Relay, error) {
	provider, err := s.provider(instanceID, false)
	if err != nil {
		return nil, err
	}
	return provider.Relays(ctx)
}

func (s *StarryControlService) SimulateAllocation(ctx context.Context, instanceID string, input starrycontrol.SimulationInput) (result starrycontrol.SimulationResult, err error) {
	provider, err := s.provider(instanceID, false)
	if err != nil {
		return result, err
	}
	result, err = provider.SimulateAllocation(ctx, input)
	s.auditResult(ctx, "server_control.simulate", instanceID, map[string]interface{}{
		"transport": input.Transport,
	}, err)
	return result, err
}

func (s *StarryControlService) GetConfig(ctx context.Context, instanceID string) (starrycontrol.ConfigDocument, error) {
	provider, err := s.provider(instanceID, false)
	if err != nil {
		return starrycontrol.ConfigDocument{}, err
	}
	return provider.GetConfig(ctx)
}

func (s *StarryControlService) GetConfigSchema(ctx context.Context, instanceID string) (starrycontrol.SchemaBundle, error) {
	provider, err := s.provider(instanceID, false)
	if err != nil {
		return starrycontrol.SchemaBundle{}, err
	}
	return provider.GetConfigSchema(ctx)
}

func (s *StarryControlService) ValidateConfig(ctx context.Context, instanceID string, input starrycontrol.ConfigCandidate) (result starrycontrol.ValidationResult, err error) {
	provider, err := s.provider(instanceID, true)
	if err != nil {
		return result, err
	}
	result, err = provider.ValidateConfig(ctx, input)
	s.auditResult(ctx, "server_control.config.validate", instanceID, candidateMetadata(input), err)
	return result, err
}

func (s *StarryControlService) PlanConfig(ctx context.Context, instanceID string, input starrycontrol.ConfigCandidate) (result starrycontrol.ConfigPlan, err error) {
	provider, err := s.provider(instanceID, true)
	if err != nil {
		return result, err
	}
	result, err = provider.PlanConfig(ctx, input)
	s.auditResult(ctx, "server_control.config.plan", instanceID, candidateMetadata(input), err)
	return result, err
}

func (s *StarryControlService) ApplyConfig(ctx context.Context, instanceID string, input starrycontrol.ApplyRequest) (starrycontrol.ApplyResult, error) {
	provider, err := s.provider(instanceID, true)
	if err != nil {
		return starrycontrol.ApplyResult{}, err
	}
	event, err := s.auditIntent(ctx, "server_control.config.apply", instanceID, map[string]interface{}{
		"plan_id": input.PlanID,
		"etag":    input.IfMatch,
	})
	if err != nil {
		return starrycontrol.ApplyResult{}, fmt.Errorf("persist apply audit intent: %w", err)
	}
	result, callErr := provider.ApplyConfig(ctx, input)
	s.finishAudit(event, callErr)
	return result, callErr
}

func (s *StarryControlService) Operation(ctx context.Context, instanceID, operationID string) (starrycontrol.Operation, error) {
	provider, err := s.provider(instanceID, false)
	if err != nil {
		return starrycontrol.Operation{}, err
	}
	return provider.Operation(ctx, operationID)
}

func (s *StarryControlService) ConfigHistory(ctx context.Context, instanceID string) ([]starrycontrol.ConfigRevision, error) {
	provider, err := s.provider(instanceID, false)
	if err != nil {
		return nil, err
	}
	return provider.ConfigHistory(ctx)
}

func (s *StarryControlService) RollbackConfig(ctx context.Context, instanceID string, input starrycontrol.RollbackRequest) (starrycontrol.ApplyResult, error) {
	provider, err := s.provider(instanceID, true)
	if err != nil {
		return starrycontrol.ApplyResult{}, err
	}
	event, err := s.auditIntent(ctx, "server_control.config.rollback", instanceID, map[string]interface{}{
		"generation": input.Generation,
		"etag":       input.IfMatch,
	})
	if err != nil {
		return starrycontrol.ApplyResult{}, fmt.Errorf("persist rollback audit intent: %w", err)
	}
	result, callErr := provider.RollbackConfig(ctx, input)
	s.finishAudit(event, callErr)
	return result, callErr
}

func (s *StarryControlService) AuditEvents(page, pageSize uint) *model.AdminAuditEventList {
	result := &model.AdminAuditEventList{Pagination: model.Pagination{Page: int64(page), PageSize: int64(pageSize)}}
	tx := DB.Model(&model.AdminAuditEvent{}).Where("target_type = ?", "starry_instance")
	tx.Count(&result.Total)
	tx.Order("id DESC").Scopes(Paginate(page, pageSize)).Find(&result.Events)
	return result
}

func (s *StarryControlService) provider(instanceID string, write bool) (starrycontrol.ServerControlProvider, error) {
	instance, exists := s.instances[instanceID]
	if !exists {
		return nil, starrycontrol.ErrInstanceNotFound
	}
	if !instance.Enabled {
		return nil, starrycontrol.ErrUnavailable
	}
	if write && s.config.ReadOnly {
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
	if input.YAML != nil {
		hash.Write([]byte(*input.YAML))
		metadata["format"] = "yaml"
	} else if input.Values != nil {
		encoded, _ := json.Marshal(input.Values)
		hash.Write(encoded)
		metadata["format"] = "structured"
	}
	metadata["candidate_digest"] = "sha256:" + hex.EncodeToString(hash.Sum(nil))
	return metadata
}

func (s *StarryControlService) auditIntent(ctx context.Context, action, instanceID string, metadata map[string]interface{}) (*model.AdminAuditEvent, error) {
	request, ok := starrycontrol.MetadataFromContext(ctx)
	if !ok || DB == nil {
		return nil, errors.New("missing audit context or database")
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

func (s *StarryControlService) auditResult(ctx context.Context, action, instanceID string, metadata map[string]interface{}, operationErr error) {
	event, err := s.auditIntent(ctx, action, instanceID, metadata)
	if err != nil {
		if s.logger != nil {
			s.logger.Errorf("persist server-control audit event: %v", err)
		}
		return
	}
	s.finishAudit(event, operationErr)
}

func (s *StarryControlService) finishAudit(event *model.AdminAuditEvent, operationErr error) {
	if event == nil || DB == nil {
		return
	}
	updates := map[string]interface{}{"result": "success", "error_code": ""}
	if operationErr != nil {
		updates["result"] = "failure"
		updates["error_code"] = stableControlErrorCode(operationErr)
	}
	if err := DB.Model(event).Updates(updates).Error; err != nil && s.logger != nil {
		s.logger.Errorf("finish server-control audit event %d: %v", event.Id, err)
	}
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
