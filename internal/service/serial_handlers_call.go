package service

import (
	"context"
	"encoding/json"

	"go.uber.org/zap"
)

// IncomingCall 来电消息结构
type IncomingCall struct {
	Timestamp int64  `json:"timestamp"`
	From      string `json:"from"`
	Type      string `json:"type"`
}

// handleIncomingCall 处理来电通知
func (s *SerialService) handleIncomingCall(msg *ParsedMessage) {
	var call IncomingCall
	if err := json.Unmarshal([]byte(msg.JSON), &call); err != nil {
		s.logger.Error("来电消息解析失败", zap.Error(err))
		return
	}

	s.logger.Info("收到来电",
		zap.String("from", call.From),
		zap.Int64("timestamp", call.Timestamp))
	if s.callRecords != nil {
		if _, err := s.callRecords.RecordIncoming(
			context.Background(),
			s.ModuleID(),
			s.ModuleName(),
			call.From,
			call.Timestamp,
		); err != nil {
			s.logger.Error("保存来电记录失败", zap.Error(err))
		}
	}

	// 转换为通用通知消息并发送
	notifMsg := NotificationMessage{
		Type:      "call",
		From:      call.From,
		Content:   "", // 来电无内容
		Timestamp: call.Timestamp,
	}

	if s.notifier != nil && s.propertyService != nil {
		go s.sendNotificationMessage(context.Background(), notifMsg)
	}
}

// handleCallDisconnected 处理通话结束通知
func (s *SerialService) handleCallDisconnected(msg *ParsedMessage) {
	timestamp := int64Value(msg.Payload, "timestamp")

	s.logger.Info("通话已结束",
		zap.Int64("timestamp", timestamp))
	if s.callRecords != nil {
		if err := s.callRecords.RecordDisconnected(context.Background(), s.ModuleID(), timestamp); err != nil {
			s.logger.Error("更新来电结束时间失败", zap.Error(err))
		}
	}
}
