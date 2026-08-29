package service

import (
	"errors"
	"testing"

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
