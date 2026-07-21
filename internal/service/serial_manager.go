package service

import (
	"context"
	"fmt"

	"github.com/dushixiang/uart_sms_forwarder/config"
	"github.com/dushixiang/uart_sms_forwarder/internal/models"
	"go.uber.org/zap"
)

type SerialModuleInfo struct {
	ID          string      `json:"id"`
	Name        string      `json:"name"`
	Alias       string      `json:"alias"`
	PhoneNumber string      `json:"phoneNumber"`
	Port        string      `json:"port"`
	Default     bool        `json:"default"`
	Disabled    bool        `json:"disabled"`
	Status      *StatusData `json:"status,omitempty"`
	StatusErr   string      `json:"status_error,omitempty"`
}

type serialModule struct {
	config  config.ModuleConfig
	service *SerialService
}

type SerialManager struct {
	logger     *zap.Logger
	modules    map[string]*serialModule
	order      []string
	defaultID  string
	properties *PropertyService
}

func NormalizeModuleConfigs(appConfig config.AppConfig) []config.ModuleConfig {
	if len(appConfig.Modules) > 0 {
		modules := make([]config.ModuleConfig, 0, len(appConfig.Modules))
		for i, module := range appConfig.Modules {
			if module.ID == "" {
				module.ID = fmt.Sprintf("module-%d", i+1)
			}
			if module.Name == "" {
				module.Name = module.ID
			}
			modules = append(modules, module)
		}
		return modules
	}

	return []config.ModuleConfig{
		{
			ID:   "default",
			Name: "默认模块",
			Port: appConfig.Serial.Port,
		},
	}
}

func NewSerialManager(
	logger *zap.Logger,
	modules []config.ModuleConfig,
	textMsgService *TextMessageService,
	notifier *Notifier,
	propertyService *PropertyService,
) *SerialManager {
	manager := &SerialManager{
		logger:     logger,
		modules:    make(map[string]*serialModule),
		properties: propertyService,
	}

	for _, module := range modules {
		if module.Disabled {
			manager.addDisabledModule(module)
			continue
		}

		serialConfig := config.SerialConfig{Port: module.Port}
		serialService := NewSerialService(logger, serialConfig, textMsgService, notifier, propertyService)
		serialService.SetModuleInfo(module.ID, module.Name)

		manager.modules[module.ID] = &serialModule{
			config:  module,
			service: serialService,
		}
		manager.order = append(manager.order, module.ID)
		if manager.defaultID == "" {
			manager.defaultID = module.ID
		}
	}

	if manager.defaultID == "" {
		fallback := config.ModuleConfig{ID: "default", Name: "默认模块"}
		serialService := NewSerialService(logger, config.SerialConfig{}, textMsgService, notifier, propertyService)
		serialService.SetModuleInfo(fallback.ID, fallback.Name)
		manager.modules[fallback.ID] = &serialModule{config: fallback, service: serialService}
		manager.order = append(manager.order, fallback.ID)
		manager.defaultID = fallback.ID
	}

	return manager
}

func (m *SerialManager) addDisabledModule(module config.ModuleConfig) {
	m.modules[module.ID] = &serialModule{config: module}
	m.order = append(m.order, module.ID)
}

func (m *SerialManager) Start() {
	for _, id := range m.order {
		module := m.modules[id]
		if module == nil || module.service == nil {
			continue
		}
		go module.service.Start()
	}
}

func (m *SerialManager) SetScheduledTaskStatusUpdater(updater ScheduledTaskStatusUpdater) {
	for _, module := range m.modules {
		if module.service == nil {
			continue
		}
		module.service.SetScheduledTaskStatusUpdater(updater)
	}
}

func (m *SerialManager) SetCallRecordService(callRecords *CallRecordService) {
	for _, module := range m.modules {
		if module.service == nil {
			continue
		}
		module.service.SetCallRecordService(callRecords)
	}
}

func (m *SerialManager) DefaultService() *SerialService {
	service, _ := m.GetService("")
	return service
}

func (m *SerialManager) GetService(moduleID string) (*SerialService, error) {
	if moduleID == "" {
		moduleID = m.defaultID
	}
	module, ok := m.modules[moduleID]
	if !ok {
		return nil, fmt.Errorf("模块不存在: %s", moduleID)
	}
	if module.service == nil {
		return nil, fmt.Errorf("模块已禁用: %s", moduleID)
	}
	return module.service, nil
}

func (m *SerialManager) ListModules(ctx context.Context) []SerialModuleInfo {
	identities := make(map[string]models.ModuleIdentity)
	if m.properties != nil {
		var err error
		identities, err = m.properties.GetModuleIdentities(ctx)
		if err != nil {
			m.logger.Warn("获取 SIM 卡资料失败", zap.Error(err))
			identities = make(map[string]models.ModuleIdentity)
		}
	}

	result := make([]SerialModuleInfo, 0, len(m.order))
	for _, id := range m.order {
		module := m.modules[id]
		if module == nil {
			continue
		}
		identity := identities[id]
		info := SerialModuleInfo{
			ID:          module.config.ID,
			Name:        module.config.Name,
			Alias:       identity.Alias,
			PhoneNumber: identity.PhoneNumber,
			Port:        module.config.Port,
			Default:     id == m.defaultID,
			Disabled:    module.config.Disabled,
		}
		if module.service != nil {
			status, err := module.service.GetStatus()
			if err != nil {
				info.StatusErr = err.Error()
			} else {
				info.Status = status
				if info.PhoneNumber == "" {
					info.PhoneNumber = status.Mobile.Number
				}
			}
		}
		result = append(result, info)
	}
	return result
}

func (m *SerialManager) GetModuleIdentity(ctx context.Context, moduleID string) (models.ModuleIdentity, error) {
	if _, ok := m.modules[moduleID]; !ok {
		return models.ModuleIdentity{}, fmt.Errorf("模块不存在: %s", moduleID)
	}
	if m.properties == nil {
		return models.ModuleIdentity{}, nil
	}
	return m.properties.GetModuleIdentity(ctx, moduleID)
}

func (m *SerialManager) SetModuleIdentity(ctx context.Context, moduleID string, identity models.ModuleIdentity) error {
	if _, ok := m.modules[moduleID]; !ok {
		return fmt.Errorf("模块不存在: %s", moduleID)
	}
	if m.properties == nil {
		return fmt.Errorf("SIM 卡资料服务不可用")
	}
	return m.properties.SetModuleIdentity(ctx, moduleID, identity)
}
