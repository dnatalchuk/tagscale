package handlers

import (
	"net/http"
	"time"

	"tagscale/internal/models"
	"tagscale/internal/services"

	"github.com/gin-gonic/gin"
	"golang.org/x/sync/errgroup"
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
	g, ctx := errgroup.WithContext(c.Request.Context())

	var (
		serviceSummary []models.CostSummary
		accountSummary []models.CostSummary
		regionSummary  []models.CostSummary
	)

	g.Go(func() error {
		var err error
		serviceSummary, err = h.costService.GetCostSummary(ctx, startDate, endDate, "service")
		return err
	})

	g.Go(func() error {
		var err error
		accountSummary, err = h.costService.GetCostSummary(ctx, startDate, endDate, "account")
		return err
	})

	g.Go(func() error {
		var err error
		regionSummary, err = h.costService.GetCostSummary(ctx, startDate, endDate, "region")
		return err
	})

	if err := g.Wait(); err != nil {
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

	trends, err := h.costService.GetCostSummary(c.Request.Context(), startDate, endDate, "date")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"trends": trends})
}
