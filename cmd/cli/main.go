package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"tagscale/internal/aws"
	"tagscale/internal/config"
	"tagscale/internal/database"
	"tagscale/internal/services"
)

// awsClientFactory allows tests to inject a mock AWS client.
var awsClientFactory = func(region, profile string) (services.CostExplorerAPI, error) {
	return aws.NewClient(region, profile)
}

// allowedOutputFormats lists the supported values for the --output flag.
var allowedOutputFormats = map[string]struct{}{
	"table": {},
	"json":  {},
}

func cliDBPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, ".tagscale")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return filepath.Join(dir, "cli.db"), nil
}

func initDB(useDB, migrate bool, cfg *config.Config) (*gorm.DB, error) {
	var (
		db  *gorm.DB
		err error
	)
	if useDB {
		db, err = database.Connect(cfg.DatabaseURL)
		if err != nil {
			return nil, err
		}
	} else {
		path, err := cliDBPath()
		if err != nil {
			return nil, err
		}
		db, err = gorm.Open(sqlite.Open(path), &gorm.Config{})
		if err != nil {
			return nil, err
		}
	}

	if migrate {
		if err := database.Migrate(db); err != nil {
			return nil, err
		}
	}

	return db, nil
}

func main() {
	if err := NewCLI().Execute(); err != nil {
		log.Fatalf("command failed: %v", err)
	}
}

func NewCLI() *cobra.Command {
	var (
		days    int
		useDB   bool
		migrate bool
		region  string
		profile string
		output  string
	)

	rootCmd := &cobra.Command{
		Use:   "tagscale",
		Short: "TagScale CLI - Cloud cost insights in your terminal",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			if _, ok := allowedOutputFormats[output]; !ok {
				_ = cmd.Help()
				return fmt.Errorf("invalid output format %q: supported formats are table and json", output)
			}
			return nil
		},
	}

	rootCmd.PersistentFlags().BoolVar(&useDB, "db", false, "Persist data using DATABASE_URL")
	rootCmd.PersistentFlags().BoolVar(&migrate, "migrate", false, "Run database migrations on startup")
	rootCmd.PersistentFlags().StringVar(&region, "region", os.Getenv("AWS_REGION"), "AWS region (default from AWS_REGION)")
	rootCmd.PersistentFlags().StringVar(&profile, "profile", os.Getenv("AWS_PROFILE"), "AWS shared config profile (default from AWS_PROFILE)")
	rootCmd.PersistentFlags().StringVar(&output, "output", "table", "Output format: table or json")

	// SCAN command
	scanCmd := &cobra.Command{
		Use:   "scan",
		Short: "Scan AWS cost and store data into DB",
		Run: func(cmd *cobra.Command, args []string) {
			runScan(days, useDB, migrate, region, profile, output)
		},
	}

	scanCmd.Flags().IntVar(&days, "days", 30, "Number of days to look back")

	// SUMMARY command
	summaryCmd := &cobra.Command{
		Use:   "summary",
		Short: "Show cost summary in terminal",
		Run: func(cmd *cobra.Command, args []string) {
			runSummary(useDB, migrate, output)
		},
	}

	rootCmd.AddCommand(scanCmd)
	rootCmd.AddCommand(summaryCmd)

	return rootCmd
}

func runScan(days int, useDB, migrate bool, region, profile, output string) {
	if output == "table" {
		fmt.Println("🔍 Running TagScale scan...")
	}

	// Load config
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("❌ Failed to load config: %v", err)
	}

	db, err := initDB(useDB, migrate, cfg)
	if err != nil {
		log.Fatalf("❌ Failed to initialize DB: %v", err)
	}

	if region == "" {
		region = cfg.AWSRegion
	}

	// Initialize AWS client
	awsClient, err := awsClientFactory(region, profile)
	if err != nil {
		log.Fatalf("❌ Failed to init AWS client: %v", err)
	}

	// Run cost collection
	costService := services.NewCostService(awsClient, db)

	err = costService.CollectCostData(days)
	if err != nil {
		log.Fatalf("❌ Cost data collection failed: %v", err)
	}

	if output == "json" {
		_ = json.NewEncoder(os.Stdout).Encode(map[string]string{"message": "cost data collected"})
	} else {
		fmt.Println("✅ Cost data collected.")
	}
}

func runSummary(useDB, migrate bool, output string) {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("❌ Failed to load config: %v", err)
	}

	db, err := initDB(useDB, migrate, cfg)
	if err != nil {
		log.Fatalf("❌ Failed to initialize DB: %v", err)
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

	if output == "json" {
		_ = json.NewEncoder(os.Stdout).Encode(latestAnalysis)
	} else {
		printAnalysis(latestAnalysis)
	}
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
