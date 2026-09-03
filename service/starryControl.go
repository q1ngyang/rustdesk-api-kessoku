package service

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/netip"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/q1ngyang/rustdesk-api-kessoku/v3/config"
	internalAuth "github.com/q1ngyang/rustdesk-api-kessoku/v3/internal/auth"
	"github.com/q1ngyang/rustdesk-api-kessoku/v3/internal/controlauth"
	"github.com/q1ngyang/rustdesk-api-kessoku/v3/internal/servercontrolregistry"
	"github.com/q1ngyang/rustdesk-api-kessoku/v3/internal/starrycontrol"
	starryProvider "github.com/q1ngyang/rustdesk-api-kessoku/v3/internal/starrycontrol/starry"
	"github.com/q1ngyang/rustdesk-api-kessoku/v3/model"
	"github.com/q1ngyang/rustdesk-api-kessoku/v3/model/custom_types"
	"github.com/sirupsen/logrus"
	"golang.org/x/sys/unix"
	"gorm.io/gorm"
)

type ServerControlInstance struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Enabled   bool   `json:"enabled"`
	Managed   bool   `json:"managed"`
	ReadOnly  bool   `json:"read_only"`
	Available bool   `json:"available"`
	ErrorCode string `json:"error_code,omitempty"`
}

type ControlLogSource struct {
	ID           string `json:"id"`
	Label        string `json:"label"`
	Component    string `json:"component"`
	InstanceID   string `json:"instance_id"`
	Available    bool   `json:"available"`
	LevelMutable bool   `json:"level_mutable"`
	CurrentLevel string `json:"current_level,omitempty"`
}

type ControlLogEntry struct {
	Sequence int    `json:"sequence"`
	Level    string `json:"level"`
	Text     string `json:"text"`
}

type ControlLogResult struct {
	Source    ControlLogSource  `json:"source"`
	Entries   []ControlLogEntry `json:"entries"`
	Truncated bool              `json:"truncated"`
}

var (
	bearerLogValue     = regexp.MustCompile(`(?i)(authorization\s*:\s*bearer\s+)[^\s,;]+`)
	structuredLogValue = regexp.MustCompile(`(?i)(["']?(?:access[_ -]?token|refresh[_ -]?token|api[_ -]?token|lease[_ -]?token|connection[_ -]?token|control[_ -]?token|route[_ -]?leases?|client[_ -]?secret|password|session[_ -]?cookie|private[_ -]?key|nonce|allocation[_ -]?(?:id|uuid)|session[_ -]?(?:id|uuid))["']?\s*[:=]\s*)(?:\[[^\]]*\]|"[^"]*"|'[^']*'|[^\s,;}]+)`)
	ipv4LogValue       = regexp.MustCompile(`(?:[0-9]{1,3}\.){3}[0-9]{1,3}`)
	ipv6LogValue       = regexp.MustCompile(`(?i)\[[0-9a-f:.]+(?:%[0-9a-z_.-]+)?\]|(?:[0-9a-f]{0,4}:){2,}[0-9a-f:.]*(?:%[0-9a-z_.-]+)?`)
)

type StarryControlService struct {
	mu              sync.RWMutex
	registryInitMu  sync.Mutex
	config          config.ServerControl
	logDirectory    string
	logSources      []config.ControlLogSource
	instances       map[string]ServerControlInstance
	providers       map[string]starrycontrol.ServerControlProvider
	providerErrs    map[string]error
	staticIDs       map[string]struct{}
	managedIDs      map[string]struct{}
	registry        *servercontrolregistry.Store
	registryErr     error
	registryGen     uint64
	closed          bool
	authManager     *internalAuth.Manager
	centerPublicKey string
	logger          *logrus.Logger
	planReviewKey   [32]byte
}

