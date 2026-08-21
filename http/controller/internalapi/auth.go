package internalapi

import (
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/q1ngyang/rustdesk-api-kessoku/v2/global"
	"github.com/q1ngyang/rustdesk-api-kessoku/v2/service"
)

type Auth struct{}

type introspectionRequest struct {
	Token string `json:"token"`
}

func (a *Auth) JWKS(c *gin.Context) {
	keys, err := service.AllService.AuthIntrospectionService.JWKS()
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": gin.H{
			"code":      "AUTH_KEY_UNAVAILABLE",
			"message":   "authentication keys are unavailable",
			"retryable": true,
		}})
		return
	}
	c.Header("Cache-Control", "private, max-age=300")
	c.JSON(http.StatusOK, keys)
}

func (a *Auth) Introspect(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	mediaType, _, err := mime.ParseMediaType(c.GetHeader("Content-Type"))
	if err != nil || mediaType != "application/json" {
		c.JSON(http.StatusUnsupportedMediaType, gin.H{"error": gin.H{
			"code":      "CONTENT_TYPE_UNSUPPORTED",
			"message":   "content type must be application/json",
			"retryable": false,
		}})
		return
	}
	request := &introspectionRequest{}
	if err := decodeStrictJSON(c, request, global.Config.Auth.Internal.EffectiveMaxBodyBytes()); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{
			"code":      "REQUEST_INVALID",
			"message":   "request body must contain only a token string",
			"retryable": false,
		}})
		return
	}
	if request.Token == "" || len(request.Token) > global.Config.Auth.EffectiveMaxTokenBytes() {
		c.JSON(http.StatusOK, service.IntrospectionResult{Active: false, Reason: "inactive"})
		return
	}
	c.JSON(http.StatusOK, service.AllService.AuthIntrospectionService.IntrospectContext(c.Request.Context(), request.Token))
}

func decodeStrictJSON(c *gin.Context, destination interface{}, maxBytes int64) error {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxBytes)
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
