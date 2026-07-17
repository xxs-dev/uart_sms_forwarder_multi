package service

import (
	"testing"

	"github.com/dushixiang/uart_sms_forwarder/internal/models"
	"go.uber.org/zap"
)

func TestHandleCallForwardingResultRoutesSubmission(t *testing.T) {
	resultCh := make(chan CallForwardingResult, 1)
	service := &SerialService{
		logger:                zap.NewNop(),
		pendingCallForwarding: map[string]chan CallForwardingResult{"request-1": resultCh},
	}

	service.handleCallForwardingResult(&ParsedMessage{Payload: map[string]interface{}{
		"request_id":    "request-1",
		"success":       true,
		"status":        "submitted",
		"enabled":       true,
		"number":        "+441234567890",
		"delay_seconds": float64(20),
	}})

	result := <-resultCh
	if !result.Success || result.Status != models.CallForwardingStatusSubmitted ||
		!result.Enabled || result.Number != "+441234567890" || result.DelaySeconds != 20 {
		t.Fatalf("unexpected forwarding result: %+v", result)
	}
}

func TestHandleCallForwardingResultAddsMissingFailureReason(t *testing.T) {
	resultCh := make(chan CallForwardingResult, 1)
	service := &SerialService{
		logger:                zap.NewNop(),
		pendingCallForwarding: map[string]chan CallForwardingResult{"request-2": resultCh},
	}

	service.handleCallForwardingResult(&ParsedMessage{Payload: map[string]interface{}{
		"request_id": "request-2",
		"success":    false,
	}})

	result := <-resultCh
	if result.Status != models.CallForwardingStatusFailed || result.Error == "" {
		t.Fatalf("missing normalized failure details: %+v", result)
	}
}
