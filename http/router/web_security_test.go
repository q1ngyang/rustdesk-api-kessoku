package router

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/q1ngyang/rustdesk-api-kessoku/v2/global"
)

func TestAdminWebStaticFilesHaveRestrictiveBrowserHeaders(t *testing.T) {
	gin.SetMode(gin.TestMode)
	resources := t.TempDir()
	adminDir := filepath.Join(resources, "admin")
	if err := os.Mkdir(adminDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(adminDir, "index.html"), []byte("<!doctype html><title>Kessoku</title>"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(adminDir, "app.js"), []byte("export default true\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	assetDir := filepath.Join(adminDir, "assets")
	if err := os.Mkdir(assetDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(assetDir, "private-map.txt"), []byte("not listed\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	previousResourcesPath := global.Config.Gin.ResourcesPath
	global.Config.Gin.ResourcesPath = resources
	t.Cleanup(func() { global.Config.Gin.ResourcesPath = previousResourcesPath })

	engine := gin.New()
	WebInit(engine)

	for _, path := range []string{"/_admin/", "/_admin/app.js"} {
		request := httptest.NewRequest(http.MethodGet, path, nil)
		response := httptest.NewRecorder()
		engine.ServeHTTP(response, request)
		if response.Code != http.StatusOK {
			t.Fatalf("GET %s returned %d: %s", path, response.Code, response.Body.String())
		}
		for name, expected := range map[string]string{
			"Cache-Control":                "no-store",
			"Cross-Origin-Opener-Policy":   "same-origin-allow-popups",
			"Cross-Origin-Resource-Policy": "same-origin",
			"Referrer-Policy":              "no-referrer",
			"X-Content-Type-Options":       "nosniff",
			"X-Frame-Options":              "DENY",
		} {
			if actual := response.Header().Get(name); actual != expected {
				t.Errorf("GET %s header %s = %q, want %q", path, name, actual, expected)
			}
		}
		csp := response.Header().Get("Content-Security-Policy")
		for _, directive := range []string{"default-src 'self'", "frame-ancestors 'none'", "object-src 'none'", "script-src 'self'"} {
			if !strings.Contains(csp, directive) {
				t.Errorf("GET %s CSP is missing %q: %q", path, directive, csp)
			}
		}
	}

	request := httptest.NewRequest(http.MethodGet, "/_admin/assets/", nil)
	response := httptest.NewRecorder()
	engine.ServeHTTP(response, request)
	if response.Code != http.StatusNotFound {
		t.Fatalf("directory listing returned %d, want 404", response.Code)
	}
	if strings.Contains(response.Body.String(), "private-map.txt") {
		t.Fatal("admin static route exposed a directory listing")
	}
}

func TestAdminWebStaticRouteRejectsMutationMethods(t *testing.T) {
	gin.SetMode(gin.TestMode)
	previousResourcesPath := global.Config.Gin.ResourcesPath
	global.Config.Gin.ResourcesPath = t.TempDir()
	t.Cleanup(func() { global.Config.Gin.ResourcesPath = previousResourcesPath })

	engine := gin.New()
	WebInit(engine)
	request := httptest.NewRequest(http.MethodPost, "/_admin/index.html", strings.NewReader("ignored"))
	response := httptest.NewRecorder()
	engine.ServeHTTP(response, request)
	if response.Code != http.StatusNotFound {
		t.Fatalf("POST static admin route returned %d, want 404", response.Code)
	}
}
