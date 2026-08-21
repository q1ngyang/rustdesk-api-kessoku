package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/q1ngyang/rustdesk-api-kessoku/v2/config"
	"github.com/q1ngyang/rustdesk-api-kessoku/v2/internal/starrycontrol"
	"github.com/q1ngyang/rustdesk-api-kessoku/v2/model"
)

type auditedControlProvider struct {
	starrycontrol.ServerControlProvider
	statusCalls   int
	applyCalls    int
	historyCalls  int
	rollbackCalls int
	operation     starrycontrol.Operation
	revisions     []starrycontrol.ConfigRevision
	rollback      starrycontrol.Operation
}

func (p *auditedControlProvider) Status(context.Context) (starrycontrol.Status, error) {
	p.statusCalls++
	return starrycontrol.Status{}, nil
}

func (p *auditedControlProvider) ApplyConfig(context.Context, starrycontrol.ApplyRequest) (starrycontrol.Operation, error) {
	p.applyCalls++
	return starrycontrol.Operation{ID: "0191f6a0-0000-7000-8000-000000000002", Kind: "config_apply", State: "pending"}, nil
}

func (p *auditedControlProvider) Operation(context.Context, string) (starrycontrol.Operation, error) {
	return p.operation, nil
}

func (p *auditedControlProvider) ConfigHistory(context.Context) ([]starrycontrol.ConfigRevision, error) {
	p.historyCalls++
	return p.revisions, nil
}

func (p *auditedControlProvider) RollbackConfig(context.Context, starrycontrol.RollbackRequest) (starrycontrol.Operation, error) {
	p.rollbackCalls++
	return p.rollback, nil
}

func TestServerControlReadsAndRejectedWritesAreAudited(t *testing.T) {
	database := securityAuditDatabase(t, true)
	provider := &auditedControlProvider{}
	control := auditedControlService(provider, true)
	ctx := starrycontrol.WithRequestMetadata(context.Background(), starrycontrol.RequestMetadata{
		ActorUserID: 42,
		RequestID:   "0191f6a0-0000-7000-8000-000000000001",
	})

	if _, err := control.Status(ctx, "starry-1"); err != nil {
		t.Fatalf("audited status call failed: %v", err)
	}
	if _, err := control.ApplyConfig(ctx, "starry-1", starrycontrol.ApplyRequest{
		PlanID: "plan-1", IfMatch: `"etag-1"`, IdempotencyKey: "apply-1",
	}); !errors.Is(err, starrycontrol.ErrReadOnly) {
		t.Fatalf("read-only apply error = %v", err)
	}
	if provider.statusCalls != 1 || provider.applyCalls != 0 {
		t.Fatalf("provider calls: status=%d apply=%d", provider.statusCalls, provider.applyCalls)
	}

	events := []model.AdminAuditEvent{}
	if err := database.Where("target_type = ?", "starry_instance").Order("id").Find(&events).Error; err != nil {
		t.Fatal(err)
	}
	if len(events) != 2 {
		t.Fatalf("server-control audit event count = %d, want 2", len(events))
	}
	if events[0].Action != "server_control.status.read" || events[0].Result != "success" {
		t.Fatalf("status audit = %+v", events[0])
	}
	if events[1].Action != "server_control.config.apply" || events[1].Result != "failure" || events[1].ErrorCode != "CONTROL_READ_ONLY" {
		t.Fatalf("rejected apply audit = %+v", events[1])
	}
}

func TestServerControlOperationExpectationPersistsUntilTerminalPoll(t *testing.T) {
	database := securityAuditDatabase(t, true)
	digest := "sha256:" + strings.Repeat("a", 64)
	operationID := "0191f6a0-0000-7000-8000-000000000002"
	provider := &auditedControlProvider{}
	control := auditedControlService(provider, false)
	ctx := starrycontrol.WithRequestMetadata(context.Background(), starrycontrol.RequestMetadata{
		ActorUserID: 42,
		RequestID:   "0191f6a0-0000-7000-8000-000000000020",
	})
	started, err := control.ApplyConfig(ctx, "starry-1", starrycontrol.ApplyRequest{CandidateDigest: digest})
	if err != nil || started.ID != operationID {
		t.Fatalf("start operation: result=%+v error=%v", started, err)
	}
	event := &model.AdminAuditEvent{}
	if err := database.Where("action = ?", "server_control.config.apply").First(event).Error; err != nil {
		t.Fatal(err)
	}
	if event.Result != "pending" {
		t.Fatalf("initial async audit result = %q", event.Result)
	}
	provider.operation = starrycontrol.Operation{
		ID: operationID, Kind: "config_apply", State: "succeeded",
		ActivationAck: &starrycontrol.ActivationAck{SourceDigest: digest},
	}
	if _, err := control.Operation(ctx, "starry-1", operationID); err != nil {
		t.Fatal(err)
	}
	if err := database.First(event, event.Id).Error; err != nil {
		t.Fatal(err)
	}
	if event.Result != "success" {
		t.Fatalf("terminal async audit result = %q", event.Result)
	}
	provider.operation.ActivationAck.SourceDigest = "sha256:" + strings.Repeat("b", 64)
	if _, err := control.Operation(ctx, "starry-1", operationID); err == nil {
		t.Fatal("mismatched terminal source digest was accepted")
	}
}

