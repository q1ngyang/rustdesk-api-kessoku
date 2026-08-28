package admin

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/q1ngyang/rustdesk-api-kessoku/v3/internal/starrycontrol"
	"github.com/q1ngyang/rustdesk-api-kessoku/v3/model"
	"github.com/q1ngyang/rustdesk-api-kessoku/v3/service"
)

type StarryControl struct{}

type ControlAPIResponse struct {
	Data interface{} `json:"data"`
}

type ControlAPIError struct {
	Code      string                 `json:"code"`
	Message   string                 `json:"message"`
	RequestID string                 `json:"request_id"`
	Retryable bool                   `json:"retryable"`
	Details   map[string]interface{} `json:"details"`
}

type ControlAPIProblem struct {
	Error ControlAPIError `json:"error"`
}

type ControlAuditEventList = model.AdminAuditEventList

// Instances lists deployment-owned Starry instances without credential paths.
// @Tags Server Control
// @Summary List configured Starry instances
// @Produce json
// @Success 200 {object} ControlAPIResponse{data=[]service.ServerControlInstance}
// @Failure 401 {object} ControlAPIProblem
// @Failure 403 {object} ControlAPIProblem
// @Router /admin/server-control/v1/instances [get]
// @Security token
func (s *StarryControl) Instances(c *gin.Context) {
	result, err := service.AllService.StarryControlService.InstancesContext(controlContext(c))
	writeControlResult(c, result, err)
}

// Capabilities negotiates the fixed Starry Control contract and instance identity.
// @Tags Server Control
// @Summary Read Starry capabilities
// @Produce json
// @Param id path string true "Deployment instance ID"
// @Success 200 {object} ControlAPIResponse{data=starrycontrol.Capabilities}
// @Failure 401 {object} ControlAPIProblem
// @Failure 403 {object} ControlAPIProblem
// @Failure 404 {object} ControlAPIProblem
// @Failure 502 {object} ControlAPIProblem
// @Failure 503 {object} ControlAPIProblem
// @Router /admin/server-control/v1/instances/{id}/capabilities [get]
// @Security token
func (s *StarryControl) Capabilities(c *gin.Context) {
	result, err := service.AllService.StarryControlService.Capabilities(controlContext(c), c.Param("id"))
	writeControlResult(c, result, err)
}

// Status returns the typed HBBS configuration and authentication state.
// @Tags Server Control
// @Summary Read Starry status
// @Produce json
// @Param id path string true "Deployment instance ID"
// @Success 200 {object} ControlAPIResponse{data=starrycontrol.Status}
// @Failure 401 {object} ControlAPIProblem
// @Failure 403 {object} ControlAPIProblem
// @Failure 404 {object} ControlAPIProblem
// @Failure 502 {object} ControlAPIProblem
// @Failure 503 {object} ControlAPIProblem
// @Router /admin/server-control/v1/instances/{id}/status [get]
// @Security token
func (s *StarryControl) Status(c *gin.Context) {
	result, err := service.AllService.StarryControlService.Status(controlContext(c), c.Param("id"))
	writeControlResult(c, result, err)
}

// Relays returns configured, native, WSS, and eligibility state separately.
// @Tags Server Control
// @Summary List Starry Relays
// @Produce json
// @Param id path string true "Deployment instance ID"
// @Success 200 {object} ControlAPIResponse{data=starrycontrol.RelayInventory}
// @Failure 401 {object} ControlAPIProblem
// @Failure 403 {object} ControlAPIProblem
// @Failure 404 {object} ControlAPIProblem
// @Failure 502 {object} ControlAPIProblem
// @Failure 503 {object} ControlAPIProblem
// @Router /admin/server-control/v1/instances/{id}/relays [get]
// @Security token
func (s *StarryControl) Relays(c *gin.Context) {
	result, err := service.AllService.StarryControlService.Relays(controlContext(c), c.Param("id"))
	writeControlResult(c, result, err)
}

