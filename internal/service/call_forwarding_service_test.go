package service

import (
	"context"
	"errors"
	"testing"

	"github.com/dushixiang/uart_sms_forwarder/internal/models"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

type fakeCallForwardingCommander struct {
	result CallForwardingResult
	err    error
	calls  int
	input  CallForwardingInput
}

func (f *fakeCallForwardingCommander) ConfigureCallForwarding(
	_ context.Context,
	input CallForwardingInput,
) (CallForwardingResult, error) {
	f.calls++
	f.input = input
	return f.result, f.err
}

func newCallForwardingTestService(t *testing.T) *CallForwardingService {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.CallForwardingConfig{}); err != nil {
		t.Fatalf("migrate call forwarding config: %v", err)
	}
	return NewCallForwardingService(db)
}

func TestApplyCallForwardingPersistsSubmittedConfig(t *testing.T) {
	service := newCallForwardingTestService(t)
	commander := &fakeCallForwardingCommander{result: CallForwardingResult{
		Success: true,
		Status:  models.CallForwardingStatusSubmitted,
	}}
	input := CallForwardingInput{Enabled: true, Number: "+441234567890", DelaySeconds: 20}

	config, err := service.Apply(context.Background(), "sim2", "Air780EHV", input, commander)
	if err != nil {
		t.Fatalf("apply forwarding: %v", err)
	}
	if commander.calls != 1 || commander.input != input {
		t.Fatalf("unexpected commander call: calls=%d input=%+v", commander.calls, commander.input)
	}
	if !config.Enabled || config.Number != input.Number || config.DelaySeconds != 20 ||
		config.LastStatus != models.CallForwardingStatusSubmitted || config.LastError != "" {
		t.Fatalf("unexpected saved config: %+v", config)
	}

	stored, err := service.Get(context.Background(), "sim2", "Air780EHV")
	if err != nil {
		t.Fatalf("get forwarding: %v", err)
	}
	if stored.Number != input.Number || stored.LastStatus != models.CallForwardingStatusSubmitted {
		t.Fatalf("unexpected persisted config: %+v", stored)
	}
}

func TestApplyCallForwardingPersistsModuleFailureReason(t *testing.T) {
	service := newCallForwardingTestService(t)
	commander := &fakeCallForwardingCommander{result: CallForwardingResult{
		Success: false,
		Status:  models.CallForwardingStatusFailed,
		Error:   "carrier rejected supplementary service request",
	}}

	config, err := service.Apply(context.Background(), "sim1", "Air780EPV", CallForwardingInput{
		Enabled:      false,
		Number:       "+8613800138000",
		DelaySeconds: 15,
	}, commander)
	if err == nil {
		t.Fatal("expected module failure")
	}
	if config.LastStatus != models.CallForwardingStatusFailed || config.LastError == "" {
		t.Fatalf("failure was not persisted: %+v", config)
	}
}

func TestApplyCallForwardingPersistsTransportFailure(t *testing.T) {
	service := newCallForwardingTestService(t)
	commander := &fakeCallForwardingCommander{err: errors.New("serial disconnected")}

	config, err := service.Apply(context.Background(), "sim2", "Air780EHV", CallForwardingInput{
		Enabled:      true,
		Number:       "13800138000",
		DelaySeconds: 30,
	}, commander)
	if err == nil {
		t.Fatal("expected transport failure")
	}
	if config.LastStatus != models.CallForwardingStatusFailed || config.LastError != "serial disconnected" {
		t.Fatalf("transport failure was not persisted: %+v", config)
	}
}

func TestApplyCallForwardingRejectsUnsafeInput(t *testing.T) {
	service := newCallForwardingTestService(t)
	tests := []CallForwardingInput{
		{Enabled: true, Number: "**21*123#", DelaySeconds: 20},
		{Enabled: true, Number: "+44123 456", DelaySeconds: 20},
		{Enabled: true, Number: "12", DelaySeconds: 20},
		{Enabled: true, Number: "+441234567890", DelaySeconds: 12},
	}

	for _, input := range tests {
		commander := &fakeCallForwardingCommander{}
		if _, err := service.Apply(context.Background(), "sim2", "Air780EHV", input, commander); err == nil {
			t.Fatalf("input should be rejected: %+v", input)
		}
		if commander.calls != 0 {
			t.Fatalf("unsafe input reached commander: %+v", input)
		}
	}
}
