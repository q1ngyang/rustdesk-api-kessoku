package service

import (
	"testing"

	"github.com/q1ngyang/rustdesk-api-kessoku/v3/model"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestActivityListsApplyOwnershipFiltersAndPagination(t *testing.T) {
	previousDB := DB
	database, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	DB = database
	t.Cleanup(func() { DB = previousDB })
	if err := database.AutoMigrate(&model.LoginLog{}, &model.ShareRecord{}, &model.AuditConn{}, &model.AuditFile{}); err != nil {
		t.Fatal(err)
	}
	for _, entry := range []model.LoginLog{{UserId: 1, DeviceId: "owned"}, {UserId: 2, DeviceId: "other"}, {UserId: 1, DeviceId: "owned-older"}} {
		if err := database.Create(&entry).Error; err != nil {
			t.Fatal(err)
		}
	}
	for _, entry := range []model.ShareRecord{{UserId: 1, PeerId: "owned"}, {UserId: 2, PeerId: "other"}} {
		if err := database.Create(&entry).Error; err != nil {
			t.Fatal(err)
		}
	}
	for _, entry := range []model.AuditConn{{PeerId: "target-a", ConnId: 1}, {PeerId: "target-b", ConnId: 2}} {
		if err := database.Create(&entry).Error; err != nil {
			t.Fatal(err)
		}
	}

	login := (&LoginLogService{}).List(1, 1, func(tx *gorm.DB) *gorm.DB { return tx.Where("user_id = ?", 1).Order("id desc") })
	if login.Total != 2 || len(login.LoginLogs) != 1 || login.LoginLogs[0].DeviceId != "owned-older" {
		t.Fatalf("login filter/pagination not applied: %+v", login)
	}
	shares := (&ShareRecordService{}).List(1, 10, func(tx *gorm.DB) *gorm.DB { return tx.Where("user_id = ?", 1) })
	if shares.Total != 1 || len(shares.ShareRecords) != 1 || shares.ShareRecords[0].PeerId != "owned" {
		t.Fatalf("share ownership filter not applied: %+v", shares)
	}
	audits := (&AuditService{}).AuditConnList(1, 10, func(tx *gorm.DB) *gorm.DB { return tx.Where("peer_id = ?", "target-b") })
	if audits.Total != 1 || len(audits.AuditConns) != 1 || audits.AuditConns[0].PeerId != "target-b" {
		t.Fatalf("connection audit filter not applied: %+v", audits)
	}
}
