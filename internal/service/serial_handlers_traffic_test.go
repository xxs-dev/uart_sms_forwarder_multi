package service

import (
	"testing"

	"go.uber.org/zap"
)

func TestHandleTrafficResultRoutesMeasuredBytes(t *testing.T) {
	resultCh := make(chan TrafficResult, 1)
	service := &SerialService{
		logger:         zap.NewNop(),
		pendingTraffic: map[string]chan TrafficResult{"request-1": resultCh},
	}

	service.handleTrafficResult(&ParsedMessage{Payload: map[string]interface{}{
		"request_id":      "request-1",
		"success":         true,
		"http_code":       float64(200),
		"uplink_bytes":    float64(712),
		"downlink_bytes":  float64(4408),
		"total_bytes":     float64(5120),
		"body_bytes":      float64(4096),
		"connection_open": false,
	}})

	result := <-resultCh
	if !result.Success || result.HTTPCode != 200 || result.TotalBytes != 5120 || result.BodyBytes != 4096 {
		t.Fatalf("unexpected traffic result: %+v", result)
	}
}

func TestHandleTrafficResultAddsMissingFailureReason(t *testing.T) {
	resultCh := make(chan TrafficResult, 1)
	service := &SerialService{
		logger:         zap.NewNop(),
		pendingTraffic: map[string]chan TrafficResult{"request-2": resultCh},
	}
	service.handleTrafficResult(&ParsedMessage{Payload: map[string]interface{}{
		"request_id": "request-2",
		"success":    false,
	}})

	if result := <-resultCh; result.Error == "" {
		t.Fatal("expected fallback failure reason")
	}
}
