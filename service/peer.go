package service

import (
	"context"
	"errors"
	"time"

	"github.com/q1ngyang/rustdesk-api-kessoku/v3/model"
	"github.com/q1ngyang/rustdesk-api-kessoku/v3/utils"
	"gorm.io/gorm"
)

var ErrPeerIdentityConflict = errors.New("peer identity conflicts with an existing device")
var ErrPeerIdentityUnverified = errors.New("peer identity is not verified")

const PeerSysinfoRefreshInterval = 24 * time.Hour

// NetworkPeerVerifier checks an ID/UUID pair against the Starry rendezvous
// registry. Public client-report endpoints must not create arbitrary peers
// without either this proof or a currently active native-client login.
type NetworkPeerVerifier interface {
	VerifyPeerIdentity(context.Context, string, string) (bool, error)
}

type PeerService struct {
}

// FindById 根据id查找
func (ps *PeerService) FindById(id string) *model.Peer {
	p := &model.Peer{}
	DB.Where("id = ?", utils.NormalizeRustDeskID(id)).First(p)
	return p
}

func (ps *PeerService) FindByUserIdAndId(userID uint, id string) *model.Peer {
	p := &model.Peer{}
	DB.Where("user_id = ? and id = ?", userID, utils.NormalizeRustDeskID(id)).First(p)
	return p
}
func (ps *PeerService) FindByUuid(uuid string) *model.Peer {
	p := &model.Peer{}
	DB.Where("uuid = ?", uuid).First(p)
	return p
}
func (ps *PeerService) InfoByRowId(id uint) *model.Peer {
	p := &model.Peer{}
	DB.Where("row_id = ?", id).First(p)
	return p
}

// FindByUserIdAndUuid 根据用户id和uuid查找peer
func (ps *PeerService) FindByUserIdAndUuid(uuid string, userId uint) *model.Peer {
	p := &model.Peer{}
	DB.Where("uuid = ? and user_id = ?", uuid, userId).First(p)
	return p
}

// ResolveReportIdentity authorizes a public RustDesk client report and returns
// the canonical peer row. Existing exact identities are cheap; missing or
// changed identities require an active native login or Starry registry proof.
func (ps *PeerService) ResolveReportIdentity(ctx context.Context, deviceID, uuid string) (*model.Peer, error) {
	deviceID = utils.NormalizeRustDeskID(deviceID)
	if deviceID == "" || uuid == "" {
		return nil, ErrPeerIdentityUnverified
	}
	peer := ps.FindById(deviceID)
	if peer.RowId > 0 && peer.Uuid == uuid {
		return peer, nil
	}

	now := time.Now().Unix()
	userID := uint(0)
	if AllService != nil {
		userID = AllService.UserService.FindActiveNativeUserID(uuid, deviceID, now)
	}
	// A login is enough to create a missing identity or safely fill an ID-only
	// administrator placeholder. It cannot overwrite a conflicting UUID.
	if userID > 0 && (peer.RowId == 0 || peer.Uuid == "") {
		if err := ps.BindLoginIdentity(deviceID, uuid, userID); err == nil {
			resolved := ps.FindById(deviceID)
			if resolved.RowId > 0 && resolved.Uuid == uuid && resolved.UserId == userID {
				return resolved, nil
			}
		}
	}

	if AllService == nil || AllService.NetworkPeerVerifier == nil {
		return nil, ErrPeerIdentityUnverified
	}
	verified, err := AllService.NetworkPeerVerifier.VerifyPeerIdentity(ctx, deviceID, uuid)
	if err != nil || !verified {
		return nil, ErrPeerIdentityUnverified
	}
	if err := ps.BindRegistryIdentity(deviceID, uuid, userID); err != nil {
		return nil, err
	}
	resolved := ps.FindById(deviceID)
	if resolved.RowId == 0 || resolved.Uuid != uuid {
		return nil, ErrPeerIdentityUnverified
	}
	return resolved, nil
}

