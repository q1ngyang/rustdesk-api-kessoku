package service

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/q1ngyang/rustdesk-api-kessoku/v3/config"
	"github.com/q1ngyang/rustdesk-api-kessoku/v3/internal/starrycontrol"
	"github.com/q1ngyang/rustdesk-api-kessoku/v3/model"
)

type auditedControlProvider struct {
	starrycontrol.ServerControlProvider
	statusCalls   int
	applyCalls    int
	historyCalls  int
	rollbackCalls int
	schemaDigest  string
	operation     starrycontrol.Operation
	revisions     []starrycontrol.ConfigRevision
	rollback      starrycontrol.Operation
}

func (p *auditedControlProvider) Capabilities(context.Context) (starrycontrol.Capabilities, error) {
	digest := p.schemaDigest
	if digest == "" {
		digest = "sha256:" + strings.Repeat("d", 64)
	}
	return starrycontrol.Capabilities{Config: starrycontrol.ConfigCapabilities{SchemaDigest: digest}}, nil
}

func (p *auditedControlProvider) VerifyPeer(ctx context.Context, input starrycontrol.PeerIdentityInput) (starrycontrol.PeerVerification, error) {
	metadata, ok := starrycontrol.MetadataFromContext(ctx)
	if !ok || !metadata.Service || metadata.ActorUserID != 0 {
		return starrycontrol.PeerVerification{}, errors.New("peer verification was not service-authenticated")
	}
	return starrycontrol.PeerVerification{InstanceID: "starry-1", Registered: input.ID == "301132036" && input.UUID == "uuid-1"}, nil
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

func TestPeerRegistryVerificationUsesServiceIdentityWithoutAdminAudit(t *testing.T) {
	database := securityAuditDatabase(t, true)
	provider := &auditedControlProvider{}
	control := auditedControlService(provider, false)
	verified, err := control.VerifyPeerIdentity(context.Background(), "301132036", "uuid-1")
	if err != nil || !verified {
		t.Fatalf("peer registry verification = %v, err=%v", verified, err)
	}
	verified, err = control.VerifyPeerIdentity(context.Background(), "999", "wrong")
	if err != nil || verified {
		t.Fatalf("unknown peer registry verification = %v, err=%v", verified, err)
	}
	var count int64
	if err := database.Model(&model.AdminAuditEvent{}).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("background peer checks created %d administrator audit events", count)
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
	plan := starrycontrol.ConfigPlan{
		PlanID: "plan-1", CandidateDigest: digest, BaseETag: `"etag-1"`, BaseGeneration: 41,
		Changes: []json.RawMessage{json.RawMessage(`{"pointer":"/fast_mode/relay/fast_media_v1_enabled","kind":"replace"}`)},
		Impact:  starrycontrol.PlanImpact{Risk: "high"}, ExpiresAt: time.Now().Add(time.Minute),
	}
	reviewToken, tokenErr := control.sealPlanReview(ctx, "starry-1", plan, "sha256:"+strings.Repeat("d", 64))
	if tokenErr != nil {
		t.Fatal(tokenErr)
	}
	started, err := control.ApplyConfig(ctx, "starry-1", starrycontrol.ApplyRequest{
		PlanID: plan.PlanID, CandidateDigest: digest, IfMatch: plan.BaseETag, ReviewToken: reviewToken,
		RiskConfirmation: "confirm:" + plan.PlanID + ":" + digest,
	})
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
		ActivationAck: &starrycontrol.ActivationAck{
			Generation: 42, SchemaVersion: 5, SourceDigest: digest,
			EffectiveDigest: "sha256:" + strings.Repeat("e", 64),
			SubsystemAcks:   []starrycontrol.SubsystemAck{{Subsystem: "hbbs", Accepted: true, Detail: "must not be persisted"}},
		},
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
	metadata := map[string]interface{}{}
	if err := json.Unmarshal(event.Metadata, &metadata); err != nil {
		t.Fatal(err)
	}
	before, _ := metadata["before"].(map[string]interface{})
	ack, _ := metadata["activation_ack"].(map[string]interface{})
	ackItems, _ := ack["subsystem_acks"].([]interface{})
	if metadata["schema_digest"] != "sha256:"+strings.Repeat("d", 64) || metadata["risk"] != "high" ||
		before["etag"] != plan.BaseETag || before["generation"] != float64(41) || ack["generation"] != float64(42) ||
		ack["schema_version"] != float64(5) || len(ackItems) != 1 {
		t.Fatalf("apply audit evidence = %#v", metadata)
	}
	if _, leaked := ackItems[0].(map[string]interface{})["detail"]; leaked {
		t.Fatalf("subsystem detail was persisted in audit metadata: %#v", ackItems[0])
	}
	provider.operation.ActivationAck.SourceDigest = "sha256:" + strings.Repeat("b", 64)
	if _, err := control.Operation(ctx, "starry-1", operationID); err == nil {
		t.Fatal("mismatched terminal source digest was accepted")
	}
}

func TestPlanReviewTokenIsBoundToAdministrator(t *testing.T) {
	securityAuditDatabase(t, true)
	provider := &auditedControlProvider{}
	control := auditedControlService(provider, false)
	ownerContext := starrycontrol.WithRequestMetadata(context.Background(), starrycontrol.RequestMetadata{
		ActorUserID: 42, RequestID: "0191f6a0-0000-7000-8000-000000000060",
	})
	digest := "sha256:" + strings.Repeat("a", 64)
	plan := starrycontrol.ConfigPlan{
		PlanID: "plan-owner", CandidateDigest: digest, BaseETag: `"etag-owner"`,
		Impact: starrycontrol.PlanImpact{Risk: "low"}, ExpiresAt: time.Now().Add(time.Minute),
	}
	token, err := control.sealPlanReview(ownerContext, "starry-1", plan, "sha256:"+strings.Repeat("b", 64))
	if err != nil {
		t.Fatal(err)
	}
	otherContext := starrycontrol.WithRequestMetadata(context.Background(), starrycontrol.RequestMetadata{
		ActorUserID: 43, RequestID: "0191f6a0-0000-7000-8000-000000000061",
	})
	_, err = control.ApplyConfig(otherContext, "starry-1", starrycontrol.ApplyRequest{
		PlanID: plan.PlanID, CandidateDigest: digest, IfMatch: plan.BaseETag, ReviewToken: token,
	})
	var providerError *starrycontrol.ProviderError
	if !errors.As(err, &providerError) || providerError.Code != "PLAN_REVIEW_INVALID" || provider.applyCalls != 0 {
		t.Fatalf("cross-administrator review token: calls=%d error=%v", provider.applyCalls, err)
	}
}

func TestPlanReviewRejectsSchemaDigestDriftBeforeApply(t *testing.T) {
	securityAuditDatabase(t, true)
	provider := &auditedControlProvider{schemaDigest: "sha256:" + strings.Repeat("e", 64)}
	control := auditedControlService(provider, false)
	ctx := starrycontrol.WithRequestMetadata(context.Background(), starrycontrol.RequestMetadata{
		ActorUserID: 42, RequestID: "0191f6a0-0000-7000-8000-000000000062",
	})
	digest := "sha256:" + strings.Repeat("a", 64)
	plan := starrycontrol.ConfigPlan{
		PlanID: "plan-schema-drift", CandidateDigest: digest, BaseETag: `"etag-schema-drift"`,
		Impact: starrycontrol.PlanImpact{Risk: "medium"}, ExpiresAt: time.Now().Add(time.Minute),
	}
	token, err := control.sealPlanReview(ctx, "starry-1", plan, "sha256:"+strings.Repeat("d", 64))
	if err != nil {
		t.Fatal(err)
	}
	_, err = control.ApplyConfig(ctx, "starry-1", starrycontrol.ApplyRequest{
		PlanID: plan.PlanID, CandidateDigest: digest, IfMatch: plan.BaseETag, ReviewToken: token,
	})
	var providerError *starrycontrol.ProviderError
	if !errors.As(err, &providerError) || providerError.Code != "PLAN_REVIEW_SCHEMA_CHANGED" || provider.applyCalls != 0 {
		t.Fatalf("schema-drift review token: calls=%d error=%v", provider.applyCalls, err)
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
	started, err := control.RollbackConfig(ctx, "starry-1", starrycontrol.RollbackRequest{
		RevisionID: revisionID, RiskConfirmation: "confirm:rollback:starry-1:" + revisionID,
	})
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
	revisionID := "0191f6a0-0000-7000-8000-000000000041"
	_, err := control.RollbackConfig(ctx, "starry-1", starrycontrol.RollbackRequest{
		RevisionID: revisionID, RiskConfirmation: "confirm:rollback:starry-1:" + revisionID,
	})
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

func TestServerControlRollbackRequiresExactHighRiskConfirmationBeforeProviderCall(t *testing.T) {
	database := securityAuditDatabase(t, true)
	revisionID := "0191f6a0-0000-7000-8000-000000000050"
	provider := &auditedControlProvider{
		revisions: []starrycontrol.ConfigRevision{{ID: revisionID, CandidateDigest: "sha256:" + strings.Repeat("d", 64)}},
	}
	control := auditedControlService(provider, false)
	ctx := starrycontrol.WithRequestMetadata(context.Background(), starrycontrol.RequestMetadata{
		ActorUserID: 42,
		RequestID:   "0191f6a0-0000-7000-8000-000000000051",
	})

	_, err := control.RollbackConfig(ctx, "starry-1", starrycontrol.RollbackRequest{
		RevisionID: revisionID, RiskConfirmation: "confirm:rollback:starry-1:wrong-revision",
	})
	if err == nil {
		t.Fatal("rollback without the exact high-risk confirmation was accepted")
	}
	if provider.historyCalls != 0 || provider.rollbackCalls != 0 {
		t.Fatalf("provider called before rollback confirmation: history=%d rollback=%d", provider.historyCalls, provider.rollbackCalls)
	}
	event := &model.AdminAuditEvent{}
	if err := database.Where("action = ?", "server_control.config.rollback").First(event).Error; err != nil {
		t.Fatal(err)
	}
	if event.Result != "failure" || event.ErrorCode != "HIGH_RISK_CONFIRMATION_REQUIRED" {
		t.Fatalf("rollback confirmation audit = %+v", event)
	}
}

func auditedControlService(provider starrycontrol.ServerControlProvider, readOnly bool) *StarryControlService {
	control := &StarryControlService{
		config: config.ServerControl{ReadOnly: readOnly},
		instances: map[string]ServerControlInstance{
			"starry-1": {ID: "starry-1", Name: "Primary", Enabled: true, Available: true, ReadOnly: readOnly},
		},
		providers: map[string]starrycontrol.ServerControlProvider{"starry-1": provider},
	}
	control.planReviewKey = sha256.Sum256([]byte("audited-control-test-review-key"))
	return control
}
