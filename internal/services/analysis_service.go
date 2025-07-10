// internal/services/analysis_service.go
package services

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"tagscale/internal/models"

	"gorm.io/gorm"
)

type AnalysisService struct {
	db *gorm.DB
}

func NewAnalysisService(db *gorm.DB) *AnalysisService {
	return &AnalysisService{db: db}
}

func (s *AnalysisService) RunAnalysis() error {
	today := time.Now().Truncate(24 * time.Hour)

	// Get total cost
	var totalCost float64
	s.db.Model(&models.CostRecord{}).
		Where("date >= ? AND date < ?", today.AddDate(0, 0, -1), today).
		Select("SUM(cost)").
		Scan(&totalCost)

	// Get untagged cost (assuming empty or "{}" tags means untagged)
	var untaggedCost float64
	s.db.Model(&models.CostRecord{}).
		Where("date >= ? AND date < ? AND (tags = '' OR tags = '{}')", today.AddDate(0, 0, -1), today).
		Select("SUM(cost)").
		Scan(&untaggedCost)

	untaggedPercent := float64(0)
	if totalCost > 0 {
		untaggedPercent = (untaggedCost / totalCost) * 100
	}

	// Get top services
	topServices, _ := s.GetTopCosts(5, "service")
	topServicesJSON, _ := json.Marshal(topServices)

	// Get top accounts
	topAccounts, _ := s.GetTopCosts(5, "account")
	topAccountsJSON, _ := json.Marshal(topAccounts)

	// Get top regions
	topRegions, _ := s.GetTopCosts(5, "region")
	topRegionsJSON, _ := json.Marshal(topRegions)

	// Infer team ownership
	costByTeam := s.InferTeamOwnership()
	costByTeamJSON, _ := json.Marshal(costByTeam)

	// Generate insights
	insights := s.GenerateInsights(totalCost, untaggedPercent, topServices)
	insightsJSON, _ := json.Marshal(insights)

	// Save analysis
	analysis := models.CostAnalysis{
		Date:            today,
		TotalCost:       totalCost,
		UntaggedCost:    untaggedCost,
		UntaggedPercent: untaggedPercent,
		TopServices:     string(topServicesJSON),
		TopAccounts:     string(topAccountsJSON),
		TopRegions:      string(topRegionsJSON),
		CostByTeam:      string(costByTeamJSON),
		Insights:        string(insightsJSON),
	}

	if err := s.db.Create(&analysis).Error; err != nil {
		return fmt.Errorf("failed to save analysis: %w", err)
	}

	return nil
}

func (s *AnalysisService) GetTopCosts(limit int, groupBy string) ([]models.CostSummary, error) {
	var results []models.CostSummary

	query := s.db.Model(&models.CostRecord{}).
		Select(fmt.Sprintf("%s, SUM(cost) as total_cost", groupBy)).
		Group(groupBy).
		Order("total_cost DESC").
		Limit(limit)

	if err := query.Scan(&results).Error; err != nil {
		return nil, fmt.Errorf("failed to get top costs: %w", err)
	}

	return results, nil
}

func (s *AnalysisService) InferTeamOwnership() []models.CostSummary {
	var costRecords []models.CostRecord
	s.db.Find(&costRecords)

	teamCosts := make(map[string]float64)

	// Get team mappings
	var teamMappings []models.TeamMapping
	s.db.Order("priority DESC").Find(&teamMappings)

	for _, record := range costRecords {
		team := s.inferTeamFromRecord(record, teamMappings)
		teamCosts[team] += record.Cost
	}

	var results []models.CostSummary
	for team, cost := range teamCosts {
		results = append(results, models.CostSummary{
			Team:      team,
			TotalCost: cost,
		})
	}

	return results
}

func (s *AnalysisService) inferTeamFromRecord(record models.CostRecord, mappings []models.TeamMapping) string {
	for _, mapping := range mappings {
		switch mapping.PatternType {
		case "service":
			if matched, _ := regexp.MatchString(mapping.Pattern, record.Service); matched {
				return mapping.Team
			}
		case "resource_name":
			if matched, _ := regexp.MatchString(mapping.Pattern, record.ResourceID); matched {
				return mapping.Team
			}
		case "tag":
			// Check if tags contain the pattern
			if strings.Contains(record.Tags, mapping.Pattern) {
				return mapping.Team
			}
		}
	}

	return "unassigned"
}

func (s *AnalysisService) GenerateInsights(totalCost, untaggedPercent float64, topServices []models.CostSummary) []string {
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
