package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"tagscale/internal/aws"
	"tagscale/internal/config"
	"tagscale/internal/database"
	"tagscale/internal/server"
	"tagscale/internal/services"
)

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatal("Failed to load configuration:", err)
	}

	// Initialize database
	db, err := database.Connect(cfg.DatabaseURL)
	if err != nil {
		log.Fatal("Failed to connect to database:", err)
	}

	// Run migrations
	if err := database.Migrate(db); err != nil {
		log.Fatal("Failed to run migrations:", err)
	}

	// Initialize AWS client
	awsClient, err := aws.NewClient(cfg.AWSRegion)
	if err != nil {
		log.Fatal("Failed to initialize AWS client:", err)
	}

	// Initialize services
	costService := services.NewCostService(awsClient, db)
	analysisService := services.NewAnalysisService(db)
	notificationService := services.NewNotificationService(cfg)

	// Initialize server
	srv := server.New(cfg, costService, analysisService, notificationService)

	// Start background workers
	go startBackgroundWorkers(costService, analysisService, notificationService, cfg)

	// Start server
	httpServer := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.Port),
		Handler: srv.Router(),
	}

	go func() {
		log.Printf("Server starting on port %d", cfg.Port)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("Server failed to start:", err)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Server shutting down...")

	// Graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := httpServer.Shutdown(ctx); err != nil {
		log.Fatal("Server forced to shutdown:", err)
	}

	log.Println("Server exited")
}

func startBackgroundWorkers(costService *services.CostService, analysisService *services.AnalysisService, notificationService *services.NotificationService, cfg *config.Config) {
	// Data collection worker
	go func() {
		ticker := time.NewTicker(time.Duration(cfg.DataCollectionInterval) * time.Minute)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				log.Println("Starting cost data collection...")
				if err := costService.CollectCostData(); err != nil {
					log.Printf("Cost data collection failed: %v", err)
				}
			}
		}
	}()

	// Analysis worker
	go func() {
		ticker := time.NewTicker(time.Duration(cfg.AnalysisInterval) * time.Minute)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				log.Println("Starting cost analysis...")
				if err := analysisService.RunAnalysis(); err != nil {
					log.Printf("Cost analysis failed: %v", err)
				}
			}
		}
	}()

	// Notification worker
	go func() {
		ticker := time.NewTicker(24 * time.Hour) // Daily notifications
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				log.Println("Sending daily notifications...")
				if err := notificationService.SendDailyDigest(); err != nil {
					log.Printf("Notification sending failed: %v", err)
				}
			}
		}
	}()
}
