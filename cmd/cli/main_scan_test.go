package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/costexplorer"
	"github.com/aws/aws-sdk-go-v2/service/costexplorer/types"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"tagscale/internal/models"
	"tagscale/internal/services"
)

type mockAWSClient struct {
	output *costexplorer.GetCostAndUsageOutput
	err    error
}

func (m *mockAWSClient) GetCostAndUsage(ctx context.Context, startDate, endDate time.Time) (*costexplorer.GetCostAndUsageOutput, error) {
	return m.output, m.err
}

func TestRunScanInsertsCostRecords(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "cli.db")
	os.Setenv("DATABASE_URL", "sqlite://"+dbPath)
	defer os.Unsetenv("DATABASE_URL")

	output := &costexplorer.GetCostAndUsageOutput{
		ResultsByTime: []types.ResultByTime{
			{
				TimePeriod: &types.DateInterval{Start: aws.String("2023-01-01"), End: aws.String("2023-01-02")},
				Groups: []types.Group{
					{
						Keys: []string{"AmazonEC2", "123456789012", "us-east-1", "i-abc123", "backend"},
						Metrics: map[string]types.MetricValue{
							"BlendedCost": {Amount: aws.String("5"), Unit: aws.String("USD")},
						},
					},
				},
			},
		},
	}
	mockClient := &mockAWSClient{output: output}

	origFactory := awsClientFactory
	awsClientFactory = func(region, profile string) (services.CostExplorerAPI, error) { return mockClient, nil }
	defer func() { awsClientFactory = origFactory }()

	runScan(1, true, true, "", "", "table")

	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	require.NoError(t, err)

	require.True(t, db.Migrator().HasTable(&models.CostRecord{}))
	require.True(t, db.Migrator().HasTable(&models.CostAnalysis{}))
	require.True(t, db.Migrator().HasTable(&models.TeamMapping{}))

	var count int64
	require.NoError(t, db.Model(&models.CostRecord{}).Count(&count).Error)
	require.Equal(t, int64(1), count)
}
