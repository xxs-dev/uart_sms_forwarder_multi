package service

import (
	"context"
	"fmt"
	"net/url"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

const trafficRequestTimeout = 90 * time.Second

type TrafficResult struct {
	RequestID      string `json:"request_id"`
	Success        bool   `json:"success"`
	HTTPCode       int64  `json:"http_code"`
	UplinkBytes    int64  `json:"uplink_bytes"`
	DownlinkBytes  int64  `json:"downlink_bytes"`
	TotalBytes     int64  `json:"total_bytes"`
	BodyBytes      int64  `json:"body_bytes"`
	ConnectionOpen bool   `json:"connection_open"`
	Error          string `json:"error"`
}

func (s *SerialService) ConsumeTraffic(ctx context.Context, targetKB int, endpoint string) (TrafficResult, error) {
	result := TrafficResult{RequestID: uuid.NewString()}
	if targetKB <= 0 {
		return result, fmt.Errorf("目标流量必须大于 0 KB")
	}
	parsedEndpoint, err := url.ParseRequestURI(endpoint)
	if err != nil || (parsedEndpoint.Scheme != "http" && parsedEndpoint.Scheme != "https") || parsedEndpoint.Host == "" {
		return result, fmt.Errorf("流量测试地址无效")
	}

	resultCh := make(chan TrafficResult, 1)
	s.trafficMu.Lock()
	s.pendingTraffic[result.RequestID] = resultCh
	s.trafficMu.Unlock()
	defer func() {
		s.trafficMu.Lock()
		delete(s.pendingTraffic, result.RequestID)
		s.trafficMu.Unlock()
	}()

	cmd := map[string]any{
		"action":       "consume_data",
		"request_id":   result.RequestID,
		"url":          endpoint,
		"target_bytes": targetKB * 1024,
	}
	if err := s.sendJSONCommand(cmd); err != nil {
		return result, fmt.Errorf("发送流量任务命令失败: %w", err)
	}

	timer := time.NewTimer(trafficRequestTimeout)
	defer timer.Stop()
	select {
	case result = <-resultCh:
		return result, nil
	case <-ctx.Done():
		return result, fmt.Errorf("等待模块流量结果取消: %w", ctx.Err())
	case <-timer.C:
		return result, fmt.Errorf("等待模块流量结果超时")
	}
}

func (s *SerialService) handleTrafficResult(msg *ParsedMessage) {
	result := TrafficResult{
		RequestID:      stringValue(msg.Payload, "request_id"),
		Success:        boolValue(msg.Payload, "success"),
		HTTPCode:       int64Value(msg.Payload, "http_code"),
		UplinkBytes:    int64Value(msg.Payload, "uplink_bytes"),
		DownlinkBytes:  int64Value(msg.Payload, "downlink_bytes"),
		TotalBytes:     int64Value(msg.Payload, "total_bytes"),
		BodyBytes:      int64Value(msg.Payload, "body_bytes"),
		ConnectionOpen: boolValue(msg.Payload, "connection_open"),
		Error:          stringValue(msg.Payload, "error"),
	}
	if result.RequestID == "" {
		s.logger.Warn("收到流量结果但缺少 request_id", zap.Any("payload", msg.Payload))
		return
	}
	if !result.Success && result.Error == "" {
		result.Error = "模块未返回失败原因"
	}

	s.trafficMu.Lock()
	resultCh, ok := s.pendingTraffic[result.RequestID]
	s.trafficMu.Unlock()
	if !ok {
		s.logger.Warn("收到已过期或未知的流量结果", zap.String("request_id", result.RequestID))
		return
	}

	select {
	case resultCh <- result:
	default:
		s.logger.Warn("流量结果通道已满", zap.String("request_id", result.RequestID))
	}
}

func stringValue(payload map[string]interface{}, key string) string {
	value, _ := payload[key].(string)
	return value
}

func boolValue(payload map[string]interface{}, key string) bool {
	value, _ := payload[key].(bool)
	return value
}

func int64Value(payload map[string]interface{}, key string) int64 {
	switch value := payload[key].(type) {
	case float64:
		return int64(value)
	case int64:
		return value
	case int:
		return int64(value)
	default:
		return 0
	}
}
