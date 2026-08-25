package router

import (
	"net/http"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/q1ngyang/rustdesk-api-kessoku/v3/global"
	"github.com/q1ngyang/rustdesk-api-kessoku/v3/http/controller/web"
)

func webClientSecurityHeaders() gin.HandlerFunc {
	connectSources := append([]string{"'self'"}, global.Config.WebClient.CSPConnectSources()...)
	csp := "default-src 'none'; base-uri 'none'; connect-src " + strings.Join(connectSources, " ") + "; font-src 'self'; form-action 'none'; frame-ancestors 'none'; img-src 'self' data:; manifest-src 'self'; media-src 'none'; object-src 'none'; script-src 'self'; style-src 'self'; worker-src 'none'"
	return func(c *gin.Context) {
		if strings.HasPrefix(c.Request.URL.Path, "/assets/") {
			c.Header("Cache-Control", "public, max-age=31536000, immutable")
		} else {
			c.Header("Cache-Control", "no-store")
		}
		c.Header("Content-Security-Policy", csp)
		// Do not send COOP on this listener. The admin origin uses
		// same-origin-allow-popups and must retain this cross-origin WindowProxy
		// only long enough for the exact-origin one-shot grant handoff. The client
		// clears its opener reference as soon as it acknowledges the grant.
		c.Header("Cross-Origin-Resource-Policy", "same-origin")
		c.Header("Permissions-Policy", "camera=(), geolocation=(), microphone=(), payment=(), usb=()")
		c.Header("Referrer-Policy", "no-referrer")
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "DENY")
		c.Next()
	}
}

// WebClientInit is used only by the dedicated browser-client listener. It
// intentionally registers no Kessoku API, admin, Swagger, upload, control, or
// internal-auth routes.
func WebClientInit(g *gin.Engine) {
	g.Use(webClientSecurityHeaders())
	controller := &web.Index{}
	g.GET("/config/v1.json", controller.ClientConfig)

	clientRoot := filepath.Join(global.Config.Gin.ResourcesPath, "client")
	indexPath := filepath.Join(clientRoot, "index.html")
	g.GET("/", func(c *gin.Context) { c.File(indexPath) })
	g.HEAD("/", func(c *gin.Context) { c.File(indexPath) })
	assetsRoot := noDirectoryListingFS{FileSystem: http.Dir(filepath.Join(clientRoot, "assets"))}
	assetFiles := http.StripPrefix("/assets", http.FileServer(assetsRoot))
	assets := g.Group("/assets")
	assets.GET("/*filepath", gin.WrapH(assetFiles))
	assets.HEAD("/*filepath", gin.WrapH(assetFiles))
}
