// internal/aws/client.go
package aws

import (
	"context"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/costexplorer"
	"github.com/aws/aws-sdk-go-v2/service/costexplorer/types"
)

type Client struct {
	costExplorer *costexplorer.Client
}

func NewClient(region string) (*Client, error) {
	cfg, err := config.LoadDefaultConfig(context.TODO(), config.WithRegion(region))
	if err != nil {
		return nil, err
	}

	return &Client{
		costExplorer: costexplorer.NewFromConfig(cfg),
	}, nil
}

func (c *Client) GetCostAndUsage(ctx context.Context, startDate, endDate time.Time) (*costexplorer.GetCostAndUsageOutput, error) {
	input := &costexplorer.GetCostAndUsageInput{
		TimePeriod: &types.DateInterval{
			Start: aws.String(startDate.Format("2006-01-02")),
			End:   aws.String(endDate.Format("2006-01-02")),
		},
		Granularity: types.GranularityDaily,
		Metrics:     []string{"BlendedCost", "UnblendedCost"},
		GroupBy: []types.GroupDefinition{
			{
				Type: types.GroupDefinitionTypeDimension,
				Key:  aws.String("SERVICE"),
			},
			{
				Type: types.GroupDefinitionTypeDimension,
				Key:  aws.String("LINKED_ACCOUNT"),
			},
			{
				Type: types.GroupDefinitionTypeDimension,
				Key:  aws.String("REGION"),
			},
		},
	}

	return c.costExplorer.GetCostAndUsage(ctx, input)
}

func (c *Client) GetRightsizingRecommendation(ctx context.Context) (*costexplorer.GetRightsizingRecommendationOutput, error) {
	input := &costexplorer.GetRightsizingRecommendationInput{
		Service: aws.String("AmazonEC2"),
	}

	return c.costExplorer.GetRightsizingRecommendation(ctx, input)
}
