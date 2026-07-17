package repo

import (
	"context"

	"github.com/dushixiang/uart_sms_forwarder/internal/models"
	"gorm.io/gorm"
)

type TrafficRecordRepo struct {
	db *gorm.DB
}

func NewTrafficRecordRepo(db *gorm.DB) *TrafficRecordRepo {
	return &TrafficRecordRepo{db: db}
}

func (r *TrafficRecordRepo) Create(ctx context.Context, record *models.TrafficRecord) error {
	return r.db.WithContext(ctx).Create(record).Error
}

func (r *TrafficRecordRepo) List(ctx context.Context, moduleID string, limit int) ([]models.TrafficRecord, error) {
	query := r.db.WithContext(ctx).Order("created_at DESC").Limit(limit)
	if moduleID != "" {
		query = query.Where("module_id = ?", moduleID)
	}
	var records []models.TrafficRecord
	return records, query.Find(&records).Error
}
