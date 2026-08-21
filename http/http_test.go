package http

import (
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestConfigureTrustedProxiesFailsClosedByDefault(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	if err := configureTrustedProxies(engine, ""); err != nil {
		t.Fatal(err)
	}
	engine.GET("/ip", func(c *gin.Context) { c.String(200, c.ClientIP()) })

	request := httptest.NewRequest("GET", "/ip", nil)
	request.RemoteAddr = "198.51.100.20:1234"
	request.Header.Set("X-Forwarded-For", "203.0.113.90")
	response := httptest.NewRecorder()
	engine.ServeHTTP(response, request)
	if response.Body.String() != "198.51.100.20" {
		t.Fatalf("unconfigured proxy changed client IP to %q", response.Body.String())
	}
}

func TestConfigureTrustedProxiesAcceptsOnlyConfiguredProxy(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	if err := configureTrustedProxies(engine, " 127.0.0.1 , "); err != nil {
		t.Fatal(err)
	}
	engine.GET("/ip", func(c *gin.Context) { c.String(200, c.ClientIP()) })

	request := httptest.NewRequest("GET", "/ip", nil)
	request.RemoteAddr = "127.0.0.1:1234"
	request.Header.Set("X-Forwarded-For", "203.0.113.90")
	response := httptest.NewRecorder()
	engine.ServeHTTP(response, request)
	if response.Body.String() != "203.0.113.90" {
		t.Fatalf("configured proxy client IP = %q", response.Body.String())
	}
}
