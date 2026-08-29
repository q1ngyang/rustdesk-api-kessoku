package service

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"strings"

	"github.com/q1ngyang/rustdesk-api-kessoku/v3/model"
	"gorm.io/gorm"
)

type AuditService struct {
}

func (as *AuditService) AuditConnList(page, pageSize uint, where func(tx *gorm.DB) *gorm.DB) (res *model.AuditConnList) {
	res = &model.AuditConnList{}
	res.Page = int64(page)
	res.PageSize = int64(pageSize)
	tx := DB.Model(&model.AuditConn{})
	if where != nil {
		tx = where(tx)
	}
	tx.Count(&res.Total)
	tx.Order("created_at DESC").Order("id DESC").Scopes(Paginate(page, pageSize)).Find(&res.AuditConns)
	as.enrichAuditConns(res.AuditConns)
	return
}

// Create 创建
func (as *AuditService) CreateAuditConn(u *model.AuditConn) error {
	if u == nil {
		return errors.New("cannot create empty connection audit")
	}
	as.enrichAuditConns([]*model.AuditConn{u})
	res := DB.Create(u).Error
	return res
}
func (as *AuditService) DeleteAuditConn(u *model.AuditConn) error {
	return as.DeleteAuditConnContext(context.Background(), 0, "", u)
}

func (as *AuditService) DeleteAuditConnContext(ctx context.Context, actorUserID uint, requestID string, u *model.AuditConn) (operationErr error) {
	if u == nil || u.Id == 0 {
		return errors.New("cannot delete empty connection audit")
	}
	event, auditErr := beginSecurityAudit(ctx, actorUserID, requestID, "audit.connection.deleted", "audit_connection", strconv.FormatUint(uint64(u.Id), 10), nil)
	if auditErr != nil {
		return auditErr
	}
	defer finalizeSecurityAudit(event, &operationErr, "AUDIT_CONNECTION_DELETE_FAILED")
	return DB.Delete(u).Error
}

// Update 更新
func (as *AuditService) UpdateAuditConn(u *model.AuditConn) error {
	return DB.Model(u).Updates(u).Error
}

func (as *AuditService) CloseWebClientAudit(ctx context.Context, id, userID uint, sessionID string, closeTime int64) error {
	if id == 0 || userID == 0 || sessionID == "" || closeTime <= 0 {
		return errors.New("invalid WebClient connection audit close")
	}
	query := DB.WithContext(ctx).Model(&model.AuditConn{}).
		Where("id = ? AND user_id = ? AND client = ? AND session_id = ?", id, userID, model.LoginLogClientWeb, sessionID)
	result := query.Where("close_time = 0").Update("close_time", closeTime)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 1 {
		return nil
	}
	var count int64
	if err := DB.WithContext(ctx).Model(&model.AuditConn{}).
		Where("id = ? AND user_id = ? AND client = ? AND session_id = ?", id, userID, model.LoginLogClientWeb, sessionID).
		Count(&count).Error; err != nil {
		return err
	}
	if count == 1 {
		return nil
	}
	return errors.New("WebClient connection audit not found")
}

// InfoByPeerIdAndConnId
func (as *AuditService) InfoByPeerIdAndConnId(peerId string, connId int64) (res *model.AuditConn) {
	res = &model.AuditConn{}
	DB.Where("peer_id = ? and conn_id = ?", peerId, connId).First(res)
	return
}

// ConnInfoById
func (as *AuditService) ConnInfoById(id uint) (res *model.AuditConn) {
	res = &model.AuditConn{}
	DB.Where("id = ?", id).First(res)
	return
}

// FileInfoById
func (as *AuditService) FileInfoById(id uint) (res *model.AuditFile) {
	res = &model.AuditFile{}
	DB.Where("id = ?", id).First(res)
	return
}

func (as *AuditService) AuditFileList(page, pageSize uint, where func(tx *gorm.DB) *gorm.DB) (res *model.AuditFileList) {
	res = &model.AuditFileList{}
	res.Page = int64(page)
	res.PageSize = int64(pageSize)
	tx := DB.Model(&model.AuditFile{})
	if where != nil {
		tx = where(tx)
	}
	tx.Count(&res.Total)
	tx.Order("created_at DESC").Order("id DESC").Scopes(Paginate(page, pageSize)).Find(&res.AuditFiles)
	as.enrichAuditFiles(res.AuditFiles)
	return
}

