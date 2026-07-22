package service

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/dushixiang/uart_sms_forwarder/internal/models"
	"github.com/dushixiang/uart_sms_forwarder/internal/repo"
	"github.com/go-orz/orz"

	"github.com/google/uuid"
	"github.com/robfig/cron/v3"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

const (
	trafficFlightModeHold   = 5 * time.Second
	trafficNetworkReadyWait = 20 * time.Second
)

type trafficModule interface {
	ConsumeTraffic(context.Context, int, string) (TrafficResult, error)
	SetFlymode(bool) error
	FlyMode() bool
	ModuleID() string
	ModuleName() string
}

// SchedulerService 定时任务调度服务（包含任务管理功能）
type SchedulerService struct {
	logger              *zap.Logger
	cron                *cron.Cron
	repo                *repo.ScheduledTaskRepo
	serialManager       *SerialManager
	trafficEndpoint     string
	trafficRecords      *TrafficRecordService
	trafficRecoveryWait func(context.Context, time.Duration) error
	executionMu         sync.Mutex
}

// NewSchedulerService 创建定时任务服务实例
func NewSchedulerService(
	logger *zap.Logger,
	db *gorm.DB,
	serialManager *SerialManager,
	trafficEndpoint string,
	trafficRecords *TrafficRecordService,
) *SchedulerService {
	return &SchedulerService{
		logger:              logger,
		repo:                repo.NewScheduledTaskRepo(db),
		serialManager:       serialManager,
		trafficEndpoint:     trafficEndpoint,
		trafficRecords:      trafficRecords,
		trafficRecoveryWait: waitForTrafficRecovery,
	}
}

func (s *SchedulerService) BackfillDefaults(ctx context.Context) error {
	defaultModuleID := ""
	if defaultService := s.serialManager.DefaultService(); defaultService != nil {
		defaultModuleID = defaultService.ModuleID()
	}
	return s.repo.BackfillDefaults(ctx, defaultModuleID)
}

// ==================== 任务管理方法 ====================

// GetAll 获取所有定时任务
func (s *SchedulerService) GetAll(ctx context.Context) ([]models.ScheduledTask, error) {
	tasks, err := s.repo.FindAll(ctx)
	if err != nil {
		return nil, err
	}
	for i := range tasks {
		s.normalizeTask(&tasks[i])
	}
	return tasks, nil
}

// GetAllEnabled 获取所有启用的定时任务
func (s *SchedulerService) GetAllEnabled(ctx context.Context) ([]models.ScheduledTask, error) {
	tasks, err := s.repo.FindAllEnabled(ctx)
	if err != nil {
		return nil, err
	}
	for i := range tasks {
		s.normalizeTask(&tasks[i])
	}
	return tasks, nil
}

// GetById 根据ID获取定时任务
func (s *SchedulerService) GetById(ctx context.Context, id string) (*models.ScheduledTask, error) {
	task, err := s.repo.FindById(ctx, id)
	if err != nil {
		return nil, err
	}
	s.normalizeTask(&task)
	return &task, nil
}

// Create 创建定时任务
func (s *SchedulerService) Create(ctx context.Context, task *models.ScheduledTask) error {
	s.normalizeTask(task)
	if err := s.validateTaskTarget(task); err != nil {
		return err
	}
	now := time.Now().UnixMilli()
	task.ID = uuid.New().String()
	task.CreatedAt = now
	task.UpdatedAt = now
	return s.repo.Create(ctx, task)
}

// Update 更新定时任务
func (s *SchedulerService) Update(ctx context.Context, task *models.ScheduledTask) error {
	s.normalizeTask(task)
	if err := s.validateTaskTarget(task); err != nil {
		return err
	}
	existingTask, err := s.GetById(ctx, task.ID)
	if err != nil {
		return err
	}
	existingTask.Name = task.Name
	existingTask.Enabled = task.Enabled
	existingTask.IntervalDays = task.IntervalDays
	existingTask.TaskType = task.TaskType
	existingTask.ModuleID = task.ModuleID
	existingTask.PhoneNumber = task.PhoneNumber
	existingTask.Content = task.Content
	existingTask.TrafficKB = task.TrafficKB

	return s.repo.Save(ctx, existingTask)
}

// Delete 删除定时任务
func (s *SchedulerService) Delete(ctx context.Context, id string) error {
	return s.repo.DeleteById(ctx, id)
}

// TriggerTask 立即触发执行指定的任务
func (s *SchedulerService) TriggerTask(ctx context.Context, id string) error {
	// 获取任务
	task, err := s.GetById(ctx, id)
	if err != nil {
		return fmt.Errorf("获取任务失败: %w", err)
	}

	// 执行任务
	if err := s.executeTask(ctx, *task); err != nil {
		return fmt.Errorf("执行任务失败: %w", err)
	}

	return nil
}

