package main

import (
	"context"
	"encoding/json"
	"fmt"
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

// Version is the CLI version. It can be set at build time using -ldflags.
var Version = "dev"

// awsClientFactory allows tests to inject a mock AWS client.
var awsClientFactory = func(ctx context.Context, region, profile string) (services.CostExplorerAPI, error) {
	return aws.NewClient(ctx, region, profile)
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

// errorf formats an error message and prefixes it with a warning emoji when
// using table output. In JSON mode, the message is returned without the emoji
// so it can be encoded cleanly.
func errorf(output, format string, args ...interface{}) error {
	if output != "json" {
		format = "❌ " + format
	}
	return fmt.Errorf(format, args...)
}

// printError writes an error to the appropriate output stream depending on the
// selected format. When JSON output is requested, the error is printed as a
// JSON object; otherwise it is written to stderr as plain text.
func printError(cmd *cobra.Command, err error) {
	output := "table"
	if flag := cmd.Flag("output"); flag != nil {
		output = flag.Value.String()
	}
	if output == "json" {
		_ = json.NewEncoder(cmd.OutOrStdout()).Encode(map[string]string{"error": err.Error()})
	} else {
		fmt.Fprintln(cmd.ErrOrStderr(), err)
	}
}

func main() {
	cli := NewCLI()
	if err := cli.Execute(); err != nil {
		printError(cli, err)
		os.Exit(1)
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
		timeout   int
		limit     int
		groupBy   string
		quiet     bool
		verbose   bool
	)

	cfg, _ := config.Load()
	timeout = cfg.AWSRequestTimeout

	rootCmd := &cobra.Command{
		Use:           "tagscale",
		Short:         "TagScale CLI - Cloud cost insights in your terminal",
		Version:       Version,
		SilenceErrors: true,
		SilenceUsage:  true,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			if _, ok := allowedOutputFormats[output]; !ok {
				_ = cmd.Help()
				return fmt.Errorf("invalid output format %q: supported formats are table and json", output)
			}
			if quiet && verbose {
				_ = cmd.Help()
				return fmt.Errorf("cannot use --quiet and --verbose together")
			}
			return nil
		},
	}

	rootCmd.SetVersionTemplate("{{.Version}}\n")

	rootCmd.PersistentFlags().BoolVar(&useDB, "db", false, "Persist data using DATABASE_URL")
	rootCmd.PersistentFlags().BoolVar(&migrate, "migrate", false, "Run database migrations on startup")
	rootCmd.PersistentFlags().StringVar(&region, "region", os.Getenv("AWS_REGION"), "AWS region (default from AWS_REGION)")
	rootCmd.PersistentFlags().StringVar(&profile, "profile", os.Getenv("AWS_PROFILE"), "AWS shared config profile (default from AWS_PROFILE)")
	rootCmd.PersistentFlags().StringVar(&output, "output", "table", "Output format: table or json")
	rootCmd.PersistentFlags().BoolVarP(&quiet, "quiet", "q", false, "Suppress progress output")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Show verbose progress output")

	// VERSION command
	versionCmd := &cobra.Command{
		Use:   "version",
		Short: "Print the CLI version",
		Run: func(cmd *cobra.Command, args []string) {
			if output == "json" {
				_ = json.NewEncoder(cmd.OutOrStdout()).Encode(map[string]string{"version": Version})
				return
			}
			fmt.Fprintln(cmd.OutOrStdout(), Version)
		},
	}

	// SCAN command
	scanCmd := &cobra.Command{
		Use:   "scan",
		Short: "Scan AWS cost and store data into DB",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runScan(dateRange, useDB, migrate, region, profile, output, timeout, quiet, verbose)
		},
	}

	scanCmd.Flags().StringVar(&dateRange, "range", "30", "Date range: N (days) or YYYY-MM-DD[:YYYY-MM-DD]")
	scanCmd.Flags().IntVar(&timeout, "timeout", cfg.AWSRequestTimeout, "AWS request timeout in seconds")

	// SUMMARY command
	summaryCmd := &cobra.Command{
		Use:   "summary",
		Short: "Show cost summary in terminal",
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if limit <= 0 {
				_ = cmd.Help()
				return fmt.Errorf("limit must be greater than 0")
			}
			if _, ok := allowedGroupByOptions[groupBy]; !ok {
				_ = cmd.Help()
				return fmt.Errorf("invalid group-by value %q: supported options are service, account, region, and team", groupBy)
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSummary(dateRange, useDB, migrate, output, groupBy, limit, quiet, verbose)
		},
	}

	summaryCmd.Flags().IntVar(&limit, "limit", 5, "Limit number of results (must be > 0)")
	summaryCmd.Flags().StringVar(&groupBy, "group-by", "service", "Group costs by: service, account, region, or team")
	summaryCmd.Flags().StringVar(&dateRange, "range", "30", "Date range: N (days) or YYYY-MM-DD[:YYYY-MM-DD]")

	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(scanCmd)
	rootCmd.AddCommand(summaryCmd)

	return rootCmd
}