// SimulateAllocation asks Starry to authoritatively evaluate two IPs and a transport.
// @Tags Server Control
// @Summary Simulate Relay allocation
// @Accept json
// @Produce json
// @Param id path string true "Deployment instance ID"
// @Param body body starrycontrol.SimulationInput true "Typed simulation input"
// @Success 200 {object} ControlAPIResponse{data=starrycontrol.SimulationResult}
// @Failure 400 {object} ControlAPIProblem
// @Failure 401 {object} ControlAPIProblem
// @Failure 403 {object} ControlAPIProblem
// @Failure 404 {object} ControlAPIProblem
// @Failure 502 {object} ControlAPIProblem
// @Failure 503 {object} ControlAPIProblem
// @Router /admin/server-control/v1/instances/{id}/allocation-simulations [post]
// @Security token
func (s *StarryControl) SimulateAllocation(c *gin.Context) {
	input := starrycontrol.SimulationInput{}
	if err := decodeControlJSON(c, &input); err != nil {
		controlError(c, starrycontrol.ErrRequestInvalid)
		return
	}
	result, err := service.AllService.StarryControlService.SimulateAllocation(controlContext(c), c.Param("id"), input)
	writeControlResult(c, result, err)
}

// GetConfig reads the exact managed YAML document plus active runtime metadata.
// @Tags Server Control
// @Summary Read active Starry configuration
// @Produce json
// @Param id path string true "Deployment instance ID"
// @Success 200 {object} ControlAPIResponse{data=starrycontrol.ConfigDocument}
// @Failure 401 {object} ControlAPIProblem
// @Failure 403 {object} ControlAPIProblem
// @Failure 404 {object} ControlAPIProblem
// @Failure 502 {object} ControlAPIProblem
// @Failure 503 {object} ControlAPIProblem
// @Router /admin/server-control/v1/instances/{id}/config [get]
// @Security token
func (s *StarryControl) GetConfig(c *gin.Context) {
	result, err := service.AllService.StarryControlService.GetConfig(controlContext(c), c.Param("id"))
	if err == nil && result.ETag != "" {
		c.Header("ETag", result.ETag)
	}
	writeControlResult(c, result, err)
}

// GetConfigSchema reads the Agent-owned JSON/UI schema bundle.
// @Tags Server Control
// @Summary Read Starry configuration schema
// @Produce json
// @Param id path string true "Deployment instance ID"
// @Success 200 {object} ControlAPIResponse{data=starrycontrol.SchemaBundle}
// @Failure 401 {object} ControlAPIProblem
// @Failure 403 {object} ControlAPIProblem
// @Failure 404 {object} ControlAPIProblem
// @Failure 502 {object} ControlAPIProblem
// @Failure 503 {object} ControlAPIProblem
// @Router /admin/server-control/v1/instances/{id}/config/schema [get]
// @Security token
func (s *StarryControl) GetConfigSchema(c *gin.Context) {
	result, err := service.AllService.StarryControlService.GetConfigSchema(controlContext(c), c.Param("id"))
	if err == nil && result.ETag != "" {
		c.Header("ETag", result.ETag)
	}
	writeControlResult(c, result, err)
}

// ValidateConfig performs Agent-authoritative validation without applying a change.
// @Tags Server Control
// @Summary Validate a Starry configuration candidate
// @Accept json
// @Produce json
// @Param id path string true "Deployment instance ID"
// @Param body body starrycontrol.ConfigCandidate true "UTF-8 YAML document"
// @Success 200 {object} ControlAPIResponse{data=starrycontrol.ValidationResult}
// @Failure 400 {object} ControlAPIProblem
// @Failure 401 {object} ControlAPIProblem
// @Failure 403 {object} ControlAPIProblem
// @Failure 404 {object} ControlAPIProblem
// @Failure 502 {object} ControlAPIProblem
// @Failure 503 {object} ControlAPIProblem
// @Router /admin/server-control/v1/instances/{id}/config/validate [post]
// @Security token
func (s *StarryControl) ValidateConfig(c *gin.Context) {
	input, ok := configCandidate(c, false)
	if !ok {
		return
	}
	result, err := service.AllService.StarryControlService.ValidateConfig(controlContext(c), c.Param("id"), input)
	writeControlResult(c, result, err)
}

