package service

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/q1ngyang/rustdesk-api-kessoku/v3/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var ErrAdminScopeDenied = errors.New("administrator scope access denied")

type AdminScopeService struct{}

func uniqueNonZeroIDs(ids []uint) ([]uint, error) {
	if len(ids) > MaxBatchSize {
		return nil, fmt.Errorf("scope contains more than %d resources", MaxBatchSize)
	}
	seen := make(map[uint]struct{}, len(ids))
	result := make([]uint, 0, len(ids))
	for _, id := range ids {
		if id == 0 {
			return nil, errors.New("scope resource id must be greater than zero")
		}
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		result = append(result, id)
	}
	return result, nil
}

func (s *AdminScopeService) normalizeScopeSet(scope model.AdminScopeSet) (model.AdminScopeSet, error) {
	var err error
	if scope.GroupIds, err = uniqueNonZeroIDs(scope.GroupIds); err != nil {
		return scope, err
	}
	if scope.UserIds, err = uniqueNonZeroIDs(scope.UserIds); err != nil {
		return scope, err
	}
	if scope.CollectionIds, err = uniqueNonZeroIDs(scope.CollectionIds); err != nil {
		return scope, err
	}
	if scope.PeerIds, err = uniqueNonZeroIDs(scope.PeerIds); err != nil {
		return scope, err
	}
	return scope, nil
}

func countExisting(tx *gorm.DB, target interface{}, primaryKey string, ids []uint) error {
	if len(ids) == 0 {
		return nil
	}
	var count int64
	if err := tx.Model(target).Where(primaryKey+" IN ?", ids).Count(&count).Error; err != nil {
		return err
	}
	if count != int64(len(ids)) {
		return errors.New("one or more scoped resources do not exist")
	}
	return nil
}

func (s *AdminScopeService) validateScopeSet(tx *gorm.DB, scope model.AdminScopeSet) error {
	if err := countExisting(tx, &model.Group{}, "id", scope.GroupIds); err != nil {
		return err
	}
	if len(scope.UserIds) > 0 {
		var users []*model.User
		if err := tx.Where("id IN ?", scope.UserIds).Find(&users).Error; err != nil {
			return err
		}
		if len(users) != len(scope.UserIds) {
			return errors.New("one or more scoped users do not exist")
		}
		for _, user := range users {
			if AllService.UserService.Role(user) != model.UserRoleUser {
				return errors.New("only ordinary users can be assigned to a scoped administrator")
			}
		}
	}
	if err := countExisting(tx, &model.AddressBookCollection{}, "id", scope.CollectionIds); err != nil {
		return err
	}
	if err := countExisting(tx, &model.Peer{}, "row_id", scope.PeerIds); err != nil {
		return err
	}
	return nil
}

func scopeRows(adminUserID uint, scopeType model.AdminScopeType, ids []uint) []model.AdminResourceScope {
	rows := make([]model.AdminResourceScope, 0, len(ids))
	for _, id := range ids {
		rows = append(rows, model.AdminResourceScope{AdminUserId: adminUserID, ScopeType: scopeType, ScopeId: id})
	}
	return rows
}

