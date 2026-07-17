package service

import (
	"context"
	"time"

	"github.com/dushixiang/uart_sms_forwarder/internal/models"
	"github.com/dushixiang/uart_sms_forwarder/internal/repo"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type CallRecordService struct {
	repo *repo.CallRecordRepo
}

func NewCallRecordService(db *gorm.DB) *CallRecordService {
	return &CallRecordService{repo: repo.NewCallRecordRepo(db)}
}

func normalizeCallTimestamp(timestamp int64) int64 {
	if timestamp <= 0 {
		return time.Now().UnixMilli()
	}
	if timestamp < 1_000_000_000_000 {
		return timestamp * 1000
	}
	return timestamp
}

func (s *CallRecordService) RecordIncoming(
	ctx context.Context,
	moduleID string,
	moduleName string,
	from string,
	timestamp int64,
) (*models.CallRecord, error) {
	if from == "" {
		from = "unknown"
	}
	record := &models.CallRecord{
		ID:         uuid.NewString(),
		ModuleID:   moduleID,
		ModuleName: moduleName,
		From:       from,
		State:      models.CallStateRinging,
		StartedAt:  normalizeCallTimestamp(timestamp),
	}
	return record, s.repo.Create(ctx, record)
}

func (s *CallRecordService) RecordDisconnected(ctx context.Context, moduleID string, timestamp int64) error {
	return s.repo.EndLatestOpen(ctx, moduleID, normalizeCallTimestamp(timestamp))
}

func (s *CallRecordService) List(ctx context.Context, moduleID string, limit int) ([]models.CallRecord, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	records, err := s.repo.List(ctx, moduleID, limit)
	if records == nil {
		records = []models.CallRecord{}
	}
	return records, err
}
