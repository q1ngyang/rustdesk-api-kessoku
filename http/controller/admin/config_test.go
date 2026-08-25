package admin

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/q1ngyang/rustdesk-api-kessoku/v3/config"
	"github.com/q1ngyang/rustdesk-api-kessoku/v3/global"
)

func TestAppConfigPublishesOnlyWebClientModeAndValidatedOrigin(t *testing.T) {
	oldConfig := global.Config
	t.Cleanup(func() { global.Config = oldConfig })
	global.Config.WebClient = config.WebClient{
		Mode:         config.WebClientBuiltin,
		PublicOrigin: "https://client.example.test",
	}

	engine := gin.New()
	engine.GET("/app", (&Config{}).AppConfig)
	response := httptest.NewRecorder()
	engine.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/app", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("app config returned %d", response.Code)
	}
	if response.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("app config is cacheable: %v", response.Header())
	}
	var payload struct {
		Data map[string]interface{} `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if len(payload.Data) != 2 || payload.Data["web_client_mode"] != config.WebClientBuiltin || payload.Data["web_client_public_origin"] != "https://client.example.test" {
		t.Fatalf("unexpected app config contract: %v", payload.Data)
	}
	for _, forbidden := range []string{"provider", "license", "url", "digest", "token"} {
		if _, exists := payload.Data[forbidden]; exists {
			t.Fatalf("app config exposed forbidden field %q", forbidden)
		}
	}
}
