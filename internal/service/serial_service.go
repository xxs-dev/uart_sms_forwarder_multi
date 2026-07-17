package service

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/dushixiang/uart_sms_forwarder/config"
	"github.com/dushixiang/uart_sms_forwarder/internal/models"
	"github.com/go-orz/cache"
	"github.com/google/uuid"
	"github.com/jpillora/backoff"
	"go.bug.st/serial"
	"go.uber.org/zap"
)

const (
	// 缓存键
	CacheKeyDeviceStatus = "device_status"
	// 缓存刷新间隔
	CacheRefreshInterval = 10 * time.Second
	// 缓存过期时间
	CacheTTL = 5 * time.Minute
)

type ScheduledTaskStatusUpdater func(ctx context.Context, msgID string, status models.LastRunStatus) error

// SerialService 串口管理服务
type SerialService struct {
	logger                     *zap.Logger
	config                     config.SerialConfig
	moduleID                   string
	moduleName                 string
	port                       serial.Port
	textMsgService             *TextMessageService
	notifier                   *Notifier
	propertyService            *PropertyService
	handlers                   map[string]messageHandler
	scheduledTaskStatusUpdater ScheduledTaskStatusUpdater
	trafficMu                  sync.Mutex
	pendingTraffic             map[string]chan TrafficResult
	wg                         sync.WaitGroup
	// 设备信息缓存
	deviceCache cache.Cache[string, *StatusData]
	// 连接状态管理
	mu        sync.RWMutex
	portName  string // 当前使用的串口名称
	connected bool   // 连接状态

	// 设备的飞行模式查询永远返回 false，无奈只能在应用层处理
	flyMode atomic.Bool
}

// NewSerialService 创建串口服务实例
func NewSerialService(
	logger *zap.Logger,
	config config.SerialConfig,
	textMsgService *TextMessageService,
	notifier *Notifier,
	propertyService *PropertyService,
) *SerialService {
	service := &SerialService{
		logger:          logger,
		config:          config,
		moduleID:        "default",
		moduleName:      "默认模块",
		textMsgService:  textMsgService,
		notifier:        notifier,
		propertyService: propertyService,
		deviceCache:     cache.New[string, *StatusData](CacheTTL),
		pendingTraffic:  make(map[string]chan TrafficResult),
	}
	service.initMessageHandlers()
	return service
}

func (s *SerialService) SetModuleInfo(id, name string) {
	if id == "" {
		id = "default"
	}
	if name == "" {
		name = id
	}
	s.moduleID = id
	s.moduleName = name
	s.logger = s.logger.With(zap.String("module_id", id), zap.String("module_name", name))
}

func (s *SerialService) ModuleID() string {
	return s.moduleID
}

func (s *SerialService) ModuleName() string {
	return s.moduleName
}

func (s *SerialService) SetScheduledTaskStatusUpdater(updater ScheduledTaskStatusUpdater) {
	s.scheduledTaskStatusUpdater = updater
}

// Start 启动串口服务（使用 backoff 重连机制）
func (s *SerialService) Start() {

	// 启动主循环
	b := &backoff.Backoff{
		Min:    5 * time.Second,
		Max:    1 * time.Minute,
		Factor: 2,
		Jitter: true,
	}

	for {
		err := s.runOnce(b.Reset)

		// 连接失败或断开，使用 backoff 重试
		if err != nil {
			s.setConnected(false)
			retryAfter := b.Duration()
			s.logger.Warn("串口连接异常，将重试",
				zap.Error(err),
				zap.Duration("retry_after", retryAfter))
			s.deviceCache.Delete(CacheKeyDeviceStatus)

			time.Sleep(retryAfter)
		}
	}
}

// setConnected 设置连接状态
func (s *SerialService) setConnected(connected bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.connected = connected
}

// setPortName 设置串口名称
func (s *SerialService) setPortName(portName string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.portName = portName
}

// getConnectionInfo 获取连接信息
func (s *SerialService) getConnectionInfo() (portName string, connected bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.portName, s.connected
}

// runOnce 执行一次连接尝试
func (s *SerialService) runOnce(resetBackoff func()) error {
	// 如果显式配置了串口路径，直接使用它。Docker/LXC 中常把设备映射成
	// /dev/ttySMS1 这类稳定别名，此时 serial.GetPortsList 可能不会列出该别名。
	if s.config.Port != "" {
		selectedPort := s.config.Port
		s.logger.Info("使用配置的串口", zap.String("port", selectedPort))
		return s.connectAndServe(selectedPort, resetBackoff)
	}

	// 获取串口列表
	ports, err := serial.GetPortsList()
	if err != nil {
		return fmt.Errorf("获取串口列表失败: %w", err)
	}

	if len(ports) == 0 {
		return fmt.Errorf("未发现可用串口")
	}

	s.logger.Debug("发现可用串口", zap.Strings("ports", ports))

	// 自动检测
	s.logger.Info("开始自动检测串口...")
	selectedPort, err := s.autoDetectPort(ports)
	if err != nil {
		return fmt.Errorf("自动检测串口失败: %w", err)
	}
	s.logger.Info("自动检测到可用串口", zap.String("port", selectedPort))

	return s.connectAndServe(selectedPort, resetBackoff)
}

