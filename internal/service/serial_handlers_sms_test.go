package service

import (
	"context"
	"fmt"
	"testing"

	"github.com/dushixiang/uart_sms_forwarder/internal/models"
	"github.com/dushixiang/uart_sms_forwarder/internal/repo"
	"github.com/glebarez/sqlite"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

func newIncomingSMSTestService(t *testing.T) (*SerialService, *TextMessageService) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.TextMessage{}); err != nil {
		t.Fatalf("migrate text messages: %v", err)
	}
	messageService := NewTextMessageService(zap.NewNop(), repo.NewTextMessageRepo(db))
	serial := &SerialService{
		logger:         zap.NewNop(),
		moduleID:       "epv",
		textMsgService: messageService,
		smsPDUDecoder:  NewSMSPDUDecoder(),
	}
	return serial, messageService
}

func TestHandleIncomingSMSUsesDecodedPDUContent(t *testing.T) {
	serial, messages := newIncomingSMSTestService(t)
	pdu := "07911614220991F1040B911605935713F200008140806113912304D7F79B0E"
	serial.handleIncomingSMS(&ParsedMessage{JSON: fmt.Sprintf(
		`{"type":"incoming_sms","timestamp":1700000000,"from":"legacy-sender","content":"broken","content_hex":"62726F6B656E","pdu_hex":%q}`,
		pdu,
	)})

	stored, err := messages.GetConversationMessages(context.Background(), "+61503975312", "epv")
	if err != nil {
		t.Fatalf("list messages: %v", err)
	}
	if len(stored) != 1 {
		t.Fatalf("stored message count = %d, want 1", len(stored))
	}
	message := stored[0]
	if message.Content != "Woot" || message.RawContent != "broken" || message.RawFrom != "legacy-sender" {
		t.Fatalf("unexpected decoded message: %+v", message)
	}
	if message.DecodeStatus != models.MessageDecodeStatusDecoded || message.Alphabet != "gsm7" || message.PDUHex != pdu {
		t.Fatalf("unexpected PDU diagnostics: %+v", message)
	}
}

func TestHandleIncomingSMSPreservesFailedPDU(t *testing.T) {
	serial, messages := newIncomingSMSTestService(t)
	serial.handleIncomingSMS(&ParsedMessage{JSON: `{"type":"incoming_sms","timestamp":1700000000,"from":"giffgaff","content":"broken","content_hex":"62726F6B656E","pdu_hex":"BAD"}`})

	stored, err := messages.GetConversationMessages(context.Background(), "giffgaff", "epv")
	if err != nil {
		t.Fatalf("list messages: %v", err)
	}
	if len(stored) != 1 {
		t.Fatalf("stored message count = %d, want 1", len(stored))
	}
	message := stored[0]
	if message.Content != "broken" || message.PDUHex != "BAD" || message.DecodeStatus != models.MessageDecodeStatusFailed || message.DecodeError == "" {
		t.Fatalf("failed PDU was not preserved: %+v", message)
	}
}
