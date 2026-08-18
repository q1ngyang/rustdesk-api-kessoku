package http

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net"
	stdhttp "net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/q1ngyang/rustdesk-api-kessoku/v2/global"
	"github.com/q1ngyang/rustdesk-api-kessoku/v2/http/middleware"
	"github.com/q1ngyang/rustdesk-api-kessoku/v2/http/router"
)

func StartInternalAuthServer() error {
	cfg := global.Config.Auth.Internal
	if !cfg.Enabled {
		return nil
	}
	if cfg.Listen == "" || cfg.ServerCertFile == "" || cfg.ServerKeyFile == "" || cfg.ClientCAFile == "" {
		return errors.New("internal auth listener, server certificate/key, and client CA are required")
	}
	if len(cfg.AllowedURISANs) == 0 && len(cfg.AllowedDNSSANs) == 0 {
		return errors.New("internal auth requires at least one allowed client SAN")
	}
	certificate, err := tls.LoadX509KeyPair(cfg.ServerCertFile, cfg.ServerKeyFile)
	if err != nil {
		return fmt.Errorf("load internal server certificate: %w", err)
	}
	caPEM, err := os.ReadFile(cfg.ClientCAFile)
	if err != nil {
		return fmt.Errorf("read internal client CA: %w", err)
	}
	clientCAs := x509.NewCertPool()
	if !clientCAs.AppendCertsFromPEM(caPEM) {
		return errors.New("internal client CA file contains no certificates")
	}

	engine := gin.New()
	engine.Use(gin.Recovery(), middleware.InternalMTLS(cfg), middleware.InternalRateLimit(cfg), internalRequestTimeout(cfg.EffectiveRequestTimeout()))
	router.InternalAuthInit(engine)
	engine.NoRoute(func(c *gin.Context) { c.Status(stdhttp.StatusNotFound) })

	listener, err := net.Listen("tcp", cfg.Listen)
	if err != nil {
		return fmt.Errorf("listen for internal auth: %w", err)
	}
	tlsListener := tls.NewListener(listener, &tls.Config{
		Certificates: []tls.Certificate{certificate},
		ClientAuth:   tls.RequireAndVerifyClientCert,
		ClientCAs:    clientCAs,
		MinVersion:   tls.VersionTLS13,
	})
	server := &stdhttp.Server{
		Handler:           engine,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       cfg.EffectiveRequestTimeout(),
		WriteTimeout:      cfg.EffectiveRequestTimeout(),
		IdleTimeout:       30 * time.Second,
		MaxHeaderBytes:    16 << 10,
	}
	go func() {
		if serveErr := server.Serve(tlsListener); serveErr != nil && !errors.Is(serveErr, stdhttp.ErrServerClosed) {
			global.Logger.Errorf("internal auth server stopped: %v", serveErr)
		}
	}()
	return nil
}

func internalRequestTimeout(timeout time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), timeout)
		defer cancel()
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}
