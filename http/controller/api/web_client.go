package api

import (
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"github.com/q1ngyang/rustdesk-api-kessoku/v2/global"
	internalAuth "github.com/q1ngyang/rustdesk-api-kessoku/v2/internal/auth"
	"github.com/q1ngyang/rustdesk-api-kessoku/v2/model"
	"github.com/q1ngyang/rustdesk-api-kessoku/v2/service"
)

const webClientRequestLimit = 16 << 10

type WebClient struct{}

type webClientLoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
	DeviceID string `json:"device_id"`
	Platform string `json:"platform"`
}

type webClientGrantRequest struct {
	DeviceID string `json:"device_id"`
	Platform string `json:"platform"`
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
	if err := decodeWebClientJSON(c, request); err != nil || !validWebClientCredential(request.Username, 2, 32) || !validWebClientPassword(request.Password) || !validOptionalWebClientText(request.DeviceID, 128) || !validOptionalWebClientText(request.Platform, 64) {
		global.LoginLimiter.RecordFailedAttempt(clientIP)
		webClientProblem(c, http.StatusBadRequest, "REQUEST_INVALID", "request body is invalid")
		return
	}
	user := service.AllService.UserService.InfoByUsernamePassword(request.Username, request.Password)
	if user.Id == 0 || !service.AllService.UserService.CheckUserEnable(user) {
		global.LoginLimiter.RecordFailedAttempt(clientIP)
		webClientProblem(c, http.StatusUnauthorized, "AUTHENTICATION_FAILED", "authentication failed")
		return
	}
	token := service.AllService.UserService.LoginConnection(user, &model.LoginLog{
		UserId:   user.Id,
		Client:   model.LoginLogClientWeb,
		DeviceId: request.DeviceID,
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
	if err := decodeWebClientJSON(c, request); err != nil || !validOptionalWebClientText(request.DeviceID, 128) || !validOptionalWebClientText(request.Platform, 64) {
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
