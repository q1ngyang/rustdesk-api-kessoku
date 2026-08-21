package http

import (
	"errors"
	"fmt"
	"net"
	stdhttp "net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/q1ngyang/rustdesk-api-kessoku/v2/global"
	"github.com/q1ngyang/rustdesk-api-kessoku/v2/http/router"
)

func StartWebClientServer() error {
	if !global.Config.WebClient.Enabled() {
		return nil
	}
	indexPath := filepath.Join(global.Config.Gin.ResourcesPath, "client", "index.html")
	info, err := os.Lstat(indexPath)
	if err != nil {
		return fmt.Errorf("stat web client index: %w", err)
	}
	if !info.Mode().IsRegular() {
		return errors.New("web client index must be a regular file")
	}
	listener, err := net.Listen("tcp", global.Config.WebClient.Listen)
	if err != nil {
		return fmt.Errorf("listen for web client: %w", err)
	}
	engine := gin.New()
	engine.Use(gin.Recovery())
	router.WebClientInit(engine)
	engine.NoRoute(func(c *gin.Context) { c.Status(stdhttp.StatusNotFound) })
	server := &stdhttp.Server{
		Handler:           engine,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    16 << 10,
	}
	go func() {
		if serveErr := server.Serve(listener); serveErr != nil && !errors.Is(serveErr, stdhttp.ErrServerClosed) && global.Logger != nil {
			global.Logger.Errorf("web client server stopped: %v", serveErr)
		}
	}()
	return nil
}
