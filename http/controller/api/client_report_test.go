package api

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	"github.com/q1ngyang/rustdesk-api-kessoku/v3/config"
	"github.com/q1ngyang/rustdesk-api-kessoku/v3/global"
	request "github.com/q1ngyang/rustdesk-api-kessoku/v3/http/request/api"
	"github.com/q1ngyang/rustdesk-api-kessoku/v3/lib/lock"
	"github.com/q1ngyang/rustdesk-api-kessoku/v3/model"
	"github.com/q1ngyang/rustdesk-api-kessoku/v3/service"
	"github.com/sirupsen/logrus"
	"golang.org/x/text/language"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestClientReportsRequireExactRegisteredDeviceIdentity(t *testing.T) {
	database := clientReportDatabase(t)
	gin.SetMode(gin.TestMode)

	sysinfo := func(body string) *httptest.ResponseRecorder {
		recorder := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(recorder)
		ctx.Request = httptest.NewRequest(http.MethodPost, "/api/sysinfo", bytes.NewBufferString(body))
		ctx.Request.Header.Set("Content-Type", "application/json")
		(&Peer{}).SysInfo(ctx)
		return recorder
	}

	unknown := sysinfo(`{"id":"desk-1","uuid":"uuid-1","hostname":"host"}`)
	if unknown.Code != http.StatusOK || unknown.Body.String() != "ID_NOT_FOUND" {
		t.Fatalf("unregistered sysinfo = status %d body %q", unknown.Code, unknown.Body.String())
	}
	if err := database.Create(&model.LoginLog{UserId: 99, Client: model.LoginLogClientWebAdmin, DeviceId: "desk-1", Uuid: "uuid-1"}).Error; err != nil {
		t.Fatal(err)
	}
	webOnly := sysinfo(`{"id":"desk-1","uuid":"uuid-1","hostname":"host"}`)
	if webOnly.Code != http.StatusOK || webOnly.Body.String() != "ID_NOT_FOUND" {
		t.Fatalf("web-only identity accepted as a native device = status %d body %q", webOnly.Code, webOnly.Body.String())
	}
	if err := database.Create(&model.LoginLog{UserId: 42, Client: model.LoginLogClientNative, DeviceId: "desk-1", Uuid: "uuid-1"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := database.Create(&model.Peer{Id: "desk-1", Alias: "manual placeholder"}).Error; err != nil {
		t.Fatal(err)
	}
	accepted := sysinfo(`{"id":" desk - 1 ","uuid":"uuid-1","hostname":"host"}`)
	if accepted.Code != http.StatusOK || accepted.Body.String() != "SYSINFO_UPDATED" {
		t.Fatalf("registered sysinfo = status %d body %q", accepted.Code, accepted.Body.String())
	}
	peer := &model.Peer{}
	if err := database.Where("id = ?", "desk-1").First(peer).Error; err != nil {
		t.Fatal(err)
	}
	if peer.Uuid != "uuid-1" || peer.UserId != 42 || peer.Alias != "manual placeholder" {
		t.Fatalf("stored peer identity = %+v", peer)
	}

	mismatch := sysinfo(`{"id":"desk-1","uuid":"uuid-2","hostname":"changed"}`)
	if mismatch.Code != http.StatusOK || mismatch.Body.String() != "ID_NOT_FOUND" {
		t.Fatalf("mismatched sysinfo = status %d body %q", mismatch.Code, mismatch.Body.String())
	}
	if err := database.Where("id = ?", "desk-1").First(peer).Error; err != nil {
		t.Fatal(err)
	}
	if peer.Uuid != "uuid-1" || peer.Hostname != "host" {
		t.Fatalf("mismatched report changed peer = %+v", peer)
	}

	audit := func(body string) *httptest.ResponseRecorder {
		recorder := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(recorder)
		ctx.Request = httptest.NewRequest(http.MethodPost, "/api/audit/conn", bytes.NewBufferString(body))
		ctx.Request.Header.Set("Content-Type", "application/json")
		(&Audit{}).AuditConn(ctx)
		return recorder
	}
	acceptedAudit := audit(`{"action":"new","conn_id":7,"id":" desk - 1 ","uuid":"uuid-1","peer":[" remote - id ","remote-name"]}`)
	if acceptedAudit.Code != http.StatusOK {
		t.Fatalf("registered audit = status %d body %q", acceptedAudit.Code, acceptedAudit.Body.String())
	}
	mismatchedAudit := audit(`{"action":"new","conn_id":8,"id":"desk-1","uuid":"uuid-2","peer":["remote-id"]}`)
	if mismatchedAudit.Code != http.StatusBadRequest {
		t.Fatalf("mismatched audit = status %d body %q", mismatchedAudit.Code, mismatchedAudit.Body.String())
	}
	var count int64
	if err := database.Model(&model.AuditConn{}).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("stored audit count = %d, want 1", count)
	}
	storedAudit := &model.AuditConn{}
	if err := database.First(storedAudit).Error; err != nil {
		t.Fatal(err)
	}
	if storedAudit.PeerId != "desk-1" || storedAudit.FromPeer != "remote-id" {
		t.Fatalf("stored audit IDs were not normalized: %+v", storedAudit)
	}

	if err := database.Create(&model.LoginLog{UserId: 42, Client: model.LoginLogClientNative, DeviceId: "audit-only", Uuid: "uuid-audit"}).Error; err != nil {
		t.Fatal(err)
	}
	auditBeforeSysinfo := audit(`{"action":"new","conn_id":9,"id":" audit - only ","uuid":"uuid-audit","peer":["target"]}`)
	if auditBeforeSysinfo.Code != http.StatusOK {
		t.Fatalf("audit before sysinfo = status %d body %q", auditBeforeSysinfo.Code, auditBeforeSysinfo.Body.String())
	}
	claimed := &model.Peer{}
	if err := database.Where("id = ?", "audit-only").First(claimed).Error; err != nil || claimed.Uuid != "uuid-audit" || claimed.UserId != 42 {
		t.Fatalf("audit did not claim native login identity: peer=%+v err=%v", claimed, err)
	}

	fileAudit := func(body string) *httptest.ResponseRecorder {
		recorder := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(recorder)
		ctx.Request = httptest.NewRequest(http.MethodPost, "/api/audit/file", bytes.NewBufferString(body))
		ctx.Request.Header.Set("Content-Type", "application/json")
		(&Audit{}).AuditFile(ctx)
		return recorder
	}
	acceptedFile := fileAudit(`{"id":" audit - only ","uuid":"uuid-audit","peer_id":" remote - id ","info":"{\"name\":\"remote\",\"ip\":\"198.51.100.10\",\"num\":1}","path":"document.txt","is_file":true,"type":1}`)
	if acceptedFile.Code != http.StatusOK {
		t.Fatalf("registered file audit = status %d body %q", acceptedFile.Code, acceptedFile.Body.String())
	}
	storedFile := &model.AuditFile{}
	if err := database.First(storedFile).Error; err != nil || storedFile.PeerId != "audit-only" || storedFile.FromPeer != "remote-id" {
		t.Fatalf("stored file audit IDs were not normalized: audit=%+v err=%v", storedFile, err)
	}
}

func TestClientReportValidationBounds(t *testing.T) {
	if validPeerReport(nil) || validPeerReport(&request.PeerForm{}) {
		t.Fatal("missing peer identity was accepted")
	}
	tooLong := make([]byte, 129)
	for i := range tooLong {
		tooLong[i] = 'a'
	}
	if boundedReportField(string(tooLong), 128) || boundedOptionalReportField("embedded\x00nul", 128) {
		t.Fatal("invalid report field was accepted")
	}
}

func clientReportDatabase(t *testing.T) *gorm.DB {
	t.Helper()
	oldServiceConfig, oldServiceDB, oldServiceLogger, oldServiceAuth, oldServiceLock, oldServices := service.Config, service.DB, service.Logger, service.Auth, service.Lock, service.AllService
	oldGlobalDB, oldGlobalLogger, oldLocalizer := global.DB, global.Logger, global.Localizer
	t.Cleanup(func() {
		service.Config, service.DB, service.Logger, service.Auth, service.Lock, service.AllService = oldServiceConfig, oldServiceDB, oldServiceLogger, oldServiceAuth, oldServiceLock, oldServices
		global.DB, global.Logger, global.Localizer = oldGlobalDB, oldGlobalLogger, oldLocalizer
	})
	database, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := database.AutoMigrate(&model.Peer{}, &model.LoginLog{}, &model.AuditConn{}, &model.AuditFile{}); err != nil {
		t.Fatal(err)
	}
	logger := logrus.New()
	service.New(&config.Config{}, database, logger, nil, lock.NewLocal())
	global.DB, global.Logger = database, logger
	bundle := i18n.NewBundle(language.English)
	global.Localizer = func(string) *i18n.Localizer { return i18n.NewLocalizer(bundle, language.English.String()) }
	return database
}