// PlanConfig creates an expiring Agent plan bound to actor, instance, ETag, and digest.
// @Tags Server Control
// @Summary Plan a Starry configuration change
// @Accept json
// @Produce json
// @Param id path string true "Deployment instance ID"
// @Param If-Match header string true "Current configuration ETag"
// @Param body body starrycontrol.ConfigCandidate true "UTF-8 YAML document"
// @Success 200 {object} ControlAPIResponse{data=starrycontrol.ConfigPlan}
// @Failure 400 {object} ControlAPIProblem
// @Failure 401 {object} ControlAPIProblem
// @Failure 403 {object} ControlAPIProblem
// @Failure 404 {object} ControlAPIProblem
// @Failure 412 {object} ControlAPIProblem
// @Failure 428 {object} ControlAPIProblem
// @Failure 502 {object} ControlAPIProblem
// @Failure 503 {object} ControlAPIProblem
// @Router /admin/server-control/v1/instances/{id}/config/plan [post]
// @Security token
func (s *StarryControl) PlanConfig(c *gin.Context) {
	input, ok := configCandidate(c, true)
	if !ok {
		return
	}
	result, err := service.AllService.StarryControlService.PlanConfig(controlContext(c), c.Param("id"), input)
	writeControlResult(c, result, err)
}

// ApplyConfig applies only a previously reviewed plan with concurrency/idempotency guards.
// @Tags Server Control
// @Summary Apply a planned Starry configuration
// @Accept json
// @Produce json
// @Param id path string true "Deployment instance ID"
// @Param If-Match header string true "Current configuration ETag"
// @Param Idempotency-Key header string true "Unique apply key"
// @Param body body starrycontrol.ApplyRequest true "Planned apply request"
// @Success 202 {object} ControlAPIResponse{data=starrycontrol.Operation}
// @Failure 400 {object} ControlAPIProblem
// @Failure 401 {object} ControlAPIProblem
// @Failure 403 {object} ControlAPIProblem
// @Failure 404 {object} ControlAPIProblem
// @Failure 412 {object} ControlAPIProblem
// @Failure 428 {object} ControlAPIProblem
// @Failure 502 {object} ControlAPIProblem
// @Failure 503 {object} ControlAPIProblem
// @Router /admin/server-control/v1/instances/{id}/config/apply [post]
// @Security token
func (s *StarryControl) ApplyConfig(c *gin.Context) {
	input := starrycontrol.ApplyRequest{}
	if err := decodeControlJSON(c, &input); err != nil {
		controlError(c, starrycontrol.ErrRequestInvalid)
		return
	}
	input.IfMatch = c.GetHeader("If-Match")
	input.IdempotencyKey = c.GetHeader("Idempotency-Key")
	if input.IfMatch == "" {
		controlProblem(c, http.StatusPreconditionRequired, "PRECONDITION_REQUIRED", "If-Match is required", false)
		return
	}
	if input.IdempotencyKey == "" {
		controlProblem(c, http.StatusPreconditionRequired, "PRECONDITION_REQUIRED", "Idempotency-Key is required", false)
		return
	}
	result, err := service.AllService.StarryControlService.ApplyConfig(controlContext(c), c.Param("id"), input)
	writeControlAccepted(c, result, err)
}

