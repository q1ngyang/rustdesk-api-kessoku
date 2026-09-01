package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/q1ngyang/rustdesk-api-kessoku/v3/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	PresenceLeaseTTL            = 45 * time.Second
	PresenceLegacyFallbackAfter = 90 * time.Second
	presenceLastOnlineInterval  = 30 * time.Second
	maxPresenceRouteLeases      = 16
)

var (
	ErrPresenceActivationInvalid    = errors.New("presence activation is invalid")
	ErrPresenceActivationStale      = errors.New("presence activation is stale")
	ErrPresenceActivationUnverified = errors.New("presence activation is not verified")
	ErrPresenceLeaseInvalid         = errors.New("presence lease is invalid")
	ErrPresenceLeaseExpired         = errors.New("presence lease has expired")
)

type PresenceLeaseGrant struct {
	ActivationEpoch uint64
	ActivationID    string
	LeaseID         string
	LeaseToken      string
	ExpiresAt       int64
	OnlineUntil     int64
}

func (ps *PeerService) VerifyReportActivation(
	ctx context.Context,
	peer *model.Peer,
	activationEpoch uint64,
	activationID string,
	routeLeases []string,
) error {
	if peer == nil || peer.RowId == 0 {
		return ErrPresenceActivationInvalid
	}
	return ps.VerifyActivationIdentity(ctx, peer.Id, peer.Uuid, activationEpoch, activationID, routeLeases)
}

func (ps *PeerService) VerifyActivationIdentity(
	ctx context.Context,
	deviceID string,
	deviceUUID string,
	activationEpoch uint64,
	activationID string,
	routeLeases []string,
) error {
	if deviceID == "" || deviceUUID == "" || validatePresenceActivation(activationEpoch, activationID) != nil || !validRouteLeaseProofs(routeLeases) {
		return ErrPresenceActivationInvalid
	}
	if AllService == nil || AllService.NetworkActivationVerifier == nil {
		return ErrPresenceActivationUnverified
	}
	verified, err := AllService.NetworkActivationVerifier.VerifyPeerActivation(
		ctx,
		deviceID,
		deviceUUID,
		activationEpoch,
		activationID,
		routeLeases,
	)
	if err != nil || !verified {
		return ErrPresenceActivationUnverified
	}
	return nil
}

func (ps *PeerService) StartPresenceLease(
	ctx context.Context,
	peer *model.Peer,
	activationEpoch uint64,
	activationID string,
	clientIP string,
	now int64,
) (*PresenceLeaseGrant, error) {
	if peer == nil || peer.RowId == 0 {
		return nil, ErrPeerIdentityUnverified
	}
	if err := validatePresenceActivation(activationEpoch, activationID); err != nil {
		return nil, err
	}
	unlock := lockPresenceMutation(peer)
	defer unlock()
	leaseID, leaseToken, err := newPresenceLeaseCredentials()
	if err != nil {
		return nil, err
	}
	tokenHash := presenceTokenHash(leaseToken)
	expiresAt := now + int64(PresenceLeaseTTL/time.Second)
	grant := &PresenceLeaseGrant{
		ActivationEpoch: activationEpoch,
		ActivationID:    activationID,
		LeaseID:         leaseID,
		LeaseToken:      leaseToken,
		ExpiresAt:       expiresAt,
	}
	err = DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		current, err := lockPresencePeer(tx, peer)
		if err != nil {
			return err
		}
		if current.PresenceActivationEpoch > activationEpoch ||
			(current.PresenceActivationEpoch == activationEpoch && current.PresenceActivationID != "" && current.PresenceActivationID != activationID) {
			return ErrPresenceActivationStale
		}
		if current.PresenceActivationEpoch == activationEpoch && current.PresenceActivationID == activationID && current.PresenceActivationRetired {
			return ErrPresenceActivationStale
		}
		if activationEpoch > current.PresenceActivationEpoch {
			if err := tx.Model(&model.PeerPresenceLease{}).
				Where("peer_row_id = ? AND ended_at = 0", current.RowId).
				Update("ended_at", now).Error; err != nil {
				return err
			}
		}
		lease := &model.PeerPresenceLease{
			LeaseID:             leaseID,
			PeerRowID:           current.RowId,
			NetworkIdentityUUID: current.Uuid,
			ActivationEpoch:     activationEpoch,
			ActivationID:        activationID,
			TokenHash:           tokenHash,
			StartedAt:           now,
			RenewedAt:           now,
			ExpiresAt:           expiresAt,
			LastOnlineIP:        clientIP,
		}
		if err := tx.Create(lease).Error; err != nil {
			return err
		}
		current.PresenceActivationEpoch = activationEpoch
		current.PresenceActivationID = activationID
		current.PresenceActivationRetired = false
		onlineUntil, err := activePresenceUntil(tx, current, now)
		if err != nil {
			return err
		}
		grant.OnlineUntil = onlineUntil
		return updatePresencePeer(tx, current, clientIP, now, onlineUntil, true, true)
	})
	if err != nil {
		return nil, err
	}
	return grant, nil
}

