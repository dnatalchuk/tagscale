package handlers

import (
	"net/http"
	"time"

	"tagscale/internal/services"

	"github.com/gin-gonic/gin"
)

type DashboardHandler struct {
	costService     *services.CostService
	analysisService *services.AnalysisService
}

func NewDashboardHandler(costService *services.CostService, analysisService *services.AnalysisService) *DashboardHandler {
	return &DashboardHandler{
		costService:     costService,
		analysisService: analysisService,
	}
}

func (h *DashboardHandler) GetOverview(c *gin.Context) {
	// Get cost summary for the last 30 days
	endDate := time.Now()
	startDate := endDate.AddDate(0, 0, -30)

	serviceSummary, err := h.costService.GetCostSummary(startDate, endDate, "service")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	accountSummary, err := h.costService.GetCostSummary(startDate, endDate, "account")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	regionSummary, err := h.costService.GetCostSummary(startDate, endDate, "region")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"services": serviceSummary,
		"accounts": accountSummary,
		"regions":  regionSummary,
	})
}

func (h *DashboardHandler) GetTrends(c *gin.Context) {
	// Get daily cost trends for the last 30 days
	endDate := time.Now()
	startDate := endDate.AddDate(0, 0, -30)

	trends, err := h.costService.GetCostSummary(startDate, endDate, "date")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"trends": trends})
}
