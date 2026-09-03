package clientgen

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

func TestClientSetsSecurityHeadersAndMapsProblems(t *testing.T) {
	const (
		configETag    = `"sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"`
		idempotencyID = "control-apply-0001"
	)
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer scoped-token" || r.Header.Get("X-Request-ID") != "0191f6a0-0000-7000-8000-000000000001" {
			t.Errorf("missing security headers: %+v", r.Header)
		}
		if r.URL.Path == "/control/v1/config:apply" {
			if r.Header.Get("If-Match") != configETag || r.Header.Get("Idempotency-Key") != idempotencyID {
				t.Errorf("missing transaction headers: %+v", r.Header)
			}
			w.Header().Set("Content-Type", "application/problem+json")
			w.WriteHeader(http.StatusPreconditionFailed)
			_, _ = w.Write([]byte(`{"error":{"code":"CONFIG_ETAG_MISMATCH","message":"configuration changed","request_id":"agent-request","retryable":false,"details":{}}}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()
	client, err := New(server.URL, server.Client(), func(_ context.Context, scope string) (string, error) {
		if scope != "starry.config.apply" {
			t.Fatalf("scope = %q", scope)
		}
		return "scoped-token", nil
	}, 1024)
	if err != nil {
		t.Fatal(err)
	}
	request := Request{
		Method:         http.MethodPost,
		Path:           "/control/v1/config:apply",
		Scope:          "starry.config.apply",
		RequestID:      "0191f6a0-0000-7000-8000-000000000001",
		Body:           map[string]string{"plan_id": "plan-1"},
		IfMatch:        configETag,
		IdempotencyKey: idempotencyID,
	}
	_, err = client.Do(context.Background(), request, &map[string]interface{}{})
	var httpError *HTTPError
	if !errors.As(err, &httpError) || httpError.Problem.Code != "CONFIG_ETAG_MISMATCH" || httpError.Problem.Status != 412 {
		t.Fatalf("problem mapping = %#v, err=%v", httpError, err)
	}
}

func TestConcurrentApplyPreservesGenerationETagConflict(t *testing.T) {
	const etag = `"sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"`
	var stateMu sync.Mutex
	generation := uint64(41)
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		stateMu.Lock()
		defer stateMu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		if r.Header.Get("If-Match") != etag || generation != 41 {
			w.Header().Set("Content-Type", "application/problem+json")
			w.WriteHeader(http.StatusPreconditionFailed)
			_, _ = w.Write([]byte(`{"error":{"code":"CONFIG_ETAG_MISMATCH","message":"configuration generation changed","retryable":false}}`))
			return
		}
		generation++
		_, _ = w.Write([]byte(`{"accepted":true,"generation":42}`))
	}))
	defer server.Close()
	client, err := New(server.URL, server.Client(), func(context.Context, string) (string, error) { return "token", nil }, 1024)
	if err != nil {
		t.Fatal(err)
	}

	start := make(chan struct{})
	results := make(chan error, 2)
	var wait sync.WaitGroup
	for index := 0; index < 2; index++ {
		wait.Add(1)
		go func(index int) {
			defer wait.Done()
			<-start
			_, callErr := client.Do(context.Background(), Request{
				Method: http.MethodPost, Path: "/control/v1/config:apply", Scope: "starry.config.apply",
				RequestID: "0191f6a0-0000-7000-8000-00000000000" + string(rune('1'+index)),
				Body:      map[string]string{"plan_id": "plan"}, IfMatch: etag,
				IdempotencyKey: "concurrent-apply-000" + string(rune('1'+index)),
			}, &map[string]interface{}{})
			results <- callErr
		}(index)
	}
	close(start)
	wait.Wait()
	close(results)

	succeeded, conflicted := 0, 0
	for callErr := range results {
		if callErr == nil {
			succeeded++
			continue
		}
		var httpError *HTTPError
		if errors.As(callErr, &httpError) && httpError.Problem.Status == http.StatusPreconditionFailed && httpError.Problem.Code == "CONFIG_ETAG_MISMATCH" {
			conflicted++
			continue
		}
		t.Fatalf("unexpected concurrent apply result: %v", callErr)
	}
	if succeeded != 1 || conflicted != 1 || generation != 42 {
		t.Fatalf("concurrent apply outcomes: success=%d conflict=%d generation=%d", succeeded, conflicted, generation)
	}
}

func TestClientMatchesConditionalHeaderContract(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/control/v1/config:validate" || r.Header.Get("If-Match") != "" {
			t.Errorf("unexpected validation request: path=%q headers=%+v", r.URL.Path, r.Header)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"valid":false,"source_digest":null,"effective_digest":null,"diagnostics":[{"code":"CONFIG_INVALID","severity":"error","message":"invalid"}]}`))
	}))
	defer server.Close()
	client, err := New(server.URL, server.Client(), func(context.Context, string) (string, error) {
		return "token", nil
	}, 1024)
	if err != nil {
		t.Fatal(err)
	}
	request := Request{
		Method:    http.MethodPost,
		Path:      "/control/v1/config:validate",
		Scope:     "starry.config.validate",
		RequestID: "0191f6a0-0000-7000-8000-000000000005",
		Body:      map[string]string{"document": "", "format": "yaml"},
	}
	if _, err := client.Do(context.Background(), request, &map[string]interface{}{}); err != nil {
		t.Fatalf("unconditional validation rejected: %v", err)
	}
	request.IfMatch = `"sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"`
	if _, err := client.Do(context.Background(), request, nil); err == nil {
		t.Fatal("unexpected If-Match on validation request accepted")
	}

	request.Path = "/control/v1/config:apply"
	request.Scope = "starry.config.apply"
	request.IfMatch = `"sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"`
	request.IdempotencyKey = "too-short"
	if _, err := client.Do(context.Background(), request, nil); err == nil {
		t.Fatal("short idempotency key accepted")
	}
	request.IdempotencyKey = "control-apply-0002"
	request.IfMatch = `"weak-etag"`
	if _, err := client.Do(context.Background(), request, nil); err == nil {
		t.Fatal("non-SHA-256 If-Match accepted")
	}
}