func (ps *PeerService) RenewPresenceLease(
	ctx context.Context,
	peer *model.Peer,
	activationEpoch uint64,
	activationID string,
	leaseID string,
	leaseToken string,
	clientIP string,
	now int64,
) (*PresenceLeaseGrant, error) {
	if peer == nil || peer.RowId == 0 {
		return nil, ErrPeerIdentityUnverified
	}
	if err := validatePresenceActivation(activationEpoch, activationID); err != nil {
		return nil, err
	}
	if !validPresenceToken(leaseToken) {
		return nil, ErrPresenceLeaseInvalid
	}
	if leaseID != "" && !validPresenceLeaseID(leaseID) {
		return nil, ErrPresenceLeaseInvalid
	}
	unlock := lockPresenceMutation(peer)
	defer unlock()
	expiresAt := now + int64(PresenceLeaseTTL/time.Second)
	grant := &PresenceLeaseGrant{ActivationEpoch: activationEpoch, ActivationID: activationID, LeaseID: leaseID, ExpiresAt: expiresAt}
	err := DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		current, err := lockPresencePeer(tx, peer)
		if err != nil {
			return err
		}
		if current.PresenceActivationEpoch != activationEpoch || current.PresenceActivationID != activationID || current.PresenceActivationRetired {
			return ErrPresenceActivationStale
		}
		lease := &model.PeerPresenceLease{}
		err = presenceLeaseQuery(tx.Clauses(clause.Locking{Strength: "UPDATE"}), current, activationEpoch, activationID, leaseID, leaseToken).
			First(lease).Error
		if errors.Is(err, gorm.ErrRecordNotFound) || lease.EndedAt != 0 {
			return ErrPresenceLeaseInvalid
		}
		if err != nil {
			return err
		}
		if lease.ExpiresAt <= now {
			return ErrPresenceLeaseExpired
		}
		grant.LeaseID = lease.LeaseID
		result := tx.Model(&model.PeerPresenceLease{}).
			Where("row_id = ? AND ended_at = 0 AND expires_at > ?", lease.RowID, now).
			Updates(map[string]interface{}{"renewed_at": now, "expires_at": expiresAt, "last_online_ip": clientIP})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return ErrPresenceLeaseExpired
		}
		onlineUntil, err := activePresenceUntil(tx, current, now)
		if err != nil {
			return err
		}
		grant.OnlineUntil = onlineUntil
		return updatePresencePeer(tx, current, clientIP, now, onlineUntil, true, true)
	})
	if err != nil {
		return nil, err
	}
	return grant, nil
}

