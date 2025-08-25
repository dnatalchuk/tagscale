// internal/handlers/cost_handler.go
package handlers

import (
	"net/http"
	"strconv"
	"time"

	"tagscale/internal/config"
	"tagscale/internal/services"

	"github.com/gin-gonic/gin"
)

type CostHandler struct {
	costService *services.CostService
	config      *config.Config
}

func NewCostHandler(costService *services.CostService, cfg *config.Config) *CostHandler {
	return &CostHandler{costService: costService, config: cfg}
}

func (h *CostHandler) GetCostSummary(c *gin.Context) {
	startDateStr := c.Query("start_date")
	endDateStr := c.Query("end_date")
	groupBy := c.DefaultQuery("group_by", "service")

	if !services.IsGroupByAllowed(groupBy) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid group_by parameter"})
		return
	}

	startDate, err := time.Parse("2006-01-02", startDateStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid start_date format"})
		return
	}

	endDate, err := time.Parse("2006-01-02", endDateStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid end_date format"})
		return
	}

	summary, err := h.costService.GetCostSummary(c.Request.Context(), startDate, endDate, groupBy)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": summary})
}

func (h *CostHandler) GetTopCosts(c *gin.Context) {
	limitStr := c.DefaultQuery("limit", "10")
	groupBy := c.DefaultQuery("group_by", "service")

	if !services.IsGroupByAllowed(groupBy) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid group_by parameter"})
		return
	}

	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid limit parameter"})
		return
	}

	topCosts, err := h.costService.GetTopCosts(c.Request.Context(), limit, groupBy)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": topCosts})
}

func (h *CostHandler) CollectCosts(c *gin.Context) {
	daysStr := c.DefaultQuery("days", "30")
	days, err := strconv.Atoi(daysStr)
	if err != nil || days <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid days parameter"})
		return
	}

	endDate := time.Now()
	startDate := endDate.AddDate(0, 0, -days)
	timeout := time.Duration(h.config.AWSRequestTimeout) * time.Second

	if err := h.costService.CollectCostData(c.Request.Context(), startDate, endDate, timeout, h.config.CostBatchSize); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Cost collection started"})
}
