package repo

import (
	"context"
	"errors"

	"github.com/dushixiang/uart_sms_forwarder/internal/models"
	"gorm.io/gorm"
)

type CallRecordRepo struct {
	db *gorm.DB
}

func NewCallRecordRepo(db *gorm.DB) *CallRecordRepo {
	return &CallRecordRepo{db: db}
}

func (r *CallRecordRepo) Create(ctx context.Context, record *models.CallRecord) error {
	return r.db.WithContext(ctx).Create(record).Error
}

func (r *CallRecordRepo) EndLatestOpen(ctx context.Context, moduleID string, endedAt int64) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var record models.CallRecord
		err := tx.Where("module_id = ? AND ended_at = 0", moduleID).
			Order("started_at DESC").
			First(&record).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		if err != nil {
			return err
		}
		return tx.Model(&models.CallRecord{}).
			Where("id = ?", record.ID).
			Updates(map[string]interface{}{
				"state":    models.CallStateEnded,
				"ended_at": endedAt,
			}).Error
	})
}

func (r *CallRecordRepo) List(ctx context.Context, moduleID string, limit int) ([]models.CallRecord, error) {
	query := r.db.WithContext(ctx).Order("started_at DESC").Limit(limit)
	if moduleID != "" {
		query = query.Where("module_id = ?", moduleID)
	}
	var records []models.CallRecord
	return records, query.Find(&records).Error
}
