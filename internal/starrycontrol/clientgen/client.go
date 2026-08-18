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
	RequestID string          `json:"request_id"`
	Retryable bool            `json:"retryable"`
	Errors    json.RawMessage `json:"errors"`
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
	if request.RequestID == "" || request.Scope == "" || !strings.HasPrefix(request.Path, "/control/v1/") && request.Path != "/control/v1/capabilities" {
		return nil, errors.New("invalid fixed Starry control request")
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
			_ = json.Unmarshal(encoded, &problem)
			if problem.Status == 0 {
				problem.Status = response.StatusCode
			}
		}
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
	return response.Header.Clone(), nil
}
