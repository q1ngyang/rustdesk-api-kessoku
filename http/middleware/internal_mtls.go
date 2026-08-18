package middleware

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/q1ngyang/rustdesk-api-kessoku/v2/config"
)

const InternalCertificateFingerprintKey = "internalCertificateFingerprint"

func InternalMTLS(cfg config.InternalAuth) gin.HandlerFunc {
	allowedURIs := stringSet(cfg.AllowedURISANs)
	allowedDNS := stringSet(cfg.AllowedDNSSANs)
	return func(c *gin.Context) {
		if c.Request.TLS == nil || len(c.Request.TLS.VerifiedChains) == 0 || len(c.Request.TLS.PeerCertificates) == 0 {
			internalAuthProblem(c, http.StatusForbidden, "CLIENT_CERT_DENIED", "verified client certificate required")
			return
		}
		leaf := c.Request.TLS.PeerCertificates[0]
		now := time.Now()
		if now.Before(leaf.NotBefore) || now.After(leaf.NotAfter) || !certificateIdentityAllowed(leaf.URIs, leaf.DNSNames, allowedURIs, allowedDNS) {
			internalAuthProblem(c, http.StatusForbidden, "CLIENT_CERT_DENIED", "client certificate identity denied")
			return
		}
		fingerprint := sha256.Sum256(leaf.Raw)
		c.Set(InternalCertificateFingerprintKey, hex.EncodeToString(fingerprint[:]))
		c.Next()
	}
}

func certificateIdentityAllowed(uris []*url.URL, dnsNames []string, allowedURIs, allowedDNS map[string]struct{}) bool {
	if len(allowedURIs) == 0 && len(allowedDNS) == 0 {
		return false
	}
	for _, uri := range uris {
		if uri != nil {
			if _, ok := allowedURIs[uri.String()]; ok {
				return true
			}
		}
	}
	for _, dnsName := range dnsNames {
		if _, ok := allowedDNS[dnsName]; ok {
			return true
		}
	}
	return false
}

func stringSet(values []string) map[string]struct{} {
	set := make(map[string]struct{}, len(values))
	for _, value := range values {
		if value != "" {
			set[value] = struct{}{}
		}
	}
	return set
}

type internalRateLimiter struct {
	mu          sync.Mutex
	window      int64
	global      int
	perCert     map[string]int
	globalLimit int
	certLimit   int
}

func InternalRateLimit(cfg config.InternalAuth) gin.HandlerFunc {
	limiter := &internalRateLimiter{
		perCert:     make(map[string]int),
		globalLimit: cfg.GlobalRequestsPS,
		certLimit:   cfg.PerCertRequestsPS,
	}
	return func(c *gin.Context) {
		fingerprint, _ := c.Get(InternalCertificateFingerprintKey)
		if !limiter.allow(fingerprintString(fingerprint), time.Now().Unix()) {
			internalAuthProblem(c, http.StatusTooManyRequests, "RATE_LIMITED", "request rate limit exceeded")
			return
		}
		c.Next()
	}
}

func (l *internalRateLimiter) allow(fingerprint string, second int64) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.window != second {
		l.window = second
		l.global = 0
		clear(l.perCert)
	}
	if l.globalLimit > 0 && l.global >= l.globalLimit {
		return false
	}
	if l.certLimit > 0 && l.perCert[fingerprint] >= l.certLimit {
		return false
	}
	l.global++
	l.perCert[fingerprint]++
	return true
}

func fingerprintString(value interface{}) string {
	if fingerprint, ok := value.(string); ok {
		return fingerprint
	}
	return "missing"
}

func internalAuthProblem(c *gin.Context, status int, code, message string) {
	c.AbortWithStatusJSON(status, gin.H{"error": gin.H{
		"code":      code,
		"message":   message,
		"retryable": status == http.StatusTooManyRequests,
	}})
}
