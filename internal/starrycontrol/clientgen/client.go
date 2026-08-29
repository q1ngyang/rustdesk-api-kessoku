// Package clientgen is the checked-in transport layer for Starry Control API
// v1. It is generated-compatible: contract DTOs stay outside controllers and
// no production build downloads schemas or generators.
package clientgen

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"strings"

	"github.com/google/uuid"
)

type TokenSource func(context.Context, string) (string, error)

type Client struct {
	baseURL          *url.URL
	httpClient       *http.Client
	tokenSource      TokenSource
	maxRequestBytes  int64
	maxResponseBytes int64
}

type Request struct {
	Method         string
	Path           string
	Scope          string
	RequestID      string
	Body           interface{}
	IfMatch        string
	IdempotencyKey string
}

type Problem struct {
	Type      string          `json:"type"`
	Title     string          `json:"title"`
	Status    int             `json:"status"`
	Code      string          `json:"code"`
	Detail    string          `json:"detail"`
	Message   string          `json:"message"`
	RequestID string          `json:"request_id"`
	Retryable bool            `json:"retryable"`
	Errors    json.RawMessage `json:"errors"`
	Details   json.RawMessage `json:"details"`
}

type problemEnvelope struct {
	Error Problem `json:"error"`
}

type HTTPError struct {
	Problem Problem
}

func (e *HTTPError) Error() string {
	return fmt.Sprintf("Starry Control API returned %d %s", e.Problem.Status, e.Problem.Code)
}

func New(baseURL string, httpClient *http.Client, tokenSource TokenSource, maxResponseBytes int64) (*Client, error) {
	parsed, err := url.Parse(baseURL)
	if err != nil {
		return nil, fmt.Errorf("parse Starry base URL: %w", err)
	}
	if parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return nil, errors.New("Starry base URL must be a fixed HTTPS origin without credentials, query, or fragment")
	}
	if parsed.Path != "" && parsed.Path != "/" {
		return nil, errors.New("Starry base URL must not contain a path")
	}
	if httpClient == nil || tokenSource == nil {
		return nil, errors.New("Starry HTTP client and token source are required")
	}
	if maxResponseBytes <= 0 {
		maxResponseBytes = 1 << 20
	}
	clone := *httpClient
	clone.CheckRedirect = func(_ *http.Request, _ []*http.Request) error {
		return http.ErrUseLastResponse
	}
	return &Client{
		baseURL:          parsed,
		httpClient:       &clone,
		tokenSource:      tokenSource,
		maxRequestBytes:  1 << 20,
		maxResponseBytes: maxResponseBytes,
	}, nil
}

func (c *Client) Do(ctx context.Context, request Request, destination interface{}) (http.Header, error) {
	policy, validRoute := controlRequestPolicy(request.Method, request.Path)
	if !validRoute || request.Scope != policy.scope {
		return nil, errors.New("invalid fixed Starry control request")
	}
	if _, err := uuid.Parse(request.RequestID); err != nil {
		return nil, errors.New("Starry control request id must be a UUID")
	}
	if policy.bodyRequired != (request.Body != nil) ||
		policy.ifMatchRequired != (request.IfMatch != "") ||
		policy.ifMatchRequired && !validStrongETag(request.IfMatch) ||
		policy.idempotencyRequired != (request.IdempotencyKey != "") {
		return nil, errors.New("invalid Starry control request body or ETag")
	}
	if policy.idempotencyRequired && !validIdempotencyKey(request.IdempotencyKey) {
		return nil, errors.New("invalid Starry control idempotency key")
	}
	var body io.Reader
	if request.Body != nil {
		encoded, err := json.Marshal(request.Body)
		if err != nil {
			return nil, fmt.Errorf("encode Starry request: %w", err)
		}
		if int64(len(encoded)) > c.maxRequestBytes {
			return nil, errors.New("Starry request exceeds maximum size")
		}
		body = bytes.NewReader(encoded)
	}
	target := *c.baseURL
	target.Path = request.Path
	token, err := c.tokenSource(ctx, request.Scope)
	if err != nil {
		return nil, fmt.Errorf("authorize Starry request: %w", err)
	}
	httpRequest, err := http.NewRequestWithContext(ctx, request.Method, target.String(), body)
	if err != nil {
		return nil, fmt.Errorf("create Starry request: %w", err)
	}
	httpRequest.Header.Set("Accept", "application/json, application/problem+json")
	httpRequest.Header.Set("Authorization", "Bearer "+token)
	httpRequest.Header.Set("X-Request-ID", request.RequestID)
	if request.Body != nil {
		httpRequest.Header.Set("Content-Type", "application/json")
	}
	if request.IfMatch != "" {
		httpRequest.Header.Set("If-Match", request.IfMatch)
	}
	if request.IdempotencyKey != "" {
		httpRequest.Header.Set("Idempotency-Key", request.IdempotencyKey)
	}

	response, err := c.httpClient.Do(httpRequest)
	if err != nil {
		return nil, fmt.Errorf("call Starry Control API: %w", err)
	}
	defer response.Body.Close()
	limited := io.LimitReader(response.Body, c.maxResponseBytes+1)
	encoded, err := io.ReadAll(limited)
	if err != nil {
		return nil, fmt.Errorf("read Starry response: %w", err)
	}
	if int64(len(encoded)) > c.maxResponseBytes {
		return nil, errors.New("Starry response exceeds maximum size")
	}
	mediaType, _, _ := mime.ParseMediaType(response.Header.Get("Content-Type"))
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		problem := Problem{Status: response.StatusCode, Code: "STARRY_CONTROL_ERROR", Retryable: response.StatusCode >= 500}
		if mediaType == "application/problem+json" || mediaType == "application/json" {
			flat := Problem{}
			if json.Unmarshal(encoded, &flat) == nil && flat.Code != "" {
				problem = flat
			}
			envelope := problemEnvelope{}
			if json.Unmarshal(encoded, &envelope) == nil && envelope.Error.Code != "" {
				problem = envelope.Error
			}
		}
		problem.Status = response.StatusCode
		problem.Code = normalizeProblemCode(problem.Code)
		return response.Header.Clone(), &HTTPError{Problem: problem}
	}
	if destination == nil || response.StatusCode == http.StatusNoContent {
		return response.Header.Clone(), nil
	}
	if mediaType != "application/json" {
		return nil, errors.New("Starry response has an unsupported content type")
	}
	decoder := json.NewDecoder(bytes.NewReader(encoded))
	if err := decoder.Decode(destination); err != nil {
		return nil, fmt.Errorf("decode Starry response: %w", err)
	}
	var trailing interface{}
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return nil, errors.New("Starry response contains multiple JSON values")
	}
	return response.Header.Clone(), nil
}