// BindLoginIdentity creates or claims the minimal peer identity established by
// an authenticated native-client login. System information is deliberately
// filled by /api/sysinfo later; an existing non-empty, different UUID is never
// overwritten by login metadata.
func (ps *PeerService) BindLoginIdentity(deviceID, uuid string, userID uint) error {
	deviceID = utils.NormalizeRustDeskID(deviceID)
	if deviceID == "" || uuid == "" || userID == 0 {
		return nil
	}
	verifiedAt := time.Now().Unix()
	return DB.Transaction(func(tx *gorm.DB) error {
		byIDs := make([]model.Peer, 0, 2)
		if err := tx.Where("id = ?", deviceID).Limit(2).Find(&byIDs).Error; err != nil {
			return err
		}
		byUUIDs := make([]model.Peer, 0, 2)
		if err := tx.Where("uuid = ?", uuid).Limit(2).Find(&byUUIDs).Error; err != nil {
			return err
		}
		if len(byIDs) > 1 || len(byUUIDs) > 1 {
			return ErrPeerIdentityConflict
		}
		byID := &model.Peer{}
		if len(byIDs) == 1 {
			byID = &byIDs[0]
		}
		byUUID := &model.Peer{}
		if len(byUUIDs) == 1 {
			byUUID = &byUUIDs[0]
		}

		if byID.RowId > 0 {
			if byID.Uuid != "" && byID.Uuid != uuid {
				return ErrPeerIdentityConflict
			}
			if byID.UserId != 0 && byID.UserId != userID {
				return ErrPeerIdentityConflict
			}
			if byUUID.RowId > 0 && byUUID.RowId != byID.RowId {
				return ErrPeerIdentityConflict
			}
			return tx.Model(byID).Updates(map[string]interface{}{
				"uuid": uuid, "user_id": userID,
				"identity_source": model.PeerIdentitySourceLogin, "identity_verified_at": verifiedAt,
			}).Error
		}
		if byUUID.RowId > 0 {
			if byUUID.UserId != 0 && byUUID.UserId != userID {
				return ErrPeerIdentityConflict
			}
			return tx.Model(byUUID).Updates(map[string]interface{}{
				"id": deviceID, "user_id": userID,
				"identity_source": model.PeerIdentitySourceLogin, "identity_verified_at": verifiedAt,
			}).Error
		}
		return tx.Create(&model.Peer{Id: deviceID, Uuid: uuid, UserId: userID, IdentitySource: model.PeerIdentitySourceLogin, IdentityVerifiedAt: verifiedAt}).Error
	})
}

// BindRegistryIdentity reconciles Kessoku with the authoritative Starry
// rendezvous identity. If an ID now belongs to a different UUID, stale dynamic
// inventory and ownership are cleared unless a valid native login claims the
// newly verified identity.
func (ps *PeerService) BindRegistryIdentity(deviceID, uuid string, userID uint) error {
	deviceID = utils.NormalizeRustDeskID(deviceID)
	if deviceID == "" || uuid == "" {
		return ErrPeerIdentityUnverified
	}
	verifiedAt := time.Now().Unix()
	return DB.Transaction(func(tx *gorm.DB) error {
		byIDs := make([]model.Peer, 0, 2)
		if err := tx.Where("id = ?", deviceID).Limit(2).Find(&byIDs).Error; err != nil {
			return err
		}
		byUUIDs := make([]model.Peer, 0, 2)
		if err := tx.Where("uuid = ?", uuid).Limit(2).Find(&byUUIDs).Error; err != nil {
			return err
		}
		if len(byIDs) > 1 || len(byUUIDs) > 1 {
			return ErrPeerIdentityConflict
		}
		var byID, byUUID *model.Peer
		if len(byIDs) == 1 {
			byID = &byIDs[0]
		}
		if len(byUUIDs) == 1 {
			byUUID = &byUUIDs[0]
		}
		if byID != nil {
			if byUUID != nil && byUUID.RowId != byID.RowId {
				return ErrPeerIdentityConflict
			}
			updates := map[string]interface{}{
				"uuid": uuid, "identity_source": model.PeerIdentitySourceStarry,
				"identity_verified_at": verifiedAt,
			}
			if byID.Uuid != "" && byID.Uuid != uuid {
				updates["cpu"] = ""
				updates["hostname"] = ""
				updates["memory"] = ""
				updates["os"] = ""
				updates["username"] = ""
				updates["version"] = ""
				updates["last_sysinfo_time"] = int64(0)
				updates["user_id"] = userID
			} else if userID > 0 {
				updates["user_id"] = userID
			}
			return tx.Model(byID).Updates(updates).Error
		}
		if byUUID != nil {
			updates := map[string]interface{}{
				"id": deviceID, "identity_source": model.PeerIdentitySourceStarry,
				"identity_verified_at": verifiedAt,
			}
			if userID > 0 {
				updates["user_id"] = userID
			}
			return tx.Model(byUUID).Updates(updates).Error
		}
		return tx.Create(&model.Peer{
			Id: deviceID, Uuid: uuid, UserId: userID,
			IdentitySource: model.PeerIdentitySourceStarry, IdentityVerifiedAt: verifiedAt,
		}).Error
	})
}

