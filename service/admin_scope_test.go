package service

import (
	"context"
	"testing"

	"github.com/q1ngyang/rustdesk-api-kessoku/v3/config"
	"github.com/q1ngyang/rustdesk-api-kessoku/v3/lib/lock"
	"github.com/q1ngyang/rustdesk-api-kessoku/v3/model"
	"github.com/sirupsen/logrus"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func adminScopeTestDatabase(t *testing.T) *gorm.DB {
	t.Helper()
	oldConfig, oldDB, oldLogger, oldAuth, oldLock, oldServices := Config, DB, Logger, Auth, Lock, AllService
	t.Cleanup(func() {
		Config, DB, Logger, Auth, Lock, AllService = oldConfig, oldDB, oldLogger, oldAuth, oldLock, oldServices
	})
	database, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := database.AutoMigrate(
		&model.User{}, &model.UserToken{}, &model.Group{}, &model.Peer{}, &model.AddressBook{}, &model.Tag{},
		&model.AddressBookCollection{}, &model.AddressBookCollectionRule{}, &model.AdminResourceScope{}, &model.AdminAuditEvent{}, &model.SecurityInvariantLock{},
	); err != nil {
		t.Fatal(err)
	}
	if err := database.Create(&model.SecurityInvariantLock{Name: "enabled-admin"}).Error; err != nil {
		t.Fatal(err)
	}
	New(&config.Config{}, database, logrus.New(), nil, lock.NewLocal())
	return database
}

func TestScopedAdministratorEffectiveResourceUnion(t *testing.T) {
	database := adminScopeTestDatabase(t)
	super := &model.User{Username: "super", Role: model.UserRoleSuperAdmin, Status: model.COMMON_STATUS_ENABLE, AuthVersion: 1}
	admin := &model.User{Username: "manager", Role: model.UserRoleAdmin, Status: model.COMMON_STATUS_ENABLE, AuthVersion: 1}
	group := &model.Group{Name: "managed", Type: model.GroupTypeDefault}
	otherGroup := &model.Group{Name: "other", Type: model.GroupTypeDefault}
	if err := database.Create(&[]*model.User{super, admin}).Error; err != nil {
		t.Fatal(err)
	}
	if err := database.Create(&[]*model.Group{group, otherGroup}).Error; err != nil {
		t.Fatal(err)
	}
	groupUser := &model.User{Username: "group-user", Role: model.UserRoleUser, GroupId: group.Id, Status: model.COMMON_STATUS_ENABLE, AuthVersion: 1}
	explicitUser := &model.User{Username: "explicit-user", Role: model.UserRoleUser, GroupId: otherGroup.Id, Status: model.COMMON_STATUS_ENABLE, AuthVersion: 1}
	outsideUser := &model.User{Username: "outside-user", Role: model.UserRoleUser, GroupId: otherGroup.Id, Status: model.COMMON_STATUS_ENABLE, AuthVersion: 1}
	if err := database.Create(&[]*model.User{groupUser, explicitUser, outsideUser}).Error; err != nil {
		t.Fatal(err)
	}
	groupPeer := &model.Peer{Id: "group-peer", UserId: groupUser.Id}
	explicitPeer := &model.Peer{Id: "explicit-peer", UserId: outsideUser.Id}
	outsidePeer := &model.Peer{Id: "outside-peer", UserId: outsideUser.Id}
	collection := &model.AddressBookCollection{Name: "public", UserId: outsideUser.Id}
	outsideCollection := &model.AddressBookCollection{Name: "private", UserId: outsideUser.Id}
	if err := database.Create(&[]*model.Peer{groupPeer, explicitPeer, outsidePeer}).Error; err != nil {
		t.Fatal(err)
	}
	if err := database.Create(&[]*model.AddressBookCollection{collection, outsideCollection}).Error; err != nil {
		t.Fatal(err)
	}

	requested := model.AdminScopeSet{
		GroupIds: []uint{group.Id}, UserIds: []uint{explicitUser.Id}, CollectionIds: []uint{collection.Id}, PeerIds: []uint{explicitPeer.RowId},
	}
	if err := AllService.AdminScopeService.ReplaceScopesContext(context.Background(), super.Id, admin.Id, "0191f6a0-0000-7000-8000-000000000021", requested); err != nil {
		t.Fatal(err)
	}
	admin = AllService.UserService.InfoById(admin.Id)
	if admin.AuthVersion != 2 {
		t.Fatalf("authorization version = %d, want 2", admin.AuthVersion)
	}
	if !AllService.AdminScopeService.CanManageUser(admin, groupUser) || !AllService.AdminScopeService.CanManageUser(admin, explicitUser) || AllService.AdminScopeService.CanManageUser(admin, outsideUser) {
		t.Fatal("effective user scope does not match group-plus-explicit union")
	}
	if !AllService.AdminScopeService.CanManagePeer(admin, groupPeer.RowId) || !AllService.AdminScopeService.CanManagePeer(admin, explicitPeer.RowId) || AllService.AdminScopeService.CanManagePeer(admin, outsidePeer.RowId) {
		t.Fatal("effective peer scope does not match inherited-plus-explicit union")
	}
	if !AllService.AdminScopeService.CanManageCollection(admin, collection.Id) || AllService.AdminScopeService.CanManageCollection(admin, outsideCollection.Id) {
		t.Fatal("collection grants were not explicit and fail-closed")
	}
	if AllService.AdminScopeService.AllPeersManageable(admin, []uint{groupPeer.RowId, outsidePeer.RowId}) {
		t.Fatal("mixed-scope peer batch unexpectedly authorized")
	}

	users := AllService.UserService.List(1, 100, func(tx *gorm.DB) {
		AllService.AdminScopeService.ApplyUserScope(tx, admin)
	})
	if users.Total != 2 {
		t.Fatalf("scoped user list total = %d, want 2", users.Total)
	}
	peers := AllService.PeerService.List(1, 100, func(tx *gorm.DB) {
		AllService.AdminScopeService.ApplyPeerScope(tx, admin)
	})
	if peers.Total != 2 {
		t.Fatalf("scoped peer list total = %d, want 2", peers.Total)
	}
	event := &model.AdminAuditEvent{}
	if err := database.Where("action = ?", "authorization.scope.replaced").First(event).Error; err != nil || event.Result != "success" {
		t.Fatalf("scope audit event = %+v, err=%v", event, err)
	}
}

func TestScopeReplacementRejectsAdministrativeTargetsAtomically(t *testing.T) {
	database := adminScopeTestDatabase(t)
	super := &model.User{Username: "super", Role: model.UserRoleSuperAdmin, Status: model.COMMON_STATUS_ENABLE, AuthVersion: 1}
	admin := &model.User{Username: "manager", Role: model.UserRoleAdmin, Status: model.COMMON_STATUS_ENABLE, AuthVersion: 1}
	otherAdmin := &model.User{Username: "other-manager", Role: model.UserRoleAdmin, Status: model.COMMON_STATUS_ENABLE, AuthVersion: 1}
	group := &model.Group{Name: "managed", Type: model.GroupTypeDefault}
	if err := database.Create(&[]*model.User{super, admin, otherAdmin}).Error; err != nil {
		t.Fatal(err)
	}
	if err := database.Create(group).Error; err != nil {
		t.Fatal(err)
	}
	initial := model.AdminScopeSet{GroupIds: []uint{group.Id}}
	if err := AllService.AdminScopeService.ReplaceScopesContext(context.Background(), super.Id, admin.Id, "", initial); err != nil {
		t.Fatal(err)
	}
	invalid := model.AdminScopeSet{UserIds: []uint{otherAdmin.Id}}
	if err := AllService.AdminScopeService.ReplaceScopesContext(context.Background(), super.Id, admin.Id, "", invalid); err == nil {
		t.Fatal("administrator target assignment unexpectedly succeeded")
	}
	remaining, err := AllService.AdminScopeService.ScopeSet(admin.Id)
	if err != nil {
		t.Fatal(err)
	}
	if len(remaining.GroupIds) != 1 || remaining.GroupIds[0] != group.Id || len(remaining.UserIds) != 0 {
		t.Fatalf("failed replacement changed prior grants: %+v", remaining)
	}
}

func TestRoleRoutesSeparateScopedAndGlobalAdministration(t *testing.T) {
	admin := &model.User{Role: model.UserRoleAdmin}
	super := &model.User{Role: model.UserRoleSuperAdmin}
	userService := &UserService{}
	routes := userService.RouteNames(admin)
	for _, route := range routes {
		if route == "*" || route == "ServerControl" || route == "Oauth" {
			t.Fatalf("scoped administrator received global route %q", route)
		}
	}
	if got := userService.RouteNames(super); len(got) != 1 || got[0] != "*" {
		t.Fatalf("super administrator routes = %v", got)
	}
}

func TestDeletingCollectionRemovesDependentsAndScopeGrants(t *testing.T) {
	database := adminScopeTestDatabase(t)
	owner := &model.User{Username: "owner", Role: model.UserRoleUser, Status: model.COMMON_STATUS_ENABLE, AuthVersion: 1}
	admin := &model.User{Username: "manager", Role: model.UserRoleAdmin, Status: model.COMMON_STATUS_ENABLE, AuthVersion: 1}
	if err := database.Create(&[]*model.User{owner, admin}).Error; err != nil {
		t.Fatal(err)
	}
	collection := &model.AddressBookCollection{Name: "temporary", UserId: owner.Id}
	if err := database.Create(collection).Error; err != nil {
		t.Fatal(err)
	}
	addressBook := &model.AddressBook{Id: "peer-1", UserId: owner.Id, CollectionId: collection.Id}
	tag := &model.Tag{Name: "production", UserId: owner.Id, CollectionId: collection.Id}
	rule := &model.AddressBookCollectionRule{UserId: owner.Id, CollectionId: collection.Id, Type: model.ShareAddressBookRuleTypePersonal, ToId: admin.Id, Rule: model.ShareAddressBookRuleRuleRead}
	grant := &model.AdminResourceScope{AdminUserId: admin.Id, ScopeType: model.AdminScopeTypeCollection, ScopeId: collection.Id}
	if err := database.Create(addressBook).Error; err != nil {
		t.Fatal(err)
	}
	if err := database.Create(tag).Error; err != nil {
		t.Fatal(err)
	}
	if err := database.Create(rule).Error; err != nil {
		t.Fatal(err)
	}
	if err := database.Create(grant).Error; err != nil {
		t.Fatal(err)
	}

	if err := AllService.AddressBookService.DeleteCollection(collection); err != nil {
		t.Fatal(err)
	}
	checks := []struct {
		name   string
		target interface{}
		where  string
		args   []interface{}
	}{
		{name: "collection", target: &model.AddressBookCollection{}, where: "id = ?", args: []interface{}{collection.Id}},
		{name: "address book", target: &model.AddressBook{}, where: "collection_id = ?", args: []interface{}{collection.Id}},
		{name: "tag", target: &model.Tag{}, where: "collection_id = ?", args: []interface{}{collection.Id}},
		{name: "rule", target: &model.AddressBookCollectionRule{}, where: "collection_id = ?", args: []interface{}{collection.Id}},
		{name: "scope grant", target: &model.AdminResourceScope{}, where: "scope_type = ? AND scope_id = ?", args: []interface{}{model.AdminScopeTypeCollection, collection.Id}},
	}
	for _, check := range checks {
		var count int64
		if err := database.Model(check.target).Where(check.where, check.args...).Count(&count).Error; err != nil {
			t.Fatal(err)
		}
		if count != 0 {
			t.Fatalf("%s rows remain after collection deletion: %d", check.name, count)
		}
	}
}

func TestRoleChangeClearsBothAssignedAndTargetedStaleScopes(t *testing.T) {
	database := adminScopeTestDatabase(t)
	target := &model.User{Username: "promoted-user", Role: model.UserRoleUser, Status: model.COMMON_STATUS_ENABLE, AuthVersion: 1}
	manager := &model.User{Username: "existing-manager", Role: model.UserRoleAdmin, Status: model.COMMON_STATUS_ENABLE, AuthVersion: 1}
	if err := database.Create(&[]*model.User{target, manager}).Error; err != nil {
		t.Fatal(err)
	}
	group := &model.Group{Name: "stale", Type: model.GroupTypeDefault}
	if err := database.Create(group).Error; err != nil {
		t.Fatal(err)
	}
	stale := []model.AdminResourceScope{
		{AdminUserId: target.Id, ScopeType: model.AdminScopeTypeGroup, ScopeId: group.Id},
		{AdminUserId: manager.Id, ScopeType: model.AdminScopeTypeUser, ScopeId: target.Id},
	}
	if err := database.Create(&stale).Error; err != nil {
		t.Fatal(err)
	}
	update := *target
	update.Role = model.UserRoleAdmin
	update.IsAdmin = nil
	if err := AllService.UserService.UpdateContext(context.Background(), manager.Id, "", &update); err != nil {
		t.Fatal(err)
	}
	var count int64
	if err := database.Model(&model.AdminResourceScope{}).Where("admin_user_id = ? OR (scope_type = ? AND scope_id = ?)", target.Id, model.AdminScopeTypeUser, target.Id).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("stale authorization grants remain after role change: %d", count)
	}
}
