package http

import (
	"github.com/gin-gonic/gin"
	"github.com/q1ngyang/rustdesk-api-kessoku/v2/global"
	"github.com/q1ngyang/rustdesk-api-kessoku/v2/http/middleware"
	"github.com/q1ngyang/rustdesk-api-kessoku/v2/http/router"
	"github.com/sirupsen/logrus"
	"net/http"
	"strings"
)

func ApiInit() {
	if err := StartInternalAuthServer(); err != nil {
		global.Logger.Fatalf("start internal authentication API: %v", err)
	}
	gin.SetMode(global.Config.Gin.Mode)
	g := gin.New()

	if err := configureTrustedProxies(g, global.Config.Gin.TrustProxy); err != nil {
		panic(err)
	}

	if global.Config.Gin.Mode == gin.ReleaseMode {
		//修改gin Recovery日志 输出为logger的输出点
		if global.Logger != nil {
			gin.DefaultErrorWriter = global.Logger.WriterLevel(logrus.ErrorLevel)
		}
	}
	g.NoRoute(func(c *gin.Context) {
		c.String(http.StatusNotFound, "404 not found")
	})
	g.Use(middleware.Logger(), middleware.Limiter(), gin.Recovery())
	router.WebInit(g)
	router.Init(g)
	router.ApiInit(g)
	Run(g, global.Config.Gin.ApiAddr)
}

func configureTrustedProxies(engine *gin.Engine, configured string) error {
	proxies := make([]string, 0)
	for _, value := range strings.Split(configured, ",") {
		if proxy := strings.TrimSpace(value); proxy != "" {
			proxies = append(proxies, proxy)
		}
	}
	// Gin otherwise trusts every proxy by default. An empty list deliberately
	// disables forwarded-address trust until the operator names exact proxies.
	return engine.SetTrustedProxies(proxies)
}
