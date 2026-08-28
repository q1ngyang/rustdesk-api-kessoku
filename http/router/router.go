package router

import (
	"io/fs"
	"net/http"
	"path"

	"github.com/gin-gonic/gin"
	"github.com/q1ngyang/rustdesk-api-kessoku/v3/global"
	"github.com/q1ngyang/rustdesk-api-kessoku/v3/http/controller/web"
)

type noDirectoryListingFS struct {
	http.FileSystem
}

func (fileSystem noDirectoryListingFS) Open(name string) (http.File, error) {
	file, err := fileSystem.FileSystem.Open(name)
	if err != nil {
		return nil, err
	}
	info, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return nil, err
	}
	if !info.IsDir() {
		return file, nil
	}
	index, err := fileSystem.FileSystem.Open(path.Join(name, "index.html"))
	if err != nil {
		_ = file.Close()
		return nil, fs.ErrNotExist
	}
	_ = index.Close()
	return file, nil
}

func adminWebSecurityHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Cache-Control", "no-store")
		// The avatar cropper previews a user-selected local image through an
		// object URL before any bytes leave the browser. Keep blob: scoped to
		// images so that preview works without relaxing script or connect policy.
		c.Header("Content-Security-Policy", "default-src 'self'; base-uri 'self'; connect-src 'self'; font-src 'self' data:; form-action 'self'; frame-ancestors 'none'; img-src 'self' data: blob: https:; object-src 'none'; script-src 'self'; style-src 'self' 'unsafe-inline'")
		// Preserve only the explicitly opened Web Client popup so the admin can
		// deliver a short-lived, connection-only grant with exact-origin
		// postMessage. The client listener deliberately opts out until that
		// one-shot handoff has completed.
		c.Header("Cross-Origin-Opener-Policy", "same-origin-allow-popups")
		c.Header("Cross-Origin-Resource-Policy", "same-origin")
		c.Header("Permissions-Policy", "camera=(), geolocation=(), microphone=(), payment=(), usb=()")
		c.Header("Referrer-Policy", "no-referrer")
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "DENY")
		c.Next()
	}
}

func WebInit(g *gin.Engine) {
	i := &web.Index{}
	g.GET("/", i.Index)

	mediaRoot := noDirectoryListingFS{FileSystem: http.Dir(global.Config.Media.Directory)}
	mediaFiles := http.StripPrefix("/media", http.FileServer(mediaRoot))
	media := g.Group("/media")
	media.Use(func(c *gin.Context) {
		c.Header("Cache-Control", "public, max-age=31536000, immutable")
		c.Header("Cross-Origin-Resource-Policy", "cross-origin")
		c.Header("X-Content-Type-Options", "nosniff")
		c.Next()
	})
	media.GET("/*filepath", gin.WrapH(mediaFiles))
	media.HEAD("/*filepath", gin.WrapH(mediaFiles))

	adminRoot := noDirectoryListingFS{FileSystem: http.Dir(global.Config.Gin.ResourcesPath + "/admin")}
	adminFiles := http.StripPrefix("/dash", http.FileServer(adminRoot))
	admin := g.Group("/dash").Use(adminWebSecurityHeaders())
	admin.GET("/*filepath", gin.WrapH(adminFiles))
	admin.HEAD("/*filepath", gin.WrapH(adminFiles))

	// Keep old bookmarks functional while ensuring every new navigation and
	// OAuth callback lands on the user-facing dashboard path.
	legacy := g.Group("/_admin").Use(adminWebSecurityHeaders())
	legacy.GET("/*filepath", func(c *gin.Context) { c.Redirect(http.StatusPermanentRedirect, "/dash"+c.Param("filepath")) })
	legacy.HEAD("/*filepath", func(c *gin.Context) { c.Redirect(http.StatusPermanentRedirect, "/dash"+c.Param("filepath")) })
}
