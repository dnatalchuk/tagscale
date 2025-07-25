// internal/server/server.go
package server

import (
	"net/http"
	"tagscale/internal/config"
	"tagscale/internal/handlers"
	"tagscale/internal/middleware"
	"tagscale/internal/services"

	"github.com/gin-gonic/gin"
	"github.com/rs/cors"
)

type Server struct {
	config              *config.Config
	costService         *services.CostService
	analysisService     *services.AnalysisService
	notificationService *services.NotificationService
}

func New(config *config.Config, costService *services.CostService, analysisService *services.AnalysisService, notificationService *services.NotificationService) *Server {
	return &Server{
		config:              config,
		costService:         costService,
		analysisService:     analysisService,
		notificationService: notificationService,
	}
}

func (s *Server) Router() http.Handler {
	r := gin.New()
	r.Use(middleware.LoggingMiddleware())
	r.Use(gin.Recovery())

	// Initialize handlers
	costHandler := handlers.NewCostHandler(s.costService)
	analysisHandler := handlers.NewAnalysisHandler(s.analysisService)
	dashboardHandler := handlers.NewDashboardHandler(s.costService, s.analysisService)

	// API routes
	api := r.Group("/api/v1", middleware.AuthMiddleware(s.config.APIKey))
	{
		api.GET("/costs/summary", costHandler.GetCostSummary)
		api.GET("/costs/top", costHandler.GetTopCosts)
		api.POST("/costs/collect", costHandler.CollectCosts)

		api.GET("/analysis/latest", analysisHandler.GetLatestAnalysis)
		api.POST("/analysis/run", analysisHandler.RunAnalysis)

		api.GET("/dashboard/overview", dashboardHandler.GetOverview)
		api.GET("/dashboard/trends", dashboardHandler.GetTrends)
	}

	// Health check
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// CORS middleware
	c := cors.New(cors.Options{
		AllowedOrigins:   s.config.AllowedOrigins,
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"*"},
		AllowCredentials: true,
	})

	return c.Handler(r)
}
