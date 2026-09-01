package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	request "github.com/q1ngyang/rustdesk-api-kessoku/v3/http/request/api"
	"github.com/q1ngyang/rustdesk-api-kessoku/v3/model"
	"github.com/q1ngyang/rustdesk-api-kessoku/v3/service"
	"github.com/q1ngyang/rustdesk-api-kessoku/v3/utils"
)

var errPresenceProfileIDForbidden = errors.New("client-local profile_id is forbidden")

type PresenceLeaseResponse struct {
	PresenceV2      bool   `json:"presence_v2"`
	Accepted        bool   `json:"accepted"`
	ActivationEpoch uint64 `json:"activation_epoch,omitempty"`
	ActivationID    string `json:"activation_id,omitempty"`
	LeaseID         string `json:"lease_id,omitempty"`
	LeaseToken      string `json:"lease_token,omitempty"`
	ExpiresAt       int64  `json:"expires_at,omitempty"`
	ExpiresIn       int64  `json:"expires_in,omitempty"`
	OnlineUntil     int64  `json:"online_until,omitempty"`
	Sysinfo         bool   `json:"sysinfo,omitempty"`
	Error           string `json:"error,omitempty"`
}

type presenceLeaseResponse = PresenceLeaseResponse

// PresenceStart authenticates an active Starry profile activation and grants
// one independently renewable presence lease.
// @Tags System
// @Summary Start a presence lease
// @Accept json
// @Produce json
// @Param body body request.PresenceStartRequest true "Network identity and active route proof"
// @Success 200 {object} PresenceLeaseResponse
// @Failure 400 {object} PresenceLeaseResponse
// @Failure 403 {object} PresenceLeaseResponse
// @Failure 409 {object} PresenceLeaseResponse
// @Failure 500 {object} PresenceLeaseResponse
// @Failure 501 {object} PresenceLeaseResponse
// @Router /presence/v2/start [post]
func (i *Index) PresenceStart(c *gin.Context) {
	defer observePresenceRequest(c, service.PresenceOperationStart)
	input, ok := presenceRequest(c)
	if !ok {
		return
	}
	if service.AllService == nil || service.AllService.NetworkActivationVerifier == nil {
		presenceFailure(c, http.StatusNotImplemented, service.ErrPresenceActivationUnverified)
		return
	}
	if err := service.AllService.PeerService.VerifyActivationIdentity(
		c.Request.Context(), input.Id, input.Uuid, input.ActivationEpoch, input.ActivationID, input.RouteLeases,
	); err != nil {
		presenceFailure(c, presenceErrorStatus(err), err)
		return
	}
	if err := service.AllService.PeerService.BindRegistryIdentity(input.Id, input.Uuid, 0); err != nil {
		presenceFailure(c, presenceErrorStatus(err), err)
		return
	}
	peer := service.AllService.PeerService.FindByIdentity(input.Id, input.Uuid)
	if peer.RowId == 0 || peer.Uuid != input.Uuid {
		presenceFailure(c, http.StatusForbidden, service.ErrPeerIdentityUnverified)
		return
	}
	now := time.Now().Unix()
	refreshSysinfo := service.AllService.PeerService.NeedsSysinfoRefresh(peer, now)
	grant, err := service.AllService.PeerService.StartPresenceLease(
		c.Request.Context(), peer, input.ActivationEpoch, input.ActivationID, c.ClientIP(), now,
	)
	if err != nil {
		presenceFailure(c, presenceErrorStatus(err), err)
		return
	}
	presenceSuccess(c, grant, refreshSysinfo)
}

// PresenceRenew extends only the lease selected by its activation and bearer
// token. lease_id is returned by start and, when supplied, is also matched.
// @Tags System
// @Summary Renew a presence lease
// @Accept json
// @Produce json
// @Param body body request.PresenceLeaseMutationRequest true "Exact lease proof"
// @Success 200 {object} PresenceLeaseResponse
// @Failure 400 {object} PresenceLeaseResponse
// @Failure 401 {object} PresenceLeaseResponse
// @Failure 403 {object} PresenceLeaseResponse
// @Failure 409 {object} PresenceLeaseResponse
// @Failure 410 {object} PresenceLeaseResponse
// @Failure 500 {object} PresenceLeaseResponse
// @Router /presence/v2/renew [post]
func (i *Index) PresenceRenew(c *gin.Context) {
	defer observePresenceRequest(c, service.PresenceOperationRenew)
	input, peer, ok := presenceRequestPeer(c)
	if !ok {
		return
	}
	now := time.Now().Unix()
	refreshSysinfo := service.AllService.PeerService.NeedsSysinfoRefresh(peer, now)
	grant, err := service.AllService.PeerService.RenewPresenceLease(
		c.Request.Context(), peer, input.ActivationEpoch, input.ActivationID, input.LeaseID, input.LeaseToken, c.ClientIP(), now,
	)
	if err != nil {
		presenceFailure(c, presenceErrorStatus(err), err)
		return
	}
	presenceSuccess(c, grant, refreshSysinfo)
}

