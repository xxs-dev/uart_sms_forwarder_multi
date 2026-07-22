package service

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/dushixiang/uart_sms_forwarder/config"
	"github.com/dushixiang/uart_sms_forwarder/internal/models"
	"github.com/glebarez/sqlite"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type fakeTrafficModule struct {
	results      []TrafficResult
	consumeCalls int
	flymodeCalls []bool
}

func (f *fakeTrafficModule) ConsumeTraffic(context.Context, int, string) (TrafficResult, error) {
	result := f.results[f.consumeCalls]
	f.consumeCalls++
	return result, nil
}

func (f *fakeTrafficModule) SetFlymode(enabled bool) error {
	f.flymodeCalls = append(f.flymodeCalls, enabled)
	return nil
}

func (f *fakeTrafficModule) FlyMode() bool      { return false }
func (f *fakeTrafficModule) ModuleID() string   { return "sim2" }
func (f *fakeTrafficModule) ModuleName() string { return "Air780EHV" }

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
	if task.TaskType != models.ScheduledTaskTypeSMS || task.ModuleID != "sim1" || task.TrafficKB != models.FixedTrafficKB {
		t.Fatalf("unexpected defaults: type=%q module=%q trafficKB=%d", task.TaskType, task.ModuleID, task.TrafficKB)
	}
}

func TestSchedulerMigratesExistingFiveKBTrafficTask(t *testing.T) {
	service, db := newSchedulerTestService(t)
	if err := db.Exec(`INSERT INTO scheduled_tasks
		(id, name, enabled, interval_days, task_type, module_id, traffic_kb)
		VALUES (?, ?, ?, ?, ?, ?, ?)`, "traffic-5kb", "legacy traffic", true, 90, "traffic", "sim1", 5).Error; err != nil {
		t.Fatalf("insert traffic task: %v", err)
	}

	if err := service.BackfillDefaults(context.Background()); err != nil {
		t.Fatalf("backfill defaults: %v", err)
	}
	var trafficKB int
	if err := db.Model(&models.ScheduledTask{}).
		Select("traffic_kb").
		Where("id = ?", "traffic-5kb").
		Scan(&trafficKB).Error; err != nil {
		t.Fatalf("read migrated task: %v", err)
	}
	if trafficKB != models.FixedTrafficKB {
		t.Fatalf("trafficKB=%d, want %d", trafficKB, models.FixedTrafficKB)
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
		TrafficKB:    models.FixedTrafficKB,
	}
	if err := service.Create(context.Background(), task); err == nil {
		t.Fatal("expected unknown module error")
	}
}

func TestSchedulerUsesFixedFiftyKBPayload(t *testing.T) {
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
	if task.TrafficKB != models.FixedTrafficKB {
		t.Fatalf("trafficKB=%d, want %d", task.TrafficKB, models.FixedTrafficKB)
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

func TestTrafficTimeoutResetsCellularDataAndRetriesOnce(t *testing.T) {
	service, db := newSchedulerTestService(t)
	service.trafficRecoveryWait = func(context.Context, time.Duration) error { return nil }
	task := models.ScheduledTask{
		ID:           "traffic-timeout",
		Name:         "90 day keepalive",
		Enabled:      true,
		IntervalDays: 90,
		TaskType:     models.ScheduledTaskTypeTraffic,
		ModuleID:     "sim2",
		TrafficKB:    models.FixedTrafficKB,
	}
	if err := db.Create(&task).Error; err != nil {
		t.Fatalf("create task: %v", err)
	}

	module := &fakeTrafficModule{results: []TrafficResult{
		{RequestID: "first", HTTPCode: -8, Error: "HTTP status -8"},
		{
			RequestID: "second", Success: true, HTTPCode: 200,
			UplinkBytes: 1480, DownlinkBytes: 53256, TotalBytes: 54736, BodyBytes: 51200,
		},
	}}
	if err := service.executeTrafficTask(context.Background(), task, module); err != nil {
		t.Fatalf("execute traffic task: %v", err)
	}
	if module.consumeCalls != 2 {
		t.Fatalf("consume calls=%d, want 2", module.consumeCalls)
	}
	if len(module.flymodeCalls) != 2 || !module.flymodeCalls[0] || module.flymodeCalls[1] {
		t.Fatalf("flymode calls=%v, want [true false]", module.flymodeCalls)
	}

	updated, err := service.GetById(context.Background(), task.ID)
	if err != nil {
		t.Fatalf("get updated task: %v", err)
	}
	if updated.LastRunStatus != models.LastRunStatusSuccess {
		t.Fatalf("last status=%q, want success", updated.LastRunStatus)
	}
	if !strings.Contains(updated.LastRunDetail, "自动重建蜂窝数据连接后重试成功") {
		t.Fatalf("last detail=%q, want recovery detail", updated.LastRunDetail)
	}

	var firstRecord models.TrafficRecord
	if err := db.Where("request_id = ?", "first").First(&firstRecord).Error; err != nil {
		t.Fatalf("get first traffic record: %v", err)
	}
	if firstRecord.Success || !strings.Contains(firstRecord.Error, "连接或读取超时") {
		t.Fatalf("first record=%+v, want explained timeout", firstRecord)
	}
	var secondRecord models.TrafficRecord
	if err := db.Where("request_id = ?", "second").First(&secondRecord).Error; err != nil {
		t.Fatalf("get second traffic record: %v", err)
	}
	if !secondRecord.Success || secondRecord.HTTPCode != 200 {
		t.Fatalf("second record=%+v, want successful retry", secondRecord)
	}
}

func TestTrafficNonTimeoutDoesNotResetCellularData(t *testing.T) {
	service, db := newSchedulerTestService(t)
	task := models.ScheduledTask{
		ID:           "traffic-server-error",
		Name:         "server error",
		Enabled:      true,
		IntervalDays: 90,
		TaskType:     models.ScheduledTaskTypeTraffic,
		ModuleID:     "sim2",
		TrafficKB:    models.FixedTrafficKB,
	}
	if err := db.Create(&task).Error; err != nil {
		t.Fatalf("create task: %v", err)
	}
	module := &fakeTrafficModule{results: []TrafficResult{{
		RequestID: "server-error", HTTPCode: 500, Error: "HTTP status 500",
	}}}

	if err := service.executeTrafficTask(context.Background(), task, module); err == nil {
		t.Fatal("expected traffic failure")
	}
	if module.consumeCalls != 1 {
		t.Fatalf("consume calls=%d, want 1", module.consumeCalls)
	}
	if len(module.flymodeCalls) != 0 {
		t.Fatalf("flymode calls=%v, want none", module.flymodeCalls)
	}
}