// StoreSysinfo atomically replaces every dynamic inventory field, including
// empty values, and refreshes matching address-book metadata. Map updates are
// intentional: GORM struct updates skip zero values and would leave stale data.
func (ps *PeerService) StoreSysinfo(report *model.Peer, clientIP string, now int64) error {
	if report == nil {
		return ErrPeerIdentityUnverified
	}
	report.Id = utils.NormalizeRustDeskID(report.Id)
	return DB.Transaction(func(tx *gorm.DB) error {
		current := &model.Peer{}
		if err := tx.Where("id = ? AND uuid = ?", report.Id, report.Uuid).First(current).Error; err != nil {
			return err
		}
		updates := map[string]interface{}{
			"cpu": report.Cpu, "hostname": report.Hostname, "memory": report.Memory,
			"os": report.Os, "username": report.Username, "version": report.Version,
			"last_sysinfo_time": now, "last_online_time": now, "last_online_ip": clientIP,
		}
		result := tx.Model(&model.Peer{}).Where("row_id = ? AND uuid = ?", current.RowId, report.Uuid).Updates(updates)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return ErrPeerIdentityConflict
		}
		platform := ""
		if AllService != nil {
			platform = AllService.AddressBookService.PlatformFromOs(report.Os)
		}
		return tx.Model(&model.AddressBook{}).Where("id = ?", report.Id).Updates(map[string]interface{}{
			"username": report.Username, "hostname": report.Hostname, "platform": platform,
		}).Error
	})
}

func (ps *PeerService) UpdatePresence(peer *model.Peer, clientIP string, now int64) error {
	if peer == nil || peer.RowId == 0 {
		return ErrPeerIdentityUnverified
	}
	return DB.Model(&model.Peer{}).Where("row_id = ? AND uuid = ?", peer.RowId, peer.Uuid).
		Updates(map[string]interface{}{"last_online_time": now, "last_online_ip": clientIP}).Error
}

func (ps *PeerService) NeedsSysinfoRefresh(peer *model.Peer, now int64) bool {
	return peer == nil || peer.LastSysinfoTime <= 0 || now-peer.LastSysinfoTime >= int64(PeerSysinfoRefreshInterval/time.Second)
}

// UuidBindUserId is retained for callers outside the native login controller.
func (ps *PeerService) UuidBindUserId(deviceID, uuid string, userID uint) {
	_ = ps.BindLoginIdentity(deviceID, uuid, userID)
}

// UuidUnbindUserId 解绑用户id, 用于用户注销
func (ps *PeerService) UuidUnbindUserId(uuid string, userId uint) {
	peer := ps.FindByUserIdAndUuid(uuid, userId)
	if peer.RowId > 0 {
		DB.Model(peer).Update("user_id", 0)
	}
}

// EraseUserId 清除用户id, 用于用户删除
func (ps *PeerService) EraseUserId(userId uint) error {
	return DB.Model(&model.Peer{}).Where("user_id = ?", userId).Update("user_id", 0).Error
}

