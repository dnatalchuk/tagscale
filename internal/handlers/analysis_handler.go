// internal/handlers/analysis_handler.go
package handlers

import (
	"net/http"
	"tagscale/internal/services"

	"github.com/gin-gonic/gin"
)

type AnalysisHandler struct {
	analysisService *services.AnalysisService
}

func NewAnalysisHandler(analysisService *services.AnalysisService) *AnalysisHandler {
	return &AnalysisHandler{analysisService: analysisService}
}

func (h *AnalysisHandler) GetLatestAnalysis(c *gin.Context) {
	// This would get the latest analysis from the database
	c.JSON(http.StatusOK, gin.H{"message": "Latest analysis endpoint"})
}

func (h *AnalysisHandler) RunAnalysis(c *gin.Context) {
	go func() {
		h.analysisService.RunAnalysis()
	}()

	c.JSON(http.StatusOK, gin.H{"message": "Analysis started"})
}
