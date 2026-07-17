package handler

import (
	"net/http"
	"strconv"

	"github.com/dushixiang/uart_sms_forwarder/internal/service"
	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
)

type TrafficRecordHandler struct {
	logger  *zap.Logger
	service *service.TrafficRecordService
}

func NewTrafficRecordHandler(logger *zap.Logger, service *service.TrafficRecordService) *TrafficRecordHandler {
	return &TrafficRecordHandler{logger: logger, service: service}
}

func (h *TrafficRecordHandler) List(c echo.Context) error {
	limit := 20
	if rawLimit := c.QueryParam("limit"); rawLimit != "" {
		parsedLimit, err := strconv.Atoi(rawLimit)
		if err != nil || parsedLimit < 1 {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "limit 必须是正整数"})
		}
		limit = parsedLimit
	}

	records, err := h.service.List(c.Request().Context(), c.QueryParam("moduleId"), limit)
	if err != nil {
		h.logger.Error("获取流量记录失败", zap.Error(err))
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "获取流量记录失败"})
	}
	return c.JSON(http.StatusOK, records)
}
