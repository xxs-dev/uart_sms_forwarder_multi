package service

import (
	"context"
	"testing"

	"github.com/dushixiang/uart_sms_forwarder/internal/models"
	"github.com/glebarez/sqlite"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

func TestPropertyServicePersistsModuleIdentity(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.Property{}); err != nil {
		t.Fatalf("migrate properties: %v", err)
	}

	service := NewPropertyService(zap.NewNop(), db)
	ctx := context.Background()
	if err := service.InitializeDefaultConfigs(ctx); err != nil {
		t.Fatalf("initialize properties: %v", err)
	}

	want := models.ModuleIdentity{Alias: "英国卡", PhoneNumber: "+447700900123"}
	if err := service.SetModuleIdentity(ctx, "sim1", want); err != nil {
		t.Fatalf("set identity: %v", err)
	}
	got, err := service.GetModuleIdentity(ctx, "sim1")
	if err != nil {
		t.Fatalf("get identity: %v", err)
	}
	if got != want {
		t.Fatalf("identity = %#v, want %#v", got, want)
	}

	if err := service.SetModuleIdentity(ctx, "sim1", models.ModuleIdentity{}); err != nil {
		t.Fatalf("clear identity: %v", err)
	}
	got, err = service.GetModuleIdentity(ctx, "sim1")
	if err != nil {
		t.Fatalf("get cleared identity: %v", err)
	}
	if got != (models.ModuleIdentity{}) {
		t.Fatalf("cleared identity = %#v", got)
	}
}
