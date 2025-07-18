package handlers_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"tagscale/internal/handlers"
	"tagscale/internal/models"
	"tagscale/internal/services"
)

func setupCostService(t *testing.T) *services.CostService {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.CostRecord{}))
	return services.NewCostService(nil, db)
}

func TestGetCostSummaryInvalidGroupBy(t *testing.T) {
	service := setupCostService(t)
	handler := handlers.NewCostHandler(service)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/costs/summary", handler.GetCostSummary)

	req, _ := http.NewRequest(http.MethodGet, "/costs/summary?start_date=2023-01-01&end_date=2023-01-02&group_by=foo", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusBadRequest, w.Code)
}

func TestGetTopCostsInvalidGroupBy(t *testing.T) {
	service := setupCostService(t)
	handler := handlers.NewCostHandler(service)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/costs/top", handler.GetTopCosts)

	req, _ := http.NewRequest(http.MethodGet, "/costs/top?limit=5&group_by=foo", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusBadRequest, w.Code)
}
