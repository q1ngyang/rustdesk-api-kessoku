package middleware

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestRequestBodyLimitRejectsDeclaredAndChunkedOversizeBodies(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(RequestBodyLimit(4))
	engine.POST("/body", func(c *gin.Context) {
		if _, err := io.ReadAll(c.Request.Body); err != nil {
			c.Status(http.StatusRequestEntityTooLarge)
			return
		}
		c.Status(http.StatusNoContent)
	})

	declared := httptest.NewRequest(http.MethodPost, "/body", strings.NewReader("12345"))
	declaredResponse := httptest.NewRecorder()
	engine.ServeHTTP(declaredResponse, declared)
	if declaredResponse.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("declared oversized body status = %d", declaredResponse.Code)
	}

	chunked := httptest.NewRequest(http.MethodPost, "/body", strings.NewReader("12345"))
	chunked.ContentLength = -1
	chunkedResponse := httptest.NewRecorder()
	engine.ServeHTTP(chunkedResponse, chunked)
	if chunkedResponse.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("chunked oversized body status = %d", chunkedResponse.Code)
	}
}