func runScan(rangeStr string, useDB, migrate bool, region, profile, output string, timeout int, quiet, verbose bool) error {
	startDate, endDate, err := parseDateRange(rangeStr)
	if err != nil {
		return errorf(output, "Invalid range: %w", err)
	}

	if output == "table" && !quiet {
		if verbose {
			fmt.Printf("🔍 Running TagScale scan from %s to %s...\n", startDate.Format("2006-01-02"), endDate.Format("2006-01-02"))
		} else {
			fmt.Println("🔍 Running TagScale scan...")
		}
	}

	// Load config
	cfg, err := config.Load()
	if err != nil {
		return errorf(output, "Failed to load config: %w", err)
	}

	if timeout <= 0 {
		timeout = cfg.AWSRequestTimeout
	}

	db, err := initDB(useDB, migrate, cfg)
	if err != nil {
		return errorf(output, "Failed to initialize DB: %w", err)
	}

	dbConn, err := db.DB()
	if err != nil {
		return errorf(output, "Failed to get DB connection: %w", err)
	}
	defer dbConn.Close()

	if region == "" {
		region = cfg.AWSRegion
	}

	// Initialize AWS client with a timeout to avoid hanging during startup
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)
	defer cancel()
	awsClient, err := awsClientFactory(ctx, region, profile)
	if err != nil {
		return errorf(output, "Failed to init AWS client: %w", err)
	}

	// Run cost collection
	costService := services.NewCostService(awsClient, db)

	err = costService.CollectCostData(startDate, endDate, time.Duration(timeout)*time.Second)
	if err != nil {
		return errorf(output, "Cost data collection failed: %w", err)
	}

	if output == "json" {
		_ = json.NewEncoder(os.Stdout).Encode(map[string]string{"message": "cost data collected"})
	} else if !quiet {
		if verbose {
			fmt.Printf("✅ Cost data collected from %s to %s.\n", startDate.Format("2006-01-02"), endDate.Format("2006-01-02"))
		} else {
			fmt.Println("✅ Cost data collected.")
		}
	}

	return nil
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

func runSummary(rangeStr string, useDB, migrate bool, output, groupBy string, limit int, quiet, verbose bool) error {
	startDate, endDate, err := parseDateRange(rangeStr)
	if err != nil {
		return errorf(output, "Invalid range: %w", err)
	}

	if output == "table" && !quiet {
		if verbose {
			fmt.Printf("📊 Running TagScale summary from %s to %s grouped by %s (limit %d)...\n", startDate.Format("2006-01-02"), endDate.Format("2006-01-02"), groupBy, limit)
		} else {
			fmt.Println("📊 Running TagScale summary...")
		}
	}

	cfg, err := config.Load()
	if err != nil {
		return errorf(output, "Failed to load config: %w", err)
	}

	db, err := initDB(useDB, migrate, cfg)
	if err != nil {
		return errorf(output, "Failed to initialize DB: %w", err)
	}

	dbConn, err := db.DB()
	if err != nil {
		return errorf(output, "Failed to get DB connection: %w", err)
	}
	defer dbConn.Close()

	analysisService := services.NewAnalysisService(db)

	result, err := analysisService.RunAnalysis(limit, startDate, endDate, false)
	if err != nil {
		return errorf(output, "Analysis failed: %w", err)
	}

	if output == "json" {
		_ = json.NewEncoder(os.Stdout).Encode(result)
	} else {
		if !quiet && verbose {
			fmt.Println("✅ Analysis complete. Displaying results.")
		}
		printAnalysis(result, groupBy, quiet, verbose)
	}

	return nil
}

func printAnalysis(result services.AnalysisResult, groupBy string, quiet, verbose bool) {
	fmt.Printf("\n💰 Total Cost: $%.2f\n", result.TotalCost)
	fmt.Printf("🏷️ Untagged Cost: $%.2f (%.1f%%)\n", result.UntaggedCost, result.UntaggedPercent)

	if quiet {
		return
	}

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
		if verbose {
			fmt.Printf(" • %-30s $%.2f (%.1f%%)\n", label, s.TotalCost, s.Percentage)
		} else {
			fmt.Printf(" • %-30s $%.2f\n", label, s.TotalCost)
		}
	}

	fmt.Println("\nInsights:")
	for _, insight := range result.Insights {
		fmt.Printf(" • %s\n", insight)
	}
}
