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
	"github.com/q1ngyang/rustdesk-api-kessoku/v3/config"
	"github.com/q1ngyang/rustdesk-api-kessoku/v3/global"
	"github.com/q1ngyang/rustdesk-api-kessoku/v3/http/middleware"
	internalAuth "github.com/q1ngyang/rustdesk-api-kessoku/v3/internal/auth"
	"github.com/q1ngyang/rustdesk-api-kessoku/v3/lib/lock"
	"github.com/q1ngyang/rustdesk-api-kessoku/v3/model"
	"github.com/q1ngyang/rustdesk-api-kessoku/v3/service"
	"github.com/q1ngyang/rustdesk-api-kessoku/v3/utils"
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
	if err := database.AutoMigrate(&model.User{}, &model.UserToken{}, &model.LoginLog{}, &model.AuditConn{}, &model.Peer{}); err != nil {
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
	if err := database.Create(&model.Peer{Id: "990100001", Hostname: "design-workstation", UserId: user.Id}).Error; err != nil {
		t.Fatal(err)
	}

	controller := &WebClient{}
	engine := gin.New()
	engine.POST("/login", controller.Login)
	engine.POST("/grant", middleware.RustAuth(), controller.Grant)
	engine.POST("/logout", middleware.WebClientConnectionAuth(), controller.Logout)
	engine.POST("/session/establish", middleware.WebClientConnectionAuth(), controller.SessionEstablish)
	engine.POST("/session", controller.SessionStatus)
	engine.POST("/preferences", controller.Preferences)
	engine.POST("/audit/start", middleware.WebClientConnectionAuth(), controller.AuditConnectionStart)
	engine.POST("/audit/finish", middleware.WebClientConnectionAuth(), controller.AuditConnectionFinish)

	loginBody := `{"username":"browser-admin","password":"correct horse battery staple","device_id":"browser-1","platform":"linux"}`
	login := performWebClientJSON(engine, http.MethodPost, "/login", loginBody, "")
	if login.Code != http.StatusOK || login.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("password login = %d %s", login.Code, login.Body.String())
	}
	connectionToken := responseConnectionToken(t, login)
	assertConnectionOnlyToken(t, manager, connectionToken)
	assertPartitionedWebClientCookie(t, login)

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
	established := performWebClientJSON(engine, http.MethodPost, "/session/establish", `{"device_id":"browser-2","uuid":"browser-uuid","platform":"linux"}`, grantedToken)
	if established.Code != http.StatusOK || !strings.Contains(established.Body.String(), `"established":true`) {
		t.Fatalf("session establish = %d %s", established.Code, established.Body.String())
	}
	assertPartitionedWebClientCookie(t, established)
	auditStart := performWebClientJSON(engine, http.MethodPost, "/audit/start", `{"peer_id":"990100001","device_id":"Chrome · Linux","uuid":"browser-uuid","platform":"linux"}`, grantedToken)
	if auditStart.Code != http.StatusOK {
		t.Fatalf("connection audit start = %d %s", auditStart.Code, auditStart.Body.String())
	}
	var auditSession struct {
		AuditID      uint   `json:"audit_id"`
		SessionID    string `json:"session_id"`
		PeerHostname string `json:"peer_hostname"`
	}
	if err := json.Unmarshal(auditStart.Body.Bytes(), &auditSession); err != nil || auditSession.AuditID == 0 || len(auditSession.SessionID) < 32 || auditSession.PeerHostname != "design-workstation" {
		t.Fatalf("invalid audit session: %+v err=%v", auditSession, err)
	}
	var storedAudit model.AuditConn
	if err := database.First(&storedAudit, auditSession.AuditID).Error; err != nil || storedAudit.UserId != user.Id || storedAudit.Client != model.LoginLogClientWeb || storedAudit.PeerId != "990100001" || storedAudit.CloseTime != 0 {
		t.Fatalf("connection audit was not persisted: audit=%+v err=%v", storedAudit, err)
	}
	auditFinishBody, _ := json.Marshal(gin.H{"audit_id": auditSession.AuditID, "session_id": auditSession.SessionID})
	auditFinish := performWebClientJSON(engine, http.MethodPost, "/audit/finish", string(auditFinishBody), grantedToken)
	if auditFinish.Code != http.StatusNoContent {
		t.Fatalf("connection audit finish = %d %s", auditFinish.Code, auditFinish.Body.String())
	}
	if err := database.First(&storedAudit, auditSession.AuditID).Error; err != nil || storedAudit.CloseTime == 0 {
		t.Fatalf("connection audit was not closed: audit=%+v err=%v", storedAudit, err)
	}
	var browserCookie *http.Cookie
	for _, cookie := range established.Result().Cookies() {
		if cookie.Name == webClientSessionCookie {
			browserCookie = cookie
			break
		}
	}
	if browserCookie == nil {
		t.Fatal("session establish did not return the browser session cookie")
	}
	preferenceRequest := httptest.NewRequest(http.MethodPost, "/preferences", bytes.NewBufferString(`{"language":"ja","theme":"dark"}`))
	preferenceRequest.Header.Set("Content-Type", "application/json")
	preferenceRequest.AddCookie(browserCookie)
	preferenceResponse := httptest.NewRecorder()
	engine.ServeHTTP(preferenceResponse, preferenceRequest)
	if preferenceResponse.Code != http.StatusNoContent {
		t.Fatalf("preferences = %d %s", preferenceResponse.Code, preferenceResponse.Body.String())
	}
	var storedUser model.User
	if err := database.First(&storedUser, user.Id).Error; err != nil || storedUser.PreferenceLanguage != "ja" || storedUser.PreferenceTheme != "dark" {
		t.Fatalf("account preferences were not persisted: user=%+v err=%v", storedUser, err)
	}
	sessionRequest := httptest.NewRequest(http.MethodPost, "/session", bytes.NewBufferString(`{}`))
	sessionRequest.Header.Set("Content-Type", "application/json")
	sessionRequest.AddCookie(browserCookie)
	sessionResponse := httptest.NewRecorder()
	engine.ServeHTTP(sessionResponse, sessionRequest)
	if sessionResponse.Code != http.StatusOK || !strings.Contains(sessionResponse.Body.String(), `"preference_language":"ja"`) || !strings.Contains(sessionResponse.Body.String(), `"preference_theme":"dark"`) {
		t.Fatalf("session preferences = %d %s", sessionResponse.Code, sessionResponse.Body.String())
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

func assertPartitionedWebClientCookie(t *testing.T, response *httptest.ResponseRecorder) {
	t.Helper()
	cookies := strings.Join(response.Header().Values("Set-Cookie"), "; ")
	for _, attribute := range []string{"kessoku_web_session=", "HttpOnly", "Secure", "SameSite=None", "Partitioned"} {
		if !strings.Contains(cookies, attribute) {
			t.Fatalf("WebClient cookie is missing %q: %s", attribute, cookies)
		}
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
