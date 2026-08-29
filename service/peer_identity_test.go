package service

import (
	"errors"
	"testing"
	"time"

	"github.com/q1ngyang/rustdesk-api-kessoku/v3/model"
)

func TestBindLoginIdentityCreatesClaimsAndProtectsPeers(t *testing.T) {
	database := securityAuditDatabase(t, true)
	if err := database.AutoMigrate(&model.Peer{}); err != nil {
		t.Fatal(err)
	}

	if err := AllService.PeerService.BindLoginIdentity(" 384 308 369 ", "uuid-native", 7); err != nil {
		t.Fatal(err)
	}
	created := AllService.PeerService.FindById("384308369")
	if created.RowId == 0 || created.Id != "384308369" || created.Uuid != "uuid-native" || created.UserId != 7 {
		t.Fatalf("native login identity was not persisted: %+v", created)
	}

	placeholder := &model.Peer{Id: "301 132 036", Alias: "manually added"}
	if err := AllService.PeerService.Create(placeholder); err != nil {
		t.Fatal(err)
	}
	if placeholder.IdentitySource != model.PeerIdentitySourceManual {
		t.Fatalf("manual placeholder identity source = %q", placeholder.IdentitySource)
	}
	if err := AllService.PeerService.BindLoginIdentity("301 132 036", "uuid-placeholder", 7); err != nil {
		t.Fatal(err)
	}
	claimed := AllService.PeerService.FindById("301132036")
	if claimed.Uuid != "uuid-placeholder" || claimed.UserId != 7 || claimed.Alias != placeholder.Alias {
		t.Fatalf("manual placeholder was not safely claimed: %+v", claimed)
	}

	if err := AllService.PeerService.BindLoginIdentity("301132036", "uuid-attacker", 8); !errors.Is(err, ErrPeerIdentityConflict) {
		t.Fatalf("conflicting login = %v, want ErrPeerIdentityConflict", err)
	}
	protected := AllService.PeerService.FindById("301132036")
	if protected.Uuid != "uuid-placeholder" || protected.UserId != 7 {
		t.Fatalf("conflicting login changed peer ownership: %+v", protected)
	}

	duplicates := []model.Peer{{Id: "duplicate-id"}, {Id: "duplicate-id"}}
	if err := database.Create(&duplicates).Error; err != nil {
		t.Fatal(err)
	}
	if err := AllService.PeerService.BindLoginIdentity("duplicate-id", "uuid-duplicate", 7); !errors.Is(err, ErrPeerIdentityConflict) {
		t.Fatalf("duplicate peer rows = %v, want ErrPeerIdentityConflict", err)
	}
}

func TestNativeOwnershipRecoveryRequiresAnActiveSession(t *testing.T) {
	database := securityAuditDatabase(t, true)
	if err := database.AutoMigrate(&model.Peer{}, &model.LoginLog{}); err != nil {
		t.Fatal(err)
	}
	isAdmin := false
	user := &model.User{Username: "active-native", Status: model.COMMON_STATUS_ENABLE, IsAdmin: &isAdmin, AuthVersion: 3}
	if err := database.Create(user).Error; err != nil {
		t.Fatal(err)
	}
	now := time.Now().Unix()
	revoked := now - 1
	tokens := []model.UserToken{
		{UserId: user.Id, DeviceId: "expired", DeviceUuid: "uuid-expired", Client: model.LoginLogClientNative, AuthVersion: 3, IssuedAt: now - 7200, ExpiredAt: now - 3600},
		{UserId: user.Id, DeviceId: "revoked", DeviceUuid: "uuid-revoked", Client: model.LoginLogClientNative, AuthVersion: 3, IssuedAt: now - 10, ExpiredAt: now + 3600, RevokedAt: &revoked},
		{UserId: user.Id, DeviceId: "wrong-version", DeviceUuid: "uuid-version", Client: model.LoginLogClientNative, AuthVersion: 2, IssuedAt: now - 10, ExpiredAt: now + 3600},
		{UserId: user.Id, DeviceId: "browser", DeviceUuid: "uuid-browser", Client: model.LoginLogClientWebAdmin, AuthVersion: 3, IssuedAt: now - 10, ExpiredAt: now + 3600},
		{UserId: user.Id, DeviceId: " 123 456 789 ", DeviceUuid: "uuid-active", Client: model.LoginLogClientNative, AuthVersion: 3, IssuedAt: now, ExpiredAt: now + 3600},
	}
	if err := database.Create(&tokens).Error; err != nil {
		t.Fatal(err)
	}
	for _, identity := range [][2]string{{"expired", "uuid-expired"}, {"revoked", "uuid-revoked"}, {"wrong-version", "uuid-version"}, {"browser", "uuid-browser"}} {
		if got := AllService.UserService.FindActiveNativeUserID(identity[1], identity[0], now); got != 0 {
			t.Fatalf("inactive identity %q resolved to user %d", identity[0], got)
		}
	}
	// Current versions normalize native token IDs at issuance. This explicit
	// assertion also guards the v312 migration expectation for legacy spacing.
	if err := database.Model(&model.UserToken{}).Where("id = ?", tokens[4].Id).Update("device_id", "123456789").Error; err != nil {
		t.Fatal(err)
	}
	if got := AllService.UserService.FindActiveNativeUserID("uuid-active", "123 456 789", now); got != user.Id {
		t.Fatalf("active native identity resolved to user %d, want %d", got, user.Id)
	}
}

func TestOfficialNativeLoginCreatesDiscoverablePeer(t *testing.T) {
	database := securityAuditDatabase(t, true)
	if err := database.AutoMigrate(&model.Peer{}, &model.LoginLog{}); err != nil {
		t.Fatal(err)
	}
	isAdmin := false
	user := &model.User{Username: "native-user", Status: model.COMMON_STATUS_ENABLE, IsAdmin: &isAdmin, AuthVersion: 1}
	if err := database.Create(user).Error; err != nil {
		t.Fatal(err)
	}

	token := AllService.UserService.Login(user, &model.LoginLog{
		UserId: user.Id, Client: model.LoginLogClientNative,
		DeviceId: "123 456 789", Uuid: "official-client-uuid", Platform: "windows",
	})
	if token == nil {
		t.Fatal("official native login failed")
	}
	peer := AllService.PeerService.FindByUserIdAndId(user.Id, "123456789")
	if peer.RowId == 0 || peer.Uuid != "official-client-uuid" {
		t.Fatalf("official native login did not create a discoverable peer: %+v", peer)
	}
}

func TestPeerSysinfoRefreshInterval(t *testing.T) {
	now := time.Now().Unix()
	interval := int64(PeerSysinfoRefreshInterval / time.Second)
	peerService := &PeerService{}
	if peerService.NeedsSysinfoRefresh(&model.Peer{LastSysinfoTime: now - interval + 1}, now) {
		t.Fatal("fresh inventory was marked stale before the refresh interval")
	}
	if !peerService.NeedsSysinfoRefresh(&model.Peer{LastSysinfoTime: now - interval}, now) {
		t.Fatal("inventory was not marked stale at the refresh interval")
	}
}