func (ps *PeerService) EndPresenceLease(
	ctx context.Context,
	peer *model.Peer,
	activationEpoch uint64,
	activationID string,
	leaseID string,
	leaseToken string,
	clientIP string,
	now int64,
) (*PresenceLeaseGrant, error) {
	if peer == nil || peer.RowId == 0 {
		return nil, ErrPeerIdentityUnverified
	}
	if err := validatePresenceActivation(activationEpoch, activationID); err != nil {
		return nil, err
	}
	if !validPresenceToken(leaseToken) {
		return nil, ErrPresenceLeaseInvalid
	}
	if leaseID != "" && !validPresenceLeaseID(leaseID) {
		return nil, ErrPresenceLeaseInvalid
	}
	unlock := lockPresenceMutation(peer)
	defer unlock()
	grant := &PresenceLeaseGrant{ActivationEpoch: activationEpoch, ActivationID: activationID, LeaseID: leaseID, ExpiresAt: now}
	err := DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		current, err := lockPresencePeer(tx, peer)
		if err != nil {
			return err
		}
		lease := &model.PeerPresenceLease{}
		err = presenceLeaseQuery(tx.Clauses(clause.Locking{Strength: "UPDATE"}), current, activationEpoch, activationID, leaseID, leaseToken).
			First(lease).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrPresenceLeaseInvalid
		}
		if err != nil {
			return err
		}
		grant.LeaseID = lease.LeaseID
		if lease.EndedAt == 0 {
			if err := tx.Model(lease).Where("ended_at = 0").Update("ended_at", now).Error; err != nil {
				return err
			}
		}
		onlineUntil, err := activePresenceUntil(tx, current, now)
		if err != nil {
			return err
		}
		grant.OnlineUntil = onlineUntil
		isCurrent := current.PresenceActivationEpoch == activationEpoch && current.PresenceActivationID == activationID
		return updatePresencePeer(tx, current, clientIP, now, onlineUntil, isCurrent, false)
	})
	if err != nil {
		return nil, err
	}
	return grant, nil
}

func (ps *PeerService) DeactivatePresenceActivation(
	ctx context.Context,
	peer *model.Peer,
	activationEpoch uint64,
	activationID string,
	clientIP string,
	now int64,
) (*PresenceLeaseGrant, error) {
	if peer == nil || peer.RowId == 0 {
		return nil, ErrPeerIdentityUnverified
	}
	if err := validatePresenceActivation(activationEpoch, activationID); err != nil {
		return nil, err
	}
	unlock := lockPresenceMutation(peer)
	defer unlock()
	grant := &PresenceLeaseGrant{ActivationEpoch: activationEpoch, ActivationID: activationID, ExpiresAt: now}
	err := DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		current, err := lockPresencePeer(tx, peer)
		if err != nil {
			return err
		}
		if current.PresenceActivationEpoch > activationEpoch ||
			(current.PresenceActivationEpoch == activationEpoch && current.PresenceActivationID != "" && current.PresenceActivationID != activationID) {
			return ErrPresenceActivationStale
		}
		if err := tx.Model(&model.PeerPresenceLease{}).
			Where("peer_row_id = ? AND ended_at = 0", current.RowId).
			Update("ended_at", now).Error; err != nil {
			return err
		}
		current.PresenceActivationEpoch = activationEpoch
		current.PresenceActivationID = activationID
		current.PresenceActivationRetired = true
		grant.OnlineUntil = 0
		return updatePresencePeer(tx, current, clientIP, now, 0, true, false)
	})
	if err != nil {
		return nil, err
	}
	return grant, nil
}

func (ps *PeerService) IsOnlineAt(peer *model.Peer, now int64) bool {
	if peer == nil {
		return false
	}
	if peer.PresenceOnlineUntil > now {
		return true
	}
	if peer.PresenceV2SeenAt > 0 && now-peer.PresenceV2SeenAt <= int64(PresenceLegacyFallbackAfter/time.Second) {
		return false
	}
	return peer.LastOnlineTime > now-60
}

func validatePresenceActivation(epoch uint64, activationID string) error {
	if epoch == 0 || epoch > math.MaxInt64 || len(activationID) > 128 {
		return ErrPresenceActivationInvalid
	}
	decoded, err := base64.StdEncoding.DecodeString(activationID)
	if err != nil || len(decoded) != 16 || base64.StdEncoding.EncodeToString(decoded) != activationID {
		return ErrPresenceActivationInvalid
	}
	return nil
}

