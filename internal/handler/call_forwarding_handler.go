package handler

import (
	"errors"
	"net/http"

	"github.com/dushixiang/uart_sms_forwarder/internal/service"
	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
)

type CallForwardingHandler struct {
	logger        *zap.Logger
	service       *service.CallForwardingService
	serialManager *service.SerialManager
}

func NewCallForwardingHandler(
	logger *zap.Logger,
	callForwardingService *service.CallForwardingService,
	serialManager *service.SerialManager,
) *CallForwardingHandler {
	return &CallForwardingHandler{
		logger:        logger,
		service:       callForwardingService,
		serialManager: serialManager,
	}
}

func (h *CallForwardingHandler) Get(c echo.Context) error {
	serialService, err := h.serialManager.GetService(c.Param("moduleId"))
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": err.Error()})
	}
	config, err := h.service.Get(
		c.Request().Context(),
		serialService.ModuleID(),
		serialService.ModuleName(),
	)
	if err != nil {
		h.logger.Error("获取无应答转移配置失败", zap.Error(err))
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "获取无应答转移配置失败"})
	}
	return c.JSON(http.StatusOK, config)
}

func (h *CallForwardingHandler) Update(c echo.Context) error {
	serialService, err := h.serialManager.GetService(c.Param("moduleId"))
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": err.Error()})
	}

	var input service.CallForwardingInput
	if err := c.Bind(&input); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "请求参数错误"})
	}
	config, err := h.service.Apply(
		c.Request().Context(),
		serialService.ModuleID(),
		serialService.ModuleName(),
		input,
		serialService,
	)
	if err == nil {
		return c.JSON(http.StatusOK, config)
	}
	if errors.Is(err, service.ErrInvalidCallForwarding) {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	var commandErr *service.CallForwardingCommandError
	if errors.As(err, &commandErr) {
		h.logger.Warn("提交无应答转移失败", zap.String("module_id", serialService.ModuleID()), zap.Error(err))
		return c.JSON(http.StatusBadGateway, map[string]any{"error": err.Error(), "config": config})
	}

	h.logger.Error("保存无应答转移配置失败", zap.Error(err))
	return c.JSON(http.StatusInternalServerError, map[string]string{"error": "保存无应答转移配置失败"})
}
