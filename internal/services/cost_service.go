package services

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"tagscale/internal/models"

	"github.com/aws/aws-sdk-go-v2/service/costexplorer"
	"gorm.io/gorm"
)

// JSONMarshal is a package-level variable to allow overriding in tests.
var JSONMarshal = json.Marshal

// CostExplorerAPI describes the subset of the AWS Cost Explorer client used by
// CostService. Implementations should return cost data grouped by service,
// account, region, resource ID and team tag so that CollectCostData can attach
// tags and resource identifiers to saved records. The optional nextToken
// parameter allows callers to page through large result sets. Defining this
// interface allows the service to be tested with a mock implementation.
type CostExplorerAPI interface {
	GetCostAndUsage(ctx context.Context, startDate, endDate time.Time, nextToken *string) (*costexplorer.GetCostAndUsageOutput, error)
}

type CostService struct {
	awsClient CostExplorerAPI
	db        *gorm.DB
}

// AllowedGroupBy lists the valid fields that can be used for grouping cost data.
var AllowedGroupBy = map[string]struct{}{
	"service": {},
	"account": {},
	"region":  {},
	"date":    {},
}

// IsGroupByAllowed reports whether the provided field is in the safe list.
func IsGroupByAllowed(field string) bool {
	_, ok := AllowedGroupBy[field]
	return ok
}

func NewCostService(awsClient CostExplorerAPI, db *gorm.DB) *CostService {
	return &CostService{
		awsClient: awsClient,
		db:        db,
	}
}

func marshalTags(tagValue string) ([]byte, error) {
	if tagValue == "" {
		return []byte("{}"), nil
	}
	tags := map[string]string{"Team": tagValue}
	return JSONMarshal(tags)
}

// CollectCostData retrieves AWS cost data for the provided time range
// and stores the results in the database.
// The provided timeout controls how long each AWS API call may take before
// being canceled. A fresh context with the timeout is created for every
// request so that the limit applies per request rather than for the entire
// operation.
func (s *CostService) CollectCostData(ctx context.Context, startDate, endDate time.Time, timeout time.Duration, batchSize int) (int, error) {
	// Retrieve pages from Cost Explorer one at a time. After processing each
	// page, insert its records before requesting the next page so that large
	// result sets don't have to be held entirely in memory.
	var nextToken *string
	var totalInserted int
	for {
		reqCtx, cancel := context.WithTimeout(ctx, timeout)
		result, err := s.awsClient.GetCostAndUsage(reqCtx, startDate, endDate, nextToken)
		if err != nil {
			cancel()
			return 0, fmt.Errorf("failed to get cost data: %w", err)
		}

		var costRecords []models.CostRecord

		for _, resultByTime := range result.ResultsByTime {
			date, err := time.Parse("2006-01-02", *resultByTime.TimePeriod.Start)
			if err != nil {
				continue
			}

			for _, group := range resultByTime.Groups {
				if len(group.Keys) < 3 {
					continue
				}

				service := group.Keys[0]
				account := group.Keys[1]
				region := group.Keys[2]
				resourceID := ""
				if len(group.Keys) >= 4 {
					resourceID = group.Keys[3]
				}
				tagValue := ""
				if len(group.Keys) >= 5 {
					tagValue = group.Keys[4]
				}

				tagsJSON, err := marshalTags(tagValue)
				if err != nil {
					cancel()
					return 0, fmt.Errorf("failed to marshal tags: %w", err)
				}

				costAmount := 0.0
				if metric, ok := group.Metrics["BlendedCost"]; ok {
					if amount := metric.Amount; amount != nil {
						var parseErr error
						costAmount, parseErr = strconv.ParseFloat(*amount, 64)
						if parseErr != nil {
							cancel()
							return 0, fmt.Errorf("failed to parse cost amount %q: %w", *amount, parseErr)
						}
					}
				}

				costRecord := models.CostRecord{
					Date:       date,
					Service:    service,
					Account:    account,
					Region:     region,
					ResourceID: resourceID,
					Cost:       costAmount,
					Currency:   "USD",
					Tags:       string(tagsJSON),
				}

				costRecords = append(costRecords, costRecord)
			}
		}

		if len(costRecords) > 0 {
			if batchSize <= 0 {
				batchSize = 100
			}
			if err := s.db.WithContext(reqCtx).CreateInBatches(costRecords, batchSize).Error; err != nil {
				cancel()
				return 0, fmt.Errorf("failed to insert cost records: %w", err)
			}
			totalInserted += len(costRecords)
		}

		cancel()

		if result.NextPageToken == nil || *result.NextPageToken == "" {
			break
		}
		nextToken = result.NextPageToken
	}

	return totalInserted, nil
}

func (s *CostService) GetCostSummary(ctx context.Context, startDate, endDate time.Time, groupBy string) ([]models.CostSummary, error) {
	if !IsGroupByAllowed(groupBy) {
		return nil, fmt.Errorf("invalid group by field")
	}
	if endDate.Before(startDate) {
		return nil, fmt.Errorf("end date must be after start date")
	}

	var results []models.CostSummary

	query := s.db.WithContext(ctx).Model(&models.CostRecord{}).
		Select(fmt.Sprintf("%s, SUM(cost) as total_cost", groupBy)).
		Where("date >= ? AND date <= ?", startDate, endDate).
		Group(groupBy).
		Order("total_cost DESC")

	if err := query.Scan(&results).Error; err != nil {
		return nil, fmt.Errorf("failed to get cost summary: %w", err)
	}

	// Calculate percentages
	var totalCost float64
	for _, result := range results {
		totalCost += result.TotalCost
	}

	for i := range results {
		if totalCost > 0 {
			results[i].Percentage = (results[i].TotalCost / totalCost) * 100
		}
	}

	return results, nil
}

func (s *CostService) GetTopCosts(ctx context.Context, limit int, groupBy string) ([]models.CostSummary, error) {
	if limit <= 0 {
		return nil, fmt.Errorf("limit must be positive")
	}
	if !IsGroupByAllowed(groupBy) {
		return nil, fmt.Errorf("invalid group by field")
	}

	var results []models.CostSummary

	query := s.db.WithContext(ctx).Model(&models.CostRecord{}).
		Select(fmt.Sprintf("%s, SUM(cost) as total_cost", groupBy)).
		Group(groupBy).
		Order("total_cost DESC").
		Limit(limit)

	if err := query.Scan(&results).Error; err != nil {
		return nil, fmt.Errorf("failed to get top costs: %w", err)
	}

	return results, nil
}
