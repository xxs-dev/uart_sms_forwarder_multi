package service

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/dushixiang/uart_sms_forwarder/internal/models"
	"github.com/dushixiang/uart_sms_forwarder/internal/repo"
	"github.com/glebarez/sqlite"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

func newTextMessageServiceForTest(t *testing.T) *TextMessageService {
	t.Helper()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	if err := db.AutoMigrate(&models.TextMessage{}); err != nil {
		t.Fatalf("migrate test database: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get test database: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	return NewTextMessageService(zap.NewNop(), repo.NewTextMessageRepo(db))
}

func saveTextMessageForTest(t *testing.T, service *TextMessageService, message *models.TextMessage) {
	t.Helper()
	if err := service.Save(context.Background(), message); err != nil {
		t.Fatalf("save test message: %v", err)
	}
}

func findConversation(conversations []*Conversation, peer string) *Conversation {
	for _, conversation := range conversations {
		if conversation.Peer == peer {
			return conversation
		}
	}
	return nil
}

func TestTextMessageServiceScopesQueriesByModule(t *testing.T) {
	service := newTextMessageServiceForTest(t)
	ctx := context.Background()

	saveTextMessageForTest(t, service, &models.TextMessage{ID: "sim1-in", ModuleID: "sim1", From: "+100", Content: "one", Type: models.MessageTypeIncoming, Status: models.MessageStatusReceived, CreatedAt: 1000})
	saveTextMessageForTest(t, service, &models.TextMessage{ID: "sim2-in", ModuleID: "sim2", From: "+200", Content: "two", Type: models.MessageTypeIncoming, Status: models.MessageStatusReceived, CreatedAt: 2000})
	saveTextMessageForTest(t, service, &models.TextMessage{ID: "sim2-out", ModuleID: "sim2", To: "+100", Content: "three", Type: models.MessageTypeOutgoing, Status: models.MessageStatusSent, CreatedAt: 3000})

	sim1Conversations, err := service.GetConversations(ctx, "sim1")
	if err != nil {
		t.Fatalf("get sim1 conversations: %v", err)
	}
	if len(sim1Conversations) != 1 || sim1Conversations[0].Peer != "+100" || sim1Conversations[0].MessageCount != 1 {
		t.Fatalf("unexpected sim1 conversations: %#v", sim1Conversations)
	}

	sim2Conversations, err := service.GetConversations(ctx, "sim2")
	if err != nil {
		t.Fatalf("get sim2 conversations: %v", err)
	}
	if len(sim2Conversations) != 2 || findConversation(sim2Conversations, "+100") == nil || findConversation(sim2Conversations, "+200") == nil {
		t.Fatalf("unexpected sim2 conversations: %#v", sim2Conversations)
	}

	sim1Messages, err := service.GetConversationMessages(ctx, "+100", "sim1")
	if err != nil {
		t.Fatalf("get sim1 messages: %v", err)
	}
	if len(sim1Messages) != 1 || sim1Messages[0].ModuleID != "sim1" {
		t.Fatalf("unexpected sim1 messages: %#v", sim1Messages)
	}

	sim2Messages, err := service.GetConversationMessages(ctx, "+100", "sim2")
	if err != nil {
		t.Fatalf("get sim2 messages: %v", err)
	}
	if len(sim2Messages) != 1 || sim2Messages[0].ModuleID != "sim2" {
		t.Fatalf("unexpected sim2 messages: %#v", sim2Messages)
	}
}

func TestTextMessageServiceScopesDestructiveOperationsByModule(t *testing.T) {
	service := newTextMessageServiceForTest(t)
	ctx := context.Background()

	saveTextMessageForTest(t, service, &models.TextMessage{ID: "sim1", ModuleID: "sim1", From: "+100", Content: "one", Type: models.MessageTypeIncoming, Status: models.MessageStatusReceived})
	saveTextMessageForTest(t, service, &models.TextMessage{ID: "sim2", ModuleID: "sim2", From: "+100", Content: "two", Type: models.MessageTypeIncoming, Status: models.MessageStatusReceived})

	if err := service.DeleteConversation(ctx, "+100", "sim1"); err != nil {
		t.Fatalf("delete sim1 conversation: %v", err)
	}
	remaining, err := service.GetConversationMessages(ctx, "+100", "")
	if err != nil {
		t.Fatalf("get remaining messages: %v", err)
	}
	if len(remaining) != 1 || remaining[0].ModuleID != "sim2" {
		t.Fatalf("delete crossed module boundary: %#v", remaining)
	}

	if err := service.Clear(ctx, "sim2"); err != nil {
		t.Fatalf("clear sim2 messages: %v", err)
	}
	remaining, err = service.GetConversationMessages(ctx, "+100", "")
	if err != nil {
		t.Fatalf("get messages after clear: %v", err)
	}
	if len(remaining) != 0 {
		t.Fatalf("expected no messages after scoped clear: %#v", remaining)
	}
}

func TestTextMessageServiceBackfillsLegacyMessagesOnly(t *testing.T) {
	service := newTextMessageServiceForTest(t)
	ctx := context.Background()

	saveTextMessageForTest(t, service, &models.TextMessage{ID: "legacy", From: "+100", Content: "old", Type: models.MessageTypeIncoming, Status: models.MessageStatusReceived})
	saveTextMessageForTest(t, service, &models.TextMessage{ID: "sim2", ModuleID: "sim2", From: "+200", Content: "new", Type: models.MessageTypeIncoming, Status: models.MessageStatusReceived})

	if err := service.BackfillMissingModuleID(ctx, "sim1"); err != nil {
		t.Fatalf("backfill legacy messages: %v", err)
	}
	legacy, err := service.Get(ctx, "legacy")
	if err != nil {
		t.Fatalf("get legacy message: %v", err)
	}
	if legacy.ModuleID != "sim1" {
		t.Fatalf("legacy message module = %q, want sim1", legacy.ModuleID)
	}
	sim2, err := service.Get(ctx, "sim2")
	if err != nil {
		t.Fatalf("get sim2 message: %v", err)
	}
	if sim2.ModuleID != "sim2" {
		t.Fatalf("sim2 message module changed to %q", sim2.ModuleID)
	}
}
