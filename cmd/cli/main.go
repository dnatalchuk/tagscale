package main

import (
	"fmt"
	"log"

	"github.com/spf13/cobra"
	"tagscale/internal/aws"
	"tagscale/internal/config"
	"tagscale/internal/database"
	"tagscale/internal/services"
)

func main() {
	if err := NewCLI().Execute(); err != nil {
		log.Fatalf("command failed: %v", err)
	}
}

func NewCLI() *cobra.Command {
	var days int

	rootCmd := &cobra.Command{
		Use:   "tagscale",
		Short: "TagScale CLI - Cloud cost insights in your terminal",
	}

	// SCAN command
	scanCmd := &cobra.Command{
		Use:   "scan",
		Short: "Scan AWS cost and store data into DB",
		Run: func(cmd *cobra.Command, args []string) {
			runScan(days)
		},
	}

	scanCmd.Flags().IntVar(&days, "days", 30, "Number of days to look back")

	// SUMMARY command
	summaryCmd := &cobra.Command{
		Use:   "summary",
		Short: "Show cost summary in terminal",
		Run: func(cmd *cobra.Command, args []string) {
			runSummary()
		},
	}

	rootCmd.AddCommand(scanCmd)
	rootCmd.AddCommand(summaryCmd)

	return rootCmd
}

func runScan(days int) {
	fmt.Println("🔍 Running TagScale scan...")

	// Load config
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("❌ Failed to load config: %v", err)
	}

	// Connect DB
	db, err := database.Connect(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("❌ Failed to connect to DB: %v", err)
	}

	// Initialize AWS client
	awsClient, err := aws.NewClient(cfg.AWSRegion)
	if err != nil {
		log.Fatalf("❌ Failed to init AWS client: %v", err)
	}

	// Run cost collection
	costService := services.NewCostService(awsClient, db)

	err = costService.CollectCostData(days)
	if err != nil {
		log.Fatalf("❌ Cost data collection failed: %v", err)
	}

	fmt.Println("✅ Cost data collected.")
}

func runSummary() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("❌ Failed to load config: %v", err)
	}

	db, err := database.Connect(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("❌ Failed to connect to DB: %v", err)
	}

	analysisService := services.NewAnalysisService(db)

	// Run analysis so we have fresh data
	err = analysisService.RunAnalysis()
	if err != nil {
		log.Fatalf("❌ Analysis failed: %v", err)
	}

	// Fetch latest analysis
	latestAnalysis, err := analysisService.GetLatestAnalysis()
	if err != nil {
		log.Fatalf("❌ Failed to fetch latest analysis: %v", err)
	}

	printAnalysis(latestAnalysis)
}

func printAnalysis(result services.AnalysisResult) {
	fmt.Printf("\n💰 Total Cost: $%.2f\n", result.TotalCost)
	fmt.Printf("🏷️ Untagged Cost: $%.2f (%.1f%%)\n", result.UntaggedCost, result.UntaggedPercent)

	fmt.Println("\nTop Services:")
	for _, s := range result.TopServices {
		fmt.Printf(" • %-30s $%.2f\n", s.Service, s.TotalCost)
	}

	fmt.Println("\nInsights:")
	for _, insight := range result.Insights {
		fmt.Printf(" • %s\n", insight)
	}
}
