package api

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"math/big"
	"mime"
	"net/http"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"github.com/q1ngyang/rustdesk-api-kessoku/v3/global"
	internalAuth "github.com/q1ngyang/rustdesk-api-kessoku/v3/internal/auth"
	"github.com/q1ngyang/rustdesk-api-kessoku/v3/model"
	"github.com/q1ngyang/rustdesk-api-kessoku/v3/service"
)

const webClientRequestLimit = 16 << 10

type WebClient struct{}

type webClientLoginRequest struct {
	Username  string `json:"username"`
	Password  string `json:"password"`
	DeviceID  string `json:"device_id"`
	UUID      string `json:"uuid"`
	Platform  string `json:"platform"`
	Challenge string `json:"challenge"`
	TfaCode   string `json:"tfa_code"`
}

type webClientGrantRequest struct {
	DeviceID string `json:"device_id"`
	UUID     string `json:"uuid"`
	Platform string `json:"platform"`
}

type webClientPreferenceRequest struct {
	Language string `json:"language"`
	Theme    string `json:"theme"`
}

type webClientAuditStartRequest struct {
	PeerID   string `json:"peer_id"`
	DeviceID string `json:"device_id"`
	UUID     string `json:"uuid"`
	Platform string `json:"platform"`
}

type webClientAuditFinishRequest struct {
	AuditID   uint   `json:"audit_id"`
	SessionID string `json:"session_id"`
}

type webClientTokenResponse struct {
	ConnectionToken string `json:"connection_token"`
	TokenType       string `json:"token_type"`
	ExpiresAt       int64  `json:"expires_at"`
	ExpiresIn       int64  `json:"expires_in"`
	Audience        string `json:"audience"`
	Scope           string `json:"scope"`
}

func (w *WebClient) Login(c *gin.Context) {
	setWebClientNoStore(c)
	if global.Config.App.DisablePwdLogin {
		webClientProblem(c, http.StatusForbidden, "PASSWORD_LOGIN_DISABLED", "password login is disabled")
		return
	}
	if global.LoginLimiter == nil {
		webClientProblem(c, http.StatusServiceUnavailable, "AUTHENTICATION_UNAVAILABLE", "authentication is unavailable")
		return
	}
	clientIP := c.ClientIP()
	if banned, captchaRequired := global.LoginLimiter.CheckSecurityStatus(clientIP); banned || captchaRequired {
		webClientProblem(c, http.StatusTooManyRequests, "AUTHENTICATION_THROTTLED", "authentication is temporarily unavailable")
		return
	}
	request := &webClientLoginRequest{}
	if err := decodeWebClientJSON(c, request); err != nil {
		global.LoginLimiter.RecordFailedAttempt(clientIP)
		webClientProblem(c, http.StatusBadRequest, "REQUEST_INVALID", "request body is invalid")
		return
	}
	secondFactor := request.Challenge != "" || request.TfaCode != ""
	if !validWebClientCredential(request.Username, 2, 32) || (!secondFactor && !validWebClientPassword(request.Password)) || !validOptionalWebClientText(request.DeviceID, 128) || !validOptionalWebClientText(request.UUID, 128) || !validOptionalWebClientText(request.Platform, 64) || !validOptionalWebClientText(request.Challenge, 128) || !validOptionalWebClientText(request.TfaCode, 16) {
		global.LoginLimiter.RecordFailedAttempt(clientIP)
		webClientProblem(c, http.StatusBadRequest, "REQUEST_INVALID", "request body is invalid")
		return
	}
	binding := service.TwoFactorChallengeBinding{Client: model.LoginLogClientWeb, DeviceID: request.DeviceID, UUID: request.UUID, Platform: request.Platform}
	var user *model.User
	var authErr error
	if secondFactor {
		user, authErr = service.AllService.TwoFactorService.CompleteLoginChallenge(request.Challenge, request.Username, request.TfaCode, binding)
	} else {
		user = service.AllService.UserService.InfoByUsernamePassword(request.Username, request.Password)
	}
	if authErr != nil || user == nil || user.Id == 0 || !service.AllService.UserService.CheckUserEnable(user) {
		global.LoginLimiter.RecordFailedAttempt(clientIP)
		webClientProblem(c, http.StatusUnauthorized, "AUTHENTICATION_FAILED", "authentication failed")
		return
	}
	if !secondFactor && service.AllService.TwoFactorService.EnabledForUser(user.Id) {
		challenge, err := service.AllService.TwoFactorService.CreateLoginChallenge(user, binding)
		if err != nil {
			webClientProblem(c, http.StatusServiceUnavailable, "TWO_FACTOR_UNAVAILABLE", "two-factor challenge is unavailable")
			return
		}
		c.JSON(http.StatusOK, gin.H{"requires_two_factor": true, "challenge": challenge})
		return
	}
	session := service.AllService.UserService.Login(user, &model.LoginLog{
		UserId: user.Id, Client: model.LoginLogClientWeb, DeviceId: request.DeviceID, Uuid: request.UUID,
		Ip: clientIP, Type: model.LoginLogTypeAccount, Platform: request.Platform,
	})
	if session == nil {
		webClientProblem(c, http.StatusServiceUnavailable, "SESSION_UNAVAILABLE", "browser session is unavailable")
		return
	}
	setWebClientSessionCookie(c, session)
	token := service.AllService.UserService.LoginConnection(user, &model.LoginLog{
		UserId:   user.Id,
		Client:   model.LoginLogClientWeb,
		DeviceId: request.DeviceID,
		Uuid:     request.UUID,
		Ip:       clientIP,
		Type:     model.LoginLogTypeAccount,
		Platform: request.Platform,
	}, global.Config.WebClient.EffectiveConnectionTokenTTL())
	if token == nil {
		webClientProblem(c, http.StatusServiceUnavailable, "TOKEN_UNAVAILABLE", "connection token is unavailable")
		return
	}
	global.LoginLimiter.RemoveAttempts(clientIP)
	writeWebClientToken(c, token)
}

