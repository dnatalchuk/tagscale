package aws

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/costexplorer"
	"github.com/stretchr/testify/require"
)

func TestNewClientUsesBackgroundWhenContextNil(t *testing.T) {
	os.Setenv("AWS_EC2_METADATA_DISABLED", "true")
	t.Cleanup(func() { os.Unsetenv("AWS_EC2_METADATA_DISABLED") })

	client, err := NewClient(nil, "us-east-1", "")
	require.NoError(t, err)
	require.NotNil(t, client)
}

type fakeCostExplorer struct {
	input *costexplorer.GetCostAndUsageInput
}

func (f *fakeCostExplorer) GetCostAndUsage(ctx context.Context, input *costexplorer.GetCostAndUsageInput, _ ...func(*costexplorer.Options)) (*costexplorer.GetCostAndUsageOutput, error) {
	f.input = input
	return &costexplorer.GetCostAndUsageOutput{}, nil
}

func (f *fakeCostExplorer) GetRightsizingRecommendation(ctx context.Context, input *costexplorer.GetRightsizingRecommendationInput, _ ...func(*costexplorer.Options)) (*costexplorer.GetRightsizingRecommendationOutput, error) {
	return &costexplorer.GetRightsizingRecommendationOutput{}, nil
}

func TestGetCostAndUsageRequestsOnlyBlendedCost(t *testing.T) {
	f := &fakeCostExplorer{}
	client := &Client{costExplorer: f}

	start := time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)
	end := start.AddDate(0, 0, 1)

	_, err := client.GetCostAndUsage(context.Background(), start, end, nil)
	require.NoError(t, err)
	require.Equal(t, []string{"BlendedCost"}, f.input.Metrics)
}