// ReplaceScopesContext atomically replaces all resource grants for one scoped
// administrator and revokes that administrator's sessions so the new policy is
// visible immediately.
func (s *AdminScopeService) ReplaceScopesContext(ctx context.Context, actorUserID, adminUserID uint, requestID string, requested model.AdminScopeSet) (operationErr error) {
	actor := AllService.UserService.InfoById(actorUserID)
	if !AllService.UserService.IsSuperAdmin(actor) {
		return ErrAdminScopeDenied
	}
	target := AllService.UserService.InfoById(adminUserID)
	if target.Id == 0 || AllService.UserService.Role(target) != model.UserRoleAdmin {
		return errors.New("scope target must be a scoped administrator")
	}
	scope, err := s.normalizeScopeSet(requested)
	if err != nil {
		return err
	}
	event, auditErr := beginSecurityAudit(ctx, actorUserID, requestID, "authorization.scope.replaced", "user", strconv.FormatUint(uint64(adminUserID), 10), map[string]interface{}{
		"groups": len(scope.GroupIds), "users": len(scope.UserIds), "collections": len(scope.CollectionIds), "peers": len(scope.PeerIds),
	})
	if auditErr != nil {
		return auditErr
	}
	defer finalizeSecurityAudit(event, &operationErr, "AUTHORIZATION_SCOPE_UPDATE_FAILED")

	tx := DB.WithContext(ctx).Begin()
	if tx.Error != nil {
		return tx.Error
	}
	defer tx.Rollback()
	lockedTarget := &model.User{}
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(lockedTarget, adminUserID).Error; err != nil {
		return err
	}
	if AllService.UserService.Role(lockedTarget) != model.UserRoleAdmin {
		return errors.New("scope target must be a scoped administrator")
	}
	if err := s.validateScopeSet(tx, scope); err != nil {
		return err
	}
	if err := tx.Where("admin_user_id = ?", adminUserID).Delete(&model.AdminResourceScope{}).Error; err != nil {
		return err
	}
	rows := make([]model.AdminResourceScope, 0, len(scope.GroupIds)+len(scope.UserIds)+len(scope.CollectionIds)+len(scope.PeerIds))
	rows = append(rows, scopeRows(adminUserID, model.AdminScopeTypeGroup, scope.GroupIds)...)
	rows = append(rows, scopeRows(adminUserID, model.AdminScopeTypeUser, scope.UserIds)...)
	rows = append(rows, scopeRows(adminUserID, model.AdminScopeTypeCollection, scope.CollectionIds)...)
	rows = append(rows, scopeRows(adminUserID, model.AdminScopeTypePeer, scope.PeerIds)...)
	if len(rows) > 0 {
		if err := tx.Create(&rows).Error; err != nil {
			return err
		}
	}
	if err := AllService.UserService.bumpAuthVersionAndRevoke(tx, adminUserID, "authorization_scope_changed"); err != nil {
		return err
	}
	return tx.Commit().Error
}

func (s *AdminScopeService) ScopeSet(adminUserID uint) (model.AdminScopeSet, error) {
	var rows []model.AdminResourceScope
	if err := DB.Where("admin_user_id = ?", adminUserID).Order("scope_type, scope_id").Find(&rows).Error; err != nil {
		return model.AdminScopeSet{}, err
	}
	result := model.AdminScopeSet{
		GroupIds: []uint{}, UserIds: []uint{}, CollectionIds: []uint{}, PeerIds: []uint{},
	}
	for _, row := range rows {
		switch row.ScopeType {
		case model.AdminScopeTypeGroup:
			result.GroupIds = append(result.GroupIds, row.ScopeId)
		case model.AdminScopeTypeUser:
			result.UserIds = append(result.UserIds, row.ScopeId)
		case model.AdminScopeTypeCollection:
			result.CollectionIds = append(result.CollectionIds, row.ScopeId)
		case model.AdminScopeTypePeer:
			result.PeerIds = append(result.PeerIds, row.ScopeId)
		}
	}
	return result, nil
}

func (s *AdminScopeService) Details(adminUserID uint) (*model.AdminScopeDetails, error) {
	admin := AllService.UserService.InfoById(adminUserID)
	if admin.Id == 0 || AllService.UserService.Role(admin) != model.UserRoleAdmin {
		return nil, errors.New("scope target must be a scoped administrator")
	}
	scope, err := s.ScopeSet(adminUserID)
	if err != nil {
		return nil, err
	}
	details := &model.AdminScopeDetails{AdminUser: admin, Scope: scope, Groups: []*model.Group{}, Users: []*model.User{}, Collections: []*model.AddressBookCollection{}, Peers: []*model.Peer{}}
	if len(scope.GroupIds) > 0 {
		DB.Where("id IN ?", scope.GroupIds).Order("name").Find(&details.Groups)
	}
	if len(scope.UserIds) > 0 {
		DB.Where("id IN ?", scope.UserIds).Order("username").Find(&details.Users)
	}
	if len(scope.CollectionIds) > 0 {
		DB.Where("id IN ?", scope.CollectionIds).Order("name").Find(&details.Collections)
	}
	if len(scope.PeerIds) > 0 {
		DB.Where("row_id IN ?", scope.PeerIds).Order("id").Find(&details.Peers)
	}
	return details, nil
}

func (s *AdminScopeService) scopeIDsSubquery(adminUserID uint, scopeType model.AdminScopeType) *gorm.DB {
	return DB.Model(&model.AdminResourceScope{}).Select("scope_id").Where("admin_user_id = ? AND scope_type = ?", adminUserID, scopeType)
}

