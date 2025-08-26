package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/costexplorer"
	"github.com/aws/aws-sdk-go-v2/service/costexplorer/types"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"tagscale/internal/config"
	"tagscale/internal/models"
	"tagscale/internal/services"
)

type mockAWSClient struct {
	output     *costexplorer.GetCostAndUsageOutput
	err        error
	ctxTimeout time.Duration
}

func (m *mockAWSClient) GetCostAndUsage(ctx context.Context, startDate, endDate time.Time, nextToken *string) (*costexplorer.GetCostAndUsageOutput, error) {
	if dl, ok := ctx.Deadline(); ok {
		m.ctxTimeout = time.Until(dl)
	}
	return m.output, m.err
}

func TestRunScanInsertsCostRecords(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "cli.db")

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
	awsClientFactory = func(ctx context.Context, region, profile string) (services.CostExplorerAPI, error) {
		return mockClient, nil
	}
	defer func() { awsClientFactory = origFactory }()

	var buf bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	cfg, err := config.Load()
	require.NoError(t, err)
	require.NoError(t, runScan(context.Background(), cmd, cfg, "1", false, true, dbPath, "", "", "table", 30, true, false, false))

	require.Empty(t, strings.TrimSpace(buf.String()))

	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	require.NoError(t, err)

	require.True(t, db.Migrator().HasTable(&models.CostRecord{}))
	require.True(t, db.Migrator().HasTable(&models.CostAnalysis{}))
	require.True(t, db.Migrator().HasTable(&models.TeamMapping{}))

	var count int64
	require.NoError(t, db.Model(&models.CostRecord{}).Count(&count).Error)
	require.Equal(t, int64(1), count)
}

func TestParseDateRangeNegativeDays(t *testing.T) {
	_, _, err := parseDateRange("-5")
	require.EqualError(t, err, "days must be non-negative")
}

func TestParseDateRangeInvertedRange(t *testing.T) {
	_, _, err := parseDateRange("2023-01-02:2023-01-01")
	require.Error(t, err)
	require.Contains(t, err.Error(), "start date")
}

func TestParseDateRangeTooManyParts(t *testing.T) {
	_, _, err := parseDateRange("2023-01-01:2023-01-02:2023-01-03")
	require.Error(t, err)
	require.Contains(t, err.Error(), "expected start[:end]")
}

func TestParseDateRangeDaysWithSpaces(t *testing.T) {
	now := time.Now().UTC().Truncate(24 * time.Hour)
	start, end, err := parseDateRange(" 7 ")
	require.NoError(t, err)
	require.Equal(t, now.AddDate(0, 0, -7), start)
	require.Equal(t, now, end)
}

func TestParseDateRangeRangeWithSpaces(t *testing.T) {
	start, end, err := parseDateRange(" 2023-01-01 : 2023-01-02 ")
	require.NoError(t, err)
	require.Equal(t, time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC), start)
	require.Equal(t, time.Date(2023, 1, 2, 0, 0, 0, 0, time.UTC), end)
}

func TestParseDateRangeFutureEnd(t *testing.T) {
	now := time.Now().UTC().Truncate(24 * time.Hour)
	future := now.AddDate(0, 0, 1)
	start, end, err := parseDateRange("2023-01-01:" + future.Format("2006-01-02"))
	require.NoError(t, err)
	require.Equal(t, time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC), start)
	require.Equal(t, now, end)
}

func TestScanTimeoutFlag(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "cli.db")
	os.Setenv("DATABASE_URL", "sqlite://"+dbPath)
	defer os.Unsetenv("DATABASE_URL")

	const timeoutSec = 7
	mockClient := &mockAWSClient{output: &costexplorer.GetCostAndUsageOutput{}}
	var initTimeout time.Duration

	origFactory := awsClientFactory
	awsClientFactory = func(ctx context.Context, region, profile string) (services.CostExplorerAPI, error) {
		if dl, ok := ctx.Deadline(); ok {
			initTimeout = time.Until(dl)
		}
		return mockClient, nil
	}
	defer func() { awsClientFactory = origFactory }()

	cli := NewCLI()
	cli.SetArgs([]string{"scan", "--db", "--migrate", "--timeout", strconv.Itoa(timeoutSec)})
	require.NoError(t, cli.Execute())

	require.InDelta(t, float64(timeoutSec), initTimeout.Seconds(), 1)
	require.InDelta(t, float64(timeoutSec), mockClient.ctxTimeout.Seconds(), 1)
}
