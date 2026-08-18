package clientgen

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestClientSetsSecurityHeadersAndMapsProblems(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer scoped-token" || r.Header.Get("X-Request-ID") != "0191f6a0-0000-7000-8000-000000000001" {
			t.Errorf("missing security headers: %+v", r.Header)
		}
		if r.URL.Path == "/control/v1/config:apply" {
			if r.Header.Get("If-Match") != `"etag"` || r.Header.Get("Idempotency-Key") != "idem-1" {
				t.Errorf("missing transaction headers: %+v", r.Header)
			}
			w.Header().Set("Content-Type", "application/problem+json")
			w.WriteHeader(http.StatusPreconditionFailed)
			_, _ = w.Write([]byte(`{"status":412,"code":"CONFIG_ETAG_MISMATCH","request_id":"agent-request","retryable":false}`))
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
		IfMatch:        `"etag"`,
		IdempotencyKey: "idem-1",
	}
	_, err = client.Do(context.Background(), request, &map[string]interface{}{})
	var httpError *HTTPError
	if !errors.As(err, &httpError) || httpError.Problem.Code != "CONFIG_ETAG_MISMATCH" || httpError.Problem.Status != 412 {
		t.Fatalf("problem mapping = %#v, err=%v", httpError, err)
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
	base := Request{Method: http.MethodGet, Scope: "starry.control.read", RequestID: "request"}
	base.Path = "/control/v1/status"
	if _, err := client.Do(context.Background(), base, &map[string]interface{}{}); err == nil {
		t.Fatal("redirect accepted")
	}
	if credentialLeak {
		t.Fatal("authorization header leaked across redirect")
	}
	base.Path = "/control/v1/relays"
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
