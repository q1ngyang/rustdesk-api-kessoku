package router

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	"github.com/q1ngyang/rustdesk-api-kessoku/v2/config"
	"github.com/q1ngyang/rustdesk-api-kessoku/v2/global"
	"github.com/q1ngyang/rustdesk-api-kessoku/v2/http/middleware"
	internalAuth "github.com/q1ngyang/rustdesk-api-kessoku/v2/internal/auth"
	"github.com/q1ngyang/rustdesk-api-kessoku/v2/model"
	"github.com/q1ngyang/rustdesk-api-kessoku/v2/service"
	"github.com/sirupsen/logrus"
	"golang.org/x/text/language"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestAdminLogoutRequiresAuthenticationAndRevokesPresentedToken(t *testing.T) {
	oldConfig, oldDB, oldLogger, oldAuth, oldServices := service.Config, service.DB, service.Logger, service.Auth, service.AllService
	oldLocalizer, oldGlobalLogger := global.Localizer, global.Logger
	t.Cleanup(func() {
		service.Config, service.DB, service.Logger, service.Auth, service.AllService = oldConfig, oldDB, oldLogger, oldAuth, oldServices
		global.Localizer, global.Logger = oldLocalizer, oldGlobalLogger
	})

	database, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "logout.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := database.AutoMigrate(&model.User{}, &model.UserToken{}); err != nil {
		t.Fatal(err)
	}

	logger := logrus.New()
	cfg := &config.Config{}
	service.Config, service.DB, service.Logger, service.Auth = cfg, database, logger, nil
	service.AllService = &service.Service{
		UserService: &service.UserService{},
		PeerService: &service.PeerService{},
	}
	global.Logger = logger
	bundle := i18n.NewBundle(language.English)
	_ = bundle.AddMessages(language.English, &i18n.Message{ID: "NeedLogin", Other: "login required"})
	global.Localizer = func(string) *i18n.Localizer { return i18n.NewLocalizer(bundle, "en") }

	isAdmin := true
	user := &model.User{
		Username:    "logout-route-user",
		IsAdmin:     &isAdmin,
		Status:      model.COMMON_STATUS_ENABLE,
		AuthVersion: 1,
	}
	if err := database.Create(user).Error; err != nil {
		t.Fatal(err)
	}
	const presentedToken = "normal-functional-test-token"
	hash := internalAuth.TokenHashHex(presentedToken)
	tokenRow := &model.UserToken{
		UserId:      user.Id,
		TokenHash:   &hash,
		AuthVersion: 1,
		ExpiredAt:   time.Now().Add(time.Hour).Unix(),
	}
	if err := database.Create(tokenRow).Error; err != nil {
		t.Fatal(err)
	}

	engine := gin.New()
	group := engine.Group("/api/admin")
	group.Use(middleware.BackendUserAuth())
	AuthenticatedLoginBind(group)

	unauthenticated := httptest.NewRecorder()
	engine.ServeHTTP(unauthenticated, httptest.NewRequest(http.MethodPost, "/api/admin/logout", nil))
	if unauthenticated.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated logout status = %d, want %d", unauthenticated.Code, http.StatusUnauthorized)
	}

	authenticated := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/admin/logout", nil)
	request.Header.Set("api-token", presentedToken)
	engine.ServeHTTP(authenticated, request)
	if authenticated.Code != http.StatusOK {
		t.Fatalf("authenticated logout status = %d, want %d", authenticated.Code, http.StatusOK)
	}

	var revoked model.UserToken
	if err := database.First(&revoked, tokenRow.Id).Error; err != nil {
		t.Fatal(err)
	}
	if revoked.RevokedAt == nil || revoked.RevokedReason != "logout" {
		t.Fatalf("logout did not revoke the presented token: %+v", revoked)
	}

	reused := httptest.NewRecorder()
	reuseRequest := httptest.NewRequest(http.MethodPost, "/api/admin/logout", nil)
	reuseRequest.Header.Set("api-token", presentedToken)
	engine.ServeHTTP(reused, reuseRequest)
	if reused.Code != http.StatusUnauthorized {
		t.Fatalf("revoked token reuse status = %d, want %d", reused.Code, http.StatusUnauthorized)
	}
}
