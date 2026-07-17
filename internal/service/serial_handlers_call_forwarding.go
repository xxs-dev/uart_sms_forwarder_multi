package service

import (
	"context"
	"fmt"
	"time"

	"github.com/dushixiang/uart_sms_forwarder/internal/models"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

const callForwardingRequestTimeout = 30 * time.Second

type CallForwardingResult struct {
	RequestID    string `json:"request_id"`
	Success      bool   `json:"success"`
	Status       string `json:"status"`
	Enabled      bool   `json:"enabled"`
	Number       string `json:"number"`
	DelaySeconds int    `json:"delay_seconds"`
	Error        string `json:"error"`
}

func (s *SerialService) ConfigureCallForwarding(
	ctx context.Context,
	input CallForwardingInput,
) (CallForwardingResult, error) {
	result := CallForwardingResult{RequestID: uuid.NewString()}
	normalized, err := validateCallForwardingInput(input)
	if err != nil {
		return result, err
	}

	resultCh := make(chan CallForwardingResult, 1)
	s.callForwardingMu.Lock()
	s.pendingCallForwarding[result.RequestID] = resultCh
	s.callForwardingMu.Unlock()
	defer func() {
		s.callForwardingMu.Lock()
		delete(s.pendingCallForwarding, result.RequestID)
		s.callForwardingMu.Unlock()
	}()

	cmd := map[string]any{
		"action":        "configure_call_forwarding",
		"request_id":    result.RequestID,
		"enabled":       normalized.Enabled,
		"number":        normalized.Number,
		"delay_seconds": normalized.DelaySeconds,
	}
	if err := s.sendJSONCommand(cmd); err != nil {
		return result, fmt.Errorf("发送无应答转移命令失败: %w", err)
	}

	timer := time.NewTimer(callForwardingRequestTimeout)
	defer timer.Stop()
	select {
	case result = <-resultCh:
		return result, nil
	case <-ctx.Done():
		return result, fmt.Errorf("等待模块无应答转移结果取消: %w", ctx.Err())
	case <-timer.C:
		return result, fmt.Errorf("等待模块无应答转移结果超时，请确认插件版本不低于 1.3.0")
	}
}

func (s *SerialService) handleCallForwardingResult(msg *ParsedMessage) {
	result := CallForwardingResult{
		RequestID:    stringValue(msg.Payload, "request_id"),
		Success:      boolValue(msg.Payload, "success"),
		Status:       stringValue(msg.Payload, "status"),
		Enabled:      boolValue(msg.Payload, "enabled"),
		Number:       stringValue(msg.Payload, "number"),
		DelaySeconds: int(int64Value(msg.Payload, "delay_seconds")),
		Error:        stringValue(msg.Payload, "error"),
	}
	if result.RequestID == "" {
		s.logger.Warn("收到无应答转移结果但缺少 request_id", zap.Any("payload", msg.Payload))
		return
	}
	if result.Success {
		result.Status = models.CallForwardingStatusSubmitted
	} else {
		result.Status = models.CallForwardingStatusFailed
		if result.Error == "" {
			result.Error = "模块未返回失败原因"
		}
	}

	s.callForwardingMu.Lock()
	resultCh, ok := s.pendingCallForwarding[result.RequestID]
	s.callForwardingMu.Unlock()
	if !ok {
		s.logger.Warn("收到已过期或未知的无应答转移结果", zap.String("request_id", result.RequestID))
		return
	}

	select {
	case resultCh <- result:
	default:
		s.logger.Warn("无应答转移结果通道已满", zap.String("request_id", result.RequestID))
	}
}
