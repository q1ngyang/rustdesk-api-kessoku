package starry

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/q1ngyang/rustdesk-api-kessoku/v2/config"
	"github.com/q1ngyang/rustdesk-api-kessoku/v2/internal/controlauth"
	"github.com/q1ngyang/rustdesk-api-kessoku/v2/internal/starrycontrol"
	"github.com/q1ngyang/rustdesk-api-kessoku/v2/internal/starrycontrol/clientgen"
)

type Provider struct {
	instanceID string
	client     *clientgen.Client
}

func NewProvider(instance config.StarryInstance, control config.ServerControl) (*Provider, error) {
	if !instance.Enabled {
		return nil, errors.New("Starry instance is disabled")
	}
	if instance.ID == "" || instance.ExpectedInstanceID == "" || instance.TLSServerName == "" {
		return nil, errors.New("Starry instance id, expected instance id, and TLS server name are required")
	}
	if instance.CAFile == "" || instance.ClientCertFile == "" || instance.ClientKeyFile == "" {
		return nil, errors.New("Starry mTLS CA, client certificate, and key files are required")
	}
	certificate, err := tls.LoadX509KeyPair(instance.ClientCertFile, instance.ClientKeyFile)
	if err != nil {
		return nil, fmt.Errorf("load Starry client certificate: %w", err)
	}
	caPEM, err := os.ReadFile(instance.CAFile)
	if err != nil {
		return nil, fmt.Errorf("read Starry CA file: %w", err)
	}
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(caPEM) {
		return nil, errors.New("Starry CA file contains no certificates")
	}
	signer, err := controlauth.NewSigner(
		instance.ControlIssuer,
		instance.AuthorizedParty,
		instance.ControlKeyID,
		instance.ControlKeyFile,
		2*time.Minute,
	)
	if err != nil {
		return nil, err
	}
	dialer := &net.Dialer{Timeout: 3 * time.Second, KeepAlive: 30 * time.Second}
	transport := &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		DialContext:           dialer.DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          20,
		MaxIdleConnsPerHost:   10,
		IdleConnTimeout:       60 * time.Second,
		TLSHandshakeTimeout:   3 * time.Second,
		ResponseHeaderTimeout: control.Timeout(),
		TLSClientConfig: &tls.Config{
			Certificates: []tls.Certificate{certificate},
			RootCAs:      roots,
			ServerName:   instance.TLSServerName,
			MinVersion:   tls.VersionTLS13,
		},
	}
	httpClient := &http.Client{Transport: transport, Timeout: control.Timeout()}
	client, err := clientgen.New(instance.BaseURL, httpClient, func(ctx context.Context, scope string) (string, error) {
		metadata, ok := starrycontrol.MetadataFromContext(ctx)
		if !ok {
			return "", errors.New("missing authenticated request metadata")
		}
		return signer.Sign(instance.ExpectedInstanceID, scope, metadata.ActorUserID)
	}, control.MaxResponseBytes())
	if err != nil {
		return nil, err
	}
	return &Provider{instanceID: instance.ExpectedInstanceID, client: client}, nil
}

func (p *Provider) Capabilities(ctx context.Context) (starrycontrol.Capabilities, error) {
	result := starrycontrol.Capabilities{}
	if err := p.call(ctx, clientgen.Request{Method: http.MethodGet, Path: "/control/v1/capabilities", Scope: "starry.control.read"}, &result); err != nil {
		return result, err
	}
	if result.Protocol.Name != "starry-control" || result.Protocol.Major != 1 {
		return starrycontrol.Capabilities{}, &starrycontrol.ProviderError{Status: http.StatusBadGateway, Code: "CONTRACT_VERSION_INCOMPATIBLE", Message: "Starry contract is incompatible"}
	}
	if result.Instance.ID != p.instanceID {
		return starrycontrol.Capabilities{}, &starrycontrol.ProviderError{Status: http.StatusBadGateway, Code: "INSTANCE_ID_MISMATCH", Message: "Starry instance identity did not match deployment configuration"}
	}
	return result, nil
}

func (p *Provider) Health(ctx context.Context) (starrycontrol.Health, error) {
	result := starrycontrol.Health{}
	err := p.call(ctx, clientgen.Request{Method: http.MethodGet, Path: "/control/v1/status", Scope: "starry.control.read"}, &result)
	return result, err
}

func (p *Provider) Relays(ctx context.Context) ([]starrycontrol.Relay, error) {
	response := struct {
		Relays []starrycontrol.Relay `json:"relays"`
	}{}
	err := p.call(ctx, clientgen.Request{Method: http.MethodGet, Path: "/control/v1/relays", Scope: "starry.relay.read"}, &response)
	return response.Relays, err
}

func (p *Provider) SimulateAllocation(ctx context.Context, input starrycontrol.SimulationInput) (starrycontrol.SimulationResult, error) {
	result := starrycontrol.SimulationResult{}
	if !input.Transport.Valid() {
		return result, fmt.Errorf("%w: transport", starrycontrol.ErrRequestInvalid)
	}
	if _, err := netip.ParseAddr(input.ClientA.IP); err != nil {
		return result, fmt.Errorf("%w: client_a.ip", starrycontrol.ErrRequestInvalid)
	}
	if _, err := netip.ParseAddr(input.ClientB.IP); err != nil {
		return result, fmt.Errorf("%w: client_b.ip", starrycontrol.ErrRequestInvalid)
	}
	err := p.call(ctx, clientgen.Request{Method: http.MethodPost, Path: "/control/v1/allocations:simulate", Scope: "starry.relay.simulate", Body: input}, &result)
	return result, err
}

