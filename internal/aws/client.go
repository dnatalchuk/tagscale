// internal/aws/client.go
package aws

import (
	"context"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/costexplorer"
	"github.com/aws/aws-sdk-go-v2/service/costexplorer/types"
)

type costExplorerAPI interface {
	GetCostAndUsage(context.Context, *costexplorer.GetCostAndUsageInput, ...func(*costexplorer.Options)) (*costexplorer.GetCostAndUsageOutput, error)
	GetRightsizingRecommendation(context.Context, *costexplorer.GetRightsizingRecommendationInput, ...func(*costexplorer.Options)) (*costexplorer.GetRightsizingRecommendationOutput, error)
}

type Client struct {
	costExplorer costExplorerAPI
}

// NewClient creates an AWS Cost Explorer client with the provided region and
// optional shared configuration profile. The ctx parameter allows callers to
// control initialization, for example by applying a timeout. If ctx is nil the
// background context is used. If profile is empty the default credential chain
// is used.
func NewClient(ctx context.Context, region, profile string) (*Client, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	opts := []func(*awsconfig.LoadOptions) error{awsconfig.WithRegion(region)}
	if profile != "" {
		opts = append(opts, awsconfig.WithSharedConfigProfile(profile))
	}

	cfg, err := awsconfig.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return nil, err
	}

	return &Client{
		costExplorer: costexplorer.NewFromConfig(cfg),
	}, nil
}

func (c *Client) GetCostAndUsage(ctx context.Context, startDate, endDate time.Time, nextToken *string) (*costexplorer.GetCostAndUsageOutput, error) {
	input := &costexplorer.GetCostAndUsageInput{
		TimePeriod: &types.DateInterval{
			Start: aws.String(startDate.Format("2006-01-02")),
			End:   aws.String(endDate.Format("2006-01-02")),
		},
		Granularity: types.GranularityDaily,
		Metrics:     []string{"BlendedCost"}, // Only request needed metric for lower API overhead
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
			{
				Type: types.GroupDefinitionTypeDimension,
				Key:  aws.String("RESOURCE_ID"),
			},
			{
				Type: types.GroupDefinitionTypeTag,
				Key:  aws.String("Team"),
			},
		},
		NextPageToken: nextToken,
	}

	return c.costExplorer.GetCostAndUsage(ctx, input)
}

func (c *Client) GetRightsizingRecommendation(ctx context.Context) (*costexplorer.GetRightsizingRecommendationOutput, error) {
	input := &costexplorer.GetRightsizingRecommendationInput{
		Service: aws.String("AmazonEC2"),
	}

	return c.costExplorer.GetRightsizingRecommendation(ctx, input)
}
