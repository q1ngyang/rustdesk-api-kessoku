package middleware

import (
	"crypto/tls"
	"crypto/x509"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/q1ngyang/rustdesk-api-kessoku/v2/config"
)

func TestInternalMTLSRequiresVerifiedAllowedSAN(t *testing.T) {
	allowed, _ := url.Parse("spiffe://example.com/starry/production")
	denied, _ := url.Parse("spiffe://example.com/other")
	now := time.Now()

	tests := []struct {
		name       string
		state      *tls.ConnectionState
		allowedSAN []string
		wantStatus int
	}{
		{name: "missing TLS", wantStatus: http.StatusForbidden},
		{
			name: "unverified certificate",
			state: &tls.ConnectionState{PeerCertificates: []*x509.Certificate{{
				Raw: []byte("leaf"), URIs: []*url.URL{allowed}, NotBefore: now.Add(-time.Minute), NotAfter: now.Add(time.Minute),
			}}},
			allowedSAN: []string{allowed.String()},
			wantStatus: http.StatusForbidden,
		},
		{
			name: "wrong SAN",
			state: verifiedState(&x509.Certificate{
				Raw: []byte("leaf"), URIs: []*url.URL{denied}, NotBefore: now.Add(-time.Minute), NotAfter: now.Add(time.Minute),
			}),
			allowedSAN: []string{allowed.String()},
			wantStatus: http.StatusForbidden,
		},
		{
			name: "expired certificate",
			state: verifiedState(&x509.Certificate{
				Raw: []byte("leaf"), URIs: []*url.URL{allowed}, NotBefore: now.Add(-time.Hour), NotAfter: now.Add(-time.Minute),
			}),
			allowedSAN: []string{allowed.String()},
			wantStatus: http.StatusForbidden,
		},
		{
			name: "allowed URI SAN",
			state: verifiedState(&x509.Certificate{
				Raw: []byte("leaf"), URIs: []*url.URL{allowed}, NotBefore: now.Add(-time.Minute), NotAfter: now.Add(time.Minute),
			}),
			allowedSAN: []string{allowed.String()},
			wantStatus: http.StatusNoContent,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			engine := gin.New()
			engine.Use(InternalMTLS(config.InternalAuth{AllowedURISANs: tt.allowedSAN}))
			engine.GET("/", func(c *gin.Context) { c.Status(http.StatusNoContent) })
			request := httptest.NewRequest(http.MethodGet, "/", nil)
			request.TLS = tt.state
			recorder := httptest.NewRecorder()
			engine.ServeHTTP(recorder, request)
			if recorder.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d; body=%s", recorder.Code, tt.wantStatus, recorder.Body.String())
			}
		})
	}
}

func verifiedState(leaf *x509.Certificate) *tls.ConnectionState {
	return &tls.ConnectionState{
		PeerCertificates: []*x509.Certificate{leaf},
		VerifiedChains:   [][]*x509.Certificate{{leaf}},
	}
}

func TestInternalRateLimitIsGlobalAndPerCertificate(t *testing.T) {
	limiter := &internalRateLimiter{perCert: map[string]int{}, globalLimit: 2, certLimit: 1}
	if !limiter.allow("a", 100) {
		t.Fatal("first certificate request denied")
	}
	if limiter.allow("a", 100) {
		t.Fatal("per-certificate limit not enforced")
	}
	if !limiter.allow("b", 100) {
		t.Fatal("second certificate request denied before global limit")
	}
	if limiter.allow("c", 100) {
		t.Fatal("global limit not enforced")
	}
	if !limiter.allow("a", 101) {
		t.Fatal("window did not reset")
	}
}
