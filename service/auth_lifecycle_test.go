package service

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/q1ngyang/rustdesk-api-kessoku/v2/config"
	internalAuth "github.com/q1ngyang/rustdesk-api-kessoku/v2/internal/auth"
	"github.com/q1ngyang/rustdesk-api-kessoku/v2/lib/lock"
	"github.com/q1ngyang/rustdesk-api-kessoku/v2/model"
	"github.com/sirupsen/logrus"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestTokenLifecycleHashRevocationAndAuthVersion(t *testing.T) {
	oldConfig, oldDB, oldLogger, oldAuth, oldLock, oldServices := Config, DB, Logger, Auth, Lock, AllService
	t.Cleanup(func() {
		Config, DB, Logger, Auth, Lock, AllService = oldConfig, oldDB, oldLogger, oldAuth, oldLock, oldServices
	})

	database, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := database.AutoMigrate(&model.User{}, &model.UserToken{}, &model.LoginLog{}, &model.AdminAuditEvent{}, &model.SecurityInvariantLock{}); err != nil {
		t.Fatal(err)
	}
	if err := database.Create(&model.SecurityInvariantLock{Name: "enabled-admin"}).Error; err != nil {
		t.Fatal(err)
	}
	manager := lifecycleAuthManager(t)
	cfg := &config.Config{Auth: config.Auth{Enabled: true}}
	logger := logrus.New()
	New(cfg, database, logger, manager, lock.NewLocal())

	isAdmin := false
	user := &model.User{
		Username:    "alice",
		Status:      model.COMMON_STATUS_ENABLE,
		IsAdmin:     &isAdmin,
		AuthVersion: 1,
	}
	if err := database.Create(user).Error; err != nil {
		t.Fatal(err)
	}

	first := AllService.UserService.Login(user, &model.LoginLog{UserId: user.Id})
	if first == nil || first.Token == "" {
		t.Fatal("login did not return an access token")
	}
	stored := &model.UserToken{}
	if err := database.First(stored, first.Id).Error; err != nil {
		t.Fatal(err)
	}
	if stored.Token != "" || stored.TokenHash == nil || !internalAuth.ConstantTimeHashEqual(first.Token, *stored.TokenHash) {
		t.Fatalf("token persistence is not hash-only: %+v", stored)
	}
	encoded, err := json.Marshal(stored)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), first.Token) || strings.Contains(string(encoded), *stored.TokenHash) {
		t.Fatalf("serialized token metadata leaked a credential: %s", encoded)
	}
	if result := AllService.AuthIntrospectionService.Introspect(first.Token); !result.Active || result.Subject != "1" {
		t.Fatalf("fresh token introspection = %+v", result)
	}
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	if result := AllService.AuthIntrospectionService.IntrospectContext(cancelled, first.Token); result.Active {
		t.Fatalf("cancelled introspection remained active: %+v", result)
	}
	if err := AllService.UserService.LogoutContext(cancelled, user, first.Token); err == nil {
		t.Fatal("cancelled logout silently reported success")
	}
	if result := AllService.AuthIntrospectionService.Introspect(first.Token); !result.Active {
		t.Fatalf("cancelled logout revoked the token: %+v", result)
	}

	if err := AllService.UserService.Logout(user, first.Token); err != nil {
		t.Fatal(err)
	}
	if result := AllService.AuthIntrospectionService.Introspect(first.Token); result.Active {
		t.Fatalf("logged out token remained active: %+v", result)
	}

	second := AllService.UserService.Login(user, &model.LoginLog{UserId: user.Id})
	third := AllService.UserService.Login(user, &model.LoginLog{UserId: user.Id})
	if second == nil || third == nil {
		t.Fatal("subsequent login failed")
	}
	if err := AllService.UserService.FlushToken(user); err != nil {
		t.Fatal(err)
	}
	for _, token := range []string{second.Token, third.Token} {
		if result := AllService.AuthIntrospectionService.Introspect(token); result.Active {
			t.Fatalf("global logout left token active: %+v", result)
		}
	}
	refreshed := AllService.UserService.InfoById(user.Id)
	if refreshed.AuthVersion != 2 {
		t.Fatalf("auth version = %d, want 2", refreshed.AuthVersion)
	}

	fourth := AllService.UserService.Login(refreshed, &model.LoginLog{UserId: user.Id})
	if fourth == nil {
		t.Fatal("login after global revocation failed")
	}
	update := &model.User{IdModel: model.IdModel{Id: user.Id}, Status: model.COMMON_STATUS_DISABLED}
	if err := AllService.UserService.Update(update); err != nil {
		t.Fatal(err)
	}
	if result := AllService.AuthIntrospectionService.Introspect(fourth.Token); result.Active {
		t.Fatalf("disabled user's token remained active: %+v", result)
	}
	auditEvents := []model.AdminAuditEvent{}
	if err := database.Where("action IN ?", []string{"auth.sessions.global_revoked", "auth.user.disabled"}).Order("id").Find(&auditEvents).Error; err != nil {
		t.Fatal(err)
	}
	if len(auditEvents) != 2 || auditEvents[0].Result != "success" || auditEvents[1].Result != "success" {
		t.Fatalf("authentication audit events = %+v", auditEvents)
	}
	encodedAudit, err := json.Marshal(auditEvents)
	if err != nil {
		t.Fatal(err)
	}
	for _, token := range []string{first.Token, second.Token, third.Token, fourth.Token} {
		if strings.Contains(string(encodedAudit), token) {
			t.Fatal("authentication audit leaked a bearer token")
		}
	}
}