func (s *SerialService) connectAndServe(selectedPort string, resetBackoff func()) error {
	// 连接串口
	if err := s.connectSerial(selectedPort); err != nil {
		return fmt.Errorf("连接串口失败: %w", err)
	}

	// 设置连接状态和串口名称
	s.setPortName(selectedPort)
	s.setConnected(true)

	// 重置 backoff（连接成功）
	resetBackoff()

	s.logger.Info("串口连接成功", zap.String("port", selectedPort))

	// 为本次连接创建独立的 context，用于管理连接的生命周期
	connCtx, connCancel := context.WithCancel(context.Background())
	defer connCancel() // 确保退出时取消 context

	// 启动监听 goroutine
	s.wg.Add(1)
	go s.listenSerialData(connCtx, connCancel)

	// 启动定时更新缓存的 goroutine
	s.wg.Add(1)
	go s.periodicCacheUpdate(connCtx)

	// 首次立即发送缓存更新请求
	go s.RequestCacheUpdate()

	// 等待连接断开
	s.wg.Wait()

	// 连接已断开，更新状态
	s.setConnected(false)

	return nil
}

// connectSerial 连接串口
func (s *SerialService) connectSerial(portName string) error {
	mode := &serial.Mode{
		BaudRate: 115200,
		DataBits: 8,
		StopBits: serial.OneStopBit,
		Parity:   serial.NoParity,
	}

	port, err := serial.Open(portName, mode)
	if err != nil {
		return err
	}

	s.port = port
	return nil
}

// autoDetectPort 自动检测可用串口
func (s *SerialService) autoDetectPort(ports []string) (string, error) {
	for _, portName := range ports {
		s.logger.Debug("测试串口", zap.String("port", portName))

		mode := &serial.Mode{
			BaudRate: 115200,
			DataBits: 8,
			StopBits: serial.OneStopBit,
			Parity:   serial.NoParity,
		}

		port, err := serial.Open(portName, mode)
		if err != nil {
			s.logger.Debug("打开串口失败", zap.String("port", portName), zap.Error(err))
			continue
		}

		// 设置读取超时
		port.SetReadTimeout(1 * time.Second)

		// 发送测试命令（使用正确的协议格式）
		testCmd := map[string]string{"action": "get_status"}
		jsonData, _ := json.Marshal(testCmd)
		// 添加协议包围标志
		message := fmt.Sprintf("CMD_START:%s:CMD_END\r\n", string(jsonData))

		_, err = port.Write([]byte(message))
		if err != nil {
			port.Close()
			continue
		}

		// 等待响应
		time.Sleep(500 * time.Millisecond)

		buffer := make([]byte, 4096)
		n, err := port.Read(buffer)
		port.Close()

		if err == nil && n > 0 {
			response := string(buffer[:n])
			if isValidResponse(response) {
				s.logger.Debug("检测到可用串口", zap.String("port", portName))
				return portName, nil
			}
		}
	}

	return "", fmt.Errorf("未检测到可用串口")
}

// listenSerialData 监听串口数据（在独立 goroutine 中运行）
func (s *SerialService) listenSerialData(connCtx context.Context, connCancel context.CancelFunc) {
	defer s.wg.Done()
	defer func() {
		if r := recover(); r != nil {
			s.logger.Error("串口监听 goroutine panic", zap.Any("recover", r))
		}
		// 关闭串口
		if s.port != nil {
			s.port.Close()
			s.port = nil
		}
		// 取消连接 context，通知其他 goroutine 连接已断开
		connCancel()
	}()

	reader := bufio.NewReader(s.port)

	for {
		select {
		case <-connCtx.Done():
			s.logger.Info("串口监听停止")
			return
		default:
			line, err := reader.ReadString('\n')
			if err != nil {
				if err == io.EOF {
					// EOF 可能表示设备断开
					s.logger.Warn("串口读取 EOF，设备可能已断开")
					return
				}
				// 检查 context 是否已取消
				if connCtx.Err() != nil {
					return
				}
				// 其他错误，可能是设备断开或硬件错误
				s.logger.Error("读取串口数据错误，退出监听", zap.Error(err))
				return
			}

			s.processReceivedData(strings.TrimSpace(line))
		}
	}
}

// periodicCacheUpdate 定时更新缓存
func (s *SerialService) periodicCacheUpdate(connCtx context.Context) {
	defer s.wg.Done()
	defer func() {
		if r := recover(); r != nil {
			s.logger.Error("定时更新缓存 goroutine panic", zap.Any("recover", r))
		}
	}()

	ticker := time.NewTicker(CacheRefreshInterval)
	defer ticker.Stop()

	for {
		select {
		case <-connCtx.Done():
			s.logger.Info("停止定时更新缓存")
			return
		case <-ticker.C:
			s.RequestCacheUpdate()
		}
	}
}

