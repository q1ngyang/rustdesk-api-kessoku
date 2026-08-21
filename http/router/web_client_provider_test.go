package router

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/q1ngyang/rustdesk-api-kessoku/v2/config"
	"github.com/q1ngyang/rustdesk-api-kessoku/v2/global"
	"github.com/q1ngyang/rustdesk-api-kessoku/v2/http/controller/admin"
)

func TestWebClientProviderRoutesAreDisabledByDefault(t *testing.T) {
	oldConfig := global.Config
	t.Cleanup(func() { global.Config = oldConfig })
	global.Config = config.Config{}

	engine := gin.New()
	ConfigBind(engine.Group("/api/admin"))
	AddressBookBind(engine.Group("/api/admin"))
	WebInit(engine)
	for _, route := range engine.Routes() {
		if route.Path == "/api/admin/config/server" {
			t.Fatalf("legacy browser-client server-key endpoint remains reachable: %s %s", route.Method, route.Path)
		}
		if strings.Contains(route.Path, "webclient") || strings.Contains(route.Path, "web-client-provider") || strings.Contains(route.Path, "shareByWebClient") {
			t.Fatalf("disabled provider exposed route %s %s", route.Method, route.Path)
		}
	}
}

func TestLegacyWebClientAPIRoutesAreRemoved(t *testing.T) {
	oldConfig := global.Config
	oldWorkingDirectory, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	repositoryRoot, err := filepath.Abs(filepath.Join(oldWorkingDirectory, "../.."))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(repositoryRoot); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		global.Config = oldConfig
		_ = os.Chdir(oldWorkingDirectory)
	})
	global.Config = config.Config{Gin: config.Gin{ResourcesPath: "resources"}}

	engine := gin.New()
	ApiInit(engine)
	for _, route := range engine.Routes() {
		switch route.Path {
		case "/api/shared-peer", "/api/server-config", "/api/server-config-v2":
			t.Fatalf("legacy browser-client API remains reachable: %s %s", route.Method, route.Path)
		}
	}
}

func TestExternalProviderManifestContainsOnlyPublicFields(t *testing.T) {
	oldConfig := global.Config
	t.Cleanup(func() { global.Config = oldConfig })
	provider := config.WebClientProvider{
		Mode:                config.WebClientProviderExternal,
		AuthorizationRecord: "private approval record KES-42",
		Manifest: config.WebClientProviderManifest{
			ID:            "approved-client",
			Name:          "Approved client",
			LaunchURL:     "https://client.example.test/launch",
			AllowedOrigin: "https://client.example.test",
			License:       "Apache-2.0",
			SourceURL:     "https://source.example.test/client",
			Version:       "1.2.3",
			Digest:        "sha256:" + strings.Repeat("a", 64),
		},
	}
	global.Config.WebClientProvider = provider

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodGet, "/api/admin/config/web-client-provider", nil)
	(&admin.Config{}).WebClientProviderManifest(context)
	if recorder.Code != http.StatusOK || recorder.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("status=%d headers=%v", recorder.Code, recorder.Header())
	}
	var envelope struct {
		Data map[string]interface{} `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if len(envelope.Data) != 8 {
		t.Fatalf("manifest fields = %v", envelope.Data)
	}
	for _, key := range []string{"id", "name", "launch_url", "allowed_origin", "license", "source_url", "version", "digest"} {
		if _, exists := envelope.Data[key]; !exists {
			t.Fatalf("manifest omitted %q: %v", key, envelope.Data)
		}
	}
	body := recorder.Body.String()
	if strings.Contains(body, provider.AuthorizationRecord) || strings.Contains(strings.ToLower(body), "access_token") || strings.Contains(strings.ToLower(body), "api-token") {
		t.Fatalf("manifest leaked deployment or token data: %s", body)
	}
}