func (s *AdminScopeService) effectiveUserIDsSubquery(adminUserID uint) *gorm.DB {
	directUsers := s.scopeIDsSubquery(adminUserID, model.AdminScopeTypeUser)
	groups := s.scopeIDsSubquery(adminUserID, model.AdminScopeTypeGroup)
	return DB.Model(&model.User{}).Select("id").Where("role = ? AND (id IN (?) OR group_id IN (?))", model.UserRoleUser, directUsers, groups)
}

func (s *AdminScopeService) ApplyUserScope(tx *gorm.DB, actor *model.User) *gorm.DB {
	if AllService.UserService.IsSuperAdmin(actor) {
		return tx
	}
	if AllService.UserService.Role(actor) != model.UserRoleAdmin {
		return tx.Where("1 = 0")
	}
	return tx.Where("users.id IN (?)", s.effectiveUserIDsSubquery(actor.Id))
}

func (s *AdminScopeService) ApplyGroupScope(tx *gorm.DB, actor *model.User) *gorm.DB {
	if AllService.UserService.IsSuperAdmin(actor) {
		return tx
	}
	if AllService.UserService.Role(actor) != model.UserRoleAdmin {
		return tx.Where("1 = 0")
	}
	return tx.Where("groups.id IN (?)", s.scopeIDsSubquery(actor.Id, model.AdminScopeTypeGroup))
}

func (s *AdminScopeService) ApplyPeerScope(tx *gorm.DB, actor *model.User) *gorm.DB {
	if AllService.UserService.IsSuperAdmin(actor) {
		return tx
	}
	if AllService.UserService.Role(actor) != model.UserRoleAdmin {
		return tx.Where("1 = 0")
	}
	return tx.Where("peers.row_id IN (?) OR peers.user_id IN (?)", s.scopeIDsSubquery(actor.Id, model.AdminScopeTypePeer), s.effectiveUserIDsSubquery(actor.Id))
}

func (s *AdminScopeService) ApplyCollectionScope(tx *gorm.DB, actor *model.User) *gorm.DB {
	if AllService.UserService.IsSuperAdmin(actor) {
		return tx
	}
	if AllService.UserService.Role(actor) != model.UserRoleAdmin {
		return tx.Where("1 = 0")
	}
	return tx.Where("address_book_collections.id IN (?)", s.scopeIDsSubquery(actor.Id, model.AdminScopeTypeCollection))
}

func (s *AdminScopeService) ApplyAddressBookScope(tx *gorm.DB, actor *model.User) *gorm.DB {
	if AllService.UserService.IsSuperAdmin(actor) {
		return tx
	}
	if AllService.UserService.Role(actor) != model.UserRoleAdmin {
		return tx.Where("1 = 0")
	}
	return tx.Where("address_books.collection_id IN (?) AND address_books.collection_id > 0", s.scopeIDsSubquery(actor.Id, model.AdminScopeTypeCollection))
}

func (s *AdminScopeService) ApplyTagScope(tx *gorm.DB, actor *model.User) *gorm.DB {
	if AllService.UserService.IsSuperAdmin(actor) {
		return tx
	}
	if AllService.UserService.Role(actor) != model.UserRoleAdmin {
		return tx.Where("1 = 0")
	}
	return tx.Where("tags.collection_id IN (?) AND tags.collection_id > 0", s.scopeIDsSubquery(actor.Id, model.AdminScopeTypeCollection))
}

func (s *AdminScopeService) ApplyRuleScope(tx *gorm.DB, actor *model.User) *gorm.DB {
	if AllService.UserService.IsSuperAdmin(actor) {
		return tx
	}
	if AllService.UserService.Role(actor) != model.UserRoleAdmin {
		return tx.Where("1 = 0")
	}
	return tx.Where("address_book_collection_rules.collection_id IN (?)", s.scopeIDsSubquery(actor.Id, model.AdminScopeTypeCollection))
}

func (s *AdminScopeService) CanManageUser(actor, target *model.User) bool {
	if actor == nil || target == nil || target.Id == 0 {
		return false
	}
	if AllService.UserService.IsSuperAdmin(actor) {
		return true
	}
	if AllService.UserService.Role(actor) != model.UserRoleAdmin || AllService.UserService.Role(target) != model.UserRoleUser {
		return false
	}
	var count int64
	DB.Model(&model.User{}).Where("id = ? AND id IN (?)", target.Id, s.effectiveUserIDsSubquery(actor.Id)).Count(&count)
	return count == 1
}