func (w *WebClient) Grant(c *gin.Context) {
	setWebClientNoStore(c)
	request := &webClientGrantRequest{}
	if err := decodeWebClientJSON(c, request); err != nil || !validOptionalWebClientText(request.DeviceID, 128) || !validOptionalWebClientText(request.UUID, 128) || !validOptionalWebClientText(request.Platform, 64) {
		webClientProblem(c, http.StatusBadRequest, "REQUEST_INVALID", "request body is invalid")
		return
	}
	user := service.AllService.UserService.CurUser(c)
	if user == nil || user.Id == 0 {
		webClientProblem(c, http.StatusUnauthorized, "AUTHENTICATION_FAILED", "authentication failed")
		return
	}
	token := service.AllService.UserService.LoginConnection(user, &model.LoginLog{
		UserId:   user.Id,
		Client:   model.LoginLogClientWeb,
		DeviceId: request.DeviceID,
		Uuid:     request.UUID,
		Ip:       c.ClientIP(),
		Type:     model.LoginLogTypeGrant,
		Platform: request.Platform,
	}, global.Config.WebClient.EffectiveConnectionTokenTTL())
	if token == nil {
		webClientProblem(c, http.StatusServiceUnavailable, "TOKEN_UNAVAILABLE", "connection token is unavailable")
		return
	}
	writeWebClientToken(c, token)
}

// Preferences stores only non-sensitive presentation choices in host-scoped
// cookies. Cookies are used because browser storage is deliberately prohibited
// in the isolated WebClient, while the admin and WebClient listeners may use
// different ports on the same deployment host.
func (w *WebClient) Preferences(c *gin.Context) {
	setWebClientNoStore(c)
	request := &webClientPreferenceRequest{}
	if err := decodeWebClientJSON(c, request); err != nil || request.Language == "" && request.Theme == "" {
		webClientProblem(c, http.StatusBadRequest, "REQUEST_INVALID", "preference request is invalid")
		return
	}
	if request.Language != "" && !service.ValidPreferenceLanguage(request.Language) {
		webClientProblem(c, http.StatusBadRequest, "REQUEST_INVALID", "language preference is invalid")
		return
	}
	if request.Theme != "" && !service.ValidPreferenceTheme(request.Theme) {
		webClientProblem(c, http.StatusBadRequest, "REQUEST_INVALID", "theme preference is invalid")
		return
	}
	// Anonymous visitors retain a local preference only. Once a revocable
	// browser session exists, the same choice follows the account to the admin
	// console and to other WebClient origins.
	if user, _ := webClientSessionUser(c); user != nil {
		if err := service.AllService.UserService.UpdatePreferencesContext(c.Request.Context(), user, request.Language, request.Theme); err != nil {
			webClientProblem(c, http.StatusInternalServerError, "PREFERENCE_UPDATE_FAILED", "preference update failed")
			return
		}
	}
	if request.Language != "" {
		setWebClientPreferenceCookie(c, "kessoku-language", request.Language)
	}
	if request.Theme != "" {
		setWebClientPreferenceCookie(c, "kessoku-theme", request.Theme)
	}
	c.Status(http.StatusNoContent)
}

