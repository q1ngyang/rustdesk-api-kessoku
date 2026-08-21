package service

import (
	"context"
	"errors"
	"strconv"

	"github.com/q1ngyang/rustdesk-api-kessoku/v2/model"
	"gorm.io/gorm"
)

type AuditService struct {
}

func (as *AuditService) AuditConnList(page, pageSize uint, where func(tx *gorm.DB)) (res *model.AuditConnList) {
	res = &model.AuditConnList{}
	res.Page = int64(page)
	res.PageSize = int64(pageSize)
	tx := DB.Model(&model.AuditConn{})
	if where != nil {
		where(tx)
	}
	tx.Count(&res.Total)
	tx.Scopes(Paginate(page, pageSize))
	tx.Find(&res.AuditConns)
	return
}

// Create 创建
func (as *AuditService) CreateAuditConn(u *model.AuditConn) error {
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

func (as *AuditService) AuditFileList(page, pageSize uint, where func(tx *gorm.DB)) (res *model.AuditFileList) {
	res = &model.AuditFileList{}
	res.Page = int64(page)
	res.PageSize = int64(pageSize)
	tx := DB.Model(&model.AuditFile{})
	if where != nil {
		where(tx)
	}
	tx.Count(&res.Total)
	tx.Scopes(Paginate(page, pageSize))
	tx.Find(&res.AuditFiles)
	return
}

// CreateAuditFile
func (as *AuditService) CreateAuditFile(u *model.AuditFile) error {
	res := DB.Create(u).Error
	return res
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
