package service

import (
	"errors"

	"github.com/q1ngyang/rustdesk-api-kessoku/v3/model"
	"github.com/q1ngyang/rustdesk-api-kessoku/v3/utils"
	"gorm.io/gorm"
)

var ErrPeerIdentityConflict = errors.New("peer identity conflicts with an existing device")

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

// BindLoginIdentity creates or claims the minimal peer identity established by
// an authenticated native-client login. System information is deliberately
// filled by /api/sysinfo later; an existing non-empty, different UUID is never
// overwritten by login metadata.
func (ps *PeerService) BindLoginIdentity(deviceID, uuid string, userID uint) error {
	deviceID = utils.NormalizeRustDeskID(deviceID)
	if deviceID == "" || uuid == "" || userID == 0 {
		return nil
	}
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
			return tx.Model(byID).Updates(map[string]interface{}{"uuid": uuid, "user_id": userID}).Error
		}
		if byUUID.RowId > 0 {
			if byUUID.UserId != 0 && byUUID.UserId != userID {
				return ErrPeerIdentityConflict
			}
			return tx.Model(byUUID).Updates(map[string]interface{}{"id": deviceID, "user_id": userID}).Error
		}
		return tx.Create(&model.Peer{Id: deviceID, Uuid: uuid, UserId: userID}).Error
	})
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