// CreateAuditFile
func (as *AuditService) CreateAuditFile(u *model.AuditFile) error {
	if u == nil {
		return errors.New("cannot create empty file audit")
	}
	as.enrichAuditFiles([]*model.AuditFile{u})
	res := DB.Create(u).Error
	return res
}

type auditIdentityMaps struct {
	peers map[string]model.Peer
	users map[uint]string
}

func loadAuditIdentities(peerIDs []string, userIDs []uint) auditIdentityMaps {
	result := auditIdentityMaps{peers: map[string]model.Peer{}, users: map[uint]string{}}
	uniquePeerIDs := map[string]struct{}{}
	for _, id := range peerIDs {
		if id != "" {
			uniquePeerIDs[id] = struct{}{}
		}
	}
	ids := make([]string, 0, len(uniquePeerIDs))
	for id := range uniquePeerIDs {
		ids = append(ids, id)
	}
	var peers []model.Peer
	if DB != nil && len(ids) > 0 && DB.Migrator().HasTable(&model.Peer{}) {
		DB.Where("id IN ?", ids).Find(&peers)
	}
	for _, peer := range peers {
		result.peers[peer.Id] = peer
		if peer.UserId > 0 {
			userIDs = append(userIDs, peer.UserId)
		}
	}
	uniqueUserIDs := map[uint]struct{}{}
	for _, id := range userIDs {
		if id > 0 {
			uniqueUserIDs[id] = struct{}{}
		}
	}
	userIDList := make([]uint, 0, len(uniqueUserIDs))
	for id := range uniqueUserIDs {
		userIDList = append(userIDList, id)
	}
	var users []model.User
	if DB != nil && len(userIDList) > 0 && DB.Migrator().HasTable(&model.User{}) {
		DB.Select("id", "username").Where("id IN ?", userIDList).Find(&users)
	}
	for _, user := range users {
		result.users[user.Id] = user.Username
	}
	return result
}

func enrichAuditEndpoint(controllerID, controlledID string, explicitControllerUserID uint, identities auditIdentityMaps) (controllerUsername, controlledUsername, controlledIP, controlledUUID string) {
	controllerUserID := explicitControllerUserID
	if controllerUserID == 0 {
		controllerUserID = identities.peers[controllerID].UserId
	}
	controlledPeer := identities.peers[controlledID]
	return identities.users[controllerUserID], identities.users[controlledPeer.UserId], controlledPeer.LastOnlineIp, controlledPeer.Uuid
}

func (as *AuditService) enrichAuditConns(items []*model.AuditConn) {
	peerIDs := make([]string, 0, len(items)*2)
	userIDs := make([]uint, 0, len(items))
	for _, item := range items {
		if item == nil {
			continue
		}
		peerIDs = append(peerIDs, item.FromPeer, item.PeerId)
		userIDs = append(userIDs, item.UserId)
	}
	identities := loadAuditIdentities(peerIDs, userIDs)
	for _, item := range items {
		if item == nil {
			continue
		}
		controller, controlled, controlledIP, uuid := enrichAuditEndpoint(item.FromPeer, item.PeerId, item.UserId, identities)
		if controller != "" {
			item.ControllerUsername = controller
			item.FromName = controller
		}
		if controlled != "" {
			item.ControlledUsername = controlled
		}
		if item.ControlledIP == "" {
			item.ControlledIP = controlledIP
		}
		if item.Uuid == "" {
			item.Uuid = uuid
		}
	}
}

func controlledAuditPaths(basePath, rawInfo string) []string {
	var info struct {
		Files [][]interface{} `json:"files"`
	}
	if rawInfo != "" && json.Unmarshal([]byte(rawInfo), &info) == nil && len(info.Files) > 0 {
		paths := make([]string, 0, len(info.Files))
		for _, file := range info.Files {
			name := ""
			if len(file) > 0 {
				name, _ = file[0].(string)
			}
			if fullPath := joinAuditPath(basePath, name); fullPath != "" {
				paths = append(paths, fullPath)
			}
		}
		if len(paths) > 0 {
			return paths
		}
	}
	if basePath == "" {
		return nil
	}
	return []string{basePath}
}

