package service

import (
	"context"
	"encoding/base64"
	"errors"
	"strings"
	"sync"
	"testing"

	servicelock "github.com/q1ngyang/rustdesk-api-kessoku/v3/lib/lock"
	"github.com/q1ngyang/rustdesk-api-kessoku/v3/model"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestPresenceLeaseRejectsDelayedPreviousActivation(t *testing.T) {
	database, peer := presenceTestDatabase(t)
	firstID := presenceActivationID(1)
	secondID := presenceActivationID(2)

	first, err := AllService.PeerService.StartPresenceLease(context.Background(), peer, 1, firstID, "192.0.2.1", 1_000)
	if err != nil {
		t.Fatal(err)
	}
	second, err := AllService.PeerService.StartPresenceLease(context.Background(), peer, 2, secondID, "192.0.2.2", 1_005)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := AllService.PeerService.RenewPresenceLease(context.Background(), peer, 1, firstID, first.LeaseID, first.LeaseToken, "192.0.2.1", 1_006); !errors.Is(err, ErrPresenceActivationStale) {
		t.Fatalf("old activation renewal error = %v", err)
	}
	if _, err := AllService.PeerService.EndPresenceLease(context.Background(), peer, 1, firstID, first.LeaseID, first.LeaseToken, "192.0.2.1", 1_007); err != nil {
		t.Fatalf("idempotent delayed end failed: %v", err)
	}

	stored := &model.Peer{}
	if err := database.First(stored, peer.RowId).Error; err != nil {
		t.Fatal(err)
	}
	if stored.PresenceActivationEpoch != 2 || stored.PresenceActivationID != secondID || stored.PresenceOnlineUntil != second.ExpiresAt {
		t.Fatalf("delayed old activation changed current presence: %+v", stored)
	}
	if !AllService.PeerService.IsOnlineAt(stored, 1_007) {
		t.Fatal("current activation was taken offline by delayed old end")
	}
}

func TestPresenceLeaseUsesAnyActiveLeaseAndStoresOnlyTokenHash(t *testing.T) {
	database, peer := presenceTestDatabase(t)
	activationID := presenceActivationID(3)
	first, err := AllService.PeerService.StartPresenceLease(context.Background(), peer, 4, activationID, "198.51.100.1", 2_000)
	if err != nil {
		t.Fatal(err)
	}
	second, err := AllService.PeerService.StartPresenceLease(context.Background(), peer, 4, activationID, "198.51.100.2", 2_001)
	if err != nil {
		t.Fatal(err)
	}
	if first.LeaseToken == second.LeaseToken || first.LeaseID == second.LeaseID {
		t.Fatal("parallel leases reused a lease credential")
	}
	if !validPresenceToken(first.LeaseToken) || !validPresenceToken(second.LeaseToken) {
		t.Fatal("start did not return a canonical 256-bit lease token")
	}
	if !validPresenceLeaseID(first.LeaseID) || !validPresenceLeaseID(second.LeaseID) {
		t.Fatalf("invalid lease IDs: first=%q second=%q", first.LeaseID, second.LeaseID)
	}
	if _, err := AllService.PeerService.EndPresenceLease(context.Background(), peer, 4, activationID, first.LeaseID, first.LeaseToken, "198.51.100.1", 2_002); err != nil {
		t.Fatal(err)
	}

	stored := &model.Peer{}
	if err := database.First(stored, peer.RowId).Error; err != nil {
		t.Fatal(err)
	}
	if stored.PresenceOnlineUntil != second.ExpiresAt || !AllService.PeerService.IsOnlineAt(stored, 2_002) {
		t.Fatalf("ending one parallel lease incorrectly took peer offline: %+v", stored)
	}
	var leases []model.PeerPresenceLease
	if err := database.Order("row_id").Find(&leases).Error; err != nil {
		t.Fatal(err)
	}
	if len(leases) != 2 {
		t.Fatalf("lease count = %d, want 2", len(leases))
	}
	for _, lease := range leases {
		if lease.TokenHash == first.LeaseToken || lease.TokenHash == second.LeaseToken || strings.Contains(lease.TokenHash, first.LeaseToken) || len(lease.TokenHash) != 64 {
			t.Fatalf("raw bearer token was persisted: %+v", lease)
		}
		if lease.NetworkIdentityUUID != peer.Uuid || !validPresenceLeaseID(lease.LeaseID) {
			t.Fatalf("lease identity was not bound to the network UUID: %+v", lease)
		}
	}
}

func TestPresenceLeaseIDAndTokenMustSelectTheSameLease(t *testing.T) {
	_, peer := presenceTestDatabase(t)
	activationID := presenceActivationID(12)
	first, err := AllService.PeerService.StartPresenceLease(context.Background(), peer, 12, activationID, "198.51.100.20", 2_100)
	if err != nil {
		t.Fatal(err)
	}
	second, err := AllService.PeerService.StartPresenceLease(context.Background(), peer, 12, activationID, "198.51.100.21", 2_101)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := AllService.PeerService.RenewPresenceLease(context.Background(), peer, 12, activationID, second.LeaseID, first.LeaseToken, "198.51.100.20", 2_102); !errors.Is(err, ErrPresenceLeaseInvalid) {
		t.Fatalf("mismatched lease ID/token renewal error = %v", err)
	}
	if _, err := AllService.PeerService.EndPresenceLease(context.Background(), peer, 12, activationID, first.LeaseID, second.LeaseToken, "198.51.100.21", 2_103); !errors.Is(err, ErrPresenceLeaseInvalid) {
		t.Fatalf("mismatched lease ID/token end error = %v", err)
	}
}

func TestPresenceActivationDeactivationRetiresEpochAndEndsEveryLease(t *testing.T) {
	database, peer := presenceTestDatabase(t)
	activationID := presenceActivationID(7)
	if _, err := AllService.PeerService.StartPresenceLease(context.Background(), peer, 20, activationID, "198.51.100.10", 2_500); err != nil {
		t.Fatal(err)
	}
	if _, err := AllService.PeerService.StartPresenceLease(context.Background(), peer, 20, activationID, "198.51.100.11", 2_501); err != nil {
		t.Fatal(err)
	}
	if _, err := AllService.PeerService.DeactivatePresenceActivation(context.Background(), peer, 20, activationID, "198.51.100.12", 2_502); err != nil {
		t.Fatal(err)
	}
	stored := &model.Peer{}
	if err := database.First(stored, peer.RowId).Error; err != nil {
		t.Fatal(err)
	}
	if !stored.PresenceActivationRetired || stored.PresenceOnlineUntil != 0 || AllService.PeerService.IsOnlineAt(stored, 2_502) {
		t.Fatalf("deactivated activation remained online: %+v", stored)
	}
	var active int64
	if err := database.Model(&model.PeerPresenceLease{}).Where("peer_row_id = ? AND ended_at = 0", peer.RowId).Count(&active).Error; err != nil {
		t.Fatal(err)
	}
	if active != 0 {
		t.Fatalf("active leases after activation deactivation = %d", active)
	}
	if _, err := AllService.PeerService.StartPresenceLease(context.Background(), peer, 20, activationID, "198.51.100.13", 2_503); !errors.Is(err, ErrPresenceActivationStale) {
		t.Fatalf("retired activation replay error = %v", err)
	}
	if _, err := AllService.PeerService.StartPresenceLease(context.Background(), peer, 21, presenceActivationID(8), "198.51.100.14", 2_504); err != nil {
		t.Fatalf("higher activation did not supersede retired epoch: %v", err)
	}
}

func TestPresenceActivationCanBeRetiredBeforeDelayedStartArrives(t *testing.T) {
	_, peer := presenceTestDatabase(t)
	activationID := presenceActivationID(9)
	if _, err := AllService.PeerService.DeactivatePresenceActivation(context.Background(), peer, 30, activationID, "203.0.113.30", 2_700); err != nil {
		t.Fatal(err)
	}
	if _, err := AllService.PeerService.StartPresenceLease(context.Background(), peer, 30, activationID, "203.0.113.31", 2_701); !errors.Is(err, ErrPresenceActivationStale) {
		t.Fatalf("delayed start after activation retirement error = %v", err)
	}
}

func TestConcurrentPresenceStartAndDeactivateCannotResurrectActivation(t *testing.T) {
	database, peer := presenceTestDatabase(t)
	for iteration := 0; iteration < 20; iteration++ {
		epoch := uint64(100 + iteration)
		activationID := presenceActivationID(byte(iteration + 40))
		start := make(chan struct{})
		errorsByOperation := make(chan struct {
			operation string
			err       error
		}, 2)
		var wait sync.WaitGroup
		wait.Add(2)
		go func() {
			defer wait.Done()
			<-start
			_, err := AllService.PeerService.StartPresenceLease(context.Background(), peer, epoch, activationID, "203.0.113.40", int64(3_000+iteration))
			errorsByOperation <- struct {
				operation string
				err       error
			}{"start", err}
		}()
		go func() {
			defer wait.Done()
			<-start
			_, err := AllService.PeerService.DeactivatePresenceActivation(context.Background(), peer, epoch, activationID, "203.0.113.41", int64(3_000+iteration))
			errorsByOperation <- struct {
				operation string
				err       error
			}{"deactivate", err}
		}()
		close(start)
		wait.Wait()
		close(errorsByOperation)
		for result := range errorsByOperation {
			if result.operation == "deactivate" && result.err != nil {
				t.Fatalf("iteration %d deactivate error = %v", iteration, result.err)
			}
			if result.operation == "start" && result.err != nil && !errors.Is(result.err, ErrPresenceActivationStale) {
				t.Fatalf("iteration %d start error = %v", iteration, result.err)
			}
		}
		stored := &model.Peer{}
		if err := database.First(stored, peer.RowId).Error; err != nil {
			t.Fatal(err)
		}
		if stored.PresenceActivationEpoch != epoch || stored.PresenceActivationID != activationID || !stored.PresenceActivationRetired || stored.PresenceOnlineUntil != 0 {
			t.Fatalf("iteration %d activation resurrected: %+v", iteration, stored)
		}
	}
}

func TestPresenceLeaseExpiryAndLegacyFallback(t *testing.T) {
	database, peer := presenceTestDatabase(t)
	activationID := presenceActivationID(4)
	grant, err := AllService.PeerService.StartPresenceLease(context.Background(), peer, 7, activationID, "203.0.113.1", 3_000)
	if err != nil {
		t.Fatal(err)
	}
	stored := &model.Peer{}
	if err := database.First(stored, peer.RowId).Error; err != nil {
		t.Fatal(err)
	}
	if !AllService.PeerService.IsOnlineAt(stored, grant.ExpiresAt-1) {
		t.Fatal("lease became offline before its TTL")
	}
	if AllService.PeerService.IsOnlineAt(stored, grant.ExpiresAt) {
		t.Fatal("expired v2 lease remained online through the legacy timestamp")
	}

	legacyHeartbeatAt := int64(3_080)
	if err := database.Model(stored).Update("last_online_time", legacyHeartbeatAt).Error; err != nil {
		t.Fatal(err)
	}
	if err := database.First(stored, peer.RowId).Error; err != nil {
		t.Fatal(err)
	}
	if !AllService.PeerService.IsOnlineAt(stored, 3_091) {
		t.Fatal("legacy heartbeat did not recover after the v2 downgrade window")
	}
}

func TestPresenceLeaseClientCrashExpiresWithoutDatabaseHeartbeatWrites(t *testing.T) {
	database, peer := presenceTestDatabase(t)
	activationID := presenceActivationID(13)
	grant, err := AllService.PeerService.StartPresenceLease(context.Background(), peer, 13, activationID, "203.0.113.13", 5_000)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := AllService.PeerService.RenewPresenceLease(context.Background(), peer, 13, activationID, grant.LeaseID, grant.LeaseToken, "203.0.113.13", 5_015); err != nil {
		t.Fatal(err)
	}
	stored := &model.Peer{}
	if err := database.First(stored, peer.RowId).Error; err != nil {
		t.Fatal(err)
	}
	if stored.LastOnlineTime != 5_000 {
		t.Fatalf("renew increased legacy heartbeat write frequency: last_online_time=%d", stored.LastOnlineTime)
	}
	if !AllService.PeerService.IsOnlineAt(stored, 5_059) || AllService.PeerService.IsOnlineAt(stored, 5_060) {
		t.Fatalf("crashed client TTL boundary is wrong: %+v", stored)
	}
}

func TestPresenceLeaseABAAndProfileIsolation(t *testing.T) {
	database, profileA := presenceTestDatabase(t)
	profileA.UserId = 41
	if err := database.Model(profileA).Update("user_id", profileA.UserId).Error; err != nil {
		t.Fatal(err)
	}
	profileB := &model.Peer{Id: "301132037", Uuid: "MDEyMzQ1Njc4OWFiY2RlZw==", UserId: 42}
	if err := database.Create(profileB).Error; err != nil {
		t.Fatal(err)
	}

	a1, err := AllService.PeerService.StartPresenceLease(context.Background(), profileA, 1, presenceActivationID(20), "192.0.2.41", 7_000)
	if err != nil {
		t.Fatal(err)
	}
	b1, err := AllService.PeerService.StartPresenceLease(context.Background(), profileB, 1, presenceActivationID(21), "192.0.2.42", 7_001)
	if err != nil {
		t.Fatal(err)
	}
	a2, err := AllService.PeerService.StartPresenceLease(context.Background(), profileA, 2, presenceActivationID(22), "192.0.2.43", 7_002)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := AllService.PeerService.EndPresenceLease(context.Background(), profileA, 1, presenceActivationID(20), a1.LeaseID, a1.LeaseToken, "192.0.2.41", 7_003); err != nil {
		t.Fatal(err)
	}
	if _, err := AllService.PeerService.EndPresenceLease(context.Background(), profileB, 1, presenceActivationID(21), b1.LeaseID, b1.LeaseToken, "192.0.2.42", 7_004); err != nil {
		t.Fatal(err)
	}

	storedA, storedB := &model.Peer{}, &model.Peer{}
	if err := database.First(storedA, profileA.RowId).Error; err != nil {
		t.Fatal(err)
	}
	if err := database.First(storedB, profileB.RowId).Error; err != nil {
		t.Fatal(err)
	}
	if storedA.Uuid != profileA.Uuid || storedA.UserId != 41 || storedA.PresenceOnlineUntil != a2.ExpiresAt || !AllService.PeerService.IsOnlineAt(storedA, 7_004) {
		t.Fatalf("A→B→A lost the current A identity or presence: %+v", storedA)
	}
	if storedB.Uuid != profileB.Uuid || storedB.UserId != 42 || AllService.PeerService.IsOnlineAt(storedB, 7_004) {
		t.Fatalf("profile B identity or presence leaked into profile A: %+v", storedB)
	}
}

func TestOutOfOrderRenewAfterEndCannotResurrectLease(t *testing.T) {
	database, peer := presenceTestDatabase(t)
	activationID := presenceActivationID(23)
	grant, err := AllService.PeerService.StartPresenceLease(context.Background(), peer, 23, activationID, "198.51.100.23", 8_000)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := AllService.PeerService.EndPresenceLease(context.Background(), peer, 23, activationID, grant.LeaseID, grant.LeaseToken, "198.51.100.23", 8_001); err != nil {
		t.Fatal(err)
	}
	if _, err := AllService.PeerService.RenewPresenceLease(context.Background(), peer, 23, activationID, grant.LeaseID, grant.LeaseToken, "198.51.100.23", 8_002); !errors.Is(err, ErrPresenceLeaseInvalid) {
		t.Fatalf("renew after end error = %v", err)
	}
	if _, err := AllService.PeerService.EndPresenceLease(context.Background(), peer, 23, activationID, grant.LeaseID, grant.LeaseToken, "198.51.100.23", 8_003); err != nil {
		t.Fatalf("duplicate end was not idempotent: %v", err)
	}
	stored := &model.Peer{}
	if err := database.First(stored, peer.RowId).Error; err != nil {
		t.Fatal(err)
	}
	if stored.PresenceOnlineUntil != 0 || AllService.PeerService.IsOnlineAt(stored, 8_003) {
		t.Fatalf("ended lease was resurrected: %+v", stored)
	}
}

func TestConcurrentPresenceRenewAndEndNeverResurrectsLease(t *testing.T) {
	database, peer := presenceTestDatabase(t)
	for iteration := 0; iteration < 20; iteration++ {
		epoch := uint64(300 + iteration)
		activationID := presenceActivationID(byte(60 + iteration))
		grant, err := AllService.PeerService.StartPresenceLease(context.Background(), peer, epoch, activationID, "203.0.113.60", int64(9_000+iteration*100))
		if err != nil {
			t.Fatal(err)
		}
		start := make(chan struct{})
		errorsFound := make(chan error, 2)
		var wait sync.WaitGroup
		wait.Add(2)
		go func() {
			defer wait.Done()
			<-start
			_, renewErr := AllService.PeerService.RenewPresenceLease(context.Background(), peer, epoch, activationID, grant.LeaseID, grant.LeaseToken, "203.0.113.61", int64(9_001+iteration*100))
			errorsFound <- renewErr
		}()
		go func() {
			defer wait.Done()
			<-start
			_, endErr := AllService.PeerService.EndPresenceLease(context.Background(), peer, epoch, activationID, grant.LeaseID, grant.LeaseToken, "203.0.113.62", int64(9_001+iteration*100))
			errorsFound <- endErr
		}()
		close(start)
		wait.Wait()
		close(errorsFound)
		for operationErr := range errorsFound {
			if operationErr != nil && !errors.Is(operationErr, ErrPresenceLeaseInvalid) {
				t.Fatalf("iteration %d concurrent operation error = %v", iteration, operationErr)
			}
		}
		stored := &model.Peer{}
		if err := database.First(stored, peer.RowId).Error; err != nil {
			t.Fatal(err)
		}
		if stored.PresenceOnlineUntil != 0 || AllService.PeerService.IsOnlineAt(stored, int64(9_001+iteration*100)) {
			t.Fatalf("iteration %d concurrent end was resurrected: %+v", iteration, stored)
		}
	}
}

func TestPresenceMetricsContainOnlyBoundedCountersAndDatabaseGauges(t *testing.T) {
	database, peer := presenceTestDatabase(t)
	resetPresenceMetricsForTest()
	t.Cleanup(resetPresenceMetricsForTest)
	activationID := presenceActivationID(90)
	if _, err := AllService.PeerService.StartPresenceLease(context.Background(), peer, 90, activationID, "203.0.113.90", 12_000); err != nil {
		t.Fatal(err)
	}
	if _, err := AllService.PeerService.StartPresenceLease(context.Background(), peer, 90, activationID, "203.0.113.91", 12_000); err != nil {
		t.Fatal(err)
	}
	expiredLease := &model.PeerPresenceLease{
		LeaseID: base64.RawURLEncoding.EncodeToString(make([]byte, 16)), PeerRowID: peer.RowId,
		NetworkIdentityUUID: peer.Uuid, ActivationEpoch: 90, ActivationID: activationID,
		TokenHash: strings.Repeat("e", 64), StartedAt: 11_900, RenewedAt: 11_900,
		ExpiresAt: 11_999,
	}
	if err := database.Create(expiredLease).Error; err != nil {
		t.Fatal(err)
	}
	ObservePresenceRequest(PresenceOperationStart, 200)
	ObservePresenceRequest(PresenceOperationRenew, 401)
	ObservePresenceRequest(PresenceOperationEnd, 500)
	snapshot, err := AllService.PeerService.PresenceMetricsSnapshot(context.Background(), 12_001)
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.SchemaVersion != 1 || snapshot.CounterScope != "process" || snapshot.GaugeScope != "database" || snapshot.ActiveLeases != 2 || snapshot.OnlinePeers != 1 || snapshot.ExpiredUnendedLeases != 1 {
		t.Fatalf("unexpected presence gauges: %+v", snapshot)
	}
	if snapshot.StartAcceptedTotal != 1 || snapshot.RenewRejectedTotal != 1 || snapshot.EndErrorsTotal != 1 {
		t.Fatalf("unexpected presence counters: %+v", snapshot)
	}
}

func TestPresenceActivationValidationRejectsMismatchedSameEpoch(t *testing.T) {
	_, peer := presenceTestDatabase(t)
	if _, err := AllService.PeerService.StartPresenceLease(context.Background(), peer, 11, presenceActivationID(5), "192.0.2.10", 4_000); err != nil {
		t.Fatal(err)
	}
	if _, err := AllService.PeerService.StartPresenceLease(context.Background(), peer, 11, presenceActivationID(6), "192.0.2.11", 4_001); !errors.Is(err, ErrPresenceActivationStale) {
		t.Fatalf("same-epoch different activation error = %v", err)
	}
	if validRouteLeaseProofs([]string{base64.StdEncoding.EncodeToString(make([]byte, 31))}) {
		t.Fatal("short route lease proof was accepted")
	}
}

func presenceTestDatabase(t *testing.T) (*gorm.DB, *model.Peer) {
	t.Helper()
	oldDB, oldServices, oldLock := DB, AllService, Lock
	t.Cleanup(func() { DB, AllService, Lock = oldDB, oldServices, oldLock })
	database, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := database.AutoMigrate(&model.Peer{}, &model.PeerPresenceLease{}); err != nil {
		t.Fatal(err)
	}
	DB = database
	Lock = servicelock.NewLocal()
	AllService = &Service{PeerService: &PeerService{}}
	peer := &model.Peer{Id: "301132036", Uuid: "MDEyMzQ1Njc4OWFiY2RlZg=="}
	if err := database.Create(peer).Error; err != nil {
		t.Fatal(err)
	}
	return database, peer
}

func presenceActivationID(value byte) string {
	return base64.StdEncoding.EncodeToString(make([]byte, 15))[:20] + base64.StdEncoding.EncodeToString([]byte{value})
}