// Operation reads one typed asynchronous apply/rollback operation.
// @Tags Server Control
// @Summary Read a Starry configuration operation
// @Produce json
// @Param id path string true "Deployment instance ID"
// @Param operation_id path string true "Operation UUID"
// @Success 200 {object} ControlAPIResponse{data=starrycontrol.Operation}
// @Failure 400 {object} ControlAPIProblem
// @Failure 401 {object} ControlAPIProblem
// @Failure 403 {object} ControlAPIProblem
// @Failure 404 {object} ControlAPIProblem
// @Failure 502 {object} ControlAPIProblem
// @Failure 503 {object} ControlAPIProblem
// @Router /admin/server-control/v1/instances/{id}/operations/{operation_id} [get]
// @Security token
func (s *StarryControl) Operation(c *gin.Context) {
	result, err := service.AllService.StarryControlService.Operation(controlContext(c), c.Param("id"), c.Param("operation_id"))
	writeControlResult(c, result, err)
}

// ConfigHistory lists typed configuration revisions without secret material.
// @Tags Server Control
// @Summary List Starry configuration history
// @Produce json
// @Param id path string true "Deployment instance ID"
// @Success 200 {object} ControlAPIResponse{data=[]starrycontrol.ConfigRevision}
// @Failure 401 {object} ControlAPIProblem
// @Failure 403 {object} ControlAPIProblem
// @Failure 404 {object} ControlAPIProblem
// @Failure 502 {object} ControlAPIProblem
// @Failure 503 {object} ControlAPIProblem
// @Router /admin/server-control/v1/instances/{id}/config/history [get]
// @Security token
func (s *StarryControl) ConfigHistory(c *gin.Context) {
	result, err := service.AllService.StarryControlService.ConfigHistory(controlContext(c), c.Param("id"))
	writeControlResult(c, result, err)
}

// RollbackConfig requests a guarded rollback to a known revision.
// @Tags Server Control
// @Summary Roll back Starry configuration
// @Accept json
// @Produce json
// @Param id path string true "Deployment instance ID"
// @Param If-Match header string true "Current configuration ETag"
// @Param Idempotency-Key header string true "Unique rollback key"
// @Param body body starrycontrol.RollbackRequest true "Rollback revision"
// @Success 202 {object} ControlAPIResponse{data=starrycontrol.Operation}
// @Failure 400 {object} ControlAPIProblem
// @Failure 401 {object} ControlAPIProblem
// @Failure 403 {object} ControlAPIProblem
// @Failure 404 {object} ControlAPIProblem
// @Failure 412 {object} ControlAPIProblem
// @Failure 428 {object} ControlAPIProblem
// @Failure 502 {object} ControlAPIProblem
// @Failure 503 {object} ControlAPIProblem
// @Router /admin/server-control/v1/instances/{id}/config/rollback [post]
// @Security token
func (s *StarryControl) RollbackConfig(c *gin.Context) {
	input := starrycontrol.RollbackRequest{}
	if err := decodeControlJSON(c, &input); err != nil {
		controlError(c, starrycontrol.ErrRequestInvalid)
		return
	}
	input.IfMatch = c.GetHeader("If-Match")
	input.IdempotencyKey = c.GetHeader("Idempotency-Key")
	if input.IfMatch == "" {
		controlProblem(c, http.StatusPreconditionRequired, "PRECONDITION_REQUIRED", "If-Match is required", false)
		return
	}
	if input.IdempotencyKey == "" {
		controlProblem(c, http.StatusPreconditionRequired, "PRECONDITION_REQUIRED", "Idempotency-Key is required", false)
		return
	}
	result, err := service.AllService.StarryControlService.RollbackConfig(controlContext(c), c.Param("id"), input)
	writeControlAccepted(c, result, err)
}

