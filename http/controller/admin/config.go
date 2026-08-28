package admin

import (
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/q1ngyang/rustdesk-api-kessoku/v3/global"
	"github.com/q1ngyang/rustdesk-api-kessoku/v3/http/response"
	"github.com/q1ngyang/rustdesk-api-kessoku/v3/model"
	"github.com/q1ngyang/rustdesk-api-kessoku/v3/service"
)

type Config struct {
}

// brandingForm deliberately excludes persistence metadata such as id and
// timestamps.  Binding the database model made a GET response impossible to
// POST back because its serialized timestamp format is not a request format.
type brandingForm struct {
	AdminTitle                  string `json:"admin_title"`
	AdminSubtitle               string `json:"admin_subtitle"`
	BrandLogoLightURL           string `json:"brand_logo_light_url"`
	BrandLogoDarkURL            string `json:"brand_logo_dark_url"`
	BrandIconLightURL           string `json:"brand_icon_light_url"`
	BrandIconDarkURL            string `json:"brand_icon_dark_url"`
	LoginBackgroundLightURL     string `json:"login_background_light_url"`
	LoginBackgroundDarkURL      string `json:"login_background_dark_url"`
	WebClientBackgroundLightURL string `json:"web_client_background_light_url"`
	WebClientBackgroundDarkURL  string `json:"web_client_background_dark_url"`
	LoginKicker                 string `json:"login_kicker"`
	LoginHeading                string `json:"login_heading"`
	LoginCopy                   string `json:"login_copy"`
	FooterHTML                  string `json:"footer_html"`
	LoginCustomHTML             string `json:"login_custom_html"`
	LoginCustomCSS              string `json:"login_custom_css"`
	WebClientTitle              string `json:"web_client_title"`
}

func (form *brandingForm) model() *model.BrandingSetting {
	return &model.BrandingSetting{
		AdminTitle: form.AdminTitle, AdminSubtitle: form.AdminSubtitle,
		BrandLogoLightURL: form.BrandLogoLightURL, BrandLogoDarkURL: form.BrandLogoDarkURL,
		BrandIconLightURL: form.BrandIconLightURL, BrandIconDarkURL: form.BrandIconDarkURL,
		LoginBackgroundLightURL: form.LoginBackgroundLightURL, LoginBackgroundDarkURL: form.LoginBackgroundDarkURL,
		WebClientBackgroundLightURL: form.WebClientBackgroundLightURL, WebClientBackgroundDarkURL: form.WebClientBackgroundDarkURL,
		LoginKicker: form.LoginKicker, LoginHeading: form.LoginHeading, LoginCopy: form.LoginCopy,
		FooterHTML: form.FooterHTML, LoginCustomHTML: form.LoginCustomHTML, LoginCustomCSS: form.LoginCustomCSS,
		WebClientTitle: form.WebClientTitle,
	}
}

// AppConfig APP服务配置
// @Tags ADMIN
// @Summary APP服务配置
// @Description APP服务配置
// @Accept  json
// @Produce  json
// @Success 200 {object} response.Response
// @Failure 500 {object} response.Response
// @Router /admin/config/app [get]
// @Security token
func (co *Config) AppConfig(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	c.Header("Pragma", "no-cache")
	publicOrigin := ""
	if global.Config.WebClient.Enabled() {
		publicOrigin = global.Config.WebClient.PublicOrigin
	}
	response.Success(c, &gin.H{
		"web_client_mode":          global.Config.WebClient.EffectiveMode(),
		"web_client_public_origin": publicOrigin,
	})
}

// AdminConfig ADMIN服务配置
// @Tags ADMIN
// @Summary ADMIN服务配置
// @Description ADMIN服务配置
// @Accept  json
// @Produce  json
// @Success 200 {object} response.Response
// @Failure 500 {object} response.Response
// @Router /admin/config/admin [get]
// @Security token
func (co *Config) AdminConfig(c *gin.Context) {

	u := &model.User{}
	token := c.GetHeader("api-token")
	if token != "" {
		u, _ = service.AllService.UserService.InfoByAccessTokenContext(c.Request.Context(), token)
		if !service.AllService.UserService.CheckUserEnable(u) {
			u.Id = 0
		}
	}

	if u.Id == 0 {
		branding := service.AllService.BrandingService.Public()
		response.Success(c, &gin.H{
			"title":    branding.AdminTitle,
			"branding": branding,
		})
		return
	}

	hello := global.Config.Admin.Hello
	branding := service.AllService.BrandingService.Public()
	if system, err := service.AllService.SystemSettingService.Get(); err == nil && system.Announcement != "" {
		hello = system.Announcement
	}
	if hello == "" {
		helloFile := global.Config.Admin.HelloFile
		if helloFile != "" {
			b, err := os.ReadFile(helloFile)
			if err == nil && len(b) > 0 {
				hello = string(b)
			}
		}
	}

	//replace {{username}} to username
	hello = strings.Replace(hello, "{{username}}", u.Username, -1)
	response.Success(c, &gin.H{
		"title":    branding.AdminTitle,
		"hello":    hello,
		"branding": branding,
	})
}

func (co *Config) Branding(c *gin.Context) {
	response.Success(c, service.AllService.BrandingService.Public())
}

func (co *Config) UpdateBranding(c *gin.Context) {
	form := &brandingForm{}
	if err := c.ShouldBindJSON(form); err != nil {
		response.Fail(c, 101, response.TranslateMsg(c, "ParamsError"))
		return
	}
	actor := service.AllService.UserService.CurUser(c)
	if err := service.AllService.BrandingService.UpdateContext(c.Request.Context(), actor.Id, controlRequestID(c), form.model()); err != nil {
		response.Fail(c, 101, err.Error())
		return
	}
	response.Success(c, nil)
}

