package service

import (
	"context"
	"time"

	"github.com/q1ngyang/rustdesk-api-kessoku/v3/model"
	"gorm.io/gorm"
)

const retentionBatchSize = 1000

// DataRetentionService bounds database growth without ever deleting an active
// login token. Operators configure the durations in Platform Settings.
type DataRetentionService struct{}

func (s *DataRetentionService) Start() {
	go func() {
		s.cleanupAndLog(context.Background())
		ticker := time.NewTicker(6 * time.Hour)
		defer ticker.Stop()
		for range ticker.C {
			s.cleanupAndLog(context.Background())
		}
	}()
}

func (s *DataRetentionService) cleanupAndLog(ctx context.Context) {
	counts, err := s.Cleanup(ctx, time.Now().UTC())
	if err != nil {
		Logger.WithError(err).Warn("database retention cleanup failed")
		return
	}
	Logger.WithField("deleted", counts).Debug("database retention cleanup completed")
}

func (s *DataRetentionService) Cleanup(ctx context.Context, now time.Time) (map[string]int64, error) {
	setting, err := AllService.SystemSettingService.Get()
	if err != nil {
		return nil, err
	}
	result := map[string]int64{}
	result["user_tokens"] = 0
	if setting.UserTokenRetentionDays > 0 {
		tokenCutoff := now.Add(-time.Duration(setting.UserTokenRetentionDays) * 24 * time.Hour).Unix()
		result["user_tokens"], err = deleteRetentionBatches(ctx, &model.UserToken{}, func(tx *gorm.DB) *gorm.DB {
			return tx.Where("(expired_at > 0 AND expired_at < ?) OR (revoked_at IS NOT NULL AND revoked_at < ?)", tokenCutoff, tokenCutoff)
		})
		if err != nil {
			return nil, err
		}
	}

	jobs := []struct {
		name  string
		model interface{}
		days  uint
		where func(*gorm.DB) *gorm.DB
	}{
		{"login_logs", &model.LoginLog{}, setting.LoginLogRetentionDays, nil},
		{"connection_logs", &model.AuditConn{}, setting.AuditConnRetentionDays, nil},
		{"file_logs", &model.AuditFile{}, setting.AuditFileRetentionDays, nil},
		{"control_audit", &model.AdminAuditEvent{}, setting.ControlAuditRetentionDays, func(tx *gorm.DB) *gorm.DB { return tx.Where("target_type = ?", "starry_instance") }},
	}
	for _, job := range jobs {
		result[job.name] = 0
		if job.days == 0 {
			continue
		}
		cutoff := now.Add(-time.Duration(job.days) * 24 * time.Hour)
		count, deleteErr := deleteRetentionBatches(ctx, job.model, func(tx *gorm.DB) *gorm.DB {
			tx = tx.Where("created_at < ?", cutoff)
			if job.where != nil {
				tx = job.where(tx)
			}
			return tx
		})
		if deleteErr != nil {
			return nil, deleteErr
		}
		result[job.name] = count
	}
	return result, nil
}

func deleteRetentionBatches(ctx context.Context, value interface{}, where func(*gorm.DB) *gorm.DB) (int64, error) {
	var deleted int64
	for batch := 0; batch < 100; batch++ {
		var ids []uint
		query := DB.WithContext(ctx).Model(value).Select("id").Order("id ASC").Limit(retentionBatchSize)
		query = where(query)
		if err := query.Pluck("id", &ids).Error; err != nil {
			return deleted, err
		}
		if len(ids) == 0 {
			return deleted, nil
		}
		result := DB.WithContext(ctx).Where("id IN ?", ids).Delete(value)
		if result.Error != nil {
			return deleted, result.Error
		}
		deleted += result.RowsAffected
		if len(ids) < retentionBatchSize {
			return deleted, nil
		}
	}
	return deleted, nil
}