// ReloadRuntime requests an audited synchronous reload bound to the expected disk digest.
// @Tags Server Control
// @Summary Reload the Starry runtime
// @Accept json
// @Produce json
// @Param id path string true "Deployment instance ID"
// @Param Idempotency-Key header string true "Unique reload key"
// @Param body body starrycontrol.RuntimeReloadRequest true "Expected source digest"
// @Success 200 {object} ControlAPIResponse{data=starrycontrol.ActivationAck}
// @Failure 400 {object} ControlAPIProblem
// @Failure 401 {object} ControlAPIProblem
// @Failure 403 {object} ControlAPIProblem
// @Failure 404 {object} ControlAPIProblem
// @Failure 428 {object} ControlAPIProblem
// @Failure 502 {object} ControlAPIProblem
// @Failure 503 {object} ControlAPIProblem
// @Router /admin/server-control/v1/instances/{id}/runtime/reload [post]
// @Security token
func (s *StarryControl) ReloadRuntime(c *gin.Context) {
	input := starrycontrol.RuntimeReloadRequest{}
	if err := decodeControlJSON(c, &input); err != nil {
		controlError(c, starrycontrol.ErrRequestInvalid)
		return
	}
	input.IdempotencyKey = c.GetHeader("Idempotency-Key")
	if input.IdempotencyKey == "" {
		controlProblem(c, http.StatusPreconditionRequired, "PRECONDITION_REQUIRED", "Idempotency-Key is required", false)
		return
	}
	result, err := service.AllService.StarryControlService.ReloadRuntime(controlContext(c), c.Param("id"), input)
	writeControlResult(c, result, err)
}

// AuditEvents lists redacted Kessoku control-plane audit records.
// @Tags Server Control
// @Summary List server-control audit events
// @Produce json
// @Param page query int false "Page number"
// @Param page_size query int false "Page size (maximum 100)"
// @Success 200 {object} ControlAPIResponse{data=ControlAuditEventList}
// @Failure 401 {object} ControlAPIProblem
// @Failure 403 {object} ControlAPIProblem
// @Failure 503 {object} ControlAPIProblem
// @Router /admin/server-control/v1/audit-events [get]
// @Security token
func (s *StarryControl) AuditEvents(c *gin.Context) {
	page := uintQuery(c.Query("page"), 1, 1_000_000)
	pageSize := uintQuery(c.Query("page_size"), 50, 100)
	result, err := service.AllService.StarryControlService.AuditEvents(controlContext(c), page, pageSize)
	writeControlResult(c, result, err)
}

func (s *StarryControl) LogSources(c *gin.Context) {
	result, err := service.AllService.StarryControlService.LogSources(controlContext(c), c.Param("id"))
	writeControlResult(c, result, err)
}

func (s *StarryControl) Logs(c *gin.Context) {
	limit := int(uintQuery(c.Query("limit"), 400, 2000))
	result, err := service.AllService.StarryControlService.Logs(controlContext(c), c.Param("id"), c.Query("source_id"), limit)
	writeControlResult(c, result, err)
}

func (s *StarryControl) SetLogLevel(c *gin.Context) {
	input := struct {
		SourceID string `json:"source_id"`
		Level    string `json:"level"`
	}{}
	if err := decodeControlJSON(c, &input); err != nil || input.SourceID == "" || input.Level == "" {
		controlError(c, starrycontrol.ErrRequestInvalid)
		return
	}
	result, err := service.AllService.StarryControlService.SetLogLevel(controlContext(c), c.Param("id"), input.SourceID, input.Level)
	writeControlResult(c, result, err)
}

func configCandidate(c *gin.Context, requireETag bool) (starrycontrol.ConfigCandidate, bool) {
	input := starrycontrol.ConfigCandidate{}
	if err := decodeControlJSON(c, &input); err != nil {
		controlError(c, starrycontrol.ErrRequestInvalid)
		return input, false
	}
	if !requireETag {
		return input, true
	}
	input.BaseETag = c.GetHeader("If-Match")
	if input.BaseETag == "" {
		controlProblem(c, http.StatusPreconditionRequired, "PRECONDITION_REQUIRED", "If-Match is required", false)
		return input, false
	}
	return input, true
}

func decodeControlJSON(c *gin.Context, destination interface{}) error {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 1<<20)
	decoder := json.NewDecoder(c.Request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return err
	}
	var trailing interface{}
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("multiple JSON values are not allowed")
		}
		return err
	}
	return nil
}