func TestLegacyFallbackCannotResurrectStrictJWTAfterKeyRemoval(t *testing.T) {
	oldConfig, oldDB, oldLogger, oldAuth, oldLock, oldServices := Config, DB, Logger, Auth, Lock, AllService
	t.Cleanup(func() {
		Config, DB, Logger, Auth, Lock, AllService = oldConfig, oldDB, oldLogger, oldAuth, oldLock, oldServices
	})

	database, err := gorm.Open(sqlite.Open("file:legacy-fallback?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := database.AutoMigrate(&model.User{}, &model.UserToken{}, &model.LoginLog{}, &model.AdminAuditEvent{}, &model.SecurityInvariantLock{}); err != nil {
		t.Fatal(err)
	}
	if err := database.Create(&model.SecurityInvariantLock{Name: "enabled-admin"}).Error; err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{Auth: config.Auth{Enabled: true, LegacyTokenReadEnabled: true}}
	New(cfg, database, logrus.New(), lifecycleAuthManager(t), lock.NewLocal())

	isAdmin := false
	user := &model.User{Username: "legacy-overlap", Status: model.COMMON_STATUS_ENABLE, IsAdmin: &isAdmin, AuthVersion: 1}
	if err := database.Create(user).Error; err != nil {
		t.Fatal(err)
	}
	strict := AllService.UserService.Login(user, &model.LoginLog{UserId: user.Id})
	if strict == nil {
		t.Fatal("strict JWT login failed")
	}

	// Replace the keyring with a different key using the same kid. Strict
	// signature verification now fails, while the exact token hash still has an
	// authoritative database row.
	Auth = lifecycleAuthManager(t)
	if authenticated, _ := AllService.UserService.InfoByAccessToken(strict.Token); authenticated.Id != 0 {
		t.Fatal("legacy compatibility resurrected a strict JWT after key removal")
	}

	legacyToken := "legacy-opaque-credential-with-database-authority"
	legacyHash := internalAuth.TokenHashHex(legacyToken)
	legacyRow := &model.UserToken{
		UserId:      user.Id,
		Token:       legacyToken,
		TokenHash:   &legacyHash,
		AuthVersion: user.AuthVersion,
		ExpiredAt:   time.Now().Add(time.Hour).Unix(),
	}
	if err := database.Create(legacyRow).Error; err != nil {
		t.Fatal(err)
	}
	if authenticated, _ := AllService.UserService.InfoByAccessToken(legacyToken); authenticated.Id != user.Id {
		t.Fatal("bounded compatibility rejected a genuine pre-EdDSA credential")
	}
}

func lifecycleAuthManager(t *testing.T) *internalAuth.Manager {
	t.Helper()
	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "access.pem")
	if err := os.WriteFile(path, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: encoded}), 0o600); err != nil {
		t.Fatal(err)
	}
	manager, err := internalAuth.NewManager(config.Auth{
		Enabled:         true,
		Issuer:          "https://api.example.test",
		Audiences:       []string{internalAuth.APIAudience, internalAuth.ConnectionAudience},
		AccessTokenTTL:  15 * time.Minute,
		MaximumTokenTTL: time.Hour,
		CurrentKey:      config.AuthKey{ID: "test-key", PrivateKeyFile: path},
	})
	if err != nil {
		t.Fatal(err)
	}
	return manager
}