func TestClientRejectsRedirectAndOversizedResponse(t *testing.T) {
	credentialLeak := false
	target := httptest.NewTLSServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		credentialLeak = r.Header.Get("Authorization") != ""
	}))
	defer target.Close()
	redirect := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/control/v1/status":
			http.Redirect(w, r, target.URL, http.StatusFound)
		case "/control/v1/relays":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"value":"` + strings.Repeat("x", 128) + `"}`))
		}
	}))
	defer redirect.Close()
	client, err := New(redirect.URL, redirect.Client(), func(context.Context, string) (string, error) { return "secret", nil }, 64)
	if err != nil {
		t.Fatal(err)
	}
	base := Request{Method: http.MethodGet, Scope: "starry.control.read", RequestID: "0191f6a0-0000-7000-8000-000000000003"}
	base.Path = "/control/v1/status"
	if _, err := client.Do(context.Background(), base, &map[string]interface{}{}); err == nil {
		t.Fatal("redirect accepted")
	}
	if credentialLeak {
		t.Fatal("authorization header leaked across redirect")
	}
	base.Path = "/control/v1/relays"
	base.Scope = "starry.relay.read"
	if _, err := client.Do(context.Background(), base, &map[string]interface{}{}); err == nil || !strings.Contains(err.Error(), "maximum size") {
		t.Fatalf("oversized response error = %v", err)
	}
}

func TestClientRequiresFixedHTTPSOrigin(t *testing.T) {
	_, err := New("http://127.0.0.1:21120", http.DefaultClient, func(context.Context, string) (string, error) { return "", nil }, 1)
	if err == nil {
		t.Fatal("plain HTTP Starry origin accepted")
	}
}

func TestClientRejectsUnlistedControlPathsAndScopeEscalation(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("invalid control request reached the Agent")
	}))
	defer server.Close()
	client, err := New(server.URL, server.Client(), func(context.Context, string) (string, error) {
		return "token", nil
	}, 1024)
	if err != nil {
		t.Fatal(err)
	}
	base := Request{Method: http.MethodPost, RequestID: "0191f6a0-0000-7000-8000-000000000004", Body: map[string]string{"command": "h"}}
	base.Path = "/control/v1/commands"
	base.Scope = "starry.control.read"
	if _, err := client.Do(context.Background(), base, nil); err == nil {
		t.Fatal("unlisted control path accepted")
	}
	base.Path = "/control/v1/allocations:simulate"
	base.Scope = "starry.config.apply"
	if _, err := client.Do(context.Background(), base, nil); err == nil {
		t.Fatal("scope escalation accepted")
	}
}