// PresenceEnd idempotently ends the exact bearer-token-authenticated lease. It
// cannot end a newer activation or another parallel lease.
// @Tags System
// @Summary End a presence lease
// @Accept json
// @Produce json
// @Param body body request.PresenceLeaseMutationRequest true "Exact lease proof including lease_token"
// @Success 200 {object} PresenceLeaseResponse
// @Failure 400 {object} PresenceLeaseResponse
// @Failure 401 {object} PresenceLeaseResponse
// @Failure 403 {object} PresenceLeaseResponse
// @Failure 500 {object} PresenceLeaseResponse
// @Router /presence/v2/end [post]
func (i *Index) PresenceEnd(c *gin.Context) {
	defer observePresenceRequest(c, service.PresenceOperationEnd)
	input, peer, ok := presenceRequestPeer(c)
	if !ok {
		return
	}
	now := time.Now().Unix()
	grant, err := service.AllService.PeerService.EndPresenceLease(
		c.Request.Context(), peer, input.ActivationEpoch, input.ActivationID, input.LeaseID, input.LeaseToken, c.ClientIP(), now,
	)
	if err != nil {
		presenceFailure(c, presenceErrorStatus(err), err)
		return
	}
	presenceSuccess(c, grant, false)
}

// PresenceDeactivate retires one Starry-authenticated activation and all of
// its leases before a profile switch. It is distinct from ending one lease.
// @Tags System
// @Summary Retire a presence activation
// @Accept json
// @Produce json
// @Param body body request.PresenceStartRequest true "Active route proof"
// @Success 200 {object} PresenceLeaseResponse
// @Failure 400 {object} PresenceLeaseResponse
// @Failure 403 {object} PresenceLeaseResponse
// @Failure 409 {object} PresenceLeaseResponse
// @Failure 500 {object} PresenceLeaseResponse
// @Failure 501 {object} PresenceLeaseResponse
// @Router /presence/v2/deactivate [post]
func (i *Index) PresenceDeactivate(c *gin.Context) {
	defer observePresenceRequest(c, service.PresenceOperationDeactivate)
	input, ok := presenceRequest(c)
	if !ok {
		return
	}
	if service.AllService == nil || service.AllService.NetworkActivationVerifier == nil {
		presenceFailure(c, http.StatusNotImplemented, service.ErrPresenceActivationUnverified)
		return
	}
	if err := service.AllService.PeerService.VerifyActivationIdentity(
		c.Request.Context(), input.Id, input.Uuid, input.ActivationEpoch, input.ActivationID, input.RouteLeases,
	); err != nil {
		presenceFailure(c, presenceErrorStatus(err), err)
		return
	}
	peer := service.AllService.PeerService.FindByIdentity(input.Id, input.Uuid)
	if peer.RowId == 0 || peer.Uuid != input.Uuid {
		presenceFailure(c, http.StatusForbidden, service.ErrPeerIdentityUnverified)
		return
	}
	grant, err := service.AllService.PeerService.DeactivatePresenceActivation(
		c.Request.Context(), peer, input.ActivationEpoch, input.ActivationID, c.ClientIP(), time.Now().Unix(),
	)
	if err != nil {
		presenceFailure(c, presenceErrorStatus(err), err)
		return
	}
	presenceSuccess(c, grant, false)
}

func observePresenceRequest(c *gin.Context, operation service.PresenceOperation) {
	service.ObservePresenceRequest(operation, c.Writer.Status())
}

