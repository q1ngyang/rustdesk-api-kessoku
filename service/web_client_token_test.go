package service

import (
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

func TestWebClientConnectionTokenIsHashOnlyRevocableAndNotAPIAudience(t *testing.T) {
	oldConfig, oldDB, oldLogger, oldAuth, oldLock, oldServices := Config, DB, Logger, Auth, Lock, AllService
	t.Cleanup(func() {
		Config, DB, Logger, Auth, Lock, AllService = oldConfig, oldDB, oldLogger, oldAuth, oldLock, oldServices
	})
	database, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := database.AutoMigrate(&model.User{}, &model.UserToken{}, &model.LoginLog{}); err != nil {
		t.Fatal(err)
	}
	manager := lifecycleAuthManager(t)
	cfg := &config.Config{Auth: config.Auth{Enabled: true}, WebClient: config.WebClient{Mode: config.WebClientBuiltin, ConnectionTokenTTL: 10 * time.Minute}}
	New(cfg, database, logrus.New(), manager, lock.NewLocal())
	isAdmin := true
	user := &model.User{Username: "web-admin", Status: model.COMMON_STATUS_ENABLE, IsAdmin: &isAdmin, AuthVersion: 1}
	if err := database.Create(user).Error; err != nil {
		t.Fatal(err)
	}
	issued := AllService.UserService.LoginConnection(user, &model.LoginLog{UserId: user.Id, Client: model.LoginLogClientWeb, Type: model.LoginLogTypeGrant}, 10*time.Minute)
	if issued == nil || issued.Token == "" {
		t.Fatal("connection token was not issued")
	}
	stored := &model.UserToken{}
	if err := database.First(stored, issued.Id).Error; err != nil {
		t.Fatal(err)
	}
	if stored.Token != "" || stored.TokenHash == nil || !internalAuth.ConstantTimeHashEqual(issued.Token, *stored.TokenHash) {
		t.Fatalf("connection token was not stored hash-only: %+v", stored)
	}
	if _, _, _, err := AllService.UserService.AuthenticateAccessToken(issued.Token, internalAuth.APIAudience, ""); err == nil {
		t.Fatal("connection token authorized the API audience")
	}
	if result := AllService.AuthIntrospectionService.Introspect(issued.Token); !result.Active || result.Subject != "1" {
		t.Fatalf("Starry introspection rejected connection token: %+v", result)
	}
	if err := AllService.UserService.Logout(user, issued.Token); err != nil {
		t.Fatal(err)
	}
	if result := AllService.AuthIntrospectionService.Introspect(issued.Token); result.Active {
		t.Fatalf("revoked connection token remained active: %+v", result)
	}
}
