// internal/handlers/cost_handler.go
package handlers

import (
	"net/http"
	"strconv"
	"time"

	"tagscale/internal/services"

	"github.com/gin-gonic/gin"
)

type CostHandler struct {
	costService *services.CostService
}

func NewCostHandler(costService *services.CostService) *CostHandler {
	return &CostHandler{costService: costService}
}

func (h *CostHandler) GetCostSummary(c *gin.Context) {
	startDateStr := c.Query("start_date")
	endDateStr := c.Query("end_date")
	groupBy := c.DefaultQuery("group_by", "service")

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

	summary, err := h.costService.GetCostSummary(startDate, endDate, groupBy)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": summary})
}

func (h *CostHandler) GetTopCosts(c *gin.Context) {
	limitStr := c.DefaultQuery("limit", "10")
	groupBy := c.DefaultQuery("group_by", "service")

	limit, err := strconv.Atoi(limitStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid limit parameter"})
		return
	}

	topCosts, err := h.costService.GetTopCosts(limit, groupBy)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": topCosts})
}

func (h *CostHandler) CollectCosts(c *gin.Context) {
	daysStr := c.DefaultQuery("days", "30")
	days, err := strconv.Atoi(daysStr)
	if err != nil {
		days = 30
	}

	go func() {
		h.costService.CollectCostData(days)
	}()

	c.JSON(http.StatusOK, gin.H{"message": "Cost collection started"})
}
