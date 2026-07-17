package service

import (
	"context"
	"testing"

	"github.com/dushixiang/uart_sms_forwarder/internal/models"
	"github.com/glebarez/sqlite"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

func TestIncomingCallEventPersistsAndClosesRecord(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.CallRecord{}); err != nil {
		t.Fatalf("migrate call records: %v", err)
	}

	records := NewCallRecordService(db)
	serial := &SerialService{
		logger:      zap.NewNop(),
		moduleID:    "sim2",
		moduleName:  "Air780EHV",
		callRecords: records,
	}
	serial.handleIncomingCall(&ParsedMessage{
		JSON: `{"type":"incoming_call","timestamp":1700000000,"from":"+441234567890"}`,
	})

	stored, err := records.List(context.Background(), "sim2", 10)
	if err != nil {
		t.Fatalf("list call records: %v", err)
	}
	if len(stored) != 1 {
		t.Fatalf("stored call count = %d, want 1", len(stored))
	}
	if stored[0].From != "+441234567890" || stored[0].ModuleID != "sim2" || stored[0].StartedAt != 1700000000000 {
		t.Fatalf("unexpected incoming call record: %+v", stored[0])
	}
	if stored[0].EndedAt != 0 {
		t.Fatalf("new call ended at %d, want 0", stored[0].EndedAt)
	}

	serial.handleCallDisconnected(&ParsedMessage{
		Payload: map[string]interface{}{"timestamp": float64(1700000030)},
	})
	stored, err = records.List(context.Background(), "sim2", 10)
	if err != nil {
		t.Fatalf("list ended call records: %v", err)
	}
	if stored[0].EndedAt != 1700000030000 {
		t.Fatalf("ended at = %d, want 1700000030000", stored[0].EndedAt)
	}
}
