package services

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"time"

	"tagscale/internal/aws"
	"tagscale/internal/models"
)

// CollectCostDataInMemory retrieves AWS cost data for the provided number of days
// and returns it without persisting to a database.
func CollectCostDataInMemory(awsClient *aws.Client, days int) ([]models.CostRecord, error) {
	if days <= 0 {
		days = 30
	}

	ctx := context.Background()

	endDate := time.Now()
	startDate := endDate.AddDate(0, 0, -days)

	result, err := awsClient.GetCostAndUsage(ctx, startDate, endDate)
	if err != nil {
		return nil, fmt.Errorf("failed to get cost data: %w", err)
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

			costAmount := 0.0
			if metric, ok := group.Metrics["BlendedCost"]; ok {
				if amount := metric.Amount; amount != nil {
					costAmount, _ = strconv.ParseFloat(*amount, 64)
				}
			}

			costRecord := models.CostRecord{
				Date:     date,
				Service:  service,
				Account:  account,
				Region:   region,
				Cost:     costAmount,
				Currency: "USD",
				Tags:     "{}",
			}

			costRecords = append(costRecords, costRecord)
		}
	}

	return costRecords, nil
}

// AnalyzeCostRecords converts a slice of CostRecord into an AnalysisResult
// entirely in memory.
func AnalyzeCostRecords(records []models.CostRecord) AnalysisResult {
	var totalCost, untaggedCost float64
	serviceTotals := make(map[string]float64)

	for _, r := range records {
		totalCost += r.Cost
		if r.Tags == "" || r.Tags == "{}" {
			untaggedCost += r.Cost
		}
		serviceTotals[r.Service] += r.Cost
	}

	var summaries []models.CostSummary
	for svc, cost := range serviceTotals {
		summaries = append(summaries, models.CostSummary{Service: svc, TotalCost: cost})
	}

	sort.Slice(summaries, func(i, j int) bool { return summaries[i].TotalCost > summaries[j].TotalCost })
	if len(summaries) > 5 {
		summaries = summaries[:5]
	}

	if totalCost > 0 {
		for i := range summaries {
			summaries[i].Percentage = (summaries[i].TotalCost / totalCost) * 100
		}
	}

	untaggedPercent := 0.0
	if totalCost > 0 {
		untaggedPercent = (untaggedCost / totalCost) * 100
	}

	insights := generateInsights(totalCost, untaggedPercent, summaries)

	return AnalysisResult{
		TotalCost:       totalCost,
		UntaggedCost:    untaggedCost,
		UntaggedPercent: untaggedPercent,
		TopServices:     summaries,
		Insights:        insights,
	}
}

func generateInsights(totalCost, untaggedPercent float64, topServices []models.CostSummary) []string {
	var insights []string

	if untaggedPercent > 30 {
		insights = append(insights, fmt.Sprintf("%.1f%% of your costs are untagged. Consider implementing tagging policies.", untaggedPercent))
	}

	if len(topServices) > 0 {
		insights = append(insights, fmt.Sprintf("Your top service %s accounts for $%.2f of spend", topServices[0].Service, topServices[0].TotalCost))
	}

	if totalCost > 10000 {
		insights = append(insights, "High spend detected. Consider reviewing rightsizing recommendations.")
	}

	return insights
}