func (s *AdminScopeService) CanManageGroup(actor *model.User, groupID uint) bool {
	if AllService.UserService.IsSuperAdmin(actor) {
		return true
	}
	if actor == nil || AllService.UserService.Role(actor) != model.UserRoleAdmin || groupID == 0 {
		return false
	}
	var count int64
	DB.Model(&model.AdminResourceScope{}).Where("admin_user_id = ? AND scope_type = ? AND scope_id = ?", actor.Id, model.AdminScopeTypeGroup, groupID).Count(&count)
	return count == 1
}

func (s *AdminScopeService) CanManagePeer(actor *model.User, peerRowID uint) bool {
	if AllService.UserService.IsSuperAdmin(actor) {
		return true
	}
	if actor == nil || AllService.UserService.Role(actor) != model.UserRoleAdmin || peerRowID == 0 {
		return false
	}
	var count int64
	tx := s.ApplyPeerScope(DB.Model(&model.Peer{}), actor)
	tx.Where("peers.row_id = ?", peerRowID).Count(&count)
	return count == 1
}

func (s *AdminScopeService) CanManageCollection(actor *model.User, collectionID uint) bool {
	if AllService.UserService.IsSuperAdmin(actor) {
		return true
	}
	if actor == nil || AllService.UserService.Role(actor) != model.UserRoleAdmin || collectionID == 0 {
		return false
	}
	var count int64
	DB.Model(&model.AdminResourceScope{}).Where("admin_user_id = ? AND scope_type = ? AND scope_id = ?", actor.Id, model.AdminScopeTypeCollection, collectionID).Count(&count)
	return count == 1
}

func (s *AdminScopeService) CanManageAddressBook(actor *model.User, addressBook *model.AddressBook) bool {
	return addressBook != nil && addressBook.RowId > 0 && addressBook.CollectionId > 0 && s.CanManageCollection(actor, addressBook.CollectionId)
}

func (s *AdminScopeService) CanManageRule(actor *model.User, rule *model.AddressBookCollectionRule) bool {
	return rule != nil && rule.Id > 0 && s.CanManageCollection(actor, rule.CollectionId)
}

func (s *AdminScopeService) CanShareWithUser(actor *model.User, userID uint) bool {
	user := AllService.UserService.InfoById(userID)
	return s.CanManageUser(actor, user)
}

func (s *AdminScopeService) CanShareWithGroup(actor *model.User, groupID uint) bool {
	return s.CanManageGroup(actor, groupID)
}

func (s *AdminScopeService) AllPeersManageable(actor *model.User, ids []uint) bool {
	unique, err := uniqueNonZeroIDs(ids)
	if err != nil || len(unique) != len(ids) {
		return false
	}
	if AllService.UserService.IsSuperAdmin(actor) {
		var count int64
		DB.Model(&model.Peer{}).Where("row_id IN ?", unique).Count(&count)
		return count == int64(len(unique))
	}
	var count int64
	s.ApplyPeerScope(DB.Model(&model.Peer{}), actor).Where("row_id IN ?", unique).Count(&count)
	return count == int64(len(unique))
}

func (s *AdminScopeService) RemoveResourceScopes(tx *gorm.DB, scopeType model.AdminScopeType, ids []uint) error {
	if len(ids) == 0 {
		return nil
	}
	if !tx.Migrator().HasTable(&model.AdminResourceScope{}) {
		return nil
	}
	return tx.Where("scope_type = ? AND scope_id IN ?", scopeType, ids).Delete(&model.AdminResourceScope{}).Error
}

func (s *AdminScopeService) RemoveAdministratorScopes(tx *gorm.DB, adminUserID uint) error {
	if !tx.Migrator().HasTable(&model.AdminResourceScope{}) {
		return nil
	}
	return tx.Where("admin_user_id = ?", adminUserID).Delete(&model.AdminResourceScope{}).Error
}

func (s *AdminScopeService) RecordDenied(ctx context.Context, actorUserID uint, requestID, targetType, targetID string) {
	event, err := beginSecurityAudit(ctx, actorUserID, requestID, "authorization.denied", targetType, targetID, nil)
	if err != nil {
		if Logger != nil {
			Logger.Errorf("record authorization denial intent: %v", err)
		}
		return
	}
	if err := finishSecurityAudit(event, ErrAdminScopeDenied, "AUTHORIZATION_SCOPE_DENIED"); err != nil && Logger != nil {
		Logger.Errorf("record authorization denial result: %v", err)
	}
}
