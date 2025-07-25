package services_test

import (
	"context"
	"encoding/json"
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

func setupDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
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
	svc := services.NewCostService(&mockAWSClient{output: output}, db)
	err := svc.CollectCostData(1)
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

func TestGetCostSummary(t *testing.T) {
	db := setupDB(t)
	now := time.Now()
	recs := []models.CostRecord{
		{Date: now, Service: "A", Cost: 10},
		{Date: now, Service: "B", Cost: 20},
	}
	require.NoError(t, db.Create(&recs).Error)
	svc := services.NewCostService(nil, db)
	results, err := svc.GetCostSummary(now.Add(-time.Hour), now.Add(time.Hour), "service")
	require.NoError(t, err)
	require.Len(t, results, 2)
	require.Equal(t, "B", results[0].Service)
	require.InDelta(t, 20.0, results[0].TotalCost, 0.001)
	require.InDelta(t, 66.6, results[0].Percentage, 1)
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
	require.NoError(t, svc.RunAnalysis())

	var analysis models.CostAnalysis
	require.NoError(t, db.First(&analysis).Error)
	require.InDelta(t, 40.0, analysis.TotalCost, 0.001)
	require.InDelta(t, 40.0, analysis.UntaggedCost, 0.001)
	require.InDelta(t, 100.0, analysis.UntaggedPercent, 0.001)

	var top []models.CostSummary
	require.NoError(t, json.Unmarshal([]byte(analysis.TopServices), &top))
	require.Len(t, top, 2)
	require.Equal(t, "S3", top[0].Service)
}
