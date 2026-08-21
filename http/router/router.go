package router

import (
	"io/fs"
	"path"

	"github.com/gin-gonic/gin"
	"github.com/q1ngyang/rustdesk-api-kessoku/v2/global"
	"github.com/q1ngyang/rustdesk-api-kessoku/v2/http/controller/web"
	"net/http"
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
		c.Header("Content-Security-Policy", "default-src 'self'; base-uri 'self'; connect-src 'self'; font-src 'self' data:; form-action 'self'; frame-ancestors 'none'; img-src 'self' data:; object-src 'none'; script-src 'self'; style-src 'self' 'unsafe-inline'")
		c.Header("Cross-Origin-Opener-Policy", "same-origin")
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

	adminRoot := noDirectoryListingFS{FileSystem: http.Dir(global.Config.Gin.ResourcesPath + "/admin")}
	adminFiles := http.StripPrefix("/_admin", http.FileServer(adminRoot))
	admin := g.Group("/_admin").Use(adminWebSecurityHeaders())
	admin.GET("/*filepath", gin.WrapH(adminFiles))
	admin.HEAD("/*filepath", gin.WrapH(adminFiles))
}
