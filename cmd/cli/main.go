package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"tagscale/internal/aws"
	"tagscale/internal/config"
	"tagscale/internal/database"
	"tagscale/internal/models"
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

// allowedGroupByOptions lists the supported values for the --group-by flag.
var allowedGroupByOptions = map[string]struct{}{
	"service": {},
	"account": {},
	"region":  {},
	"team":    {},
}

func cliDBPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, ".tagscale")
	if err := os.MkdirAll(dir, 0o700); err != nil {
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
		dateRange string
		useDB     bool
		migrate   bool
		region    string
		profile   string
		output    string
		limit     int
		groupBy   string
	)

	rootCmd := &cobra.Command{
		Use:   "tagscale",
		Short: "TagScale CLI - Cloud cost insights in your terminal",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			if _, ok := allowedOutputFormats[output]; !ok {
				_ = cmd.Help()
				return fmt.Errorf("invalid output format %q: supported formats are table and json", output)
			}
			if cmd.Name() == "summary" {
				if limit <= 0 {
					_ = cmd.Help()
					return fmt.Errorf("limit must be greater than 0")
				}
				if _, ok := allowedGroupByOptions[groupBy]; !ok {
					_ = cmd.Help()
					return fmt.Errorf("invalid group-by value %q: supported options are service, account, region, and team", groupBy)
				}
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
			runScan(dateRange, useDB, migrate, region, profile, output)
		},
	}

	scanCmd.Flags().StringVar(&dateRange, "range", "30", "Date range: N (days) or YYYY-MM-DD[:YYYY-MM-DD]")

	// SUMMARY command
	summaryCmd := &cobra.Command{
		Use:   "summary",
		Short: "Show cost summary in terminal",
		Run: func(cmd *cobra.Command, args []string) {
			runSummary(dateRange, useDB, migrate, output, groupBy, limit)
		},
	}

	summaryCmd.Flags().IntVar(&limit, "limit", 5, "Limit number of results (must be > 0)")
	summaryCmd.Flags().StringVar(&groupBy, "group-by", "service", "Group costs by: service, account, region, or team")
	summaryCmd.Flags().StringVar(&dateRange, "range", "30", "Date range: N (days) or YYYY-MM-DD[:YYYY-MM-DD]")

	rootCmd.AddCommand(scanCmd)
	rootCmd.AddCommand(summaryCmd)

	return rootCmd
}

func runScan(rangeStr string, useDB, migrate bool, region, profile, output string) {
	if output == "table" {
		fmt.Println("🔍 Running TagScale scan...")
	}

	startDate, endDate, err := parseDateRange(rangeStr)
	if err != nil {
		log.Fatalf("❌ Invalid range: %v", err)
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

	dbConn, err := db.DB()
	if err != nil {
		log.Fatalf("❌ Failed to get DB connection: %v", err)
	}
	defer dbConn.Close()

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

	err = costService.CollectCostData(startDate, endDate, time.Duration(cfg.AWSRequestTimeout)*time.Second)
	if err != nil {
		log.Fatalf("❌ Cost data collection failed: %v", err)
	}

	if output == "json" {
		_ = json.NewEncoder(os.Stdout).Encode(map[string]string{"message": "cost data collected"})
	} else {
		fmt.Println("✅ Cost data collected.")
	}
}

func parseDateRange(rangeStr string) (time.Time, time.Time, error) {
	now := time.Now().UTC().Truncate(24 * time.Hour)
	if rangeStr == "" {
		return now.AddDate(0, 0, -30), now, nil
	}

	if n, err := strconv.Atoi(rangeStr); err == nil {
		if n < 0 {
			return time.Time{}, time.Time{}, fmt.Errorf("days must be non-negative")
		}
		start := now.AddDate(0, 0, -n)
		if now.Before(start) {
			return time.Time{}, time.Time{}, fmt.Errorf("start date %s is after end date %s", start.Format("2006-01-02"), now.Format("2006-01-02"))
		}
		return start, now, nil
	}

	parts := strings.Split(rangeStr, ":")
	start, err := time.ParseInLocation("2006-01-02", parts[0], time.UTC)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("invalid start date: %w", err)
	}

	var end time.Time
	if len(parts) > 1 && parts[1] != "" {
		end, err = time.ParseInLocation("2006-01-02", parts[1], time.UTC)
		if err != nil {
			return time.Time{}, time.Time{}, fmt.Errorf("invalid end date: %w", err)
		}
	} else {
		end = now
	}

	if end.Before(start) {
		return time.Time{}, time.Time{}, fmt.Errorf("start date %s is after end date %s", start.Format("2006-01-02"), end.Format("2006-01-02"))
	}

	return start, end, nil
}

func runSummary(rangeStr string, useDB, migrate bool, output, groupBy string, limit int) {
	startDate, endDate, err := parseDateRange(rangeStr)
	if err != nil {
		log.Fatalf("❌ Invalid range: %v", err)
	}

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("❌ Failed to load config: %v", err)
	}

	db, err := initDB(useDB, migrate, cfg)
	if err != nil {
		log.Fatalf("❌ Failed to initialize DB: %v", err)
	}

	dbConn, err := db.DB()
	if err != nil {
		log.Fatalf("❌ Failed to get DB connection: %v", err)
	}
	defer dbConn.Close()

	analysisService := services.NewAnalysisService(db)

	// Run analysis so we have fresh data
	err = analysisService.RunAnalysis(limit, startDate, endDate)
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
		printAnalysis(latestAnalysis, groupBy)
	}
}

func printAnalysis(result services.AnalysisResult, groupBy string) {
	fmt.Printf("\n💰 Total Cost: $%.2f\n", result.TotalCost)
	fmt.Printf("🏷️ Untagged Cost: $%.2f (%.1f%%)\n", result.UntaggedCost, result.UntaggedPercent)

	var (
		items []models.CostSummary
		title string
	)
	switch groupBy {
	case "account":
		items = result.TopAccounts
		title = "Top Accounts"
	case "region":
		items = result.TopRegions
		title = "Top Regions"
	case "team":
		items = result.CostByTeam
		title = "Cost By Team"
	default:
		items = result.TopServices
		title = "Top Services"
	}

	fmt.Printf("\n%s:\n", title)
	for _, s := range items {
		label := s.Service
		switch groupBy {
		case "account":
			label = s.Account
		case "region":
			label = s.Region
		case "team":
			label = s.Team
		}
		fmt.Printf(" • %-30s $%.2f\n", label, s.TotalCost)
	}

	fmt.Println("\nInsights:")
	for _, insight := range result.Insights {
		fmt.Printf(" • %s\n", insight)
	}
}
