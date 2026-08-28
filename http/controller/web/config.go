package web

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/q1ngyang/rustdesk-api-kessoku/v3/global"
	"github.com/q1ngyang/rustdesk-api-kessoku/v3/service"
)

type clientPublicConfig struct {
	SchemaVersion        int                             `json:"schema_version"`
	ProfileGeneration    uint64                          `json:"profile_generation"`
	APIOrigin            string                          `json:"api_origin"`
	RendezvousWSSURL     string                          `json:"rendezvous_wss_url"`
	RelayWSSURLs         map[string]string               `json:"relay_wss_urls"`
	ServerPublicKey      string                          `json:"server_public_key"`
	ServerKeyFingerprint string                          `json:"server_key_fingerprint"`
	Branding             service.PublicWebClientBranding `json:"branding"`
	Preferences          clientPreferences               `json:"preferences"`
}

type clientPreferences struct {
	Language string `json:"language"`
	Theme    string `json:"theme"`
}

func webClientPreferences(c *gin.Context) clientPreferences {
	preferences := clientPreferences{}
	if language, err := c.Cookie("kessoku-language"); err == nil {
		for _, allowed := range []string{"zh-CN", "zh-TW", "en", "fr", "es", "ru", "ko", "ja"} {
			if language == allowed {
				preferences.Language = language
				break
			}
		}
	}
	if theme, err := c.Cookie("kessoku-theme"); err == nil && (theme == "light" || theme == "dark") {
		preferences.Theme = theme
	}
	return preferences
}

// ClientConfig returns only immutable public connection trust data. Listener,
// token lifetime, users, credentials, and internal/control endpoints are not
// represented in this DTO.
func (i *Index) ClientConfig(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	c.Header("Pragma", "no-cache")
	base := global.Config.WebClient.PublicConfig()
	branding := service.PublicWebClientBranding{}
	if service.AllService != nil && service.AllService.BrandingService != nil {
		branding = service.AllService.BrandingService.PublicForWebClient()
	}
	c.JSON(http.StatusOK, clientPublicConfig{
		SchemaVersion: base.SchemaVersion, ProfileGeneration: base.ProfileGeneration,
		APIOrigin: base.APIOrigin, RendezvousWSSURL: base.RendezvousWSSURL,
		RelayWSSURLs: base.RelayWSSURLs, ServerPublicKey: base.ServerPublicKey,
		ServerKeyFingerprint: base.ServerKeyFingerprint, Branding: branding, Preferences: webClientPreferences(c),
	})
}

// Preferences stores presentation choices on the WebClient's own origin. This
// is intentionally separate from the API listener because cookies cannot be
// shared between unrelated production domains.
func (i *Index) Preferences(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 4096)
	request := &clientPreferences{}
	decoder := json.NewDecoder(c.Request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(request); err != nil || request.Language == "" && request.Theme == "" {
		c.Status(http.StatusBadRequest)
		return
	}
	var trailing interface{}
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		c.Status(http.StatusBadRequest)
		return
	}
	if request.Language != "" {
		allowed := false
		for _, language := range []string{"zh-CN", "zh-TW", "en", "ja", "ko", "fr", "es", "ru"} {
			allowed = allowed || request.Language == language
		}
		if !allowed {
			c.Status(http.StatusBadRequest)
			return
		}
		http.SetCookie(c.Writer, &http.Cookie{Name: "kessoku-language", Value: request.Language, Path: "/", MaxAge: 365 * 24 * 60 * 60, Secure: true, SameSite: http.SameSiteLaxMode})
	}
	if request.Theme != "" {
		if request.Theme != "light" && request.Theme != "dark" {
			c.Status(http.StatusBadRequest)
			return
		}
		http.SetCookie(c.Writer, &http.Cookie{Name: "kessoku-theme", Value: request.Theme, Path: "/", MaxAge: 365 * 24 * 60 * 60, Secure: true, SameSite: http.SameSiteLaxMode})
	}
	c.Status(http.StatusNoContent)
}
