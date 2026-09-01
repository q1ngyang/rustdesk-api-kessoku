package service

import (
	"context"
	"errors"
	"sync/atomic"

	"github.com/q1ngyang/rustdesk-api-kessoku/v3/model"
)

type PresenceOperation uint8

const (
	PresenceOperationStart PresenceOperation = iota
	PresenceOperationRenew
	PresenceOperationEnd
	PresenceOperationDeactivate
	presenceOperationCount
)

type presenceOperationCounters struct {
	accepted atomic.Uint64
	rejected atomic.Uint64
	errors   atomic.Uint64
}

var presenceCounters [presenceOperationCount]presenceOperationCounters

type PresenceMetricsSnapshot struct {
	SchemaVersion           int    `json:"schema_version"`
	CollectedAt             int64  `json:"collected_at"`
	CounterScope            string `json:"counter_scope"`
	GaugeScope              string `json:"gauge_scope"`
	ActiveLeases            int64  `json:"active_leases"`
	OnlinePeers             int64  `json:"online_peers"`
	ExpiredUnendedLeases    int64  `json:"expired_unended_leases"`
	StartAcceptedTotal      uint64 `json:"start_accepted_total"`
	StartRejectedTotal      uint64 `json:"start_rejected_total"`
	StartErrorsTotal        uint64 `json:"start_errors_total"`
	RenewAcceptedTotal      uint64 `json:"renew_accepted_total"`
	RenewRejectedTotal      uint64 `json:"renew_rejected_total"`
	RenewErrorsTotal        uint64 `json:"renew_errors_total"`
	EndAcceptedTotal        uint64 `json:"end_accepted_total"`
	EndRejectedTotal        uint64 `json:"end_rejected_total"`
	EndErrorsTotal          uint64 `json:"end_errors_total"`
	DeactivateAcceptedTotal uint64 `json:"deactivate_accepted_total"`
	DeactivateRejectedTotal uint64 `json:"deactivate_rejected_total"`
	DeactivateErrorsTotal   uint64 `json:"deactivate_errors_total"`
}

// ObservePresenceRequest records only a bounded operation/result tuple. It
// deliberately accepts neither request bodies nor identifiers, so bearer
// tokens cannot become metric labels or log fields through this path.
func ObservePresenceRequest(operation PresenceOperation, status int) {
	if operation >= presenceOperationCount {
		return
	}
	counters := &presenceCounters[operation]
	switch {
	case status >= 200 && status < 300:
		counters.accepted.Add(1)
	case status >= 400 && status < 500:
		counters.rejected.Add(1)
	default:
		counters.errors.Add(1)
	}
}

func (ps *PeerService) PresenceMetricsSnapshot(ctx context.Context, now int64) (PresenceMetricsSnapshot, error) {
	result := currentPresenceCounterSnapshot(now)
	if DB == nil {
		return result, errors.New("database is unavailable")
	}
	active := DB.WithContext(ctx).Model(&model.PeerPresenceLease{}).
		Where("ended_at = 0 AND expires_at > ?", now)
	if err := active.Count(&result.ActiveLeases).Error; err != nil {
		return result, err
	}
	if err := active.Distinct("peer_row_id").Count(&result.OnlinePeers).Error; err != nil {
		return result, err
	}
	if err := DB.WithContext(ctx).Model(&model.PeerPresenceLease{}).
		Where("ended_at = 0 AND expires_at <= ?", now).
		Count(&result.ExpiredUnendedLeases).Error; err != nil {
		return result, err
	}
	return result, nil
}

func currentPresenceCounterSnapshot(now int64) PresenceMetricsSnapshot {
	return PresenceMetricsSnapshot{
		SchemaVersion:           1,
		CollectedAt:             now,
		CounterScope:            "process",
		GaugeScope:              "database",
		StartAcceptedTotal:      presenceCounters[PresenceOperationStart].accepted.Load(),
		StartRejectedTotal:      presenceCounters[PresenceOperationStart].rejected.Load(),
		StartErrorsTotal:        presenceCounters[PresenceOperationStart].errors.Load(),
		RenewAcceptedTotal:      presenceCounters[PresenceOperationRenew].accepted.Load(),
		RenewRejectedTotal:      presenceCounters[PresenceOperationRenew].rejected.Load(),
		RenewErrorsTotal:        presenceCounters[PresenceOperationRenew].errors.Load(),
		EndAcceptedTotal:        presenceCounters[PresenceOperationEnd].accepted.Load(),
		EndRejectedTotal:        presenceCounters[PresenceOperationEnd].rejected.Load(),
		EndErrorsTotal:          presenceCounters[PresenceOperationEnd].errors.Load(),
		DeactivateAcceptedTotal: presenceCounters[PresenceOperationDeactivate].accepted.Load(),
		DeactivateRejectedTotal: presenceCounters[PresenceOperationDeactivate].rejected.Load(),
		DeactivateErrorsTotal:   presenceCounters[PresenceOperationDeactivate].errors.Load(),
	}
}

func resetPresenceMetricsForTest() {
	for index := range presenceCounters {
		presenceCounters[index].accepted.Store(0)
		presenceCounters[index].rejected.Store(0)
		presenceCounters[index].errors.Store(0)
	}
}