// ==================== 调度相关方法 ====================

// Start 启动定时任务服务
func (s *SchedulerService) Start(ctx context.Context) error {
	s.cron = cron.New()

	// 添加每天执行一次的检查任务（每天早上8点执行）
	_, err := s.cron.AddFunc("0 8 * * *", func() {
		s.logger.Info("开始检查定时任务")
		if err := s.checkAndExecuteTasks(); err != nil {
			s.logger.Error("检查并执行定时任务失败", zap.Error(err))
		}
	})
	if err != nil {
		return fmt.Errorf("添加检查任务失败: %w", err)
	}

	// 启动 cron
	s.cron.Start()

	s.logger.Info("定时任务服务启动成功")
	return nil
}

// checkAndExecuteTasks 检查并执行满足条件的任务
func (s *SchedulerService) checkAndExecuteTasks() error {
	ctx := context.Background()

	// 获取所有启用的任务
	tasks, err := s.GetAllEnabled(ctx)
	if err != nil {
		s.logger.Error("获取启用的定时任务失败", zap.Error(err))
		return err
	}

	now := time.Now()
	for _, task := range tasks {
		// 检查是否需要执行
		if s.shouldExecuteTask(task, now) {
			s.logger.Info("任务满足执行条件",
				zap.String("id", task.ID),
				zap.String("name", task.Name),
				zap.Int("intervalDays", task.IntervalDays))

			if err := s.executeTask(context.Background(), task); err != nil {
				s.logger.Error("执行定时任务失败",
					zap.String("id", task.ID),
					zap.String("name", task.Name),
					zap.Error(err))
			}
		}
	}

	return nil
}

// shouldExecuteTask 判断任务是否应该执行
func (s *SchedulerService) shouldExecuteTask(task models.ScheduledTask, now time.Time) bool {
	// 如果从未执行过，则执行
	if task.LastRunAt <= 0 {
		return true
	}

	// 计算距离上次执行的天数
	lastRun := time.UnixMilli(task.LastRunAt)
	daysSinceLastRun := int(now.Sub(lastRun).Hours() / 24)

	// 如果上次执行失败，1天后就可以重试
	if task.LastRunStatus == models.LastRunStatusFailed {
		return daysSinceLastRun >= 1
	}

	// 如果满足间隔天数条件，则执行
	return daysSinceLastRun >= task.IntervalDays
}

// executeTask 执行任务
func (s *SchedulerService) executeTask(ctx context.Context, task models.ScheduledTask) error {
	s.executionMu.Lock()
	defer s.executionMu.Unlock()

	s.normalizeTask(&task)
	s.logger.Info("执行定时任务",
		zap.String("id", task.ID),
		zap.String("name", task.Name),
		zap.String("type", string(task.TaskType)),
		zap.String("module_id", task.ModuleID))

	serialService, err := s.serialManager.GetService(task.ModuleID)
	if err != nil {
		detail := fmt.Sprintf("模块 %s 不可用：%v", task.ModuleID, err)
		_ = s.UpdateLastRun(ctx, task.ID, "", models.LastRunStatusFailed, detail)
		return err
	}
	if task.TaskType == models.ScheduledTaskTypeTraffic {
		return s.executeTrafficTask(ctx, task, serialService)
	}
	return s.executeSMSTask(ctx, task, serialService)
}

func (s *SchedulerService) executeSMSTask(ctx context.Context, task models.ScheduledTask, serialService *SerialService) error {
	detailPrefix := fmt.Sprintf("模块 %s", serialService.ModuleName())

	flyMode := serialService.FlyMode()
	// 如果是飞行模式，取消飞行模式，再等待 30 秒后发送短信
	if flyMode {
		s.logger.Info("当前为飞行模式，取消飞行模式后等待 30 秒")
		// 取消飞行模式
		if err := serialService.SetFlymode(false); err != nil {
			s.logger.Error("取消飞行模式失败", zap.Error(err))
			_ = s.UpdateLastRun(ctx, task.ID, "", models.LastRunStatusFailed, detailPrefix+"；取消飞行模式失败："+err.Error())
			return err
		}
		s.logger.Info("取消飞行模式成功")
		// 等待 30 秒
		time.Sleep(30 * time.Second)
		s.logger.Info("等待 30 秒后发送短信...")
	}

	// 发送短信
	msgId, err := serialService.SendSMS(task.PhoneNumber, task.Content)
	if err != nil {
		s.logger.Error("定时任务发送短信失败",
			zap.String("id", task.ID),
			zap.String("name", task.Name),
			zap.Error(err))
		_ = s.UpdateLastRun(ctx, task.ID, msgId, models.LastRunStatusFailed, detailPrefix+"；短信提交失败："+err.Error())
		return err
	}
	s.logger.Info("定时任务执行成功",
		zap.String("id", task.ID),
		zap.String("name", task.Name))

	// 更新任务的 LastRunAt 字段到数据库
	_ = s.UpdateLastRun(ctx, task.ID, msgId, models.LastRunStatusSuccess, detailPrefix+"；短信命令已提交")

	// 如果是飞行模式，重新设置飞行模式
	if flyMode {
		s.logger.Info("等待 30 秒后重新设置飞行模式...")
		time.Sleep(30 * time.Second)
		s.logger.Info("重新设置飞行模式")
		if err := serialService.SetFlymode(true); err != nil {
			s.logger.Error("设置飞行模式失败", zap.Error(err))
			return err
		}
		s.logger.Info("设置飞行模式成功")
	}

	return nil
}