func (p *Provider) GetConfig(ctx context.Context) (starrycontrol.ConfigDocument, error) {
	result := starrycontrol.ConfigDocument{}
	headers, err := p.callWithHeaders(ctx, clientgen.Request{Method: http.MethodGet, Path: "/control/v1/config", Scope: "starry.config.read"}, &result)
	if err == nil && result.ETag == "" {
		result.ETag = headers.Get("ETag")
	}
	return result, err
}

func (p *Provider) GetConfigSchema(ctx context.Context) (starrycontrol.SchemaBundle, error) {
	result := starrycontrol.SchemaBundle{}
	headers, err := p.callWithHeaders(ctx, clientgen.Request{Method: http.MethodGet, Path: "/control/v1/config/schema", Scope: "starry.config.read"}, &result)
	if err == nil && result.ETag == "" {
		result.ETag = headers.Get("ETag")
	}
	return result, err
}

func (p *Provider) ValidateConfig(ctx context.Context, input starrycontrol.ConfigCandidate) (starrycontrol.ValidationResult, error) {
	result := starrycontrol.ValidationResult{}
	if err := validateCandidate(input); err != nil {
		return result, err
	}
	err := p.call(ctx, clientgen.Request{Method: http.MethodPost, Path: "/control/v1/config:validate", Scope: "starry.config.validate", Body: input, IfMatch: input.BaseETag}, &result)
	return result, err
}

func (p *Provider) PlanConfig(ctx context.Context, input starrycontrol.ConfigCandidate) (starrycontrol.ConfigPlan, error) {
	result := starrycontrol.ConfigPlan{}
	if err := validateCandidate(input); err != nil {
		return result, err
	}
	err := p.call(ctx, clientgen.Request{Method: http.MethodPost, Path: "/control/v1/config:plan", Scope: "starry.config.plan", Body: input, IfMatch: input.BaseETag}, &result)
	return result, err
}

func (p *Provider) ApplyConfig(ctx context.Context, input starrycontrol.ApplyRequest) (starrycontrol.ApplyResult, error) {
	result := starrycontrol.ApplyResult{}
	if input.PlanID == "" || input.IfMatch == "" || input.IdempotencyKey == "" {
		return result, starrycontrol.ErrRequestInvalid
	}
	err := p.call(ctx, clientgen.Request{Method: http.MethodPost, Path: "/control/v1/config:apply", Scope: "starry.config.apply", Body: input, IfMatch: input.IfMatch, IdempotencyKey: input.IdempotencyKey}, &result)
	return result, err
}

func (p *Provider) Operation(ctx context.Context, operationID string) (starrycontrol.Operation, error) {
	result := starrycontrol.Operation{}
	if _, err := uuid.Parse(operationID); err != nil {
		return result, starrycontrol.ErrRequestInvalid
	}
	err := p.call(ctx, clientgen.Request{Method: http.MethodGet, Path: "/control/v1/operations/" + operationID, Scope: "starry.control.read"}, &result)
	return result, err
}

func (p *Provider) ConfigHistory(ctx context.Context) ([]starrycontrol.ConfigRevision, error) {
	response := struct {
		Revisions []starrycontrol.ConfigRevision `json:"revisions"`
	}{}
	err := p.call(ctx, clientgen.Request{Method: http.MethodGet, Path: "/control/v1/config/history", Scope: "starry.config.read"}, &response)
	return response.Revisions, err
}

func (p *Provider) RollbackConfig(ctx context.Context, input starrycontrol.RollbackRequest) (starrycontrol.ApplyResult, error) {
	result := starrycontrol.ApplyResult{}
	if input.Generation == 0 || input.IfMatch == "" || input.IdempotencyKey == "" {
		return result, starrycontrol.ErrRequestInvalid
	}
	err := p.call(ctx, clientgen.Request{Method: http.MethodPost, Path: "/control/v1/config:rollback", Scope: "starry.config.rollback", Body: input, IfMatch: input.IfMatch, IdempotencyKey: input.IdempotencyKey}, &result)
	return result, err
}

func validateCandidate(input starrycontrol.ConfigCandidate) error {
	if input.BaseETag == "" || (input.YAML == nil && input.Values == nil) || (input.YAML != nil && input.Values != nil) {
		return starrycontrol.ErrRequestInvalid
	}
	return nil
}

func (p *Provider) call(ctx context.Context, request clientgen.Request, destination interface{}) error {
	_, err := p.callWithHeaders(ctx, request, destination)
	return err
}

func (p *Provider) callWithHeaders(ctx context.Context, request clientgen.Request, destination interface{}) (http.Header, error) {
	metadata, ok := starrycontrol.MetadataFromContext(ctx)
	if !ok {
		return nil, starrycontrol.ErrRequestInvalid
	}
	request.RequestID = metadata.RequestID
	headers, err := p.client.Do(ctx, request, destination)
	if err == nil {
		return headers, nil
	}
	var httpError *clientgen.HTTPError
	if errors.As(err, &httpError) {
		problem := httpError.Problem
		return headers, &starrycontrol.ProviderError{
			Status:    problem.Status,
			Code:      problem.Code,
			Message:   "Starry control request failed",
			RequestID: problem.RequestID,
			Retryable: problem.Retryable,
		}
	}
	return nil, fmt.Errorf("%w: %v", starrycontrol.ErrUnavailable, err)
}
