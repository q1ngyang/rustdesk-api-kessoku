package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/q1ngyang/rustdesk-api-kessoku/v2/config"
	"github.com/q1ngyang/rustdesk-api-kessoku/v2/global"
)

func TestWebClientCORSIsExactAndCredentialless(t *testing.T) {
	oldConfig := global.Config
	t.Cleanup(func() { global.Config = oldConfig })
	global.Config.WebClient = config.WebClient{
		Mode:         config.WebClientBuiltin,
		PublicOrigin: "https://client.example.test",
		APIOrigin:    "https://api.example.test",
	}
	engine := gin.New()
	engine.Use(WebClientCORS())
	engine.POST("/login", func(c *gin.Context) { c.Status(http.StatusNoContent) })
	engine.OPTIONS("/login", func(c *gin.Context) { c.Status(http.StatusInternalServerError) })

	preflight := httptest.NewRequest(http.MethodOptions, "/login", nil)
	preflight.Header.Set("Origin", "https://client.example.test")
	preflight.Header.Set("Access-Control-Request-Method", http.MethodPost)
	preflight.Header.Set("Access-Control-Request-Headers", "content-type, authorization")
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, preflight)
	if recorder.Code != http.StatusNoContent || recorder.Header().Get("Access-Control-Allow-Origin") != "https://client.example.test" || recorder.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("valid preflight = %d %v", recorder.Code, recorder.Header())
	}
	if recorder.Header().Get("Access-Control-Allow-Credentials") != "" || stringsContainsFold(recorder.Header().Get("Access-Control-Allow-Headers"), "api-token") {
		t.Fatalf("CORS permits credentials or admin header: %v", recorder.Header())
	}

	sameOrigin := httptest.NewRequest(http.MethodPost, "/login", nil)
	sameOrigin.Header.Set("Origin", "https://api.example.test")
	sameOriginResponse := httptest.NewRecorder()
	engine.ServeHTTP(sameOriginResponse, sameOrigin)
	if sameOriginResponse.Code != http.StatusNoContent || sameOriginResponse.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Fatalf("same API origin rejected or treated as cross-origin: %d %v", sameOriginResponse.Code, sameOriginResponse.Header())
	}

	for _, origin := range []string{"https://other.example.test", "https://client.example.test.evil"} {
		request := httptest.NewRequest(http.MethodPost, "/login", nil)
		request.Header.Set("Origin", origin)
		response := httptest.NewRecorder()
		engine.ServeHTTP(response, request)
		if response.Code != http.StatusForbidden || response.Header().Get("Access-Control-Allow-Origin") != "" {
			t.Fatalf("origin %q accepted: %d %v", origin, response.Code, response.Header())
		}
	}
}

func stringsContainsFold(value, substring string) bool {
	return len(value) >= len(substring) && strings.Contains(strings.ToLower(value), strings.ToLower(substring))
}