func setWebClientPreferenceCookie(c *gin.Context, name, value string) {
	http.SetCookie(c.Writer, &http.Cookie{Name: name, Value: value, Path: "/", MaxAge: 365 * 24 * 60 * 60, Secure: true, SameSite: http.SameSiteNoneMode, Partitioned: true})
}

const webClientSessionCookie = "kessoku_web_session"

func setWebClientSessionCookie(c *gin.Context, token *model.UserToken) {
	maxAge := int(token.ExpiredAt - time.Now().Unix())
	if maxAge < 0 {
		maxAge = 0
	}
	http.SetCookie(c.Writer, &http.Cookie{Name: webClientSessionCookie, Value: token.Token, Path: "/api/web-client/v1", MaxAge: maxAge, Expires: time.Unix(token.ExpiredAt, 0), HttpOnly: true, Secure: true, SameSite: http.SameSiteNoneMode, Partitioned: true})
}

func clearWebClientSessionCookie(c *gin.Context) {
	http.SetCookie(c.Writer, &http.Cookie{Name: webClientSessionCookie, Value: "", Path: "/api/web-client/v1", MaxAge: -1, Expires: time.Unix(1, 0), HttpOnly: true, Secure: true, SameSite: http.SameSiteNoneMode, Partitioned: true})
}

// SessionEstablish turns an exact-origin, short-lived admin connection grant
// into the WebClient's own revocable browser session. The cookie remains owned
// by the API domain and is partitioned by the WebClient top-level site, so no
// parent-domain or shared-cookie assumption is required.
func (w *WebClient) SessionEstablish(c *gin.Context) {
	setWebClientNoStore(c)
	request := &webClientGrantRequest{}
	if err := decodeWebClientJSON(c, request); err != nil || !validOptionalWebClientText(request.DeviceID, 128) || !validOptionalWebClientText(request.UUID, 128) || !validOptionalWebClientText(request.Platform, 64) {
		webClientProblem(c, http.StatusBadRequest, "REQUEST_INVALID", "request body is invalid")
		return
	}
	user := service.AllService.UserService.CurUser(c)
	if user == nil || user.Id == 0 {
		webClientProblem(c, http.StatusUnauthorized, "AUTHENTICATION_FAILED", "authentication failed")
		return
	}
	session := service.AllService.UserService.Login(user, &model.LoginLog{
		UserId: user.Id, Client: model.LoginLogClientWeb, DeviceId: request.DeviceID, Uuid: request.UUID,
		Ip: c.ClientIP(), Type: model.LoginLogTypeGrant, Platform: request.Platform,
	})
	if session == nil {
		webClientProblem(c, http.StatusServiceUnavailable, "SESSION_UNAVAILABLE", "browser session is unavailable")
		return
	}
	setWebClientSessionCookie(c, session)
	c.JSON(http.StatusOK, gin.H{"established": true})
}

func webClientSessionUser(c *gin.Context) (*model.User, string) {
	cookie, err := c.Cookie(webClientSessionCookie)
	if err != nil || cookie == "" {
		return nil, ""
	}
	user, _, _, err := service.AllService.UserService.AuthenticateAccessTokenContext(c.Request.Context(), cookie, internalAuth.APIAudience, "")
	if err != nil || user == nil || user.Id == 0 || !service.AllService.UserService.CheckUserEnable(user) {
		return nil, cookie
	}
	return user, cookie
}

func (w *WebClient) SessionStatus(c *gin.Context) {
	setWebClientNoStore(c)
	user, _ := webClientSessionUser(c)
	if user == nil {
		clearWebClientSessionCookie(c)
		c.JSON(http.StatusOK, gin.H{"authenticated": false})
		return
	}
	displayName := user.Nickname
	if displayName == "" {
		displayName = user.Username
	}
	avatar := ""
	if strings.HasPrefix(user.Avatar, "/media/avatars/") && !strings.Contains(user.Avatar, "..") {
		avatar = strings.TrimRight(global.Config.WebClient.APIOrigin, "/") + user.Avatar
	}
	c.JSON(http.StatusOK, gin.H{
		"authenticated": true, "username": user.Username, "display_name": displayName, "avatar": avatar,
		"preference_language": user.PreferenceLanguage, "preference_theme": user.PreferenceTheme,
	})
}

