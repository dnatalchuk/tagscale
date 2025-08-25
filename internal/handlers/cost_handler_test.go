package handlers_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/costexplorer"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"tagscale/internal/config"
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
	handler := handlers.NewCostHandler(service, &config.Config{AWSRequestTimeout: 30, CostBatchSize: 100})

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
	handler := handlers.NewCostHandler(service, &config.Config{AWSRequestTimeout: 30, CostBatchSize: 100})

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/costs/top", handler.GetTopCosts)

	req, _ := http.NewRequest(http.MethodGet, "/costs/top?limit=5&group_by=foo", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusBadRequest, w.Code)
}

func TestGetTopCostsZeroLimit(t *testing.T) {
	service := setupCostService(t)
	handler := handlers.NewCostHandler(service, &config.Config{AWSRequestTimeout: 30, CostBatchSize: 100})

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/costs/top", handler.GetTopCosts)

	req, _ := http.NewRequest(http.MethodGet, "/costs/top?limit=0&group_by=service", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusBadRequest, w.Code)
}

func TestGetTopCostsNegativeLimit(t *testing.T) {
	service := setupCostService(t)
	handler := handlers.NewCostHandler(service, &config.Config{AWSRequestTimeout: 30, CostBatchSize: 100})

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/costs/top", handler.GetTopCosts)

	req, _ := http.NewRequest(http.MethodGet, "/costs/top?limit=-5&group_by=service", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCollectCostsInvalidDays(t *testing.T) {
	service := setupCostService(t)
	handler := handlers.NewCostHandler(service, &config.Config{AWSRequestTimeout: 30, CostBatchSize: 100})

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/costs/collect", handler.CollectCosts)

	req, _ := http.NewRequest(http.MethodPost, "/costs/collect?days=abc", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCollectCostsNegativeDays(t *testing.T) {
	service := setupCostService(t)
	handler := handlers.NewCostHandler(service, &config.Config{AWSRequestTimeout: 30, CostBatchSize: 100})

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/costs/collect", handler.CollectCosts)

	req, _ := http.NewRequest(http.MethodPost, "/costs/collect?days=-5", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusBadRequest, w.Code)
}

type errorCostExplorer struct{}

func (e *errorCostExplorer) GetCostAndUsage(ctx context.Context, startDate, endDate time.Time, nextToken *string) (*costexplorer.GetCostAndUsageOutput, error) {
	return nil, errors.New("boom")
}

func TestCollectCostsErrorPropagation(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	service := services.NewCostService(&errorCostExplorer{}, db)
	handler := handlers.NewCostHandler(service, &config.Config{AWSRequestTimeout: 30, CostBatchSize: 100})

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/costs/collect", handler.CollectCosts)

	req, _ := http.NewRequest(http.MethodPost, "/costs/collect?days=1", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusInternalServerError, w.Code)
}
