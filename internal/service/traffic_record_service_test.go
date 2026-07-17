package service

import (
	"context"
	"testing"

	"github.com/dushixiang/uart_sms_forwarder/internal/models"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestTrafficRecordServiceListsNewestAndFiltersModule(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.TrafficRecord{}); err != nil {
		t.Fatalf("migrate traffic records: %v", err)
	}
	service := NewTrafficRecordService(db)
	ctx := context.Background()

	for _, record := range []*models.TrafficRecord{
		{RequestID: "request-1", ModuleID: "sim1", ModuleName: "EPV", TotalBytes: 5100, Success: true, CreatedAt: 1000},
		{RequestID: "request-2", ModuleID: "sim2", ModuleName: "EHV", TotalBytes: 5200, Success: true, CreatedAt: 2000},
		{RequestID: "request-3", ModuleID: "sim1", ModuleName: "EPV", Error: "timeout", CreatedAt: 3000},
	} {
		if err := service.Save(ctx, record); err != nil {
			t.Fatalf("save traffic record: %v", err)
		}
	}

	records, err := service.List(ctx, "sim1", 2)
	if err != nil {
		t.Fatalf("list traffic records: %v", err)
	}
	if len(records) != 2 || records[0].RequestID != "request-3" || records[1].RequestID != "request-1" {
		t.Fatalf("unexpected records: %+v", records)
	}
}