func (s *SchedulerService) executeTrafficTask(ctx context.Context, task models.ScheduledTask, serialService trafficModule) error {
	result, err := serialService.ConsumeTraffic(ctx, task.TrafficKB, s.trafficEndpoint)
	retriedAfterRecovery := false
	if err == nil && !serialService.FlyMode() && shouldRecoverTrafficData(result) {
		failureReason := trafficFailureReason(result)
		s.saveTrafficRecord(ctx, task, serialService, result, false, failureReason+"；准备自动重建蜂窝数据连接")
		s.logger.Warn("蜂窝流量请求超时，自动重建数据连接后重试一次",
			zap.String("task_id", task.ID),
			zap.String("module_id", serialService.ModuleID()),
			zap.String("request_id", result.RequestID))
		if recoveryErr := s.recoverTrafficData(ctx, serialService); recoveryErr != nil {
			detail := fmt.Sprintf("模块 %s；%s；自动重建蜂窝数据连接失败：%v", serialService.ModuleName(), failureReason, recoveryErr)
			_ = s.UpdateLastRun(ctx, task.ID, result.RequestID, models.LastRunStatusFailed, detail)
			return fmt.Errorf("自动重建蜂窝数据连接失败: %w", recoveryErr)
		}
		retriedAfterRecovery = true
		result, err = serialService.ConsumeTraffic(ctx, task.TrafficKB, s.trafficEndpoint)
	}
	if err != nil {
		failureReason := err.Error()
		if retriedAfterRecovery {
			failureReason = "自动重建蜂窝数据连接后重试失败：" + failureReason
		}
		s.saveTrafficRecord(ctx, task, serialService, result, false, failureReason)
		detail := fmt.Sprintf("模块 %s；流量任务失败：%s", serialService.ModuleName(), failureReason)
		_ = s.UpdateLastRun(ctx, task.ID, result.RequestID, models.LastRunStatusFailed, detail)
		return err
	}

	detail := fmt.Sprintf(
		"模块 %s；HTTP %d；上行 %d B，下行 %d B，合计 %d B；响应体 %d B",
		serialService.ModuleName(), result.HTTPCode, result.UplinkBytes,
		result.DownlinkBytes, result.TotalBytes, result.BodyBytes,
	)
	if !result.Success {
		failureReason := trafficFailureReason(result)
		if retriedAfterRecovery {
			failureReason = "自动重建蜂窝数据连接后重试仍失败：" + failureReason
		}
		s.saveTrafficRecord(ctx, task, serialService, result, false, failureReason)
		detail += "；失败原因：" + failureReason
		_ = s.UpdateLastRun(ctx, task.ID, result.RequestID, models.LastRunStatusFailed, detail)
		return fmt.Errorf("流量任务失败: %s", failureReason)
	}
	if result.ConnectionOpen {
		s.saveTrafficRecord(ctx, task, serialService, result, false, "HTTP 连接未关闭")
		detail += "；失败原因：HTTP 连接未关闭"
		_ = s.UpdateLastRun(ctx, task.ID, result.RequestID, models.LastRunStatusFailed, detail)
		return fmt.Errorf("流量任务结束后 HTTP 连接仍处于打开状态")
	}
	s.saveTrafficRecord(ctx, task, serialService, result, true, "")
	if retriedAfterRecovery {
		detail += "；自动重建蜂窝数据连接后重试成功"
	}

	if err := s.UpdateLastRun(ctx, task.ID, result.RequestID, models.LastRunStatusSuccess, detail); err != nil {
		return fmt.Errorf("保存流量任务结果失败: %w", err)
	}
	return nil
}