func NewStarryControlService(cfg *config.Config, logger *logrus.Logger, authManager *internalAuth.Manager) *StarryControlService {
	logDirectory, logSources := controlLogConfiguration(cfg)
	service := &StarryControlService{
		config:          cfg.ServerControl,
		logDirectory:    logDirectory,
		logSources:      logSources,
		instances:       make(map[string]ServerControlInstance, len(cfg.ServerControl.Instances)),
		providers:       make(map[string]starrycontrol.ServerControlProvider, len(cfg.ServerControl.Instances)),
		providerErrs:    make(map[string]error),
		staticIDs:       make(map[string]struct{}, len(cfg.ServerControl.Instances)),
		managedIDs:      make(map[string]struct{}),
		authManager:     authManager,
		centerPublicKey: strings.TrimSpace(cfg.Rustdesk.Key),
		logger:          logger,
	}
	if _, err := rand.Read(service.planReviewKey[:]); err != nil {
		if logger != nil {
			logger.Errorf("initialize server-control plan review key: %v", err)
		}
	}
	for _, instance := range cfg.ServerControl.Instances {
		service.staticIDs[instance.ID] = struct{}{}
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
	service.initializeManagedRegistry()
	return service
}

func (s *StarryControlService) Close() error {
	if s == nil {
		return nil
	}
	s.registryInitMu.Lock()
	defer s.registryInitMu.Unlock()
	s.mu.Lock()
	registry := s.registry
	s.registry = nil
	s.closed = true
	s.mu.Unlock()
	if registry == nil {
		return nil
	}
	return registry.Close()
}

func (s *StarryControlService) Instances() []ServerControlInstance {
	_ = s.refreshManagedInstances(context.Background())
	labels := map[string]string{}
	if AllService != nil && AllService.BrandingService != nil {
		labels = AllService.BrandingService.Public().ServerInstanceNames
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]ServerControlInstance, 0, len(s.instances))
	for _, instance := range s.instances {
		if label := strings.TrimSpace(labels[instance.ID]); label != "" {
			instance.Name = label
		}
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

// VerifyPeerIdentity checks every enabled Starry center without emitting an
// administrator audit event. The request is authenticated as the Kessoku
// service itself. An unavailable center never grants access; another configured
// center must positively prove the exact identity before the report is accepted.
func (s *StarryControlService) VerifyPeerIdentity(ctx context.Context, deviceID, deviceUUID string) (bool, error) {
	if s == nil || deviceID == "" || deviceUUID == "" {
		return false, starrycontrol.ErrRequestInvalid
	}
	generated, err := uuid.NewV7()
	if err != nil {
		generated = uuid.New()
	}
	ctx = starrycontrol.WithRequestMetadata(ctx, starrycontrol.RequestMetadata{RequestID: generated.String(), Service: true})
	instanceIDs := make([]string, 0, len(s.instances))
	for instanceID, instance := range s.instances {
		if instance.Enabled {
			instanceIDs = append(instanceIDs, instanceID)
		}
	}
	sort.Strings(instanceIDs)
	if len(instanceIDs) == 0 {
		return false, starrycontrol.ErrUnavailable
	}
	checked := 0
	var combined error
	for _, instanceID := range instanceIDs {
		provider, providerErr := s.provider(instanceID, false)
		if providerErr != nil {
			combined = errors.Join(combined, providerErr)
			continue
		}
		result, verifyErr := provider.VerifyPeer(ctx, starrycontrol.PeerIdentityInput{ID: deviceID, UUID: deviceUUID})
		if verifyErr != nil {
			combined = errors.Join(combined, verifyErr)
			continue
		}
		checked++
		if result.Registered {
			return true, nil
		}
	}
	if combined != nil {
		return false, combined
	}
	if checked == 0 {
		return false, starrycontrol.ErrUnavailable
	}
	return false, nil
}

func (s *StarryControlService) VerifyPeerActivation(
	ctx context.Context,
	deviceID string,
	deviceUUID string,
	activationEpoch uint64,
	activationID string,
	routeLeases []string,
) (bool, error) {
	if s == nil || deviceID == "" || deviceUUID == "" || activationEpoch == 0 || activationID == "" || len(routeLeases) == 0 {
		return false, starrycontrol.ErrRequestInvalid
	}
	generated, err := uuid.NewV7()
	if err != nil {
		generated = uuid.New()
	}
	ctx = starrycontrol.WithRequestMetadata(ctx, starrycontrol.RequestMetadata{RequestID: generated.String(), Service: true})
	instanceIDs := make([]string, 0, len(s.instances))
	for instanceID, instance := range s.instances {
		if instance.Enabled {
			instanceIDs = append(instanceIDs, instanceID)
		}
	}
	sort.Strings(instanceIDs)
	if len(instanceIDs) == 0 {
		return false, starrycontrol.ErrUnavailable
	}
	checked := 0
	var combined error
	for _, instanceID := range instanceIDs {
		provider, providerErr := s.provider(instanceID, false)
		if providerErr != nil {
			combined = errors.Join(combined, providerErr)
			continue
		}
		result, verifyErr := provider.VerifyPeer(ctx, starrycontrol.PeerIdentityInput{
			ID: deviceID, UUID: deviceUUID, ActivationEpoch: activationEpoch,
			ActivationID: activationID, RouteLeases: routeLeases,
		})
		if verifyErr != nil {
			combined = errors.Join(combined, verifyErr)
			continue
		}
		checked++
		if result.Registered {
			return true, nil
		}
	}
	if combined != nil {
		return false, combined
	}
	if checked == 0 {
		return false, starrycontrol.ErrUnavailable
	}
	return false, nil
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
		plan, err := provider.PlanConfig(ctx, input)
		if err != nil {
			return starrycontrol.ConfigPlan{}, err
		}
		capabilities, err := provider.Capabilities(ctx)
		if err != nil {
			return starrycontrol.ConfigPlan{}, err
		}
		plan.ReviewToken, err = s.sealPlanReview(ctx, instanceID, plan, capabilities.Config.SchemaDigest)
		if err != nil {
			return starrycontrol.ConfigPlan{}, err
		}
		return plan, nil
	})
}