type requestPolicy struct {
	scope               string
	bodyRequired        bool
	ifMatchRequired     bool
	idempotencyRequired bool
}

func controlRequestPolicy(method, path string) (requestPolicy, bool) {
	key := method + " " + path
	policies := map[string]requestPolicy{
		"GET /control/v1/capabilities":          {scope: "starry.control.read"},
		"GET /control/v1/status":                {scope: "starry.control.read"},
		"GET /control/v1/relays":                {scope: "starry.relay.read"},
		"POST /control/v1/peers:verify":         {scope: "starry.peer.verify", bodyRequired: true},
		"POST /control/v1/allocations:simulate": {scope: "starry.relay.simulate", bodyRequired: true},
		"GET /control/v1/config":                {scope: "starry.config.read"},
		"GET /control/v1/config/schema":         {scope: "starry.config.read"},
		"POST /control/v1/config:validate":      {scope: "starry.config.validate", bodyRequired: true},
		"POST /control/v1/config:plan":          {scope: "starry.config.plan", bodyRequired: true, ifMatchRequired: true},
		"POST /control/v1/config:apply":         {scope: "starry.config.apply", bodyRequired: true, ifMatchRequired: true, idempotencyRequired: true},
		"GET /control/v1/config/history":        {scope: "starry.config.read"},
		"POST /control/v1/config:rollback":      {scope: "starry.config.rollback", bodyRequired: true, ifMatchRequired: true, idempotencyRequired: true},
		"POST /control/v1/runtime:reload":       {scope: "starry.runtime.reload", bodyRequired: true, idempotencyRequired: true},
	}
	if policy, ok := policies[key]; ok {
		return policy, true
	}
	const operationPrefix = "/control/v1/operations/"
	if method == http.MethodGet && strings.HasPrefix(path, operationPrefix) {
		if _, err := uuid.Parse(strings.TrimPrefix(path, operationPrefix)); err == nil {
			return requestPolicy{scope: "starry.control.read"}, true
		}
	}
	return requestPolicy{}, false
}

func validHeaderValue(value string, maximum int) bool {
	if value == "" || len(value) > maximum || strings.TrimSpace(value) != value {
		return false
	}
	for _, character := range value {
		if character < 0x21 || character > 0x7e {
			return false
		}
	}
	return true
}

func validIdempotencyKey(value string) bool {
	if len(value) < 16 || !validHeaderValue(value, 128) {
		return false
	}
	for _, character := range value {
		if character >= 'a' && character <= 'z' || character >= 'A' && character <= 'Z' || character >= '0' && character <= '9' || strings.ContainsRune("._:-", character) {
			continue
		}
		return false
	}
	return true
}

func validStrongETag(value string) bool {
	if len(value) != 73 || !strings.HasPrefix(value, `"sha256:`) || !strings.HasSuffix(value, `"`) {
		return false
	}
	for _, character := range value[8:72] {
		if character < '0' || character > '9' && character < 'a' || character > 'f' {
			return false
		}
	}
	return true
}

func normalizeProblemCode(code string) string {
	if code == "" || len(code) > 96 {
		return "STARRY_CONTROL_ERROR"
	}
	for _, character := range code {
		if character >= 'A' && character <= 'Z' || character >= '0' && character <= '9' || character == '_' {
			continue
		}
		return "STARRY_CONTROL_ERROR"
	}
	return code
}
