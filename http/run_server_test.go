package http

import (
	"context"
	"net"
	stdhttp "net/http"
	"testing"
	"time"
)

func TestServePublicAPIStopsCleanlyWhenContextIsCanceled(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	server := &stdhttp.Server{
		Handler: stdhttp.HandlerFunc(func(response stdhttp.ResponseWriter, _ *stdhttp.Request) {
			response.WriteHeader(stdhttp.StatusNoContent)
		}),
	}
	result := make(chan error, 1)
	go func() {
		result <- servePublicAPI(ctx, server, listener)
	}()

	client := &stdhttp.Client{Timeout: time.Second}
	response, err := client.Get("http://" + listener.Addr().String())
	if err != nil {
		t.Fatalf("public API was not reachable before shutdown: %v", err)
	}
	_ = response.Body.Close()
	cancel()

	select {
	case err := <-result:
		if err != nil {
			t.Fatalf("graceful shutdown failed: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("public API did not stop after context cancellation")
	}
}
