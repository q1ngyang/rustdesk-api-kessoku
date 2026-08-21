package router

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/q1ngyang/rustdesk-api-kessoku/v2/config"
	"github.com/q1ngyang/rustdesk-api-kessoku/v2/global"
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
