package api

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/q1ngyang/rustdesk-api-kessoku/v2/config"
	"github.com/q1ngyang/rustdesk-api-kessoku/v2/global"
	"github.com/q1ngyang/rustdesk-api-kessoku/v2/http/middleware"
	internalAuth "github.com/q1ngyang/rustdesk-api-kessoku/v2/internal/auth"
	"github.com/q1ngyang/rustdesk-api-kessoku/v2/lib/lock"
	"github.com/q1ngyang/rustdesk-api-kessoku/v2/model"
	"github.com/q1ngyang/rustdesk-api-kessoku/v2/service"
	"github.com/q1ngyang/rustdesk-api-kessoku/v2/utils"
	"github.com/sirupsen/logrus"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestWebClientPasswordLoginGrantAndLogoutUseConnectionOnlyTokens(t *testing.T) {
	oldGlobalConfig, oldGlobalLogger, oldGlobalAuth, oldLimiter := global.Config, global.Logger, global.Auth, global.LoginLimiter
	oldConfig, oldDB, oldLogger, oldAuth, oldLock, oldServices := service.Config, service.DB, service.Logger, service.Auth, service.Lock, service.AllService
	t.Cleanup(func() {
		global.Config, global.Logger, global.Auth, global.LoginLimiter = oldGlobalConfig, oldGlobalLogger, oldGlobalAuth, oldLimiter
		service.Config, service.DB, service.Logger, service.Auth, service.Lock, service.AllService = oldConfig, oldDB, oldLogger, oldAuth, oldLock, oldServices
	})
	database, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := database.AutoMigrate(&model.User{}, &model.UserToken{}, &model.LoginLog{}); err != nil {
		t.Fatal(err)
	}
	manager := webClientTestAuthManager(t)
	global.Config = config.Config{
		Auth:      config.Auth{Enabled: true},
		WebClient: config.WebClient{Mode: config.WebClientBuiltin, ConnectionTokenTTL: 10 * time.Minute},
	}
	global.Logger = logrus.New()
	global.Auth = manager
	global.LoginLimiter = utils.NewLoginLimiter(utils.SecurityPolicy{CaptchaThreshold: -1})
	service.New(&global.Config, database, global.Logger, manager, lock.NewLocal())

	passwordHash, err := utils.EncryptPassword("correct horse battery staple")
	if err != nil {
		t.Fatal(err)
	}
	isAdmin := true
	user := &model.User{Username: "browser-admin", Password: passwordHash, Status: model.COMMON_STATUS_ENABLE, IsAdmin: &isAdmin, AuthVersion: 1}
	if err := database.Create(user).Error; err != nil {
		t.Fatal(err)
	}

	controller := &WebClient{}
	engine := gin.New()
	engine.POST("/login", controller.Login)
	engine.POST("/grant", middleware.RustAuth(), controller.Grant)
	engine.POST("/logout", middleware.WebClientConnectionAuth(), controller.Logout)

	loginBody := `{"username":"browser-admin","password":"correct horse battery staple","device_id":"browser-1","platform":"linux"}`
	login := performWebClientJSON(engine, http.MethodPost, "/login", loginBody, "")
	if login.Code != http.StatusOK || login.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("password login = %d %s", login.Code, login.Body.String())
	}
	connectionToken := responseConnectionToken(t, login)
	assertConnectionOnlyToken(t, manager, connectionToken)

	standard := service.AllService.UserService.Login(user, &model.LoginLog{UserId: user.Id, Client: model.LoginLogClientApp, Type: model.LoginLogTypeAccount})
	if standard == nil {
		t.Fatal("standard API token was not issued")
	}
	grant := performWebClientJSON(engine, http.MethodPost, "/grant", `{"device_id":"browser-2","platform":"linux"}`, standard.Token)
	if grant.Code != http.StatusOK {
		t.Fatalf("grant = %d %s", grant.Code, grant.Body.String())
	}
	grantedToken := responseConnectionToken(t, grant)
	assertConnectionOnlyToken(t, manager, grantedToken)
	if strings.Contains(grant.Body.String(), "client_origin") {
		t.Fatalf("grant returned a second origin authority: %s", grant.Body.String())
	}

	logout := performWebClientJSON(engine, http.MethodPost, "/logout", "", grantedToken)
	if logout.Code != http.StatusNoContent {
		t.Fatalf("logout = %d %s", logout.Code, logout.Body.String())
	}
	if result := service.AllService.AuthIntrospectionService.Introspect(grantedToken); result.Active {
		t.Fatalf("logout left connection token active: %+v", result)
	}

	invalid := performWebClientJSON(engine, http.MethodPost, "/login", `{"username":"browser-admin","password":"correct horse battery staple","extra":true}`, "")
	if invalid.Code != http.StatusBadRequest {
		t.Fatalf("unknown login field accepted: %d %s", invalid.Code, invalid.Body.String())
	}
}

func performWebClientJSON(engine http.Handler, method, path, body, token string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	if body != "" {
		request.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	response := httptest.NewRecorder()
	engine.ServeHTTP(response, request)
	return response
}

func responseConnectionToken(t *testing.T, response *httptest.ResponseRecorder) string {
	t.Helper()
	var body struct {
		ConnectionToken string `json:"connection_token"`
		TokenType       string `json:"token_type"`
		ExpiresAt       int64  `json:"expires_at"`
		ExpiresIn       int64  `json:"expires_in"`
		Audience        string `json:"audience"`
		Scope           string `json:"scope"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.ConnectionToken == "" || body.TokenType != "Bearer" || body.ExpiresAt <= time.Now().Unix() || body.ExpiresIn <= 0 || body.Audience != internalAuth.ConnectionAudience || body.Scope != internalAuth.ConnectScope {
		t.Fatalf("invalid token response: %+v", body)
	}
	return body.ConnectionToken
}

func assertConnectionOnlyToken(t *testing.T, manager *internalAuth.Manager, token string) {
	t.Helper()
	if _, err := manager.VerifyAccessToken(token, internalAuth.VerifyOptions{Audience: internalAuth.ConnectionAudience, RequiredScope: internalAuth.ConnectScope}); err != nil {
		t.Fatalf("Starry profile rejected token: %v", err)
	}
	if _, err := manager.VerifyAccessToken(token, internalAuth.VerifyOptions{Audience: internalAuth.APIAudience}); err == nil {
		t.Fatal("connection token authorized API audience")
	}
}

func webClientTestAuthManager(t *testing.T) *internalAuth.Manager {
	t.Helper()
	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		t.Fatal(err)
	}
	keyPath := filepath.Join(t.TempDir(), "access.pem")
	if err := os.WriteFile(keyPath, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: encoded}), 0o600); err != nil {
		t.Fatal(err)
	}
	manager, err := internalAuth.NewManager(config.Auth{
		Enabled:         true,
		Issuer:          "https://api.example.test",
		Audiences:       []string{internalAuth.APIAudience, internalAuth.ConnectionAudience},
		AccessTokenTTL:  30 * time.Minute,
		MaximumTokenTTL: time.Hour,
		CurrentKey:      config.AuthKey{ID: "web-client-test", PrivateKeyFile: keyPath},
	})
	if err != nil {
		t.Fatal(err)
	}
	return manager
}
