package service

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/dushixiang/uart_sms_forwarder/internal/models"
	"gorm.io/gorm"
)

var (
	ErrInvalidCallForwarding = errors.New("无应答转移参数无效")
	forwardingNumberPattern  = regexp.MustCompile(`^\+?[0-9]{3,20}$`)
	allowedForwardingDelays  = map[int]struct{}{5: {}, 10: {}, 15: {}, 20: {}, 25: {}, 30: {}}
)

type CallForwardingInput struct {
	Enabled      bool   `json:"enabled"`
	Number       string `json:"number"`
	DelaySeconds int    `json:"delaySeconds"`
}

type CallForwardingCommander interface {
	ConfigureCallForwarding(context.Context, CallForwardingInput) (CallForwardingResult, error)
}

type CallForwardingCommandError struct {
	Err error
}

func (e *CallForwardingCommandError) Error() string {
	return e.Err.Error()
}

func (e *CallForwardingCommandError) Unwrap() error {
	return e.Err
}

type CallForwardingService struct {
	db *gorm.DB
}

func NewCallForwardingService(db *gorm.DB) *CallForwardingService {
	return &CallForwardingService{db: db}
}

func validateCallForwardingInput(input CallForwardingInput) (CallForwardingInput, error) {
	input.Number = strings.TrimSpace(input.Number)
	if _, ok := allowedForwardingDelays[input.DelaySeconds]; !ok {
		return input, fmt.Errorf("%w: 延时只能是 5、10、15、20、25 或 30 秒", ErrInvalidCallForwarding)
	}
	if input.Number != "" && !forwardingNumberPattern.MatchString(input.Number) {
		return input, fmt.Errorf("%w: 号码只能包含可选的 + 和 3 至 20 位数字", ErrInvalidCallForwarding)
	}
	if input.Enabled && input.Number == "" {
		return input, fmt.Errorf("%w: 开启时必须填写转移号码", ErrInvalidCallForwarding)
	}
	return input, nil
}

func (s *CallForwardingService) Get(
	ctx context.Context,
	moduleID string,
	moduleName string,
) (*models.CallForwardingConfig, error) {
	var config models.CallForwardingConfig
	err := s.db.WithContext(ctx).First(&config, "module_id = ?", moduleID).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return &models.CallForwardingConfig{
			ModuleID:     moduleID,
			ModuleName:   moduleName,
			DelaySeconds: 20,
		}, nil
	}
	if err != nil {
		return nil, err
	}
	if config.ModuleName != moduleName && moduleName != "" {
		config.ModuleName = moduleName
	}
	if config.DelaySeconds == 0 {
		config.DelaySeconds = 20
	}
	return &config, nil
}

func (s *CallForwardingService) Apply(
	ctx context.Context,
	moduleID string,
	moduleName string,
	input CallForwardingInput,
	commander CallForwardingCommander,
) (*models.CallForwardingConfig, error) {
	normalized, err := validateCallForwardingInput(input)
	if err != nil {
		return nil, err
	}

	config := &models.CallForwardingConfig{
		ModuleID:     moduleID,
		ModuleName:   moduleName,
		Enabled:      normalized.Enabled,
		Number:       normalized.Number,
		DelaySeconds: normalized.DelaySeconds,
		UpdatedAt:    time.Now().UnixMilli(),
	}
	result, commandErr := commander.ConfigureCallForwarding(ctx, normalized)
	if commandErr != nil {
		config.LastStatus = models.CallForwardingStatusFailed
		config.LastError = commandErr.Error()
	} else if !result.Success {
		config.LastStatus = models.CallForwardingStatusFailed
		config.LastError = result.Error
		if config.LastError == "" {
			config.LastError = "模块未返回失败原因"
		}
		commandErr = errors.New(config.LastError)
	} else {
		config.LastStatus = models.CallForwardingStatusSubmitted
	}

	if err := s.db.WithContext(ctx).Save(config).Error; err != nil {
		return config, err
	}
	if commandErr != nil {
		return config, &CallForwardingCommandError{Err: commandErr}
	}
	return config, nil
}
