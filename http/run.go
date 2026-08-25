//go:build !windows

package http

import (
	"github.com/gin-gonic/gin"
	"github.com/q1ngyang/rustdesk-api-kessoku/v3/global"
	stdhttp "net/http"
	"time"
)

func Run(g *gin.Engine, addr string) {
	server := &stdhttp.Server{
		Addr:              addr,
		Handler:           g,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       2 * time.Minute,
		WriteTimeout:      2 * time.Minute,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    32 << 10,
	}
	if err := server.ListenAndServe(); err != nil && err != stdhttp.ErrServerClosed {
		global.Logger.Fatalf("public API server stopped: %v", err)
	}
}
