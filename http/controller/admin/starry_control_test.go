package admin

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/q1ngyang/rustdesk-api-kessoku/v2/internal/starrycontrol"
)

func TestControlErrorDoesNotExposeAgentAuthenticationStatus(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, test := range []struct {
		name       string
		agent      int
		wantPublic int
	}{
		{name: "agent unauthorized", agent: http.StatusUnauthorized, wantPublic: http.StatusBadGateway},
		{name: "agent forbidden", agent: http.StatusForbidden, wantPublic: http.StatusBadGateway},
		{name: "agent internal error", agent: http.StatusInternalServerError, wantPublic: http.StatusBadGateway},
		{name: "etag conflict", agent: http.StatusPreconditionFailed, wantPublic: http.StatusPreconditionFailed},
		{name: "rate limited", agent: http.StatusTooManyRequests, wantPublic: http.StatusTooManyRequests},
	} {
		t.Run(test.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			context, _ := gin.CreateTestContext(recorder)
			context.Request = httptest.NewRequest(http.MethodGet, "/", nil)
			controlError(context, &starrycontrol.ProviderError{Status: test.agent, Code: "STARRY_TEST_ERROR"})
			if recorder.Code != test.wantPublic {
				t.Fatalf("status = %d, want %d", recorder.Code, test.wantPublic)
			}
			if strings.Contains(recorder.Body.String(), "certificate") || strings.Contains(recorder.Body.String(), "https://") {
				t.Fatalf("response leaked provider details: %s", recorder.Body.String())
			}
		})
	}
}

func TestControlErrorMapsLocalAvailabilityFailure(t *testing.T) {
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	controlError(context, errors.New("private path: /run/secrets/control.pem"))
	if recorder.Code != http.StatusServiceUnavailable || strings.Contains(recorder.Body.String(), "/run/secrets") {
		t.Fatalf("unsafe availability response: status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}
