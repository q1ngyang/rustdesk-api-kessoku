package service

import (
	"testing"

	"github.com/q1ngyang/rustdesk-api-kessoku/v3/model"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestAuditEndpointsResolveAccountUsernamesAndControlledIdentity(t *testing.T) {
	previousDB := DB
	database, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	DB = database
	t.Cleanup(func() { DB = previousDB })
	if err := database.AutoMigrate(&model.User{}, &model.Peer{}, &model.AuditConn{}, &model.AuditFile{}); err != nil {
		t.Fatal(err)
	}
	users := []model.User{{Username: "controller-user"}, {Username: "controlled-user"}}
	if err := database.Create(&users).Error; err != nil {
		t.Fatal(err)
	}
	peers := []model.Peer{
		{Id: "100100100", UserId: users[0].Id, Uuid: "controller-uuid", LastOnlineIp: "198.51.100.10"},
		{Id: "200200200", UserId: users[1].Id, Uuid: "controlled-uuid", LastOnlineIp: "203.0.113.20"},
	}
	if err := database.Create(&peers).Error; err != nil {
		t.Fatal(err)
	}
	auditService := &AuditService{}
	connection := &model.AuditConn{FromPeer: peers[0].Id, PeerId: peers[1].Id, FromName: "machine nickname"}
	if err := auditService.CreateAuditConn(connection); err != nil {
		t.Fatal(err)
	}
	file := &model.AuditFile{FromPeer: peers[0].Id, PeerId: peers[1].Id}
	if err := auditService.CreateAuditFile(file); err != nil {
		t.Fatal(err)
	}
	if connection.ControllerUsername != "controller-user" || connection.FromName != "controller-user" || connection.ControlledUsername != "controlled-user" || connection.ControlledIP != "203.0.113.20" || connection.Uuid != "controlled-uuid" {
		t.Fatalf("connection endpoints were not enriched: %+v", connection)
	}
	if file.ControllerUsername != "controller-user" || file.ControlledUsername != "controlled-user" || file.ControlledIP != "203.0.113.20" || file.Uuid != "controlled-uuid" {
		t.Fatalf("file endpoints were not enriched: %+v", file)
	}
}