type systemSettingForm struct {
	Announcement     string `json:"announcement"`
	GeoIPEnabled     bool   `json:"geoip_enabled"`
	GeoIPCityURL     string `json:"geoip_city_url"`
	GeoIPCountryURL  string `json:"geoip_country_url"`
	GeoIPASNURL      string `json:"geoip_asn_url"`
	GeoIPUpdateHours uint   `json:"geoip_update_hours"`
}

func (co *Config) SystemSettings(c *gin.Context) {
	setting, err := service.AllService.SystemSettingService.Get()
	if err != nil {
		response.Fail(c, 101, response.TranslateMsg(c, "OperationFailed"))
		return
	}
	setting.GeoIPUpdating = service.AllService.GeoIPService.IsUpdating()
	response.Success(c, setting)
}

func (co *Config) UpdateSystemSettings(c *gin.Context) {
	form := &systemSettingForm{}
	if err := c.ShouldBindJSON(form); err != nil {
		response.Fail(c, 101, response.TranslateMsg(c, "ParamsError"))
		return
	}
	actor := service.AllService.UserService.CurUser(c)
	next := &model.SystemSetting{Announcement: form.Announcement, GeoIPEnabled: form.GeoIPEnabled, GeoIPCityURL: form.GeoIPCityURL, GeoIPCountryURL: form.GeoIPCountryURL, GeoIPASNURL: form.GeoIPASNURL, GeoIPUpdateHours: form.GeoIPUpdateHours}
	if err := service.AllService.SystemSettingService.UpdateContext(c.Request.Context(), actor.Id, controlRequestID(c), next); err != nil {
		response.Fail(c, 101, err.Error())
		return
	}
	if next.GeoIPEnabled {
		service.AllService.GeoIPService.TriggerUpdate()
	}
	response.Success(c, nil)
}

func (co *Config) UpdateGeoIPDatabase(c *gin.Context) {
	started := service.AllService.GeoIPService.TriggerUpdate()
	response.Success(c, gin.H{"started": started, "updating": started || service.AllService.GeoIPService.IsUpdating()})
}

func (co *Config) GeoIPLookup(c *gin.Context) {
	result, err := service.AllService.GeoIPService.Lookup(c.Query("ip"), c.GetHeader("Accept-Language"))
	if err != nil {
		response.Fail(c, 101, response.TranslateMsg(c, "ParamsError"))
		return
	}
	response.Success(c, result)
}

func (co *Config) UploadBrandAsset(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		response.Fail(c, 101, response.TranslateMsg(c, "ParamsError"))
		return
	}
	opened, err := file.Open()
	if err != nil {
		response.Fail(c, 101, response.TranslateMsg(c, "OperationFailed"))
		return
	}
	defer opened.Close()
	mediaURL, err := service.StoreImage(opened, "branding")
	if err != nil {
		response.Fail(c, http.StatusUnprocessableEntity, err.Error())
		return
	}
	response.Success(c, gin.H{"url": mediaURL})
}

type aboutRelay struct {
	ID      string `json:"id"`
	Version string `json:"version"`
	Native  string `json:"native_state"`
	WSS     string `json:"wss_state"`
}

type aboutInstance struct {
	ID              string       `json:"id"`
	Name            string       `json:"name"`
	Available       bool         `json:"available"`
	StarryVersion   string       `json:"starry_version"`
	UpstreamVersion string       `json:"upstream_version"`
	ProtocolVersion string       `json:"protocol_version"`
	Relays          []aboutRelay `json:"relays"`
	Error           string       `json:"error,omitempty"`
}

func (co *Config) About(c *gin.Context) {
	result := gin.H{
		"kessoku":   gin.H{"version": service.AllService.AppService.GetAppVersion(), "github": "https://github.com/q1ngyang/rustdesk-api-kessoku"},
		"starry":    gin.H{"github": "https://github.com/q1ngyang/rustdesk-server-starry"},
		"instances": []aboutInstance{},
	}
	actor := service.AllService.UserService.CurUser(c)
	if !service.AllService.UserService.IsSuperAdmin(actor) {
		response.Success(c, result)
		return
	}
	instances := service.AllService.StarryControlService.Instances()
	details := make([]aboutInstance, 0, len(instances))
	ctx := controlContext(c)
	for _, instance := range instances {
		detail := aboutInstance{ID: instance.ID, Name: instance.Name, Available: instance.Available, Relays: []aboutRelay{}}
		if instance.Available {
			capabilities, err := service.AllService.StarryControlService.Capabilities(ctx, instance.ID)
			if err != nil {
				detail.Error = "capabilities_unavailable"
			} else {
				detail.StarryVersion = capabilities.Instance.StarryVersion
				detail.UpstreamVersion = capabilities.Instance.UpstreamVersion
				detail.ProtocolVersion = capabilities.Protocol.Version
			}
			relays, err := service.AllService.StarryControlService.Relays(ctx, instance.ID)
			if err != nil {
				if detail.Error == "" {
					detail.Error = "relay_inventory_unavailable"
				}
			} else {
				for _, relay := range relays.Relays {
					version := relay.Version
					if version == "" {
						version = "not_reported"
					}
					detail.Relays = append(detail.Relays, aboutRelay{ID: relay.ID, Version: version, Native: relay.Native.State, WSS: relay.WebSocket.State})
				}
			}
		}
		details = append(details, detail)
	}
	result["instances"] = details
	response.Success(c, result)
}
