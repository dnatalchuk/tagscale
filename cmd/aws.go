package cmd

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/costexplorer"
	"github.com/spf13/cobra"
)

var (
	profile string
	days    int
)

func init() {
	var awsCmd = &cobra.Command{
		Use:   "aws",
		Short: "Fetch AWS cost data and generate report",
		Run:   runAWS,
	}

	awsCmd.Flags().StringVar(&profile, "profile", "default", "AWS CLI profile name")
	awsCmd.Flags().IntVar(&days, "days", 30, "Number of days to look back")

	rootCmd.AddCommand(awsCmd)
}

func runAWS(cmd *cobra.Command, args []string) {
	fmt.Println("Connecting to AWS...")

	// Load AWS config
	cfg, err := config.LoadDefaultConfig(context.TODO(),
		config.WithSharedConfigProfile(profile),
	)
	if err != nil {
		log.Fatalf("Failed to load AWS config: %v", err)
	}

	ceClient := costexplorer.NewFromConfig(cfg)

	end := time.Now()
	start := end.AddDate(0, 0, -days)

	input := &costexplorer.GetCostAndUsageInput{
		TimePeriod: &costexplorer.DateInterval{
			Start: awsString(start.Format("2006-01-02")),
			End:   awsString(end.Format("2006-01-02")),
		},
		Granularity: "DAILY",
		Metrics:     []string{"UnblendedCost"},
		GroupBy: []costexplorer.GroupDefinition{
			{
				Type: costexplorer.GroupDefinitionTypeService,
				Key:  awsString("SERVICE"),
			},
		},
	}

	resp, err := ceClient.GetCostAndUsage(context.TODO(), input)
	if err != nil {
		log.Fatalf("Failed to get cost data: %v", err)
	}

	for _, result := range resp.ResultsByTime {
		fmt.Println("Date:", *result.TimePeriod.Start)
		for _, group := range result.Groups {
			service := group.Keys[0]
			amount := group.Metrics["UnblendedCost"].Amount
			fmt.Printf("  %s: $%s\n", service, *amount)
		}
	}
}

func awsString(s string) *string {
	return &s
}
