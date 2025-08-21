// internal/handlers/analysis_handler.go
package handlers

import (
	"context"
	"log"
	"net/http"
	"tagscale/internal/services"
	"time"

	"github.com/gin-gonic/gin"
)

type AnalysisHandler struct {
	analysisService *services.AnalysisService
}

func NewAnalysisHandler(analysisService *services.AnalysisService) *AnalysisHandler {
	return &AnalysisHandler{analysisService: analysisService}
}

func (h *AnalysisHandler) GetLatestAnalysis(c *gin.Context) {
	result, err := h.analysisService.GetLatestAnalysis()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": result})
}

func (h *AnalysisHandler) RunAnalysis(c *gin.Context) {
	ctx := c.Request.Context()
	go func(ctx context.Context) {
		end := time.Now()
		start := end.AddDate(0, 0, -30)
		groups := []string{"service", "account", "region", "team"}
		if _, err := h.analysisService.RunAnalysis(ctx, 5, start, end, groups, true); err != nil {
			log.Printf("analysis run failed: %v", err)
		}
	}(ctx)

	c.JSON(http.StatusOK, gin.H{"message": "Analysis started"})
}
