package api

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

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
	historicalNative := sysinfo(`{"id":"desk-1","uuid":"uuid-1","hostname":"host"}`)
	if historicalNative.Body.String() != "ID_NOT_FOUND" {
		t.Fatalf("expired historical native login was trusted: %q", historicalNative.Body.String())
	}
	isAdmin := false
	user := &model.User{IdModel: model.IdModel{Id: 42}, Username: "native-owner", Status: model.COMMON_STATUS_ENABLE, AuthVersion: 1, IsAdmin: &isAdmin}
	if err := database.Create(user).Error; err != nil {
		t.Fatal(err)
	}
	now := time.Now().Unix()
	if err := database.Create(&model.UserToken{UserId: user.Id, DeviceId: "desk-1", DeviceUuid: "uuid-1", Client: model.LoginLogClientNative, AuthVersion: 1, IssuedAt: now, ExpiredAt: now + 3600}).Error; err != nil {
		t.Fatal(err)
	}
	if got := service.AllService.UserService.FindActiveNativeUserID("uuid-1", "desk-1", now); got != user.Id {
		t.Fatalf("active native test fixture resolved to user %d, want %d", got, user.Id)
	}
	if err := database.Create(&model.Peer{Id: "desk-1", Alias: "manual placeholder"}).Error; err != nil {
		t.Fatal(err)
	}
	if resolved, err := service.AllService.PeerService.ResolveReportIdentity(context.Background(), "desk-1", "uuid-1"); err != nil || resolved.UserId != user.Id {
		t.Fatalf("active native identity did not claim placeholder: peer=%+v err=%v", resolved, err)
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
	if err := database.Create(&model.UserToken{UserId: user.Id, DeviceId: "audit-only", DeviceUuid: "uuid-audit", Client: model.LoginLogClientNative, AuthVersion: 1, IssuedAt: now + 1, ExpiredAt: now + 3600}).Error; err != nil {
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

type staticNetworkPeerVerifier struct {
	id   string
	uuid string
}

type staticNetworkActivationVerifier struct {
	id              string
	uuid            string
	activationEpoch uint64
	activationID    string
	routeLease      string
}

func (v staticNetworkActivationVerifier) VerifyPeerActivation(_ context.Context, id, uuid string, epoch uint64, activationID string, routeLeases []string) (bool, error) {
	return id == v.id && uuid == v.uuid && epoch == v.activationEpoch && activationID == v.activationID && len(routeLeases) == 1 && routeLeases[0] == v.routeLease, nil
}

func TestPresenceLeaseV2HTTPLifecycle(t *testing.T) {
	database := clientReportDatabase(t)
	gin.SetMode(gin.TestMode)
	const (
		deviceID   = "301132036"
		deviceUUID = "MDEyMzQ1Njc4OWFiY2RlZg=="
	)
	activationBytes := bytes.Repeat([]byte{0x41}, 16)
	routeLeaseBytes := bytes.Repeat([]byte{0x52}, 32)
	activationID := base64.StdEncoding.EncodeToString(activationBytes)
	routeLease := base64.StdEncoding.EncodeToString(routeLeaseBytes)
	if err := database.Create(&model.Peer{Id: deviceID, Uuid: deviceUUID, IdentitySource: model.PeerIdentitySourceStarry}).Error; err != nil {
		t.Fatal(err)
	}
	service.AllService.NetworkActivationVerifier = staticNetworkActivationVerifier{
		id: deviceID, uuid: deviceUUID, activationEpoch: 9, activationID: activationID, routeLease: routeLease,
	}

	call := func(path, body string, handler func(*gin.Context)) *httptest.ResponseRecorder {
		recorder := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(recorder)
		ctx.Request = httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
		ctx.Request.Header.Set("Content-Type", "application/json")
		handler(ctx)
		return recorder
	}
	startBody := `{"id":"301 132 036","uuid":"` + deviceUUID + `","activation_epoch":9,"activation_id":"` + activationID + `","route_leases":["` + routeLease + `"]}`
	start := call("/api/presence/v2/start", startBody, (&Index{}).PresenceStart)
	if start.Code != http.StatusOK {
		t.Fatalf("presence start status=%d body=%q", start.Code, start.Body.String())
	}
	startResponse := presenceLeaseResponse{}
	if err := json.Unmarshal(start.Body.Bytes(), &startResponse); err != nil {
		t.Fatal(err)
	}
	leaseIDBytes, leaseIDErr := base64.RawURLEncoding.DecodeString(startResponse.LeaseID)
	if !startResponse.Accepted || !startResponse.PresenceV2 || startResponse.LeaseToken == "" || leaseIDErr != nil || len(leaseIDBytes) != 16 || startResponse.ActivationEpoch != 9 || startResponse.ActivationID != activationID {
		t.Fatalf("presence start response = %+v", startResponse)
	}
	if start.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("presence bearer response Cache-Control = %q", start.Header().Get("Cache-Control"))
	}

	tamperedToken := base64.RawURLEncoding.EncodeToString(bytes.Repeat([]byte{0x54}, 32))
	tamperedBody := `{"id":"` + deviceID + `","uuid":"` + deviceUUID + `","activation_epoch":9,"activation_id":"` + activationID + `","lease_id":"` + startResponse.LeaseID + `","lease_token":"` + tamperedToken + `"}`
	tampered := call("/api/presence/v2/renew", tamperedBody, (&Index{}).PresenceRenew)
	if tampered.Code != http.StatusUnauthorized || !strings.Contains(tampered.Body.String(), "presence_lease_invalid") || strings.Contains(tampered.Body.String(), tamperedToken) {
		t.Fatalf("tampered renewal status=%d body=%q", tampered.Code, tampered.Body.String())
	}

	leaseBody := `{"id":"` + deviceID + `","uuid":"` + deviceUUID + `","activation_epoch":9,"activation_id":"` + activationID + `","lease_id":"` + startResponse.LeaseID + `","lease_token":"` + startResponse.LeaseToken + `"}`
	renew := call("/api/presence/v2/renew", leaseBody, (&Index{}).PresenceRenew)
	if renew.Code != http.StatusOK || !strings.Contains(renew.Body.String(), `"accepted":true`) {
		t.Fatalf("presence renew status=%d body=%q", renew.Code, renew.Body.String())
	}
	end := call("/api/presence/v2/end", leaseBody, (&Index{}).PresenceEnd)
	if end.Code != http.StatusOK || !strings.Contains(end.Body.String(), `"accepted":true`) {
		t.Fatalf("presence end status=%d body=%q", end.Code, end.Body.String())
	}
	stored := service.AllService.PeerService.FindById(deviceID)
	if service.AllService.PeerService.IsOnlineAt(stored, time.Now().Unix()) {
		t.Fatal("ended v2 lease remained online")
	}
	restarted := call("/api/presence/v2/start", startBody, (&Index{}).PresenceStart)
	if restarted.Code != http.StatusOK {
		t.Fatalf("presence restart status=%d body=%q", restarted.Code, restarted.Body.String())
	}
	deactivated := call("/api/presence/v2/deactivate", startBody, (&Index{}).PresenceDeactivate)
	if deactivated.Code != http.StatusOK || !strings.Contains(deactivated.Body.String(), `"accepted":true`) {
		t.Fatalf("presence deactivate status=%d body=%q", deactivated.Code, deactivated.Body.String())
	}
	replayed := call("/api/presence/v2/start", startBody, (&Index{}).PresenceStart)
	if replayed.Code != http.StatusConflict || !strings.Contains(replayed.Body.String(), "presence_activation_stale") {
		t.Fatalf("retired activation replay status=%d body=%q", replayed.Code, replayed.Body.String())
	}

	profileIDBody := strings.TrimSuffix(startBody, "}") + `,"profile_id":"local-only"}`
	profileID := call("/api/presence/v2/start", profileIDBody, (&Index{}).PresenceStart)
	if profileID.Code != http.StatusBadRequest || !strings.Contains(profileID.Body.String(), "presence_profile_id_forbidden") {
		t.Fatalf("client-local profile ID status=%d body=%q", profileID.Code, profileID.Body.String())
	}
}

func (v staticNetworkPeerVerifier) VerifyPeerIdentity(_ context.Context, id, uuid string) (bool, error) {
	return id == v.id && uuid == v.uuid, nil
}

func TestNetworkDiscoveryRefreshesPeerAndEveryAddressBookReference(t *testing.T) {
	database := clientReportDatabase(t)
	gin.SetMode(gin.TestMode)
	const (
		deviceID   = "301132036"
		deviceUUID = "MDEyMzQ1Njc4OWFiY2RlZg=="
	)
	service.AllService.NetworkPeerVerifier = staticNetworkPeerVerifier{id: deviceID, uuid: deviceUUID}
	addresses := []model.AddressBook{
		{Id: deviceID, UserId: 1, Alias: "Studio", Password: "keep-one", Username: "stale-user", Hostname: "stale-host", Platform: "Linux", Tags: []byte(`["pink"]`)},
		{Id: deviceID, UserId: 2, Alias: "Support", Password: "keep-two", Username: "old", Hostname: "old", Platform: "Linux", Tags: []byte(`["blue"]`)},
	}
	if err := database.Create(&addresses).Error; err != nil {
		t.Fatal(err)
	}

	sysinfo := func(body string) *httptest.ResponseRecorder {
		recorder := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(recorder)
		ctx.Request = httptest.NewRequest(http.MethodPost, "/api/sysinfo", strings.NewReader(body))
		ctx.Request.Header.Set("Content-Type", "application/json")
		ctx.Request.RemoteAddr = "198.51.100.20:44000"
		(&Peer{}).SysInfo(ctx)
		return recorder
	}
	first := sysinfo(`{"id":"301 132 036","uuid":"MDEyMzQ1Njc4OWFiY2RlZg==","cpu":"Ryzen 9","hostname":"render-one","memory":"64 GB","os":"Windows 11","username":"artist","version":"1.4.2"}`)
	if first.Code != http.StatusOK || first.Body.String() != "SYSINFO_UPDATED" {
		t.Fatalf("network sysinfo = status %d body %q", first.Code, first.Body.String())
	}
	peer := service.AllService.PeerService.FindById(deviceID)
	if peer.RowId == 0 || peer.UserId != 0 || peer.IdentitySource != model.PeerIdentitySourceStarry || peer.Hostname != "render-one" || peer.LastSysinfoTime == 0 {
		t.Fatalf("network peer was not discovered: %+v", peer)
	}
	var updated []model.AddressBook
	if err := database.Where("id = ?", deviceID).Order("user_id").Find(&updated).Error; err != nil {
		t.Fatal(err)
	}
	if len(updated) != 2 || updated[0].Username != "artist" || updated[1].Hostname != "render-one" || updated[0].Alias != "Studio" || updated[0].Password != "keep-one" || string(updated[0].Tags) != `["pink"]` {
		t.Fatalf("address-book metadata sync changed protected fields: %+v", updated)
	}

	second := sysinfo(`{"id":"301132036","uuid":"MDEyMzQ1Njc4OWFiY2RlZg==","hostname":"render-renamed","os":"Windows 11","version":"1.4.3"}`)
	if second.Body.String() != "SYSINFO_UPDATED" {
		t.Fatalf("second sysinfo = %q", second.Body.String())
	}
	peer = service.AllService.PeerService.FindById(deviceID)
	if peer.Cpu != "" || peer.Memory != "" || peer.Username != "" || peer.Hostname != "render-renamed" || peer.Version != "1.4.3" {
		t.Fatalf("zero-value inventory did not replace stale data: %+v", peer)
	}
	if err := database.Where("id = ?", deviceID).Order("user_id").Find(&updated).Error; err != nil {
		t.Fatal(err)
	}
	if updated[0].Username != "" || updated[0].Hostname != "render-renamed" || updated[1].Hostname != "render-renamed" {
		t.Fatalf("address books retained stale target metadata: %+v", updated)
	}

	// An older client may upload its cached address-book metadata after the
	// target has refreshed. The verified server inventory must win while the
	// client's personal fields still remain editable.
	staleUpload := []*model.AddressBook{{
		Id: deviceID, Username: "artist", Hostname: "render-one", Platform: "Linux",
		Alias: "Renamed locally", Password: "new-secret", Tags: []byte(`["green"]`),
	}}
	if err := service.AllService.AddressBookService.UpdateAddressBook(staleUpload, 1); err != nil {
		t.Fatal(err)
	}
	storedAddress := service.AllService.AddressBookService.InfoByUserIdAndId(1, deviceID)
	if storedAddress.Username != "" || storedAddress.Hostname != "render-renamed" || storedAddress.Platform != "Windows" {
		t.Fatalf("stale client metadata replaced verified inventory: %+v", storedAddress)
	}
	if storedAddress.Alias != "Renamed locally" || storedAddress.Password != "new-secret" || string(storedAddress.Tags) != `["green"]` {
		t.Fatalf("verified enrichment replaced personal address-book fields: %+v", storedAddress)
	}

	if err := database.Model(&model.Peer{}).Where("row_id = ?", peer.RowId).Update("last_sysinfo_time", 0).Error; err != nil {
		t.Fatal(err)
	}
	heartbeat := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(heartbeat)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/heartbeat", strings.NewReader(`{"id":"301 132 036","uuid":"MDEyMzQ1Njc4OWFiY2RlZg==","ver":10402}`))
	ctx.Request.Header.Set("Content-Type", "application/json")
	(&Index{}).Heartbeat(ctx)
	if heartbeat.Code != http.StatusOK || !strings.Contains(heartbeat.Body.String(), `"sysinfo":true`) {
		t.Fatalf("stale peer heartbeat did not request sysinfo: status=%d body=%q", heartbeat.Code, heartbeat.Body.String())
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
	if err := database.AutoMigrate(&model.User{}, &model.UserToken{}, &model.Peer{}, &model.PeerPresenceLease{}, &model.AddressBook{}, &model.LoginLog{}, &model.AuditConn{}, &model.AuditFile{}); err != nil {
		t.Fatal(err)
	}
	logger := logrus.New()
	service.New(&config.Config{}, database, logger, nil, lock.NewLocal())
	global.DB, global.Logger = database, logger
	bundle := i18n.NewBundle(language.English)
	global.Localizer = func(string) *i18n.Localizer { return i18n.NewLocalizer(bundle, language.English.String()) }
	return database
}