func (w *WebClient) SessionGrant(c *gin.Context) {
	setWebClientNoStore(c)
	request := &webClientGrantRequest{}
	if err := decodeWebClientJSON(c, request); err != nil || !validOptionalWebClientText(request.DeviceID, 128) || !validOptionalWebClientText(request.UUID, 128) || !validOptionalWebClientText(request.Platform, 64) {
		webClientProblem(c, http.StatusBadRequest, "REQUEST_INVALID", "request body is invalid")
		return
	}
	user, _ := webClientSessionUser(c)
	if user == nil {
		clearWebClientSessionCookie(c)
		webClientProblem(c, http.StatusUnauthorized, "SESSION_EXPIRED", "browser session has expired")
		return
	}
	token := service.AllService.UserService.LoginConnection(user, &model.LoginLog{UserId: user.Id, Client: model.LoginLogClientWeb, DeviceId: request.DeviceID, Uuid: request.UUID, Ip: c.ClientIP(), Type: model.LoginLogTypeGrant, Platform: request.Platform}, global.Config.WebClient.EffectiveConnectionTokenTTL())
	if token == nil {
		webClientProblem(c, http.StatusServiceUnavailable, "TOKEN_UNAVAILABLE", "connection token is unavailable")
		return
	}
	writeWebClientToken(c, token)
}

func (w *WebClient) SessionLogout(c *gin.Context) {
	setWebClientNoStore(c)
	user, token := webClientSessionUser(c)
	clearWebClientSessionCookie(c)
	if user != nil && token != "" {
		_ = service.AllService.UserService.LogoutContext(c.Request.Context(), user, token)
	}
	c.Status(http.StatusNoContent)
}

func (w *WebClient) Logout(c *gin.Context) {
	setWebClientNoStore(c)
	user := service.AllService.UserService.CurUser(c)
	tokenValue, ok := c.Get("token")
	if user == nil || user.Id == 0 || !ok {
		webClientProblem(c, http.StatusUnauthorized, "AUTHENTICATION_FAILED", "authentication failed")
		return
	}
	token, ok := tokenValue.(string)
	if !ok || token == "" || service.AllService.UserService.LogoutContext(c.Request.Context(), user, token) != nil {
		webClientProblem(c, http.StatusServiceUnavailable, "LOGOUT_FAILED", "connection token could not be revoked")
		return
	}
	c.Status(http.StatusNoContent)
}

// AuditConnectionStart records the point at which the authenticated browser
// has completed RustDesk's remote handshake. It intentionally runs after the
// connection succeeds, so failed password attempts never look like sessions.
func (w *WebClient) AuditConnectionStart(c *gin.Context) {
	setWebClientNoStore(c)
	request := &webClientAuditStartRequest{}
	if err := decodeWebClientJSON(c, request); err != nil || !validWebClientCredential(request.PeerID, 1, 128) || !validWebClientCredential(request.DeviceID, 1, 128) || !validWebClientCredential(request.UUID, 1, 128) || !validOptionalWebClientText(request.Platform, 64) {
		webClientProblem(c, http.StatusBadRequest, "REQUEST_INVALID", "connection audit request is invalid")
		return
	}
	user := service.AllService.UserService.CurUser(c)
	if user == nil || user.Id == 0 {
		webClientProblem(c, http.StatusUnauthorized, "AUTHENTICATION_FAILED", "authentication failed")
		return
	}
	sessionBytes := make([]byte, 32)
	if _, err := rand.Read(sessionBytes); err != nil {
		webClientProblem(c, http.StatusServiceUnavailable, "AUDIT_UNAVAILABLE", "connection audit is unavailable")
		return
	}
	connectionNumber, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 63))
	if err != nil {
		webClientProblem(c, http.StatusServiceUnavailable, "AUDIT_UNAVAILABLE", "connection audit is unavailable")
		return
	}
	connID := connectionNumber.Int64()
	if connID == 0 {
		connID = 1
	}
	displayName := user.Nickname
	if displayName == "" {
		displayName = user.Username
	}
	audit := &model.AuditConn{
		UserId: user.Id, Client: model.LoginLogClientWeb, Action: model.AuditActionNew,
		ConnId: connID, PeerId: request.PeerID, FromPeer: request.DeviceID, FromName: displayName,
		Ip: c.ClientIP(), SessionId: base64.RawURLEncoding.EncodeToString(sessionBytes), Uuid: request.UUID,
	}
	if err := service.AllService.AuditService.CreateAuditConn(audit); err != nil {
		webClientProblem(c, http.StatusServiceUnavailable, "AUDIT_UNAVAILABLE", "connection audit is unavailable")
		return
	}
	peer := service.AllService.PeerService.FindByUserIdAndId(user.Id, request.PeerID)
	if peer.RowId == 0 && service.AllService.UserService.IsAdmin(user) {
		candidate := service.AllService.PeerService.FindById(request.PeerID)
		if service.AllService.AdminScopeService.CanManagePeer(user, candidate.RowId) {
			peer = candidate
		}
	}
	peerHostname := ""
	if peer.RowId > 0 && validOptionalWebClientText(peer.Hostname, 128) {
		peerHostname = peer.Hostname
	}
	c.JSON(http.StatusOK, gin.H{"audit_id": audit.Id, "session_id": audit.SessionId, "peer_hostname": peerHostname})
}

