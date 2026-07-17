package service

import (
	"context"
	"time"

	"github.com/dushixiang/uart_sms_forwarder/internal/models"
	"github.com/dushixiang/uart_sms_forwarder/internal/repo"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type TrafficRecordService struct {
	repo *repo.TrafficRecordRepo
}

func NewTrafficRecordService(db *gorm.DB) *TrafficRecordService {
	return &TrafficRecordService{repo: repo.NewTrafficRecordRepo(db)}
}

func (s *TrafficRecordService) Save(ctx context.Context, record *models.TrafficRecord) error {
	if record.ID == "" {
		record.ID = uuid.NewString()
	}
	if record.CreatedAt == 0 {
		record.CreatedAt = time.Now().UnixMilli()
	}
	return s.repo.Create(ctx, record)
}

func (s *TrafficRecordService) List(ctx context.Context, moduleID string, limit int) ([]models.TrafficRecord, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	records, err := s.repo.List(ctx, moduleID, limit)
	if records == nil {
		records = []models.TrafficRecord{}
	}
	return records, err
}