func TestServerControlDoesNotCallProviderWithoutAuditStorage(t *testing.T) {
	securityAuditDatabase(t, false)
	provider := &auditedControlProvider{}
	control := auditedControlService(provider, false)
	ctx := starrycontrol.WithRequestMetadata(context.Background(), starrycontrol.RequestMetadata{
		ActorUserID: 42,
		RequestID:   "0191f6a0-0000-7000-8000-000000000003",
	})

	if _, err := control.Status(ctx, "starry-1"); err == nil {
		t.Fatal("server-control read proceeded without audit storage")
	}
	if provider.statusCalls != 0 {
		t.Fatal("provider was called before the required audit intent was persisted")
	}
}

func TestServerControlRollbackBindsRevisionDigestAndFinalAudit(t *testing.T) {
	database := securityAuditDatabase(t, true)
	digest := "sha256:" + strings.Repeat("c", 64)
	revisionID := "0191f6a0-0000-7000-8000-000000000030"
	operationID := "0191f6a0-0000-7000-8000-000000000031"
	provider := &auditedControlProvider{
		revisions: []starrycontrol.ConfigRevision{{ID: revisionID, CandidateDigest: digest}},
		rollback:  starrycontrol.Operation{ID: operationID, Kind: "config_rollback", State: "pending"},
	}
	control := auditedControlService(provider, false)
	ctx := starrycontrol.WithRequestMetadata(context.Background(), starrycontrol.RequestMetadata{
		ActorUserID: 42,
		RequestID:   "0191f6a0-0000-7000-8000-000000000032",
	})
	started, err := control.RollbackConfig(ctx, "starry-1", starrycontrol.RollbackRequest{RevisionID: revisionID})
	if err != nil || started.ID != operationID {
		t.Fatalf("start rollback: result=%+v error=%v", started, err)
	}
	if provider.historyCalls != 1 || provider.rollbackCalls != 1 {
		t.Fatalf("rollback provider calls: history=%d rollback=%d", provider.historyCalls, provider.rollbackCalls)
	}
	expectation := &model.ControlOperationExpectation{}
	if err := database.Where("operation_id = ?", operationID).First(expectation).Error; err != nil {
		t.Fatal(err)
	}
	if expectation.Kind != "config_rollback" || expectation.ExpectedSourceDigest != digest {
		t.Fatalf("rollback expectation = %+v", expectation)
	}
	provider.operation = starrycontrol.Operation{
		ID: operationID, Kind: "config_rollback", State: "succeeded",
		ActivationAck: &starrycontrol.ActivationAck{SourceDigest: digest},
	}
	if _, err := control.Operation(ctx, "starry-1", operationID); err != nil {
		t.Fatal(err)
	}
	event := &model.AdminAuditEvent{}
	if err := database.Where("action = ?", "server_control.config.rollback").First(event).Error; err != nil {
		t.Fatal(err)
	}
	if event.Result != "success" {
		t.Fatalf("terminal rollback audit result = %q", event.Result)
	}
}

func TestServerControlRollbackRejectsUnknownRevisionBeforeMutation(t *testing.T) {
	database := securityAuditDatabase(t, true)
	provider := &auditedControlProvider{revisions: []starrycontrol.ConfigRevision{}}
	control := auditedControlService(provider, false)
	ctx := starrycontrol.WithRequestMetadata(context.Background(), starrycontrol.RequestMetadata{
		ActorUserID: 42,
		RequestID:   "0191f6a0-0000-7000-8000-000000000040",
	})
	_, err := control.RollbackConfig(ctx, "starry-1", starrycontrol.RollbackRequest{RevisionID: "0191f6a0-0000-7000-8000-000000000041"})
	if !errors.Is(err, starrycontrol.ErrRequestInvalid) {
		t.Fatalf("unknown revision error = %v", err)
	}
	if provider.historyCalls != 1 || provider.rollbackCalls != 0 {
		t.Fatalf("unknown revision provider calls: history=%d rollback=%d", provider.historyCalls, provider.rollbackCalls)
	}
	event := &model.AdminAuditEvent{}
	if err := database.Where("action = ?", "server_control.config.rollback").First(event).Error; err != nil {
		t.Fatal(err)
	}
	if event.Result != "failure" || event.ErrorCode != "REQUEST_INVALID" {
		t.Fatalf("unknown revision audit = %+v", event)
	}
}

func auditedControlService(provider starrycontrol.ServerControlProvider, readOnly bool) *StarryControlService {
	return &StarryControlService{
		config: config.ServerControl{ReadOnly: readOnly},
		instances: map[string]ServerControlInstance{
			"starry-1": {ID: "starry-1", Name: "Primary", Enabled: true, Available: true, ReadOnly: readOnly},
		},
		providers: map[string]starrycontrol.ServerControlProvider{"starry-1": provider},
	}
}
