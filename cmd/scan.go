package cmd

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/costexplorer"
	"github.com/aws/aws-sdk-go-v2/service/costexplorer/types"
	"github.com/spf13/cobra"
)

var (
	profile string
	days    int
)

func init() {
	var scanCmd = &cobra.Command{
		Use:   "scan",
		Short: "Scan AWS cost and resource data for tagging analysis",
		Run:   runScan,
	}

	scanCmd.Flags().StringVar(&profile, "profile", "default", "AWS CLI profile name")
	scanCmd.Flags().IntVar(&days, "days", 30, "Number of days to look back")

	rootCmd.AddCommand(scanCmd)
}

func runScan(cmd *cobra.Command, args []string) {
	fmt.Println("🔍 Running TagScale scan...")
	fmt.Println("DEBUG - Using AWS profile:", profile)

	// Load AWS config - prioritize environment variables (from aws-vault)
	var cfg aws.Config
	var err error

	// Check if we're running under aws-vault (environment variables set)
	if os.Getenv("AWS_ACCESS_KEY_ID") != "" && os.Getenv("AWS_SECRET_ACCESS_KEY") != "" {
		fmt.Println("DEBUG - Using environment credentials (aws-vault)")
		cfg, err = config.LoadDefaultConfig(context.TODO())
	} else {
		fmt.Println("DEBUG - Using profile credentials")
		cfg, err = config.LoadDefaultConfig(
			context.TODO(),
			config.WithSharedConfigProfile(profile),
		)
	}

	if err != nil {
		log.Fatalf("Failed to load AWS config: %v", err)
	}

	ceClient := costexplorer.NewFromConfig(cfg)

	end := time.Now()
	start := end.AddDate(0, 0, -days)

	input := &costexplorer.GetCostAndUsageInput{
		TimePeriod: &types.DateInterval{
			Start: aws.String(start.Format("2006-01-02")),
			End:   aws.String(end.Format("2006-01-02")),
		},
		Granularity: types.GranularityDaily,
		Metrics:     []string{"UnblendedCost"},
		GroupBy: []types.GroupDefinition{
			{
				Type: types.GroupDefinitionType("DIMENSION"),
				Key:  aws.String("SERVICE"),
			},
		},
	}

	resp, err := ceClient.GetCostAndUsage(context.TODO(), input)
	if err != nil {
		// Check if it's the "not enabled" error
		if strings.Contains(err.Error(), "not enabled for cost explorer access") {
			log.Fatalf("Cost Explorer is not enabled or data is not ready yet.\nPlease enable Cost Explorer in the AWS Console and wait up to 24 hours for data preparation.")
		}
		log.Fatalf("Failed to get cost data: %v", err)
	}

	if len(resp.ResultsByTime) == 0 {
		fmt.Println("No results returned for the given time range.")
		return
	}

	fmt.Println("✅ Cost data retrieved successfully.")

	for _, result := range resp.ResultsByTime {
		date := aws.ToString(result.TimePeriod.Start)
		fmt.Printf("📅 Date: %s\n", date)
		for _, group := range result.Groups {
			service := group.Keys[0]
			amount := aws.ToString(group.Metrics["UnblendedCost"].Amount)
			fmt.Printf("    %-40s $%s\n", service, amount)
		}
		fmt.Println()
	}
}
