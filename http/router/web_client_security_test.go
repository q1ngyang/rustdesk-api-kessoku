package router

import (
	"bytes"
	"encoding/base64"
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
)

func TestDedicatedWebClientListenerExposesOnlyPublicConfigAndStaticGET(t *testing.T) {
	oldConfig := global.Config
	t.Cleanup(func() { global.Config = oldConfig })
	resources := t.TempDir()
	client := filepath.Join(resources, "client")
	assets := filepath.Join(client, "assets")
	if err := os.MkdirAll(assets, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(client, "index.html"), []byte("<!doctype html><title>Kessoku Client</title>"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(assets, "app-123.js"), []byte("export default true"), 0o644); err != nil {
		t.Fatal(err)
	}
	global.Config.Gin.ResourcesPath = resources
	global.Config.WebClient = config.WebClient{
		Mode:              config.WebClientBuiltin,
		PublicOrigin:      "https://client.example.test",
		APIOrigin:         "https://api.example.test",
		RendezvousWSSURL:  "wss://starry.example.test/ws/id",
		RelayWSSURLs:      map[string]string{"relay.example.test:21117": "wss://starry.example.test/ws/relay"},
		ServerPublicKey:   base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{1}, 32)),
		ProfileGeneration: 9,
	}
	engine := gin.New()
	WebClientInit(engine)
	engine.NoRoute(func(c *gin.Context) { c.Status(http.StatusNotFound) })

	configResponse := httptest.NewRecorder()
	engine.ServeHTTP(configResponse, httptest.NewRequest(http.MethodGet, "/config/v1.json", nil))
	if configResponse.Code != http.StatusOK || configResponse.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("config response = %d %v", configResponse.Code, configResponse.Header())
	}
	var payload map[string]interface{}
	if err := json.Unmarshal(configResponse.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"schema_version", "profile_generation", "api_origin", "rendezvous_wss_url", "relay_wss_urls", "server_public_key", "server_key_fingerprint"} {
		if _, exists := payload[field]; !exists {
			t.Fatalf("public config omitted %q: %v", field, payload)
		}
	}
	for _, forbidden := range []string{"listen", "connection_token_ttl", "public_origin", "token", "user", "internal"} {
		if _, exists := payload[forbidden]; exists {
			t.Fatalf("public config leaked %q: %v", forbidden, payload)
		}
	}
	csp := configResponse.Header().Get("Content-Security-Policy")
	for _, source := range []string{"https://api.example.test", "wss://starry.example.test", "script-src 'self'", "frame-ancestors 'none'", "worker-src 'none'"} {
		if !strings.Contains(csp, source) {
			t.Fatalf("CSP omitted %q: %s", source, csp)
		}
	}
	if got := configResponse.Header().Get("Cross-Origin-Opener-Policy"); got != "" {
		t.Fatalf("client listener severed the one-shot admin grant handoff: %q", got)
	}

	assetResponse := httptest.NewRecorder()
	engine.ServeHTTP(assetResponse, httptest.NewRequest(http.MethodGet, "/assets/app-123.js", nil))
	if assetResponse.Code != http.StatusOK || !strings.Contains(assetResponse.Header().Get("Cache-Control"), "immutable") {
		t.Fatalf("asset response = %d %v", assetResponse.Code, assetResponse.Header())
	}
	for _, path := range []string{"/api/admin/config/app", "/api/internal/v1/auth/jwks", "/assets/"} {
		response := httptest.NewRecorder()
		engine.ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
		if response.Code != http.StatusNotFound {
			t.Fatalf("dedicated listener exposed %s with status %d", path, response.Code)
		}
	}
	mutation := httptest.NewRecorder()
	engine.ServeHTTP(mutation, httptest.NewRequest(http.MethodPost, "/", strings.NewReader("ignored")))
	if mutation.Code != http.StatusNotFound {
		t.Fatalf("static listener accepted mutation: %d", mutation.Code)
	}
}
