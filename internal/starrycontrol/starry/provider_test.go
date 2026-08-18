package starry

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/q1ngyang/rustdesk-api-kessoku/v2/internal/starrycontrol"
	"github.com/q1ngyang/rustdesk-api-kessoku/v2/internal/starrycontrol/clientgen"
)

func TestTypedProviderSimulationAndConfigTransactions(t *testing.T) {
	var mu sync.Mutex
	var calls []string
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		calls = append(calls, r.Method+" "+r.URL.Path)
		mu.Unlock()
		if r.Header.Get("Authorization") != "Bearer control-token" || r.Header.Get("X-Request-ID") == "" {
			t.Errorf("missing control authentication headers")
		}
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/control/v1/capabilities":
			_, _ = w.Write([]byte(`{"protocol":{"name":"starry-control","version":"1.0.0","major":1},"instance":{"id":"instance-uuid"}}`))
		case "/control/v1/allocations:simulate":
			var input starrycontrol.SimulationInput
			if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
				t.Error(err)
			}
			if input.ClientA.IP != "192.0.2.10" || input.ClientB.IP != "2001:db8::20" || input.Transport != starrycontrol.TransportMixed {
				t.Errorf("unexpected simulation input: %+v", input)
			}
			_, _ = w.Write([]byte(`{"config_generation":42,"health_snapshot_id":"snapshot","candidates":[],"selection":{"kind":"rule","non_binding":true},"warnings":["simulation has no side effects"]}`))
		case "/control/v1/config":
			w.Header().Set("ETag", `"etag-42"`)
			_, _ = w.Write([]byte(`{"generation":42,"schema_version":3,"yaml":"version: 3\n","values":{},"runtime_in_sync":true}`))
		case "/control/v1/config:plan":
			if r.Header.Get("If-Match") != `"etag-42"` {
				t.Errorf("plan missing If-Match")
			}
			var body map[string]interface{}
			_ = json.NewDecoder(r.Body).Decode(&body)
			if _, leaked := body["base_etag"]; leaked {
				t.Error("base_etag leaked into transport body")
			}
			_, _ = w.Write([]byte(`{"plan_id":"plan-1","base_etag":"\"etag-42\"","base_generation":42,"target_generation":43,"candidate_digest":"sha256:test","schema_version":3,"changes":[],"warnings":[],"restart_required":false,"expires_at":"2026-08-18T10:00:00Z"}`))
		case "/control/v1/config:apply":
			if r.Header.Get("If-Match") != `"etag-42"` || r.Header.Get("Idempotency-Key") != "idem-1" {
				t.Errorf("apply transaction headers missing: %+v", r.Header)
			}
			_, _ = w.Write([]byte(`{"operation_id":"0191f6a0-0000-7000-8000-000000000002","status":"pending"}`))
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	client, err := clientgen.New(server.URL, server.Client(), func(context.Context, string) (string, error) {
		return "control-token", nil
	}, 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	provider := &Provider{instanceID: "instance-uuid", client: client}
	ctx := starrycontrol.WithRequestMetadata(context.Background(), starrycontrol.RequestMetadata{
		ActorUserID: 42,
		RequestID:   "0191f6a0-0000-7000-8000-000000000001",
	})

	if _, err := provider.Capabilities(ctx); err != nil {
		t.Fatal(err)
	}
	simulation, err := provider.SimulateAllocation(ctx, starrycontrol.SimulationInput{
		ClientA:   starrycontrol.SimulationClient{IP: "192.0.2.10"},
		ClientB:   starrycontrol.SimulationClient{IP: "2001:db8::20"},
		Transport: starrycontrol.TransportMixed,
		Explain:   true,
	})
	if err != nil || simulation.ConfigGeneration != 42 || !simulation.Selection.NonBinding {
		t.Fatalf("simulation = %+v, err=%v", simulation, err)
	}
	beforeInvalid := len(calls)
	if _, err := provider.SimulateAllocation(ctx, starrycontrol.SimulationInput{
		ClientA: starrycontrol.SimulationClient{IP: "not-an-ip"}, ClientB: starrycontrol.SimulationClient{IP: "192.0.2.2"}, Transport: starrycontrol.TransportNative,
	}); err == nil {
		t.Fatal("invalid IP accepted")
	}
	if len(calls) != beforeInvalid {
		t.Fatal("invalid simulation reached Agent")
	}

	document, err := provider.GetConfig(ctx)
	if err != nil || document.ETag != `"etag-42"` {
		t.Fatalf("config = %+v, err=%v", document, err)
	}
	yaml := "version: 3\n"
	plan, err := provider.PlanConfig(ctx, starrycontrol.ConfigCandidate{YAML: &yaml, BaseETag: `"etag-42"`})
	if err != nil || plan.PlanID != "plan-1" {
		t.Fatalf("plan = %+v, err=%v", plan, err)
	}
	apply, err := provider.ApplyConfig(ctx, starrycontrol.ApplyRequest{PlanID: plan.PlanID, IfMatch: `"etag-42"`, IdempotencyKey: "idem-1"})
	if err != nil || !strings.HasPrefix(apply.OperationID, "0191") {
		t.Fatalf("apply = %+v, err=%v", apply, err)
	}
}