func presenceRequestPeer(c *gin.Context) (*request.PresenceLeaseRequest, *model.Peer, bool) {
	input, ok := presenceRequest(c)
	if !ok {
		return nil, nil, false
	}
	peer, err := service.AllService.PeerService.ResolveReportIdentity(c.Request.Context(), input.Id, input.Uuid)
	if err != nil {
		presenceFailure(c, http.StatusForbidden, service.ErrPeerIdentityUnverified)
		return nil, nil, false
	}
	return input, peer, true
}

func presenceRequest(c *gin.Context) (*request.PresenceLeaseRequest, bool) {
	type presenceRequestEnvelope struct {
		request.PresenceLeaseRequest
		ProfileID json.RawMessage `json:"profile_id"`
	}
	envelope := &presenceRequestEnvelope{}
	if err := c.ShouldBindJSON(envelope); err != nil {
		presenceFailure(c, http.StatusBadRequest, service.ErrPresenceActivationInvalid)
		return nil, false
	}
	if len(envelope.ProfileID) != 0 {
		presenceFailure(c, http.StatusBadRequest, errPresenceProfileIDForbidden)
		return nil, false
	}
	input := &envelope.PresenceLeaseRequest
	input.Id = utils.NormalizeRustDeskID(input.Id)
	if input.Id == "" || input.Uuid == "" || service.AllService == nil {
		presenceFailure(c, http.StatusForbidden, service.ErrPeerIdentityUnverified)
		return nil, false
	}
	return input, true
}

func presenceSuccess(c *gin.Context, grant *service.PresenceLeaseGrant, sysinfo bool) {
	now := time.Now().Unix()
	expiresIn := grant.ExpiresAt - now
	if expiresIn < 0 {
		expiresIn = 0
	}
	c.Header("Cache-Control", "no-store")
	c.Header("Pragma", "no-cache")
	c.JSON(http.StatusOK, presenceLeaseResponse{
		PresenceV2: true, Accepted: true,
		ActivationEpoch: grant.ActivationEpoch, ActivationID: grant.ActivationID,
		LeaseID: grant.LeaseID, LeaseToken: grant.LeaseToken, ExpiresAt: grant.ExpiresAt, ExpiresIn: expiresIn,
		OnlineUntil: grant.OnlineUntil, Sysinfo: sysinfo,
	})
}

func presenceFailure(c *gin.Context, status int, err error) {
	c.Header("Cache-Control", "no-store")
	c.Header("Pragma", "no-cache")
	c.JSON(status, presenceLeaseResponse{PresenceV2: true, Accepted: false, Error: presenceErrorCode(err)})
}

func presenceErrorStatus(err error) int {
	switch {
	case errors.Is(err, service.ErrPresenceActivationInvalid):
		return http.StatusBadRequest
	case errors.Is(err, errPresenceProfileIDForbidden):
		return http.StatusBadRequest
	case errors.Is(err, service.ErrPresenceActivationUnverified), errors.Is(err, service.ErrPeerIdentityUnverified):
		return http.StatusForbidden
	case errors.Is(err, service.ErrPresenceActivationStale):
		return http.StatusConflict
	case errors.Is(err, service.ErrPeerIdentityConflict):
		return http.StatusConflict
	case errors.Is(err, service.ErrPresenceLeaseInvalid):
		return http.StatusUnauthorized
	case errors.Is(err, service.ErrPresenceLeaseExpired):
		return http.StatusGone
	default:
		return http.StatusInternalServerError
	}
}

func presenceErrorCode(err error) string {
	switch {
	case errors.Is(err, service.ErrPresenceActivationInvalid):
		return "presence_activation_invalid"
	case errors.Is(err, errPresenceProfileIDForbidden):
		return "presence_profile_id_forbidden"
	case errors.Is(err, service.ErrPresenceActivationUnverified):
		return "presence_activation_unverified"
	case errors.Is(err, service.ErrPresenceActivationStale):
		return "presence_activation_stale"
	case errors.Is(err, service.ErrPeerIdentityConflict):
		return "peer_identity_conflict"
	case errors.Is(err, service.ErrPresenceLeaseInvalid):
		return "presence_lease_invalid"
	case errors.Is(err, service.ErrPresenceLeaseExpired):
		return "presence_lease_expired"
	case errors.Is(err, service.ErrPeerIdentityUnverified):
		return "peer_identity_unverified"
	default:
		return "presence_internal_error"
	}
}
