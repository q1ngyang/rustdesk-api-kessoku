//go:build !windows

package http

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/gin-gonic/gin"
	"github.com/q1ngyang/rustdesk-api-kessoku/v3/global"
)

func Run(g *gin.Engine, addr string) {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM, syscall.SIGQUIT)
	defer stop()
	if err := runPublicAPIServer(ctx, g, addr); err != nil {
		global.Logger.Fatalf("public API server stopped: %v", err)
	}
}
