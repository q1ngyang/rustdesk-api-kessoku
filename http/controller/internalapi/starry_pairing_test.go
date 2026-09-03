package internalapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/q1ngyang/rustdesk-api-kessoku/v3/internal/servercontrolregistry"
	"github.com/q1ngyang/rustdesk-api-kessoku/v3/service"
)

func TestPairingClaimDecoderRejectsUnknownTrailingAndOversizedInput(t *testing.T) {
	gin.SetMode(gin.TestMode)
	valid := `{"version":1,"purpose":"relay","action":"enroll","enrollment_id":"019b1234-5678-7000-8000-000000000001","configuration_digest":"sha256:` + strings.Repeat("1", 64) + `","secret":"` + strings.Repeat("A", 43) + `","request_digest":"sha256:` + strings.Repeat("2", 64) + `","key_fingerprint":"sha256:` + strings.Repeat("3", 64) + `","csr_pem":"csr"}`
	tests := []struct {
		name string
		body string
		ok   bool
	}{
		{name: "exact object", body: valid, ok: true},
		{name: "unknown field", body: strings.TrimSuffix(valid, "}") + `,"nonce":"forbidden"}`},
		{name: "trailing object", body: valid + `{}`},
		{name: "oversized", body: strings.Repeat(" ", 65<<10) + valid},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			context, _ := gin.CreateTestContext(recorder)
			context.Request = httptest.NewRequest(http.MethodPost, "/api/internal/v1/starry/pairing/claim", strings.NewReader(test.body))
			var request servercontrolregistry.ClaimRequest
			err := decodePairingClaim(context, &request)
			if (err == nil) != test.ok {
				t.Fatalf("decode error = %v", err)
			}
		})
	}
}

func TestPairingUnavailableResponsesAreNoStoreAndNeverEchoClaim(t *testing.T) {
	previous := service.AllService
	service.AllService = nil
	t.Cleanup(func() { service.AllService = previous })
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	controller := &StarryPairing{}
	engine.GET("/.well-known/starry-pairing-v1", controller.Preflight)
	engine.POST("/api/internal/v1/starry/pairing/claim", controller.Claim)

	preflight := httptest.NewRecorder()
	engine.ServeHTTP(preflight, httptest.NewRequest(http.MethodGet, "/.well-known/starry-pairing-v1", nil))
	if preflight.Code != http.StatusNotFound || preflight.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("preflight status=%d headers=%v", preflight.Code, preflight.Header())
	}

	secret := strings.Repeat("S", 43)
	claim := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/internal/v1/starry/pairing/claim", strings.NewReader(`{"secret":"`+secret+`"}`))
	request.Header.Set("Content-Type", "application/json")
	engine.ServeHTTP(claim, request)
	if claim.Code != http.StatusNotFound || claim.Header().Get("Cache-Control") != "no-store" || strings.Contains(claim.Body.String(), secret) {
		t.Fatalf("claim status=%d headers=%v body=%s", claim.Code, claim.Header(), claim.Body.String())
	}
}

func TestPairingClaimRateLimitIsGlobalAndStoresOnlyAddressDigests(t *testing.T) {
	limiter := &pairingClaimRateLimiter{clients: make(map[[32]byte]int), globalLimit: 2, clientLimit: 1}
	if !limiter.allow("198.51.100.1", 100) || limiter.allow("198.51.100.1", 100) {
		t.Fatal("per-client claim limit was not enforced")
	}
	if !limiter.allow("198.51.100.2", 100) || limiter.allow("198.51.100.3", 100) {
		t.Fatal("global claim limit was not enforced")
	}
	if !limiter.allow("198.51.100.1", 101) {
		t.Fatal("claim rate-limit window did not reset")
	}
	for digest := range limiter.clients {
		if strings.Contains(string(digest[:]), "198.51.100") {
			t.Fatal("claim limiter retained a complete client address")
		}
	}
}