// ListByUserIds 根据用户id取列表
func (ps *PeerService) ListByUserIds(userIds []uint, page, pageSize uint) (res *model.PeerList) {
	res = &model.PeerList{}
	res.Page = int64(page)
	res.PageSize = int64(pageSize)
	tx := DB.Model(&model.Peer{})
	tx.Where("user_id in (?)", userIds)
	tx.Count(&res.Total)
	tx.Scopes(Paginate(page, pageSize))
	tx.Find(&res.Peers)
	return
}

func (ps *PeerService) List(page, pageSize uint, where func(tx *gorm.DB)) (res *model.PeerList) {
	res = &model.PeerList{}
	res.Page = int64(page)
	res.PageSize = int64(pageSize)
	tx := DB.Model(&model.Peer{})
	if where != nil {
		where(tx)
	}
	tx.Count(&res.Total)
	tx.Scopes(Paginate(page, pageSize))
	tx.Find(&res.Peers)
	return
}

// ListFilterByUserId 根据用户id过滤Peer列表
func (ps *PeerService) ListFilterByUserId(page, pageSize uint, where func(tx *gorm.DB), userId uint) (res *model.PeerList) {
	userWhere := func(tx *gorm.DB) {
		tx.Where("user_id = ?", userId)
		// 如果还有额外的筛选条件，执行它
		if where != nil {
			where(tx)
		}
	}
	return ps.List(page, pageSize, userWhere)
}

// Create 创建
func (ps *PeerService) Create(u *model.Peer) error {
	u.Id = utils.NormalizeRustDeskID(u.Id)
	if u.IdentitySource == "" {
		u.IdentitySource = model.PeerIdentitySourceManual
	}
	res := DB.Create(u).Error
	return res
}

// Delete 删除, 同时也应该删除token
func (ps *PeerService) Delete(u *model.Peer) error {
	uuid := u.Uuid
	tx := DB.Begin()
	if tx.Error != nil {
		return tx.Error
	}
	defer tx.Rollback()
	if err := AllService.AdminScopeService.RemoveResourceScopes(tx, model.AdminScopeTypePeer, []uint{u.RowId}); err != nil {
		return err
	}
	if err := tx.Delete(u).Error; err != nil {
		return err
	}
	if err := tx.Commit().Error; err != nil {
		return err
	}
	// 删除token
	return AllService.UserService.FlushTokenByUuid(uuid)
}

// GetUuidListByIDs 根据ids获取uuid列表
func (ps *PeerService) GetUuidListByIDs(ids []uint) ([]string, error) {
	var uuids []string
	err := DB.Model(&model.Peer{}).
		Where("row_id in (?)", ids).
		Pluck("uuid", &uuids).Error
	//过滤uuids中的空字符串
	var newUuids []string
	for _, uuid := range uuids {
		if uuid != "" {
			newUuids = append(newUuids, uuid)
		}
	}
	return newUuids, err
}

// BatchDelete 批量删除, 同时也应该删除token
func (ps *PeerService) BatchDelete(ids []uint) error {
	uuids, err := ps.GetUuidListByIDs(ids)
	if err != nil {
		return err
	}
	tx := DB.Begin()
	if tx.Error != nil {
		return tx.Error
	}
	defer tx.Rollback()
	if err := AllService.AdminScopeService.RemoveResourceScopes(tx, model.AdminScopeTypePeer, ids); err != nil {
		return err
	}
	if err := tx.Where("row_id in (?)", ids).Delete(&model.Peer{}).Error; err != nil {
		return err
	}
	if err := tx.Commit().Error; err != nil {
		return err
	}
	// 删除token
	return AllService.UserService.FlushTokenByUuids(uuids)
}

// Update 更新
func (ps *PeerService) Update(u *model.Peer) error {
	if u.Id != "" {
		u.Id = utils.NormalizeRustDeskID(u.Id)
	}
	return DB.Model(u).Updates(u).Error
}