func validRouteLeaseProofs(routeLeases []string) bool {
	if len(routeLeases) == 0 || len(routeLeases) > maxPresenceRouteLeases {
		return false
	}
	for _, routeLease := range routeLeases {
		decoded, err := base64.StdEncoding.DecodeString(routeLease)
		if err != nil || len(decoded) != 32 || base64.StdEncoding.EncodeToString(decoded) != routeLease {
			return false
		}
	}
	return true
}

func validPresenceToken(token string) bool {
	decoded, err := base64.RawURLEncoding.DecodeString(token)
	return err == nil && len(decoded) == 32 && base64.RawURLEncoding.EncodeToString(decoded) == token
}

func validPresenceLeaseID(leaseID string) bool {
	decoded, err := base64.RawURLEncoding.DecodeString(leaseID)
	return err == nil && len(decoded) == 16 && base64.RawURLEncoding.EncodeToString(decoded) == leaseID
}

func newPresenceLeaseCredentials() (string, string, error) {
	credentialBytes := make([]byte, 48)
	if _, err := rand.Read(credentialBytes); err != nil {
		return "", "", err
	}
	return base64.RawURLEncoding.EncodeToString(credentialBytes[:16]),
		base64.RawURLEncoding.EncodeToString(credentialBytes[16:]), nil
}

func presenceTokenHash(token string) string {
	digest := sha256.Sum256([]byte(token))
	return hex.EncodeToString(digest[:])
}

func lockPresenceMutation(peer *model.Peer) func() {
	if Lock == nil || peer == nil || peer.RowId == 0 {
		return func() {}
	}
	key := fmt.Sprintf("presence-peer:%d", peer.RowId)
	Lock.Lock(key)
	return func() { Lock.UnLock(key) }
}

func lockPresencePeer(tx *gorm.DB, peer *model.Peer) (*model.Peer, error) {
	current := &model.Peer{}
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("row_id = ? AND id = ? AND uuid = ?", peer.RowId, peer.Id, peer.Uuid).
		First(current).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrPeerIdentityUnverified
	}
	return current, err
}

func activePresenceUntil(tx *gorm.DB, peer *model.Peer, now int64) (int64, error) {
	var maximum sql.NullInt64
	err := tx.Model(&model.PeerPresenceLease{}).
		Select("MAX(expires_at)").
		Where("peer_row_id = ? AND network_identity_uuid = ? AND activation_epoch = ? AND activation_id = ? AND ended_at = 0 AND expires_at > ?", peer.RowId, peer.Uuid, peer.PresenceActivationEpoch, peer.PresenceActivationID, now).
		Scan(&maximum).Error
	if err != nil || !maximum.Valid {
		return 0, err
	}
	return maximum.Int64, nil
}

func presenceLeaseQuery(tx *gorm.DB, peer *model.Peer, activationEpoch uint64, activationID, leaseID, leaseToken string) *gorm.DB {
	query := tx.Where(
		"peer_row_id = ? AND network_identity_uuid = ? AND activation_epoch = ? AND activation_id = ? AND token_hash = ?",
		peer.RowId, peer.Uuid, activationEpoch, activationID, presenceTokenHash(leaseToken),
	)
	if leaseID != "" {
		query = query.Where("lease_id = ?", leaseID)
	}
	return query
}

func updatePresencePeer(tx *gorm.DB, peer *model.Peer, clientIP string, now, onlineUntil int64, markV2Seen, touchLastOnline bool) error {
	updates := map[string]interface{}{
		"presence_activation_epoch":   peer.PresenceActivationEpoch,
		"presence_activation_id":      peer.PresenceActivationID,
		"presence_activation_retired": peer.PresenceActivationRetired,
		"presence_online_until":       onlineUntil,
	}
	if markV2Seen {
		updates["presence_v2_seen_at"] = now
	}
	if touchLastOnline && (peer.LastOnlineTime == 0 || now-peer.LastOnlineTime >= int64(presenceLastOnlineInterval/time.Second)) {
		updates["last_online_time"] = now
		updates["last_online_ip"] = clientIP
	}
	result := tx.Model(&model.Peer{}).Where("row_id = ? AND uuid = ?", peer.RowId, peer.Uuid).Updates(updates)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return ErrPeerIdentityConflict
	}
	return nil
}
