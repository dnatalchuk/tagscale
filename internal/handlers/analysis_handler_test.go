package handlers_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"tagscale/internal/handlers"
	"tagscale/internal/models"
	"tagscale/internal/services"
)

func setupService(t *testing.T, withData bool) *services.AnalysisService {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	require.NoError(t, db.AutoMigrate(&models.CostAnalysis{}))

	if withData {
		analysis := models.CostAnalysis{
			Date:            time.Now(),
			TotalCost:       123.45,
			UntaggedCost:    23.45,
			UntaggedPercent: 19,
			TopServices:     "[]",
			TopAccounts:     "[]",
			TopRegions:      "[]",
			CostByTeam:      "[]",
			Insights:        "[]",
		}
		require.NoError(t, db.Create(&analysis).Error)
	}

	return services.NewAnalysisService(db)
}

func TestGetLatestAnalysisSuccess(t *testing.T) {
	service := setupService(t, true)
	handler := handlers.NewAnalysisHandler(service)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/analysis/latest", handler.GetLatestAnalysis)

	req, _ := http.NewRequest(http.MethodGet, "/analysis/latest", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var resp struct {
		Data services.AnalysisResult `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.InDelta(t, 123.45, resp.Data.TotalCost, 0.001)
	require.InDelta(t, 23.45, resp.Data.UntaggedCost, 0.001)
}

func TestGetLatestAnalysisError(t *testing.T) {
	service := setupService(t, false)
	handler := handlers.NewAnalysisHandler(service)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/analysis/latest", handler.GetLatestAnalysis)

	req, _ := http.NewRequest(http.MethodGet, "/analysis/latest", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusInternalServerError, w.Code)
}
