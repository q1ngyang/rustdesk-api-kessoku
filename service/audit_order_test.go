package service

import (
	"reflect"
	"testing"
	"time"

	"github.com/q1ngyang/rustdesk-api-kessoku/v3/model"
	"github.com/q1ngyang/rustdesk-api-kessoku/v3/model/custom_types"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestAuditListsSortByCreationTimeNewestFirst(t *testing.T) {
	previousDB := DB
	database, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	DB = database
	t.Cleanup(func() { DB = previousDB })
	if err := database.AutoMigrate(&model.AuditConn{}, &model.AuditFile{}); err != nil {
		t.Fatal(err)
	}
	base := time.Date(2026, 8, 29, 12, 0, 0, 0, time.UTC)
	created := []time.Time{base, base.Add(-2 * time.Hour), base.Add(-time.Hour)}
	for index, value := range created {
		stamp := custom_types.AutoTime(value)
		if err := database.Create(&model.AuditConn{ConnId: int64(index + 1), TimeModel: model.TimeModel{CreatedAt: stamp}}).Error; err != nil {
			t.Fatal(err)
		}
		if err := database.Create(&model.AuditFile{Num: index + 1, TimeModel: model.TimeModel{CreatedAt: stamp}}).Error; err != nil {
			t.Fatal(err)
		}
	}

	connections := (&AuditService{}).AuditConnList(1, 10, nil)
	connectionOrder := make([]int64, 0, len(connections.AuditConns))
	connectionIDs := make([]uint, 0, len(connections.AuditConns))
	for _, item := range connections.AuditConns {
		connectionOrder = append(connectionOrder, item.ConnId)
		connectionIDs = append(connectionIDs, item.Id)
	}
	if !reflect.DeepEqual(connectionOrder, []int64{1, 3, 2}) {
		t.Fatalf("connection order = %v, want creation-time descending", connectionOrder)
	}
	if !reflect.DeepEqual(connectionIDs, []uint{1, 3, 2}) {
		t.Fatalf("connection database ids = %v, want persisted identifiers in creation-time order", connectionIDs)
	}

	files := (&AuditService{}).AuditFileList(1, 10, nil)
	fileOrder := make([]int, 0, len(files.AuditFiles))
	fileIDs := make([]uint, 0, len(files.AuditFiles))
	for _, item := range files.AuditFiles {
		fileOrder = append(fileOrder, item.Num)
		fileIDs = append(fileIDs, item.Id)
	}
	if !reflect.DeepEqual(fileOrder, []int{1, 3, 2}) {
		t.Fatalf("file order = %v, want creation-time descending", fileOrder)
	}
	if !reflect.DeepEqual(fileIDs, []uint{1, 3, 2}) {
		t.Fatalf("file database ids = %v, want persisted identifiers in creation-time order", fileIDs)
	}

	if err := database.Delete(&model.AuditConn{}, 2).Error; err != nil {
		t.Fatal(err)
	}
	if err := database.Delete(&model.AuditFile{}, 2).Error; err != nil {
		t.Fatal(err)
	}
	connections = (&AuditService{}).AuditConnList(1, 10, nil)
	files = (&AuditService{}).AuditFileList(1, 10, nil)
	connectionIDs = connectionIDs[:0]
	fileIDs = fileIDs[:0]
	for _, item := range connections.AuditConns {
		connectionIDs = append(connectionIDs, item.Id)
	}
	for _, item := range files.AuditFiles {
		fileIDs = append(fileIDs, item.Id)
	}
	if !reflect.DeepEqual(connectionIDs, []uint{1, 3}) || !reflect.DeepEqual(fileIDs, []uint{1, 3}) {
		t.Fatalf("database id gap was renumbered after deletion: connections=%v files=%v", connectionIDs, fileIDs)
	}
}

func TestControlledAuditPathsPreserveEndpointPathStyle(t *testing.T) {
	tests := []struct {
		name string
		base string
		info string
		want []string
	}{
		{"windows", `C:\Users\alice\Downloads`, `{"files":[["report.pdf",2048],["folder\\photo.jpg",1024]]}`, []string{`C:\Users\alice\Downloads\report.pdf`, `C:\Users\alice\Downloads\folder\photo.jpg`}},
		{"posix", "/home/alice/Downloads/", `{"files":[["report.pdf",2048]]}`, []string{"/home/alice/Downloads/report.pdf"}},
		{"single file path", `D:\archive\data.zip`, `{"files":[["",4096]]}`, []string{`D:\archive\data.zip`}},
		{"legacy path only", "/srv/files", `{}`, []string{"/srv/files"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := controlledAuditPaths(test.base, test.info); !reflect.DeepEqual(got, test.want) {
				t.Fatalf("paths = %#v, want %#v", got, test.want)
			}
		})
	}
}