func (s *StarryControlService) ApplyConfig(ctx context.Context, instanceID string, input starrycontrol.ApplyRequest) (starrycontrol.Operation, error) {
	metadata := map[string]interface{}{
		"plan_id":          input.PlanID,
		"etag":             input.IfMatch,
		"candidate_digest": input.CandidateDigest,
	}
	claims, reviewErr := s.verifyPlanReview(ctx, instanceID, input)
	if reviewErr == nil {
		metadata["before"] = map[string]interface{}{"etag": claims.BaseETag, "generation": claims.BaseGeneration}
		metadata["after"] = map[string]interface{}{"source_digest": claims.CandidateDigest}
		metadata["risk"] = claims.Risk
		metadata["schema_digest"] = claims.SchemaDigest
		metadata["change_count"] = claims.ChangeCount
	}
	return s.startControlOperation(ctx, "server_control.config.apply", instanceID, "config_apply", metadata, func() (starrycontrol.Operation, string, error) {
		provider, err := s.provider(instanceID, true)
		if err != nil {
			return starrycontrol.Operation{}, "", err
		}
		if reviewErr != nil {
			return starrycontrol.Operation{}, "", reviewErr
		}
		capabilities, err := provider.Capabilities(ctx)
		if err != nil {
			return starrycontrol.Operation{}, "", err
		}
		if capabilities.Config.SchemaDigest != claims.SchemaDigest {
			return starrycontrol.Operation{}, "", planReviewError("PLAN_REVIEW_SCHEMA_CHANGED", "the Starry configuration schema changed after this plan was reviewed")
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
		expectedConfirmation := fmt.Sprintf("confirm:rollback:%s:%s", instanceID, input.RevisionID)
		if input.RiskConfirmation != expectedConfirmation {
			return starrycontrol.Operation{}, "", pairingProblem(428, "HIGH_RISK_CONFIRMATION_REQUIRED")
		}
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

func (s *StarryControlService) AuditEvents(ctx context.Context, page, pageSize uint, dateRange CreatedAtRange) (*model.AdminAuditEventList, error) {
	return auditedControlCall(s, ctx, "server_control.audit_events.read", "*", map[string]interface{}{
		"page":      page,
		"page_size": pageSize,
	}, func() (*model.AdminAuditEventList, error) {
		result := &model.AdminAuditEventList{Pagination: model.Pagination{Page: int64(page), PageSize: int64(pageSize)}}
		tx := dateRange.Apply(DB.Model(&model.AdminAuditEvent{}).Where("target_type = ?", "starry_instance"))
		if err := tx.Count(&result.Total).Error; err != nil {
			return nil, err
		}
		if err := tx.Order("id DESC").Scopes(Paginate(page, pageSize)).Find(&result.Events).Error; err != nil {
			return nil, err
		}
		return result, nil
	})
}

func (s *StarryControlService) LogSources(ctx context.Context, instanceID string) ([]ControlLogSource, error) {
	return auditedControlCall(s, ctx, "server_control.logs.sources", instanceID, nil, func() ([]ControlLogSource, error) {
		if _, exists := s.instances[instanceID]; !exists {
			return nil, starrycontrol.ErrInstanceNotFound
		}
		result := make([]ControlLogSource, 0, len(s.logSources))
		for _, source := range s.logSources {
			if source.InstanceID != "" && source.InstanceID != "*" && source.InstanceID != instanceID {
				continue
			}
			item := s.publicLogSource(source)
			file, err := openControlLog(filepath.Join(s.logDirectory, source.File))
			if err == nil {
				item.Available = true
				_ = file.Close()
			}
			result = append(result, item)
		}
		return result, nil
	})
}

func (s *StarryControlService) Logs(ctx context.Context, instanceID, sourceID string, limit int) (ControlLogResult, error) {
	return auditedControlCall(s, ctx, "server_control.logs.read", instanceID, map[string]interface{}{"source_id": sourceID, "limit": limit}, func() (ControlLogResult, error) {
		source, err := s.logSource(instanceID, sourceID)
		if err != nil {
			return ControlLogResult{}, err
		}
		if limit < 1 {
			limit = 400
		}
		if limit > 2000 {
			limit = 2000
		}
		path := filepath.Join(s.logDirectory, source.File)
		file, err := openControlLog(path)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return ControlLogResult{Source: s.publicLogSource(source), Entries: []ControlLogEntry{}}, nil
			}
			return ControlLogResult{}, err
		}
		defer file.Close()
		const maximumRead = int64(2 << 20)
		stat, err := file.Stat()
		if err != nil {
			return ControlLogResult{}, err
		}
		start := stat.Size() - maximumRead
		truncated := start > 0
		if start < 0 {
			start = 0
		}
		if _, err := file.Seek(start, io.SeekStart); err != nil {
			return ControlLogResult{}, err
		}
		data, err := io.ReadAll(io.LimitReader(file, maximumRead))
		if err != nil {
			return ControlLogResult{}, err
		}
		if start > 0 {
			if index := strings.IndexByte(string(data), '\n'); index >= 0 {
				data = data[index+1:]
			}
		}
		lines := strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")
		if len(lines) > 0 && lines[len(lines)-1] == "" {
			lines = lines[:len(lines)-1]
		}
		if len(lines) > limit {
			lines = lines[len(lines)-limit:]
			truncated = true
		}
		entries := make([]ControlLogEntry, 0, len(lines))
		for index, line := range lines {
			if len(line) > 16<<10 {
				line = line[:16<<10] + "…"
				truncated = true
			}
			line = redactLogLine(line)
			entries = append(entries, ControlLogEntry{Sequence: index + 1, Level: detectLogLevel(line), Text: line})
		}
		public := s.publicLogSource(source)
		public.Available = true
		return ControlLogResult{Source: public, Entries: entries, Truncated: truncated}, nil
	})
}

func (s *StarryControlService) SetLogLevel(ctx context.Context, instanceID, sourceID, level string) (ControlLogSource, error) {
	return auditedControlCall(s, ctx, "server_control.logs.level.update", instanceID, map[string]interface{}{"source_id": sourceID, "level": level}, func() (ControlLogSource, error) {
		if s.config.ReadOnly {
			return ControlLogSource{}, starrycontrol.ErrReadOnly
		}
		source, err := s.logSource(instanceID, sourceID)
		if err != nil {
			return ControlLogSource{}, err
		}
		if source.Component != "kessoku" || s.logger == nil {
			return ControlLogSource{}, starrycontrol.ErrRequestInvalid
		}
		parsed, err := logrus.ParseLevel(strings.ToLower(level))
		if err != nil || parsed < logrus.PanicLevel || parsed > logrus.TraceLevel {
			return ControlLogSource{}, starrycontrol.ErrRequestInvalid
		}
		s.logger.SetLevel(parsed)
		result := s.publicLogSource(source)
		result.Available = true
		return result, nil
	})
}

func (s *StarryControlService) logSource(instanceID, sourceID string) (config.ControlLogSource, error) {
	if _, exists := s.instances[instanceID]; !exists {
		return config.ControlLogSource{}, starrycontrol.ErrInstanceNotFound
	}
	for _, source := range s.logSources {
		if source.ID == sourceID && (source.InstanceID == "" || source.InstanceID == "*" || source.InstanceID == instanceID) {
			return source, nil
		}
	}
	return config.ControlLogSource{}, starrycontrol.ErrRequestInvalid
}

// controlLogConfiguration makes Kessoku's own configured file logger visible
// without requiring deployments to duplicate the same path in log-sources.
// Starry, Relay, and control-agent logs remain an explicit deployment
// allowlist because they require deliberate read-only mounts into Kessoku.
func controlLogConfiguration(cfg *config.Config) (string, []config.ControlLogSource) {
	if cfg == nil {
		return "", nil
	}
	directory := cfg.ServerControl.LogDirectory
	sources := append([]config.ControlLogSource(nil), cfg.ServerControl.LogSources...)
	loggerPath := strings.TrimSpace(cfg.Logger.Path)
	if loggerPath == "" {
		return directory, sources
	}
	absoluteLoggerPath, err := filepath.Abs(loggerPath)
	if err != nil {
		return directory, sources
	}
	loggerDirectory := filepath.Dir(absoluteLoggerPath)
	if directory == "" {
		directory = loggerDirectory
	}
	absoluteDirectory, err := filepath.Abs(directory)
	if err != nil || filepath.Clean(absoluteDirectory) != filepath.Clean(loggerDirectory) {
		return directory, sources
	}
	for _, source := range sources {
		if source.ID == "kessoku" || source.Component == "kessoku" {
			return directory, sources
		}
	}
	sources = append(sources, config.ControlLogSource{
		ID: "kessoku", Label: "Kessoku", Component: "kessoku", InstanceID: "*", File: filepath.Base(absoluteLoggerPath),
	})
	return directory, sources
}

func (s *StarryControlService) publicLogSource(source config.ControlLogSource) ControlLogSource {
	result := ControlLogSource{ID: source.ID, Label: source.Label, Component: source.Component, InstanceID: source.InstanceID, LevelMutable: source.Component == "kessoku"}
	if result.LevelMutable && s.logger != nil {
		result.CurrentLevel = s.logger.GetLevel().String()
	}
	return result
}

func detectLogLevel(line string) string {
	upper := strings.ToUpper(line)
	for _, level := range []string{"PANIC", "FATAL", "ERROR", "WARN", "DEBUG", "TRACE", "INFO"} {
		if strings.Contains(upper, level) {
			return strings.ToLower(level)
		}
	}
	return "info"
}

func redactLogLine(line string) string {
	line = bearerLogValue.ReplaceAllString(line, "$1[REDACTED]")
	line = structuredLogValue.ReplaceAllString(line, "$1[REDACTED]")
	line = ipv4LogValue.ReplaceAllStringFunc(line, redactLogIPAddress)
	return ipv6LogValue.ReplaceAllStringFunc(line, redactLogIPAddress)
}

func redactLogIPAddress(value string) string {
	candidate := value
	if strings.HasPrefix(candidate, "[") && strings.HasSuffix(candidate, "]") {
		candidate = candidate[1 : len(candidate)-1]
	}
	if _, err := netip.ParseAddr(candidate); err != nil {
		return value
	}
	return "[REDACTED_IP]"
}

func openControlLog(path string) (*os.File, error) {
	fd, err := unix.Open(path, unix.O_RDONLY|unix.O_CLOEXEC|unix.O_NOFOLLOW|unix.O_NONBLOCK, 0)
	if err != nil {
		return nil, err
	}
	file := os.NewFile(uintptr(fd), path)
	if file == nil {
		_ = unix.Close(fd)
		return nil, errors.New("open log file")
	}
	info, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return nil, err
	}
	if !info.Mode().IsRegular() {
		_ = file.Close()
		return nil, errors.New("configured log source is not a regular file")
	}
	return file, nil
}

func (s *StarryControlService) provider(instanceID string, mutatesActiveConfig bool) (starrycontrol.ServerControlProvider, error) {
	if err := s.refreshManagedInstances(context.Background()); err != nil {
		registry, _ := s.registryState()
		if registry != nil {
			return nil, err
		}
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	instance, exists := s.instances[instanceID]
	if !exists {
		return nil, starrycontrol.ErrInstanceNotFound
	}
	if !instance.Enabled {
		return nil, starrycontrol.ErrUnavailable
	}
	if mutatesActiveConfig && instance.ReadOnly {
		return nil, starrycontrol.ErrReadOnly
	}
	provider, ok := s.providers[instanceID]
	if !ok {
		return nil, starrycontrol.ErrUnavailable
	}
	return provider, nil
}

func (s *StarryControlService) initializeManagedRegistry() {
	root := s.config.EffectiveRegistryDirectory()
	registry, err := servercontrolregistry.OpenExisting(root, servercontrolregistry.OpenOptions{HostIdentityFile: s.config.EffectiveHostIdentityFile()})
	if err != nil {
		s.mu.Lock()
		s.registryErr = err
		s.mu.Unlock()
		if s.logger != nil && !errors.Is(err, servercontrolregistry.ErrNotFound) {
			s.logger.Errorf("open independent server-control registry: %v", err)
		} else if s.logger != nil && s.config.Pairing.Enabled {
			s.logger.Warnf("server-control registry is not initialized at %s; an exact confirmed first pairing is required", root)
		}
		return
	}
	s.mu.Lock()
	s.registry = registry
	s.registryErr = nil
	s.mu.Unlock()
	if err := s.refreshManagedInstances(context.Background()); err != nil {
		s.mu.Lock()
		s.registryErr = err
		s.mu.Unlock()
		if s.logger != nil {
			s.logger.Errorf("load managed Starry instances: %v", err)
		}
		return
	}
	s.logManagedRegistryPreflight(context.Background())
}

// openManagedRegistryIfCreated observes a registry initialized by another
// Kessoku process (normally the Cobra CLI). It deliberately uses OpenExisting:
// a missing volume or changed path must remain unavailable until an exact
// confirmed first-pair action creates a new installation identity.
func (s *StarryControlService) openManagedRegistryIfCreated() (bool, error) {
	if s == nil || !s.config.Pairing.Enabled {
		return false, nil
	}
	s.registryInitMu.Lock()
	defer s.registryInitMu.Unlock()
	registry, _ := s.registryState()
	if registry != nil {
		return false, nil
	}
	s.mu.RLock()
	closed := s.closed
	s.mu.RUnlock()
	if closed {
		return false, errors.New("server-control service is closed")
	}
	opened, err := servercontrolregistry.OpenExisting(s.config.EffectiveRegistryDirectory(), servercontrolregistry.OpenOptions{HostIdentityFile: s.config.EffectiveHostIdentityFile()})
	if err != nil {
		s.mu.Lock()
		s.registryErr = err
		s.mu.Unlock()
		return false, err
	}
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		_ = opened.Close()
		return false, errors.New("server-control service is closed")
	}
	if s.registry != nil {
		s.mu.Unlock()
		_ = opened.Close()
		return false, nil
	}
	s.registry = opened
	s.registryErr = nil
	s.mu.Unlock()
	return true, nil
}

func (s *StarryControlService) refreshManagedInstances(ctx context.Context) error {
	registry, registryErr := s.registryState()
	reopened := false
	if registry == nil && s != nil && s.config.Pairing.Enabled {
		var err error
		reopened, err = s.openManagedRegistryIfCreated()
		if err != nil {
			return err
		}
		registry, registryErr = s.registryState()
	}
	if registry == nil {
		if s != nil && s.config.Pairing.Enabled && registryErr != nil {
			return registryErr
		}
		return nil
	}
	metadata, err := registry.Metadata(ctx)
	if err != nil {
		return err
	}
	managed, err := registry.ManagedInstances(ctx)
	if err != nil {
		return err
	}
	// Keep one v3.0.7-compatible static fragment per managed instance. This is
	// deliberately derived from the independent registry on every refresh so a
	// deleted export is repaired without changing registry generation.
	for _, item := range managed {
		if _, err := servercontrolregistry.WriteStaticExport(registry.Root(), item); err != nil {
			return fmt.Errorf("refresh static export for managed instance %q: %w", item.ManagedID, err)
		}
	}
	s.mu.RLock()
	currentGeneration := s.registryGen
	s.mu.RUnlock()
	if metadata.Generation == currentGeneration {
		if reopened {
			s.logManagedRegistryPreflight(context.Background())
		}
		return nil
	}
	s.mu.Lock()
	if metadata.Generation == s.registryGen {
		s.mu.Unlock()
		if reopened {
			s.logManagedRegistryPreflight(context.Background())
		}
		return nil
	}
	for id := range s.managedIDs {
		delete(s.instances, id)
		delete(s.providers, id)
		delete(s.providerErrs, id)
	}
	s.managedIDs = make(map[string]struct{}, len(managed))
	for _, item := range managed {
		if _, collision := s.staticIDs[item.ManagedID]; collision {
			s.providerErrs[item.ManagedID] = errors.New("managed instance id collides with a static instance")
			continue
		}
		instance := config.StarryInstance{
			ID: item.ManagedID, Name: item.Name, Enabled: item.State == "paired_read_only" || item.State == "paired_write_enabled",
			BaseURL: item.AgentOrigin, ExpectedInstanceID: item.InstanceUUID, TLSServerName: item.TLSServerName,
			CAFile: item.CAFile, ClientCertFile: item.ClientCertFile, ClientKeyFile: item.ClientKeyFile,
			ControlKeyFile: item.ControlKeyFile, ControlKeyID: item.ControlKeyID,
			ControlIssuer: item.ControlIssuer, AuthorizedParty: item.AuthorizedParty,
		}
		summary := ServerControlInstance{ID: item.ManagedID, Name: item.Name, Enabled: instance.Enabled, Managed: true, ReadOnly: item.ReadOnly}
		s.managedIDs[item.ManagedID] = struct{}{}
		if s.authManager != nil {
			fingerprint, fingerprintErr := controlauth.PrivateKeyPublicFingerprint(instance.ControlKeyFile)
			if fingerprintErr != nil || s.authManager.UsesPublicKeyFingerprint(fingerprint) {
				s.providerErrs[item.ManagedID] = errors.New("managed control keyring is invalid or not isolated")
				summary.ErrorCode = "CONTROL_KEYRING_NOT_ISOLATED"
				s.instances[item.ManagedID] = summary
				continue
			}
		}
		provider, providerErr := starryProvider.NewProvider(instance, s.config)
		if providerErr != nil {
			s.providerErrs[item.ManagedID] = providerErr
			summary.ErrorCode = "INSTANCE_CONFIG_INVALID"
		} else {
			s.providers[item.ManagedID] = provider
			summary.Available = true
		}
		s.instances[item.ManagedID] = summary
	}
	s.registryGen = metadata.Generation
	s.registryErr = nil
	s.mu.Unlock()
	if reopened {
		// This is emitted once for the externally created registry, after its
		// generation and managed providers have been loaded successfully.
		s.logManagedRegistryPreflight(context.Background())
	}
	return nil
}

func (s *StarryControlService) logManagedRegistryPreflight(ctx context.Context) {
	if s == nil || s.logger == nil {
		return
	}
	registry, _ := s.registryState()
	if registry == nil {
		return
	}
	metadata, err := registry.Metadata(ctx)
	if err != nil {
		s.logger.Errorf("read independent server-control registry preflight: %v", err)
		return
	}
	s.logger.WithFields(logrus.Fields{
		"registry_root":       metadata.Root,
		"registry_state":      filepath.Join(metadata.Root, "registry-v1.sqlite"),
		"registry_schema":     metadata.SchemaVersion,
		"registry_generation": metadata.Generation,
		"installation_id":     metadata.InstallationID,
		"host_fingerprint":    metadata.HostFingerprint,
	}).Info("independent server-control registry preflight passed")
	managed, err := registry.ManagedInstances(ctx)
	if err != nil {
		s.logger.Errorf("list managed identities for registry preflight: %v", err)
		return
	}
	for _, item := range managed {
		s.logger.WithFields(logrus.Fields{
			"managed_id":                item.ManagedID,
			"instance_id":               item.InstanceUUID,
			"client_certificate_sha256": item.CertificateSHA256,
			"control_key_sha256":        item.ControlKeySHA256,
			"read_only":                 item.ReadOnly,
			"state":                     item.State,
		}).Info("managed Starry identity loaded")
	}
}

func (s *StarryControlService) registryState() (*servercontrolregistry.Store, error) {
	if s == nil {
		return nil, nil
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.registry, s.registryErr
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
		updates := map[string]interface{}{"result": result, "error_code": errorCode}
		if operation.ActivationAck != nil {
			metadata, err := controlAuditMetadataWithAck(event.Metadata, operation.ActivationAck)
			if err != nil {
				return err
			}
			updates["metadata"] = metadata
		}
		return tx.Model(&model.AdminAuditEvent{}).Where("id = ?", event.Id).Updates(updates).Error
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
	return DB.Transaction(func(tx *gorm.DB) error {
		for _, expectation := range expectations {
			updates := map[string]interface{}{"result": result, "error_code": errorCode}
			if operation.ActivationAck != nil {
				event := model.AdminAuditEvent{}
				if err := tx.Select("id", "metadata").First(&event, expectation.AuditEventID).Error; err != nil {
					return err
				}
				metadata, err := controlAuditMetadataWithAck(event.Metadata, operation.ActivationAck)
				if err != nil {
					return err
				}
				updates["metadata"] = metadata
			}
			if err := tx.Model(&model.AdminAuditEvent{}).Where("id = ?", expectation.AuditEventID).Updates(updates).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func controlAuditMetadataWithAck(raw custom_types.AutoJson, ack *starrycontrol.ActivationAck) (custom_types.AutoJson, error) {
	metadata := map[string]interface{}{}
	if len(raw) != 0 {
		if err := json.Unmarshal(raw, &metadata); err != nil {
			return nil, err
		}
	}
	acks := make([]map[string]interface{}, 0, len(ack.SubsystemAcks))
	for _, item := range ack.SubsystemAcks {
		acks = append(acks, map[string]interface{}{"subsystem": item.Subsystem, "accepted": item.Accepted})
	}
	metadata["activation_ack"] = map[string]interface{}{
		"generation":       ack.Generation,
		"schema_version":   ack.SchemaVersion,
		"source_digest":    ack.SourceDigest,
		"effective_digest": ack.EffectiveDigest,
		"subsystem_acks":   acks,
	}
	encoded, err := json.Marshal(metadata)
	return custom_types.AutoJson(encoded), err
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

type planReviewClaims struct {
	InstanceID      string `json:"instance_id"`
	ActorUserID     uint   `json:"actor_user_id"`
	PlanID          string `json:"plan_id"`
	CandidateDigest string `json:"candidate_digest"`
	BaseETag        string `json:"base_etag"`
	BaseGeneration  uint64 `json:"base_generation"`
	SchemaDigest    string `json:"schema_digest"`
	ChangeCount     int    `json:"change_count"`
	Risk            string `json:"risk"`
	ExpiresAtUnix   int64  `json:"expires_at_unix"`
}

func (s *StarryControlService) sealPlanReview(ctx context.Context, instanceID string, plan starrycontrol.ConfigPlan, schemaDigest string) (string, error) {
	request, ok := starrycontrol.MetadataFromContext(ctx)
	if s == nil || !ok || allZeroBytes(s.planReviewKey[:]) || plan.PlanID == "" || plan.CandidateDigest == "" || plan.BaseETag == "" || plan.ExpiresAt.IsZero() || !validControlDigest(schemaDigest) {
		return "", errors.New("server-control plan review signing is unavailable")
	}
	claims := planReviewClaims{
		InstanceID: instanceID, ActorUserID: request.ActorUserID, PlanID: plan.PlanID,
		CandidateDigest: plan.CandidateDigest, BaseETag: plan.BaseETag, BaseGeneration: plan.BaseGeneration,
		SchemaDigest: schemaDigest, ChangeCount: len(plan.Changes), Risk: plan.Impact.Risk,
		ExpiresAtUnix: plan.ExpiresAt.Unix(),
	}
	payload, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}
	mac := hmac.New(sha256.New, s.planReviewKey[:])
	_, _ = mac.Write(payload)
	return base64.RawURLEncoding.EncodeToString(payload) + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil)), nil
}

func (s *StarryControlService) verifyPlanReview(ctx context.Context, instanceID string, input starrycontrol.ApplyRequest) (planReviewClaims, error) {
	request, hasRequest := starrycontrol.MetadataFromContext(ctx)
	if s == nil || input.ReviewToken == "" || len(input.ReviewToken) > 2048 || allZeroBytes(s.planReviewKey[:]) {
		return planReviewClaims{}, planReviewError("PLAN_REVIEW_REQUIRED", "a current Kessoku plan review token is required")
	}
	parts := strings.Split(input.ReviewToken, ".")
	if len(parts) != 2 {
		return planReviewClaims{}, planReviewError("PLAN_REVIEW_INVALID", "the Kessoku plan review token is invalid")
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return planReviewClaims{}, planReviewError("PLAN_REVIEW_INVALID", "the Kessoku plan review token is invalid")
	}
	signature, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return planReviewClaims{}, planReviewError("PLAN_REVIEW_INVALID", "the Kessoku plan review token is invalid")
	}
	mac := hmac.New(sha256.New, s.planReviewKey[:])
	_, _ = mac.Write(payload)
	if !hmac.Equal(signature, mac.Sum(nil)) {
		return planReviewClaims{}, planReviewError("PLAN_REVIEW_INVALID", "the Kessoku plan review token is invalid")
	}
	claims := planReviewClaims{}
	if json.Unmarshal(payload, &claims) != nil || !hasRequest || claims.ActorUserID != request.ActorUserID || claims.InstanceID != instanceID || claims.PlanID != input.PlanID ||
		claims.CandidateDigest != input.CandidateDigest || claims.BaseETag != input.IfMatch || claims.ExpiresAtUnix < time.Now().Unix() ||
		claims.ExpiresAtUnix > time.Now().Add(11*time.Minute).Unix() || !validControlDigest(claims.SchemaDigest) || claims.ChangeCount < 0 ||
		claims.Risk != "low" && claims.Risk != "medium" && claims.Risk != "high" && claims.Risk != "critical" {
		return planReviewClaims{}, planReviewError("PLAN_REVIEW_INVALID", "the Kessoku plan review token does not match this exact plan")
	}
	if claims.Risk == "high" || claims.Risk == "critical" {
		expected := "confirm:" + claims.PlanID + ":" + claims.CandidateDigest
		if input.RiskConfirmation != expected {
			return planReviewClaims{}, planReviewError("HIGH_RISK_CONFIRMATION_REQUIRED", "high-risk plans require a second exact confirmation")
		}
	}
	return claims, nil
}

func validControlDigest(value string) bool {
	if len(value) != 71 || !strings.HasPrefix(value, "sha256:") {
		return false
	}
	for _, character := range value[7:] {
		if character < '0' || character > '9' && character < 'a' || character > 'f' {
			return false
		}
	}
	return true
}

func planReviewError(code, message string) error {
	return &starrycontrol.ProviderError{Status: 428, Code: code, Message: message}
}

func allZeroBytes(value []byte) bool {
	for _, item := range value {
		if item != 0 {
			return false
		}
	}
	return true
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
