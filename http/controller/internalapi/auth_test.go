package internalapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/q1ngyang/rustdesk-api-kessoku/v3/config"
	"github.com/q1ngyang/rustdesk-api-kessoku/v3/global"
)

func TestIntrospectionRequiresStrictBoundedJSON(t *testing.T) {
	oldConfig := global.Config
	t.Cleanup(func() { global.Config = oldConfig })
	global.Config.Auth.Internal = config.InternalAuth{MaxBodyBytes: 32}
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.POST("/introspect", (&Auth{}).Introspect)

	tests := []struct {
		name        string
		contentType string
		body        string
		wantStatus  int
		wantCode    string
	}{
		{name: "missing content type", body: `{}`, wantStatus: http.StatusUnsupportedMediaType, wantCode: "CONTENT_TYPE_UNSUPPORTED"},
		{name: "unknown field", contentType: "application/json", body: `{"token":"","extra":true}`, wantStatus: http.StatusBadRequest, wantCode: "REQUEST_INVALID"},
		{name: "multiple values", contentType: "application/json", body: `{"token":""}{}`, wantStatus: http.StatusBadRequest, wantCode: "REQUEST_INVALID"},
		{name: "oversized", contentType: "application/json", body: `{"token":"abcdefghijklmnopqrstuvwxyz0123456789"}`, wantStatus: http.StatusBadRequest, wantCode: "REQUEST_INVALID"},
		{name: "empty token", contentType: "application/json; charset=utf-8", body: `{"token":""}`, wantStatus: http.StatusOK, wantCode: `"active":false`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, "/introspect", strings.NewReader(test.body))
			if test.contentType != "" {
				request.Header.Set("Content-Type", test.contentType)
			}
			recorder := httptest.NewRecorder()
			engine.ServeHTTP(recorder, request)
			if recorder.Code != test.wantStatus || !strings.Contains(recorder.Body.String(), test.wantCode) {
				t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
			}
		})
	}
}
