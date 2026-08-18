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
	"github.com/q1ngyang/rustdesk-api-kessoku/v2/internal/starrycontrol"
	"github.com/q1ngyang/rustdesk-api-kessoku/v2/service"
)

type StarryControl struct{}

func (s *StarryControl) Instances(c *gin.Context) {
	successControl(c, service.AllService.StarryControlService.Instances())
}

func (s *StarryControl) Capabilities(c *gin.Context) {
	result, err := service.AllService.StarryControlService.Capabilities(controlContext(c), c.Param("id"))
	writeControlResult(c, result, err)
}

func (s *StarryControl) Health(c *gin.Context) {
	result, err := service.AllService.StarryControlService.Health(controlContext(c), c.Param("id"))
	writeControlResult(c, result, err)
}

func (s *StarryControl) Relays(c *gin.Context) {
	result, err := service.AllService.StarryControlService.Relays(controlContext(c), c.Param("id"))
	writeControlResult(c, result, err)
}

func (s *StarryControl) SimulateAllocation(c *gin.Context) {
	input := starrycontrol.SimulationInput{}
	if err := decodeControlJSON(c, &input); err != nil {
		controlError(c, starrycontrol.ErrRequestInvalid)
		return
	}
	result, err := service.AllService.StarryControlService.SimulateAllocation(controlContext(c), c.Param("id"), input)
	writeControlResult(c, result, err)
}

func (s *StarryControl) GetConfig(c *gin.Context) {
	result, err := service.AllService.StarryControlService.GetConfig(controlContext(c), c.Param("id"))
	if err == nil && result.ETag != "" {
		c.Header("ETag", result.ETag)
	}
	writeControlResult(c, result, err)
}

func (s *StarryControl) GetConfigSchema(c *gin.Context) {
	result, err := service.AllService.StarryControlService.GetConfigSchema(controlContext(c), c.Param("id"))
	if err == nil && result.ETag != "" {
		c.Header("ETag", result.ETag)
	}
	writeControlResult(c, result, err)
}

func (s *StarryControl) ValidateConfig(c *gin.Context) {
	input, ok := configCandidate(c)
	if !ok {
		return
	}
	result, err := service.AllService.StarryControlService.ValidateConfig(controlContext(c), c.Param("id"), input)
	writeControlResult(c, result, err)
}

func (s *StarryControl) PlanConfig(c *gin.Context) {
	input, ok := configCandidate(c)
	if !ok {
		return
	}
	result, err := service.AllService.StarryControlService.PlanConfig(controlContext(c), c.Param("id"), input)
	writeControlResult(c, result, err)
}

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
		controlProblem(c, http.StatusBadRequest, "IDEMPOTENCY_KEY_REQUIRED", "Idempotency-Key is required", false)
		return
	}
	result, err := service.AllService.StarryControlService.ApplyConfig(controlContext(c), c.Param("id"), input)
	writeControlResult(c, result, err)
}

func (s *StarryControl) Operation(c *gin.Context) {
	result, err := service.AllService.StarryControlService.Operation(controlContext(c), c.Param("id"), c.Param("operation_id"))
	writeControlResult(c, result, err)
}

func (s *StarryControl) ConfigHistory(c *gin.Context) {
	result, err := service.AllService.StarryControlService.ConfigHistory(controlContext(c), c.Param("id"))
	writeControlResult(c, result, err)
}

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
		controlProblem(c, http.StatusBadRequest, "IDEMPOTENCY_KEY_REQUIRED", "Idempotency-Key is required", false)
		return
	}
	result, err := service.AllService.StarryControlService.RollbackConfig(controlContext(c), c.Param("id"), input)
	writeControlResult(c, result, err)
}

func (s *StarryControl) AuditEvents(c *gin.Context) {
	page := uintQuery(c.Query("page"), 1, 1_000_000)
	pageSize := uintQuery(c.Query("page_size"), 50, 100)
	successControl(c, service.AllService.StarryControlService.AuditEvents(page, pageSize))
}

func configCandidate(c *gin.Context) (starrycontrol.ConfigCandidate, bool) {
	input := starrycontrol.ConfigCandidate{}
	if err := decodeControlJSON(c, &input); err != nil {
		controlError(c, starrycontrol.ErrRequestInvalid)
		return input, false
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

func successControl(c *gin.Context, result interface{}) {
	controlRequestID(c)
	c.JSON(http.StatusOK, gin.H{"data": result})
}

func controlError(c *gin.Context, err error) {
	var providerError *starrycontrol.ProviderError
	if errors.As(err, &providerError) {
		status := providerError.Status
		if status < 400 || status > 599 {
			status = http.StatusBadGateway
		}
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
