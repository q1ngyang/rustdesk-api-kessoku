package internalapi

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/q1ngyang/rustdesk-api-kessoku/v3/internal/servercontrolregistry"
	"github.com/q1ngyang/rustdesk-api-kessoku/v3/internal/starrycontrol"
	"github.com/q1ngyang/rustdesk-api-kessoku/v3/service"
)

type StarryPairing struct{}

var publicPairingClaimLimiter = &pairingClaimRateLimiter{
	clients: make(map[[32]byte]int), globalLimit: 100, clientLimit: 10,
}

type pairingClaimRateLimiter struct {
	mu          sync.Mutex
	window      int64
	global      int
	clients     map[[32]byte]int
	globalLimit int
	clientLimit int
}

type StarryPairingAPIError struct {
	Code string `json:"code"`
}

type StarryPairingAPIProblem struct {
	Error StarryPairingAPIError `json:"error"`
}

// Preflight deliberately has no invented response document. The frozen Starry
// client requires only an HTTPS success response and verifies its SPKI pin.
func (s *StarryPairing) Preflight(c *gin.Context) {
	pairingNoStore(c)
	if service.AllService == nil || service.AllService.StarryControlService == nil || !service.AllService.StarryControlService.PairingEnabled() {
		c.Status(http.StatusNotFound)
		return
	}
	c.Status(http.StatusNoContent)
}

// Claim consumes the exact frozen SP1 claim object. The response bundle is
// purpose-specific and is never cached; failed claims intentionally expose no
// secret-validation detail.
// @Tags Starry Pairing
// @Summary Claim a pre-approved SP1 enrollment
// @Accept json
// @Produce json
// @Param X-Request-ID header string false "Request UUID"
// @Param body body servercontrolregistry.ClaimRequest true "Frozen SP1 claim"
// @Success 200 {object} servercontrolregistry.ClaimResponse
// @Failure 400 {object} StarryPairingAPIProblem
// @Failure 409 {object} StarryPairingAPIProblem
// @Failure 410 {object} StarryPairingAPIProblem
// @Failure 415 {object} StarryPairingAPIProblem
// @Failure 429 {object} StarryPairingAPIProblem
// @Failure 503 {object} StarryPairingAPIProblem
// @Router /internal/v1/starry/pairing/claim [post]
func (s *StarryPairing) Claim(c *gin.Context) {
	pairingNoStore(c)
	if service.AllService == nil || service.AllService.StarryControlService == nil || !service.AllService.StarryControlService.PairingEnabled() {
		pairingFailure(c, http.StatusNotFound, "pairing_not_available")
		return
	}
	if !publicPairingClaimLimiter.allow(c.ClientIP(), time.Now().Unix()) {
		c.Header("Retry-After", "1")
		pairingFailure(c, http.StatusTooManyRequests, "pairing_rate_limited")
		return
	}
	mediaType, _, err := mime.ParseMediaType(c.GetHeader("Content-Type"))
	if err != nil || mediaType != "application/json" {
		pairingFailure(c, http.StatusUnsupportedMediaType, "pairing_content_type_unsupported")
		return
	}
	request := servercontrolregistry.ClaimRequest{}
	if err := decodePairingClaim(c, &request); err != nil {
		pairingFailure(c, http.StatusBadRequest, "pairing_claim_invalid")
		return
	}
	requestID := strings.TrimSpace(c.GetHeader("X-Request-ID"))
	if _, err := uuid.Parse(requestID); err != nil {
		generated, generationErr := uuid.NewV7()
		if generationErr != nil {
			generated = uuid.New()
		}
		requestID = generated.String()
	}
	c.Header("X-Request-ID", requestID)
	ctx := starrycontrol.WithRequestMetadata(c.Request.Context(), starrycontrol.RequestMetadata{RequestID: requestID, Service: true})
	response, err := service.AllService.StarryControlService.ClaimPairing(ctx, request)
	if err != nil {
		status := http.StatusServiceUnavailable
		var providerError *starrycontrol.ProviderError
		if errors.As(err, &providerError) {
			switch providerError.Status {
			case http.StatusNotFound, http.StatusGone:
				status = http.StatusGone
			case http.StatusConflict:
				status = http.StatusConflict
			case http.StatusTooManyRequests:
				status = http.StatusTooManyRequests
			}
		}
		pairingFailure(c, status, "pairing_claim_rejected")
		return
	}
	c.JSON(http.StatusOK, response)
}

func decodePairingClaim(c *gin.Context, destination interface{}) error {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 64<<10)
	decoder := json.NewDecoder(c.Request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return err
	}
	var trailing interface{}
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return errors.New("pairing claim must contain exactly one JSON object")
	}
	return nil
}

func pairingNoStore(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	c.Header("Pragma", "no-cache")
	c.Header("Referrer-Policy", "no-referrer")
	c.Header("X-Content-Type-Options", "nosniff")
}

func pairingFailure(c *gin.Context, status int, code string) {
	c.JSON(status, gin.H{"error": gin.H{"code": code}})
}

func (l *pairingClaimRateLimiter) allow(clientAddress string, second int64) bool {
	digest := sha256.Sum256([]byte("kessoku-pairing-claim-client-v1\n" + clientAddress))
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.clients == nil || l.window != second {
		l.window = second
		l.global = 0
		l.clients = make(map[[32]byte]int)
	}
	if l.globalLimit > 0 && l.global >= l.globalLimit || l.clientLimit > 0 && l.clients[digest] >= l.clientLimit {
		return false
	}
	l.global++
	l.clients[digest]++
	return true
}
