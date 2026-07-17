package service

import (
	"context"
	"testing"
	"time"

	"github.com/dushixiang/uart_sms_forwarder/config"
	"github.com/dushixiang/uart_sms_forwarder/internal/models"
	"github.com/glebarez/sqlite"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

func newSchedulerTestService(t *testing.T) (*SchedulerService, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.ScheduledTask{}, &models.TrafficRecord{}); err != nil {
		t.Fatalf("migrate scheduled tasks: %v", err)
	}
	manager := NewSerialManager(
		zap.NewNop(),
		[]config.ModuleConfig{{ID: "sim1", Name: "Air780EPV"}, {ID: "sim2", Name: "Air780EHV"}},
		nil,
		nil,
		nil,
	)
	return NewSchedulerService(
		zap.NewNop(),
		db,
		manager,
		"http://example.test/payload.bin",
		NewTrafficRecordService(db),
	), db
}

func TestSchedulerBackfillsLegacyTaskDefaults(t *testing.T) {
	service, db := newSchedulerTestService(t)
	if err := db.Exec(`INSERT INTO scheduled_tasks
		(id, name, enabled, interval_days, task_type, module_id, traffic_kb)
		VALUES (?, ?, ?, ?, ?, ?, ?)`, "legacy", "legacy task", true, 30, "", "", 0).Error; err != nil {
		t.Fatalf("insert legacy task: %v", err)
	}

	if err := service.BackfillDefaults(context.Background()); err != nil {
		t.Fatalf("backfill defaults: %v", err)
	}
	task, err := service.GetById(context.Background(), "legacy")
	if err != nil {
		t.Fatalf("get task: %v", err)
	}
	if task.TaskType != models.ScheduledTaskTypeSMS || task.ModuleID != "sim1" || task.TrafficKB != 5 {
		t.Fatalf("unexpected defaults: type=%q module=%q trafficKB=%d", task.TaskType, task.ModuleID, task.TrafficKB)
	}
}

func TestSchedulerRejectsUnknownModule(t *testing.T) {
	service, _ := newSchedulerTestService(t)
	task := &models.ScheduledTask{
		Name:         "traffic",
		Enabled:      true,
		IntervalDays: 30,
		TaskType:     models.ScheduledTaskTypeTraffic,
		ModuleID:     "sim3",
		TrafficKB:    5,
	}
	if err := service.Create(context.Background(), task); err == nil {
		t.Fatal("expected unknown module error")
	}
}

func TestSchedulerUsesFixedFiveKBPayload(t *testing.T) {
	service, _ := newSchedulerTestService(t)
	task := &models.ScheduledTask{
		Name:         "traffic",
		Enabled:      true,
		IntervalDays: 30,
		TaskType:     models.ScheduledTaskTypeTraffic,
		ModuleID:     "sim2",
		TrafficKB:    999,
	}
	if err := service.Create(context.Background(), task); err != nil {
		t.Fatalf("create traffic task: %v", err)
	}
	if task.TrafficKB != 5 {
		t.Fatalf("trafficKB=%d, want 5", task.TrafficKB)
	}
}

func TestSchedulerRetryAndIntervalRules(t *testing.T) {
	service, _ := newSchedulerTestService(t)
	now := time.Date(2026, 7, 17, 8, 0, 0, 0, time.Local)

	cases := []struct {
		name string
		task models.ScheduledTask
		want bool
	}{
		{name: "never run", task: models.ScheduledTask{IntervalDays: 30}, want: true},
		{name: "failed less than one day", task: models.ScheduledTask{LastRunAt: now.Add(-23 * time.Hour).UnixMilli(), LastRunStatus: models.LastRunStatusFailed, IntervalDays: 30}, want: false},
		{name: "failed one day", task: models.ScheduledTask{LastRunAt: now.Add(-25 * time.Hour).UnixMilli(), LastRunStatus: models.LastRunStatusFailed, IntervalDays: 30}, want: true},
		{name: "interval not reached", task: models.ScheduledTask{LastRunAt: now.Add(-4 * 24 * time.Hour).UnixMilli(), LastRunStatus: models.LastRunStatusSuccess, IntervalDays: 5}, want: false},
		{name: "interval reached", task: models.ScheduledTask{LastRunAt: now.Add(-5 * 24 * time.Hour).UnixMilli(), LastRunStatus: models.LastRunStatusSuccess, IntervalDays: 5}, want: true},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			if got := service.shouldExecuteTask(test.task, now); got != test.want {
				t.Fatalf("shouldExecuteTask()=%v, want %v", got, test.want)
			}
		})
	}
}
