package admin

import (
	"github.com/gin-gonic/gin"
	appConfig "github.com/q1ngyang/rustdesk-api-kessoku/v2/config"
	"github.com/q1ngyang/rustdesk-api-kessoku/v2/global"
	"github.com/q1ngyang/rustdesk-api-kessoku/v2/http/response"
	"github.com/q1ngyang/rustdesk-api-kessoku/v2/model"
	"github.com/q1ngyang/rustdesk-api-kessoku/v2/service"
	"os"
	"strings"
)

type Config struct {
}

type WebClientProviderManifest = appConfig.WebClientProviderManifest

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
	response.Success(c, &gin.H{
		"web_client":          0,
		"web_client_provider": global.Config.WebClientProvider.EffectiveMode(),
	})
}

// WebClientProviderManifest returns only the reviewed public descriptor. The
// authorization record remains deployment-only and no access token, user, or
// session data is accepted by this endpoint.
// @Tags ADMIN
// @Summary External Web Client Provider manifest
// @Description Returns exactly eight public governance fields when external mode is enabled
// @Produce json
// @Success 200 {object} response.Response{data=WebClientProviderManifest}
// @Failure 401 {object} response.Response
// @Router /admin/config/web-client-provider [get]
// @Security token
func (co *Config) WebClientProviderManifest(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	response.Success(c, global.Config.WebClientProvider.Manifest)
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
		response.Success(c, &gin.H{
			"title": global.Config.Admin.Title,
		})
		return
	}

	hello := global.Config.Admin.Hello
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
		"title": global.Config.Admin.Title,
		"hello": hello,
	})
}