// RequestCacheUpdate 请求更新缓存（只发送命令，不等待响应）
func (s *SerialService) RequestCacheUpdate() {
	s.logger.Debug("发送缓存更新请求")

	// 发送获取设备状态命令（包含移动网络信息）
	if err := s.sendJSONCommand(map[string]string{"action": "get_status"}); err != nil {
		s.logger.Error("发送设备状态请求失败", zap.Error(err))
	}
}

// processReceivedData 处理接收到的数据
func (s *SerialService) processReceivedData(data string) {
	s.logger.Sugar().Debugf("received data: %s", data)
	msg, err := parseSMSFrame(data)
	if err != nil {
		if errors.Is(err, errNotSMSFrame) {
			return
		}
		if errors.Is(err, errMissingType) {
			s.logger.Warn("消息类型缺失", zap.String("data", data))
			return
		}
		s.logger.Error("解析串口消息失败", zap.Error(err), zap.String("data", data))
		return
	}

	s.routeMessage(msg)
}

// SendSMS 发送短信
func (s *SerialService) SendSMS(to, content string) (string, error) {
	// 先保存发送记录，状态为 "sending"
	ctx := context.Background()
	msgID := uuid.NewString()
	msg := &models.TextMessage{
		ID:        msgID,
		ModuleID:  s.moduleID,
		From:      "", // 发送方是本机
		To:        to,
		Content:   content,
		Type:      models.MessageTypeOutgoing,
		Status:    models.MessageStatusSending, // 初始状态为发送中
		CreatedAt: time.Now().UnixMilli(),
	}

	if err := s.textMsgService.Save(ctx, msg); err != nil {
		s.logger.Error("保存短信发送记录失败", zap.Error(err))
		return "", err
	}

	// 发送命令，使用消息 ID 作为 request_id
	cmd := map[string]any{
		"action":     "send_sms",
		"to":         to,
		"content":    content,
		"request_id": msgID,
	}

	if err := s.sendJSONCommand(cmd); err != nil {
		s.logger.Error("发送短信命令失败", zap.Error(err))
		// 更新状态为失败
		// 更新状态为失败
		_ = s.textMsgService.UpdateStatusById(ctx, msgID, models.MessageStatusFailed)
		return "", err
	}

	s.logger.Info("发送短信命令成功", zap.String("to", to), zap.String("request_id", msgID))

	return msgID, nil
}

// GetStatus 获取设备状态（从缓存读取，包含 mobile 信息和串口连接状态）
func (s *SerialService) GetStatus() (*StatusData, error) {
	// 获取连接信息
	portName, connected := s.getConnectionInfo()

	// 从缓存读取
	if status, ok := s.deviceCache.Get(CacheKeyDeviceStatus); ok {
		status.ModuleID = s.moduleID
		status.ModuleName = s.moduleName
		// 更新串口连接信息
		status.PortName = portName
		status.Connected = connected

		// 更新飞行模式状态
		status.Flymode = s.FlyMode()
		return status, nil
	}

	// 缓存未命中，但仍然返回连接状态
	status := &StatusData{
		ModuleID:   s.moduleID,
		ModuleName: s.moduleName,
		PortName:   portName,
		Connected:  connected,
	}
	return status, nil
}

func (s *SerialService) FlyMode() bool {
	// 返回当前飞行模式状态
	return s.flyMode.Load()
}

// SetFlymode 设置飞行模式
// enabled: true 表示启用飞行模式，false 表示禁用飞行模式
func (s *SerialService) SetFlymode(enabled bool) error {
	cmd := map[string]any{
		"action":  "set_flymode",
		"enabled": enabled,
	}
	if err := s.sendJSONCommand(cmd); err != nil {
		return err
	}
	// 更新飞行模式状态
	s.flyMode.Store(enabled)
	return nil
}

// RebootMcu 重启模块
func (s *SerialService) RebootMcu() error {
	cmd := map[string]string{"action": "reboot_mcu"}
	if err := s.sendJSONCommand(cmd); err != nil {
		return err
	}
	// 重启后，飞行模式默认关闭
	s.flyMode.Store(false)
	return nil
}

// sendJSONCommand 发送JSON命令到设备
func (s *SerialService) sendJSONCommand(cmd any) error {
	if s.port == nil {
		return fmt.Errorf("串口未连接")
	}

	message, jsonData, err := buildCommandMessage(cmd)
	if err != nil {
		return err
	}

	_, err = s.port.Write(message)
	if err != nil {
		return fmt.Errorf("串口写入失败: %w", err)
	}
	s.logger.Sugar().Debugf("send command: %s", jsonData)

	return nil
}
