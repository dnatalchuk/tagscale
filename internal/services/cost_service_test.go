package services_test

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
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
	outputs []*costexplorer.GetCostAndUsageOutput
	err     error
	call    int
}

func (m *mockAWSClient) GetCostAndUsage(ctx context.Context, startDate, endDate time.Time, nextToken *string) (*costexplorer.GetCostAndUsageOutput, error) {
	if m.err != nil {
		return nil, m.err
	}
	if m.call >= len(m.outputs) {
		return &costexplorer.GetCostAndUsageOutput{}, nil
	}
	out := m.outputs[m.call]
	m.call++
	return out, nil
}

type timeoutMockAWSClient struct {
	delay time.Duration
}

func (m *timeoutMockAWSClient) GetCostAndUsage(ctx context.Context, startDate, endDate time.Time, nextToken *string) (*costexplorer.GetCostAndUsageOutput, error) {
	select {
	case <-time.After(m.delay):
		return &costexplorer.GetCostAndUsageOutput{}, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

type streamingMockAWSClient struct {
	outputs               []*costexplorer.GetCostAndUsageOutput
	db                    *gorm.DB
	call                  int
	countBeforeSecondCall int64
}

func (m *streamingMockAWSClient) GetCostAndUsage(ctx context.Context, startDate, endDate time.Time, nextToken *string) (*costexplorer.GetCostAndUsageOutput, error) {
	if m.call == 1 {
		m.db.Model(&models.CostRecord{}).Count(&m.countBeforeSecondCall)
	}
	if m.call >= len(m.outputs) {
		return &costexplorer.GetCostAndUsageOutput{}, nil
	}
	out := m.outputs[m.call]
	m.call++
	return out, nil
}

func setupDB(t *testing.T) *gorm.DB {
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.CostRecord{}, &models.CostAnalysis{}, &models.TeamMapping{}))
	return db
}

func TestCollectCostData(t *testing.T) {
	db := setupDB(t)
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
	svc := services.NewCostService(&mockAWSClient{outputs: []*costexplorer.GetCostAndUsageOutput{output}}, db)
	start := time.Now().AddDate(0, 0, -1)
	end := time.Now()
	err := svc.CollectCostData(start, end, 30*time.Second)
	require.NoError(t, err)

	var recs []models.CostRecord
	require.NoError(t, db.Find(&recs).Error)
	require.Len(t, recs, 1)
	require.Equal(t, "AmazonEC2", recs[0].Service)
	require.Equal(t, "123456789012", recs[0].Account)
	require.Equal(t, "us-east-1", recs[0].Region)
	require.InDelta(t, 5.0, recs[0].Cost, 0.001)
	require.Equal(t, "i-abc123", recs[0].ResourceID)
	var tags map[string]string
	require.NoError(t, json.Unmarshal([]byte(recs[0].Tags), &tags))
	require.Equal(t, "backend", tags["Team"])
}

func TestCollectCostDataPagination(t *testing.T) {
	db := setupDB(t)
	page1 := &costexplorer.GetCostAndUsageOutput{
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
		NextPageToken: aws.String("token1"),
	}
	page2 := &costexplorer.GetCostAndUsageOutput{
		ResultsByTime: []types.ResultByTime{
			{
				TimePeriod: &types.DateInterval{Start: aws.String("2023-01-02"), End: aws.String("2023-01-03")},
				Groups: []types.Group{
					{
						Keys: []string{"AmazonS3", "123456789012", "us-east-1", "bucket123", "frontend"},
						Metrics: map[string]types.MetricValue{
							"BlendedCost": {Amount: aws.String("3"), Unit: aws.String("USD")},
						},
					},
				},
			},
		},
	}
	svc := services.NewCostService(&mockAWSClient{outputs: []*costexplorer.GetCostAndUsageOutput{page1, page2}}, db)
	start := time.Now().AddDate(0, 0, -1)
	end := time.Now()
	require.NoError(t, svc.CollectCostData(start, end, 30*time.Second))

	var recs []models.CostRecord
	require.NoError(t, db.Find(&recs).Error)
	require.Len(t, recs, 2)
	require.Equal(t, "AmazonEC2", recs[0].Service)
	require.Equal(t, "AmazonS3", recs[1].Service)
}

