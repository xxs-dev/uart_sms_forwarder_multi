package handler

import (
	"net/http"
	"strconv"

	"github.com/dushixiang/uart_sms_forwarder/internal/service"
	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
)

type CallRecordHandler struct {
	logger  *zap.Logger
	service *service.CallRecordService
}

func NewCallRecordHandler(logger *zap.Logger, service *service.CallRecordService) *CallRecordHandler {
	return &CallRecordHandler{logger: logger, service: service}
}

func (h *CallRecordHandler) List(c echo.Context) error {
	limit := 50
	if rawLimit := c.QueryParam("limit"); rawLimit != "" {
		parsedLimit, err := strconv.Atoi(rawLimit)
		if err != nil || parsedLimit < 1 {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "limit 必须是正整数"})
		}
		limit = parsedLimit
	}

	records, err := h.service.List(c.Request().Context(), c.QueryParam("moduleId"), limit)
	if err != nil {
		h.logger.Error("获取来电记录失败", zap.Error(err))
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "获取来电记录失败"})
	}
	return c.JSON(http.StatusOK, records)
}
