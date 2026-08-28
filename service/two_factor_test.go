package service

import (
	"context"
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base32"
	"encoding/binary"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/q1ngyang/rustdesk-api-kessoku/v3/config"
	"github.com/q1ngyang/rustdesk-api-kessoku/v3/lib/lock"
	"github.com/q1ngyang/rustdesk-api-kessoku/v3/model"
	"github.com/sirupsen/logrus"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestTwoFactorSetupChallengeAndReplayProtection(t *testing.T) {
	oldConfig, oldDB, oldLogger, oldAuth, oldLock, oldServices := Config, DB, Logger, Auth, Lock, AllService
	t.Cleanup(func() {
		Config, DB, Logger, Auth, Lock, AllService = oldConfig, oldDB, oldLogger, oldAuth, oldLock, oldServices
	})
	database, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := database.AutoMigrate(&model.User{}, &model.UserToken{}, &model.LoginLog{}, &model.AdminAuditEvent{}, &model.UserTwoFactor{}, &model.TwoFactorLoginChallenge{}); err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{TwoFactor: config.TwoFactor{Enabled: true, Issuer: "Kessoku Test", KeyFile: filepath.Join(t.TempDir(), "totp.key"), ChallengeTTL: 5 * time.Minute}}
	New(cfg, database, logrus.New(), nil, lock.NewLocal())
	if err := AllService.TwoFactorService.Init(); err != nil {
		t.Fatal(err)
	}
	user := &model.User{Username: "alice", Status: model.COMMON_STATUS_ENABLE, AuthVersion: 1}
	if err := database.Create(user).Error; err != nil {
		t.Fatal(err)
	}
	secretText, uri, err := AllService.TwoFactorService.BeginSetup(user)
	if err != nil || uri == "" {
		t.Fatalf("begin setup: secret=%q uri=%q err=%v", secretText, uri, err)
	}
	secret, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(secretText)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	if err := AllService.TwoFactorService.ConfirmSetup(context.Background(), user, "test-enable", testTOTP(secret, now)); err != nil {
		t.Fatal(err)
	}
	if !AllService.TwoFactorService.EnabledForUser(user.Id) {
		t.Fatal("two-factor setting was not enabled")
	}
	binding := TwoFactorChallengeBinding{Client: "desktop", DeviceID: "device-1", UUID: "uuid-1", Platform: "linux"}
	challenge, err := AllService.TwoFactorService.CreateLoginChallenge(user, binding)
	if err != nil {
		t.Fatal(err)
	}
	nextCode := testTOTP(secret, now.Add(30*time.Second))
	if authenticated, err := AllService.TwoFactorService.CompleteLoginChallenge(challenge, user.Username, nextCode, binding); err != nil || authenticated.Id != user.Id {
		t.Fatalf("complete challenge: user=%+v err=%v", authenticated, err)
	}
	replay, err := AllService.TwoFactorService.CreateLoginChallenge(user, binding)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := AllService.TwoFactorService.CompleteLoginChallenge(replay, user.Username, nextCode, binding); err == nil {
		t.Fatal("accepted a replayed TOTP step")
	}
}

func testTOTP(secret []byte, at time.Time) string {
	counter := make([]byte, 8)
	binary.BigEndian.PutUint64(counter, uint64(at.Unix()/30))
	mac := hmac.New(sha1.New, secret)
	_, _ = mac.Write(counter)
	sum := mac.Sum(nil)
	offset := sum[len(sum)-1] & 0x0f
	value := (binary.BigEndian.Uint32(sum[offset:offset+4]) & 0x7fffffff) % 1000000
	return fmt.Sprintf("%06d", value)
}