// joinAuditPath is deliberately OS-neutral: the API can run on Linux while
// the controlled endpoint reports Windows paths. It preserves the endpoint's
// separator style instead of applying the server filesystem's path rules.
func joinAuditPath(basePath, name string) string {
	if name == "" {
		return basePath
	}
	if basePath == "" || strings.HasPrefix(name, "/") || strings.HasPrefix(name, `\`) ||
		(len(name) >= 3 && name[1] == ':' && (name[2] == '/' || name[2] == '\\')) {
		return name
	}
	separator := "/"
	if strings.Contains(basePath, `\`) && !strings.Contains(basePath, "/") {
		separator = `\`
	}
	return strings.TrimRight(basePath, `/\`) + separator + strings.TrimLeft(name, `/\`)
}

func (as *AuditService) enrichAuditFiles(items []*model.AuditFile) {
	peerIDs := make([]string, 0, len(items)*2)
	for _, item := range items {
		if item != nil {
			peerIDs = append(peerIDs, item.FromPeer, item.PeerId)
		}
	}
	identities := loadAuditIdentities(peerIDs, nil)
	for _, item := range items {
		if item == nil {
			continue
		}
		controller, controlled, controlledIP, uuid := enrichAuditEndpoint(item.FromPeer, item.PeerId, 0, identities)
		if controller != "" {
			item.ControllerUsername = controller
			item.FromName = controller
		}
		if controlled != "" {
			item.ControlledUsername = controlled
		}
		if item.ControlledIP == "" {
			item.ControlledIP = controlledIP
		}
		if item.Uuid == "" {
			item.Uuid = uuid
		}
		item.ControlledPaths = controlledAuditPaths(item.Path, item.Info)
	}
}
func (as *AuditService) DeleteAuditFile(u *model.AuditFile) error {
	return as.DeleteAuditFileContext(context.Background(), 0, "", u)
}

func (as *AuditService) DeleteAuditFileContext(ctx context.Context, actorUserID uint, requestID string, u *model.AuditFile) (operationErr error) {
	if u == nil || u.Id == 0 {
		return errors.New("cannot delete empty file audit")
	}
	event, auditErr := beginSecurityAudit(ctx, actorUserID, requestID, "audit.file.deleted", "audit_file", strconv.FormatUint(uint64(u.Id), 10), nil)
	if auditErr != nil {
		return auditErr
	}
	defer finalizeSecurityAudit(event, &operationErr, "AUDIT_FILE_DELETE_FAILED")
	return DB.Delete(u).Error
}

// Update 更新
func (as *AuditService) UpdateAuditFile(u *model.AuditFile) error {
	return DB.Model(u).Updates(u).Error
}

func (as *AuditService) BatchDeleteAuditConn(ids []uint) error {
	return as.BatchDeleteAuditConnContext(context.Background(), 0, "", ids)
}

func (as *AuditService) BatchDeleteAuditFile(ids []uint) error {
	return as.BatchDeleteAuditFileContext(context.Background(), 0, "", ids)
}

func (as *AuditService) BatchDeleteAuditConnContext(ctx context.Context, actorUserID uint, requestID string, ids []uint) (operationErr error) {
	if len(ids) == 0 || len(ids) > MaxBatchSize {
		return errors.New("invalid connection audit batch")
	}
	event, auditErr := beginSecurityAudit(ctx, actorUserID, requestID, "audit.connection.batch_deleted", "audit_connection", "batch", map[string]interface{}{"count": len(ids)})
	if auditErr != nil {
		return auditErr
	}
	defer finalizeSecurityAudit(event, &operationErr, "AUDIT_CONNECTION_DELETE_FAILED")
	return DB.Where("id in (?)", ids).Delete(&model.AuditConn{}).Error
}

func (as *AuditService) BatchDeleteAuditFileContext(ctx context.Context, actorUserID uint, requestID string, ids []uint) (operationErr error) {
	if len(ids) == 0 || len(ids) > MaxBatchSize {
		return errors.New("invalid file audit batch")
	}
	event, auditErr := beginSecurityAudit(ctx, actorUserID, requestID, "audit.file.batch_deleted", "audit_file", "batch", map[string]interface{}{"count": len(ids)})
	if auditErr != nil {
		return auditErr
	}
	defer finalizeSecurityAudit(event, &operationErr, "AUDIT_FILE_DELETE_FAILED")
	return DB.Where("id in (?)", ids).Delete(&model.AuditFile{}).Error
}
