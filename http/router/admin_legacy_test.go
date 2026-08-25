package router

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	"github.com/q1ngyang/rustdesk-api-kessoku/v3/config"
	"github.com/q1ngyang/rustdesk-api-kessoku/v3/global"
	"github.com/q1ngyang/rustdesk-api-kessoku/v3/model"
	"github.com/q1ngyang/rustdesk-api-kessoku/v3/service"
	"golang.org/x/text/language"
)

func TestEveryServerControlRouteRequiresAdmin(t *testing.T) {
	oldServices := service.AllService
	oldLocalizer := global.Localizer
	t.Cleanup(func() {
		service.AllService = oldServices
		global.Localizer = oldLocalizer
	})
	service.AllService = &service.Service{UserService: &service.UserService{}}
	bundle := i18n.NewBundle(language.English)
	_ = bundle.AddMessages(language.English, &i18n.Message{ID: "NoAccess", Other: "no access"})
	global.Localizer = func(string) *i18n.Localizer { return i18n.NewLocalizer(bundle, "en") }

	isAdmin := false
	engine := gin.New()
	group := engine.Group("/api/admin")
	group.Use(func(c *gin.Context) {
		c.Set("curUser", &model.User{IsAdmin: &isAdmin})
		c.Next()
	})
	ServerControlBind(group)

	for _, route := range engine.Routes() {
		if !strings.HasPrefix(route.Path, "/api/admin/server-control/v1") {
			continue
		}
		path := route.Path
		path = strings.ReplaceAll(path, ":id", "instance-1")
		path = strings.ReplaceAll(path, ":operation_id", "0191f6a0-0000-7000-8000-000000000001")
		request := httptest.NewRequest(route.Method, path, bytes.NewBufferString(`{}`))
		request.Header.Set("Content-Type", "application/json")
		recorder := httptest.NewRecorder()
		engine.ServeHTTP(recorder, request)
		if recorder.Code != http.StatusForbidden {
			t.Errorf("%s %s status = %d, want %d", route.Method, route.Path, recorder.Code, http.StatusForbidden)
		}
	}
}

func TestLegacyServerCommandsAreNotRegisteredByDefault(t *testing.T) {
	oldConfig := global.Config
	t.Cleanup(func() { global.Config = oldConfig })
	global.Config = config.Config{}

	g := gin.New()
	RustdeskCmdBind(g.Group("/api/admin"))
	for _, route := range g.Routes() {
		if route.Path == "/api/admin/rustdesk/sendCmd" {
			t.Fatalf("legacy command route registered with secure zero-value configuration")
		}
	}
}

func TestLegacyServerCommandsRequireAdminAndNeverExecuteCommands(t *testing.T) {
	oldConfig := global.Config
	oldServices := service.AllService
	oldLocalizer := global.Localizer
	t.Cleanup(func() {
		global.Config = oldConfig
		service.AllService = oldServices
		global.Localizer = oldLocalizer
	})

	global.Config.ServerControl.LegacyCommandEnabled = true
	service.AllService = &service.Service{UserService: &service.UserService{}}
	bundle := i18n.NewBundle(language.English)
	_ = bundle.AddMessages(language.English, &i18n.Message{ID: "NoAccess", Other: "no access"})
	global.Localizer = func(string) *i18n.Localizer { return i18n.NewLocalizer(bundle, "en") }

	admin := false
	g := gin.New()
	rg := g.Group("/api/admin")
	rg.Use(func(c *gin.Context) {
		c.Set("curUser", &model.User{IsAdmin: &admin})
		c.Next()
	})
	RustdeskCmdBind(rg)

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/admin/rustdesk/sendCmd", nil)
	g.ServeHTTP(recorder, req)
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("ordinary user status = %d, want %d", recorder.Code, http.StatusForbidden)
	}

	admin = true
	recorder = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/admin/rustdesk/sendCmd", nil)
	g.ServeHTTP(recorder, req)
	if recorder.Code != http.StatusGone {
		t.Fatalf("admin compatibility status = %d, want %d", recorder.Code, http.StatusGone)
	}
	if got := recorder.Header().Get("Deprecation"); got != "true" {
		t.Fatalf("Deprecation header = %q, want true", got)
	}
}