// AuditConnectionFinish closes only the audit created by the same user and
// opaque browser-session identifier. Repeated close notifications are safe.
func (w *WebClient) AuditConnectionFinish(c *gin.Context) {
	setWebClientNoStore(c)
	request := &webClientAuditFinishRequest{}
	if err := decodeWebClientJSON(c, request); err != nil || request.AuditID == 0 || !validWebClientCredential(request.SessionID, 32, 128) {
		webClientProblem(c, http.StatusBadRequest, "REQUEST_INVALID", "connection audit request is invalid")
		return
	}
	user := service.AllService.UserService.CurUser(c)
	if user == nil || user.Id == 0 {
		webClientProblem(c, http.StatusUnauthorized, "AUTHENTICATION_FAILED", "authentication failed")
		return
	}
	if err := service.AllService.AuditService.CloseWebClientAudit(c.Request.Context(), request.AuditID, user.Id, request.SessionID, time.Now().Unix()); err != nil {
		webClientProblem(c, http.StatusConflict, "AUDIT_CLOSE_REJECTED", "connection audit could not be closed")
		return
	}
	c.Status(http.StatusNoContent)
}

func writeWebClientToken(c *gin.Context, token *model.UserToken) {
	expiresIn := token.ExpiredAt - time.Now().Unix()
	if expiresIn < 0 {
		expiresIn = 0
	}
	c.JSON(http.StatusOK, webClientTokenResponse{
		ConnectionToken: token.Token,
		TokenType:       "Bearer",
		ExpiresAt:       token.ExpiredAt,
		ExpiresIn:       expiresIn,
		Audience:        internalAuth.ConnectionAudience,
		Scope:           internalAuth.ConnectScope,
	})
}

func setWebClientNoStore(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	c.Header("Pragma", "no-cache")
}

func webClientProblem(c *gin.Context, status int, code, message string) {
	c.JSON(status, gin.H{"error": gin.H{"code": code, "message": message}})
}

func decodeWebClientJSON(c *gin.Context, destination interface{}) error {
	mediaType, _, err := mime.ParseMediaType(c.GetHeader("Content-Type"))
	if err != nil || mediaType != "application/json" {
		return errors.New("content type must be application/json")
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, webClientRequestLimit)
	decoder := json.NewDecoder(c.Request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return err
	}
	var trailing interface{}
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("multiple JSON values are not allowed")
		}
		return err
	}
	return nil
}

func validWebClientCredential(value string, minimum, maximum int) bool {
	return len(value) >= minimum && len(value) <= maximum && validWebClientText(value, false)
}

func validWebClientPassword(value string) bool {
	if len(value) < 4 || len(value) > 128 || !utf8.ValidString(value) {
		return false
	}
	for _, character := range value {
		if unicode.IsControl(character) {
			return false
		}
	}
	return true
}

func validOptionalWebClientText(value string, maximum int) bool {
	return value == "" || len(value) <= maximum && validWebClientText(value, false)
}

func validWebClientText(value string, allowEmpty bool) bool {
	if (!allowEmpty && value == "") || strings.TrimSpace(value) != value || !utf8.ValidString(value) {
		return false
	}
	for _, character := range value {
		if unicode.IsControl(character) {
			return false
		}
	}
	return true
}
