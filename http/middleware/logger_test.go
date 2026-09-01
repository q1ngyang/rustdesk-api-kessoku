package middleware

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/q1ngyang/rustdesk-api-kessoku/v3/global"
	"github.com/sirupsen/logrus"
)

func TestLoggerNeverRecordsLeaseTokenFromBodyOrQuery(t *testing.T) {
	oldLogger := global.Logger
	t.Cleanup(func() { global.Logger = oldLogger })
	var output bytes.Buffer
	logger := logrus.New()
	logger.SetOutput(&output)
	logger.SetLevel(logrus.DebugLevel)
	global.Logger = logger

	const token = "lease-secret-that-must-not-enter-logs"
	engine := gin.New()
	engine.Use(Logger())
	engine.POST("/api/presence/v2/renew", func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})
	request := httptest.NewRequest(
		http.MethodPost,
		"/api/presence/v2/renew?lease_token="+token,
		strings.NewReader(`{"lease_token":"`+token+`"}`),
	)
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusNoContent)
	}
	if strings.Contains(output.String(), token) || strings.Contains(output.String(), "lease_token") {
		t.Fatalf("request logger exposed a lease credential: %q", output.String())
	}
}