func (s *SchedulerService) saveTrafficRecord(
	ctx context.Context,
	task models.ScheduledTask,
	serialService trafficModule,
	result TrafficResult,
	success bool,
	errorMessage string,
) {
	if s.trafficRecords == nil {
		return
	}
	record := &models.TrafficRecord{
		TaskID:           task.ID,
		TaskName:         task.Name,
		RequestID:        result.RequestID,
		ModuleID:         serialService.ModuleID(),
		ModuleName:       serialService.ModuleName(),
		TargetKB:         task.TrafficKB,
		Success:          success,
		HTTPCode:         result.HTTPCode,
		UplinkBytes:      result.UplinkBytes,
		DownlinkBytes:    result.DownlinkBytes,
		TotalBytes:       result.TotalBytes,
		BodyBytes:        result.BodyBytes,
		ConnectionClosed: result.HTTPCode > 0 && !result.ConnectionOpen,
		Error:            errorMessage,
	}
	if err := s.trafficRecords.Save(ctx, record); err != nil {
		s.logger.Error("保存流量记录失败",
			zap.String("task_id", task.ID),
			zap.String("request_id", result.RequestID),
			zap.Error(err))
	}
}

func shouldRecoverTrafficData(result TrafficResult) bool {
	return !result.Success && result.HTTPCode == -8 && result.UplinkBytes == 0 &&
		result.DownlinkBytes == 0 && result.TotalBytes == 0 && result.BodyBytes == 0
}

func trafficFailureReason(result TrafficResult) string {
	descriptions := map[int64]string{
		-1: "HTTP 底层状态异常",
		-2: "HTTP 响应头异常",
		-3: "HTTP 响应体异常",
		-4: "连接服务器失败，可能未联网或地址无效",
		-5: "连接被提前断开",
		-6: "接收数据失败",
		-7: "下载过程失败",
		-8: "连接或读取超时",
		-9: "FOTA 数据异常",
	}
	if description, ok := descriptions[result.HTTPCode]; ok {
		return fmt.Sprintf("HTTP %d（%s）", result.HTTPCode, description)
	}
	if result.Error != "" {
		return result.Error
	}
	return "模块未返回失败原因"
}

func (s *SchedulerService) recoverTrafficData(ctx context.Context, serialService trafficModule) (err error) {
	if err = serialService.SetFlymode(true); err != nil {
		return fmt.Errorf("启用飞行模式失败: %w", err)
	}
	flymodeEnabled := true
	defer func() {
		if flymodeEnabled {
			if disableErr := serialService.SetFlymode(false); err == nil && disableErr != nil {
				err = fmt.Errorf("关闭飞行模式失败: %w", disableErr)
			}
		}
	}()

	if err = s.trafficRecoveryWait(ctx, trafficFlightModeHold); err != nil {
		return err
	}
	if err = serialService.SetFlymode(false); err != nil {
		return fmt.Errorf("关闭飞行模式失败: %w", err)
	}
	flymodeEnabled = false
	if err = s.trafficRecoveryWait(ctx, trafficNetworkReadyWait); err != nil {
		return err
	}
	return nil
}

func waitForTrafficRecovery(ctx context.Context, duration time.Duration) error {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return fmt.Errorf("等待蜂窝数据连接恢复被取消: %w", ctx.Err())
	case <-timer.C:
		return nil
	}
}

func (s *SchedulerService) normalizeTask(task *models.ScheduledTask) {
	if task.TaskType == "" {
		task.TaskType = models.ScheduledTaskTypeSMS
	}
	if task.ModuleID == "" {
		if defaultService := s.serialManager.DefaultService(); defaultService != nil {
			task.ModuleID = defaultService.ModuleID()
		}
	}
	if task.TaskType == models.ScheduledTaskTypeTraffic {
		task.TrafficKB = models.FixedTrafficKB
	} else if task.TrafficKB <= 0 {
		task.TrafficKB = models.FixedTrafficKB
	}
}

func (s *SchedulerService) validateTaskTarget(task *models.ScheduledTask) error {
	if task.TaskType != models.ScheduledTaskTypeSMS && task.TaskType != models.ScheduledTaskTypeTraffic {
		return fmt.Errorf("不支持的任务类型: %s", task.TaskType)
	}
	if _, err := s.serialManager.GetService(task.ModuleID); err != nil {
		return err
	}
	return nil
}

func (s *SchedulerService) UpdateLastRun(ctx context.Context, id, msgId string, status models.LastRunStatus, detail string) error {
	return s.repo.UpdateColumnsById(ctx, id, orz.Map{
		"last_msg_id":     msgId,
		"last_run_at":     time.Now().UnixMilli(),
		"last_run_status": status,
		"last_run_detail": detail,
	})
}

func (s *SchedulerService) UpdateLastRunStatusByMsgId(ctx context.Context, msgId string, status models.LastRunStatus) error {
	return s.repo.UpdateLastRunStatusByMsgId(ctx, msgId, status)
}
