package http

import (
	"context"
	"errors"
	"fmt"
	"net"
	stdhttp "net/http"
	"time"

	"github.com/gin-gonic/gin"
)

const publicAPIShutdownTimeout = 8 * time.Second

func runPublicAPIServer(ctx context.Context, engine *gin.Engine, addr string) error {
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", addr, err)
	}
	server := &stdhttp.Server{
		Addr:              addr,
		Handler:           engine,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       2 * time.Minute,
		WriteTimeout:      2 * time.Minute,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    32 << 10,
	}
	return servePublicAPI(ctx, server, listener)
}

func servePublicAPI(ctx context.Context, server *stdhttp.Server, listener net.Listener) error {
	serveError := make(chan error, 1)
	go func() {
		serveError <- server.Serve(listener)
	}()

	select {
	case err := <-serveError:
		if errors.Is(err, stdhttp.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), publicAPIShutdownTimeout)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			_ = server.Close()
			return fmt.Errorf("graceful shutdown: %w", err)
		}
		err := <-serveError
		if errors.Is(err, stdhttp.ErrServerClosed) {
			return nil
		}
		return err
	}
}
