package service

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/google/uuid"
	"github.com/q1ngyang/rustdesk-api-kessoku/v2/model"
	"github.com/q1ngyang/rustdesk-api-kessoku/v2/model/custom_types"
	"gorm.io/gorm"
)

func beginSecurityAudit(ctx context.Context, actorUserID uint, requestID, action, targetType, targetID string, metadata map[string]interface{}) (*model.AdminAuditEvent, error) {
	if DB == nil {
		return nil, errors.New("security audit database is unavailable")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if _, err := uuid.Parse(requestID); err != nil {
		generated, generationErr := uuid.NewV7()
		if generationErr != nil {
			generated = uuid.New()
		}
		requestID = generated.String()
	}
	if metadata == nil {
		metadata = map[string]interface{}{}
	}
	encoded, err := json.Marshal(metadata)
	if err != nil {
		return nil, err
	}
	event := &model.AdminAuditEvent{
		ActorUserID: actorUserID,
		Action:      action,
		TargetType:  targetType,
		TargetID:    targetID,
		RequestID:   requestID,
		Result:      "intent",
		Metadata:    custom_types.AutoJson(encoded),
	}
	if err := DB.WithContext(ctx).Create(event).Error; err != nil {
		return nil, err
	}
	return event, nil
}

func rewriteSecurityAuditIntent(tx *gorm.DB, event *model.AdminAuditEvent, action string, metadata map[string]interface{}) error {
	if tx == nil || event == nil || event.Id == 0 {
		return errors.New("security audit intent is unavailable")
	}
	encoded, err := json.Marshal(metadata)
	if err != nil {
		return err
	}
	result := tx.Model(&model.AdminAuditEvent{}).
		Where("id = ? AND result = ?", event.Id, "intent").
		Updates(map[string]interface{}{
			"action":   action,
			"metadata": custom_types.AutoJson(encoded),
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return errors.New("security audit intent could not be specialized")
	}
	event.Action = action
	event.Metadata = custom_types.AutoJson(encoded)
	return nil
}

func finishSecurityAudit(event *model.AdminAuditEvent, operationErr error, failureCode string) error {
	if event == nil || DB == nil {
		return errors.New("security audit event or database is unavailable")
	}
	updates := map[string]interface{}{"result": "success", "error_code": ""}
	if operationErr != nil {
		updates["result"] = "failure"
		updates["error_code"] = failureCode
	}
	if err := DB.Model(event).Updates(updates).Error; err != nil {
		if Logger != nil {
			Logger.Errorf("finish security audit event %d: %v", event.Id, err)
		}
		return err
	}
	return nil
}

func finalizeSecurityAudit(event *model.AdminAuditEvent, operationErr *error, failureCode string) {
	if operationErr == nil {
		return
	}
	if auditErr := finishSecurityAudit(event, *operationErr, failureCode); auditErr != nil {
		if *operationErr != nil {
			*operationErr = errors.Join(*operationErr, auditErr)
		} else {
			*operationErr = auditErr
		}
	}
}
