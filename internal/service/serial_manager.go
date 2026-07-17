package service

import (
	"context"
	"fmt"

	"github.com/dushixiang/uart_sms_forwarder/config"
	"go.uber.org/zap"
)

type SerialModuleInfo struct {
	ID        string      `json:"id"`
	Name      string      `json:"name"`
	Port      string      `json:"port"`
	Default   bool        `json:"default"`
	Disabled  bool        `json:"disabled"`
	Status    *StatusData `json:"status,omitempty"`
	StatusErr string      `json:"status_error,omitempty"`
}

type serialModule struct {
	config  config.ModuleConfig
	service *SerialService
}

type SerialManager struct {
	logger    *zap.Logger
	modules   map[string]*serialModule
	order     []string
	defaultID string
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
		logger:  logger,
		modules: make(map[string]*serialModule),
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
	result := make([]SerialModuleInfo, 0, len(m.order))
	for _, id := range m.order {
		module := m.modules[id]
		if module == nil {
			continue
		}
		info := SerialModuleInfo{
			ID:       module.config.ID,
			Name:     module.config.Name,
			Port:     module.config.Port,
			Default:  id == m.defaultID,
			Disabled: module.config.Disabled,
		}
		if module.service != nil {
			status, err := module.service.GetStatus()
			if err != nil {
				info.StatusErr = err.Error()
			} else {
				info.Status = status
			}
		}
		result = append(result, info)
	}
	return result
}
