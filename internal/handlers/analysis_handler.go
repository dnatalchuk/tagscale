// internal/handlers/analysis_handler.go
package handlers

import (
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
	go func() {
		end := time.Now()
		start := end.AddDate(0, 0, -30)
		if _, err := h.analysisService.RunAnalysis(5, start, end, true); err != nil {
			log.Printf("analysis run failed: %v", err)
		}
	}()

	c.JSON(http.StatusOK, gin.H{"message": "Analysis started"})
}