func TestCollectCostDataStreaming(t *testing.T) {
	db := setupDB(t)
	page1 := &costexplorer.GetCostAndUsageOutput{
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
		NextPageToken: aws.String("token1"),
	}
	page2 := &costexplorer.GetCostAndUsageOutput{
		ResultsByTime: []types.ResultByTime{
			{
				TimePeriod: &types.DateInterval{Start: aws.String("2023-01-02"), End: aws.String("2023-01-03")},
				Groups: []types.Group{
					{
						Keys: []string{"AmazonS3", "123456789012", "us-east-1", "bucket123", "frontend"},
						Metrics: map[string]types.MetricValue{
							"BlendedCost": {Amount: aws.String("3"), Unit: aws.String("USD")},
						},
					},
				},
			},
		},
	}
	mock := &streamingMockAWSClient{outputs: []*costexplorer.GetCostAndUsageOutput{page1, page2}, db: db}
	svc := services.NewCostService(mock, db)
	start := time.Now().AddDate(0, 0, -1)
	end := time.Now()
	require.NoError(t, svc.CollectCostData(start, end, 30*time.Second))

	// Ensure first page records were written before requesting the second page
	require.Greater(t, mock.countBeforeSecondCall, int64(0))

	var recs []models.CostRecord
	require.NoError(t, db.Find(&recs).Error)
	require.Len(t, recs, 2)
	require.Equal(t, "AmazonEC2", recs[0].Service)
	require.Equal(t, "AmazonS3", recs[1].Service)
}

func TestCollectCostDataTimeout(t *testing.T) {
	db := setupDB(t)
	svc := services.NewCostService(&timeoutMockAWSClient{delay: 50 * time.Millisecond}, db)
	start := time.Now().AddDate(0, 0, -1)
	end := time.Now()
	err := svc.CollectCostData(start, end, 10*time.Millisecond)
	require.Error(t, err)
	require.ErrorIs(t, err, context.DeadlineExceeded)
}

func TestGetCostSummary(t *testing.T) {
	db := setupDB(t)
	now := time.Now()
	recs := []models.CostRecord{
		{Date: now, Service: "A", Cost: 10},
		{Date: now, Service: "B", Cost: 20},
	}
	require.NoError(t, db.Create(&recs).Error)
	svc := services.NewCostService(nil, db)
	results, err := svc.GetCostSummary(context.Background(), now.Add(-time.Hour), now.Add(time.Hour), "service")
	require.NoError(t, err)
	require.Len(t, results, 2)
	require.Equal(t, "B", results[0].Service)
	require.InDelta(t, 20.0, results[0].TotalCost, 0.001)
	require.InDelta(t, 66.6, results[0].Percentage, 1)
}

func TestGetCostSummaryContextCanceled(t *testing.T) {
	db := setupDB(t)
	svc := services.NewCostService(nil, db)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := svc.GetCostSummary(ctx, time.Now().Add(-time.Hour), time.Now(), "service")
	require.Error(t, err)
	require.ErrorIs(t, err, context.Canceled)
}

func TestGetTopCostsContextCanceled(t *testing.T) {
	db := setupDB(t)
	svc := services.NewCostService(nil, db)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := svc.GetTopCosts(ctx, 5, "service")
	require.Error(t, err)
	require.ErrorIs(t, err, context.Canceled)
}

func TestRunAnalysis(t *testing.T) {
	db := setupDB(t)
	today := time.Now().Truncate(24 * time.Hour)
	yesterday := today.AddDate(0, 0, -1)

	recs := []models.CostRecord{
		{Date: yesterday, Service: "S3", Cost: 30, Tags: "{}"},
		{Date: yesterday, Service: "EC2", Cost: 10, Tags: "{}"},
	}
	require.NoError(t, db.Create(&recs).Error)

	svc := services.NewAnalysisService(db)
	res, err := svc.RunAnalysis(5, yesterday, today, true)
	require.NoError(t, err)

	var analysis models.CostAnalysis
	require.NoError(t, db.First(&analysis).Error)
	require.InDelta(t, 40.0, analysis.TotalCost, 0.001)
	require.InDelta(t, 40.0, analysis.UntaggedCost, 0.001)
	require.InDelta(t, 100.0, analysis.UntaggedPercent, 0.001)

	require.InDelta(t, 40.0, res.TotalCost, 0.001)
	require.InDelta(t, 40.0, res.UntaggedCost, 0.001)
	require.InDelta(t, 100.0, res.UntaggedPercent, 0.001)

	var top []models.CostSummary
	require.NoError(t, json.Unmarshal([]byte(analysis.TopServices), &top))
	require.Len(t, top, 2)
	require.Equal(t, "S3", top[0].Service)
}