func controlContext(c *gin.Context) context.Context {
	user := service.AllService.UserService.CurUser(c)
	actorID := uint(0)
	if user != nil {
		actorID = user.Id
	}
	return starrycontrol.WithRequestMetadata(c.Request.Context(), starrycontrol.RequestMetadata{
		ActorUserID: actorID,
		RequestID:   controlRequestID(c),
	})
}

func controlRequestID(c *gin.Context) string {
	if existing, ok := c.Get("serverControlRequestID"); ok {
		return existing.(string)
	}
	requestID := strings.TrimSpace(c.GetHeader("X-Request-ID"))
	if _, err := uuid.Parse(requestID); err != nil {
		generated, generationErr := uuid.NewV7()
		if generationErr != nil {
			generated = uuid.New()
		}
		requestID = generated.String()
	}
	c.Set("serverControlRequestID", requestID)
	c.Header("X-Request-ID", requestID)
	return requestID
}

func writeControlResult(c *gin.Context, result interface{}, err error) {
	if err != nil {
		controlError(c, err)
		return
	}
	successControl(c, result)
}

func writeControlAccepted(c *gin.Context, result interface{}, err error) {
	if err != nil {
		controlError(c, err)
		return
	}
	controlRequestID(c)
	c.JSON(http.StatusAccepted, gin.H{"data": result})
}

func successControl(c *gin.Context, result interface{}) {
	controlRequestID(c)
	c.JSON(http.StatusOK, gin.H{"data": result})
}

func controlError(c *gin.Context, err error) {
	var providerError *starrycontrol.ProviderError
	if errors.As(err, &providerError) {
		status := publicControlStatus(providerError.Status)
		code := providerError.Code
		if code == "" {
			code = "STARRY_CONTROL_ERROR"
		}
		controlProblem(c, status, code, "Starry control request failed", providerError.Retryable)
		return
	}
	switch {
	case errors.Is(err, starrycontrol.ErrInstanceNotFound):
		controlProblem(c, http.StatusNotFound, "INSTANCE_NOT_FOUND", "server-control instance not found", false)
	case errors.Is(err, starrycontrol.ErrReadOnly):
		controlProblem(c, http.StatusForbidden, "CONTROL_READ_ONLY", "server-control writes are disabled", false)
	case errors.Is(err, starrycontrol.ErrRequestInvalid):
		controlProblem(c, http.StatusBadRequest, "REQUEST_INVALID", "request is invalid", false)
	default:
		controlProblem(c, http.StatusServiceUnavailable, "STARRY_CONTROL_UNAVAILABLE", "Starry control agent is unavailable", true)
	}
}

// publicControlStatus prevents Agent authentication/configuration failures
// from masquerading as the browser user's authentication state. Only statuses
// with useful, safe management-API semantics cross the provider boundary.
func publicControlStatus(status int) int {
	switch status {
	case http.StatusNotFound,
		http.StatusConflict,
		http.StatusPreconditionFailed,
		http.StatusUnprocessableEntity,
		http.StatusTooManyRequests,
		http.StatusBadGateway,
		http.StatusServiceUnavailable,
		http.StatusGatewayTimeout:
		return status
	default:
		return http.StatusBadGateway
	}
}

func controlProblem(c *gin.Context, status int, code, message string, retryable bool) {
	c.JSON(status, gin.H{"error": gin.H{
		"code":       code,
		"message":    message,
		"request_id": controlRequestID(c),
		"retryable":  retryable,
		"details":    gin.H{},
	}})
}

func uintQuery(value string, fallback, maximum uint) uint {
	parsed, err := strconv.ParseUint(value, 10, 32)
	if err != nil || parsed == 0 {
		return fallback
	}
	if uint(parsed) > maximum {
		return maximum
	}
	return uint(parsed)
}
