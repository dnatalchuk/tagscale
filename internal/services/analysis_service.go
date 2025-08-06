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

// compiledTeamMapping augments TeamMapping with a compiled regex pattern
// for faster matching when PatternType requires regex evaluation.
type compiledTeamMapping struct {
	models.TeamMapping
	pattern *regexp.Regexp
}

// AnalysisResult represents the parsed result of a cost analysis run.
type AnalysisResult struct {
	TotalCost       float64
	UntaggedCost    float64
	UntaggedPercent float64
	TopServices     []models.CostSummary
	Insights        []string
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

	// Precompile regex patterns for mappings that require it
	compiled := make([]compiledTeamMapping, 0, len(teamMappings))
	for _, m := range teamMappings {
		cm := compiledTeamMapping{TeamMapping: m}
		if m.PatternType == "service" || m.PatternType == "resource_name" {
			if re, err := regexp.Compile(m.Pattern); err == nil {
				cm.pattern = re
			}
		}
		compiled = append(compiled, cm)
	}

	for _, record := range costRecords {
		team := s.inferTeamFromRecord(record, compiled)
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

func (s *AnalysisService) inferTeamFromRecord(record models.CostRecord, mappings []compiledTeamMapping) string {
	for _, mapping := range mappings {
		switch mapping.PatternType {
		case "service":
			if mapping.pattern != nil && mapping.pattern.MatchString(record.Service) {
				return mapping.Team
			}
		case "resource_name":
			if mapping.pattern != nil && mapping.pattern.MatchString(record.ResourceID) {
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

// GetLatestAnalysis retrieves the most recent cost analysis from the database
// and unmarshals the stored JSON fields into a convenient AnalysisResult
// structure.
func (s *AnalysisService) GetLatestAnalysis() (AnalysisResult, error) {
	var analysis models.CostAnalysis
	if err := s.db.Order("date desc").First(&analysis).Error; err != nil {
		return AnalysisResult{}, fmt.Errorf("failed to fetch latest analysis: %w", err)
	}

	result := AnalysisResult{
		TotalCost:       analysis.TotalCost,
		UntaggedCost:    analysis.UntaggedCost,
		UntaggedPercent: analysis.UntaggedPercent,
	}

	if err := json.Unmarshal([]byte(analysis.TopServices), &result.TopServices); err != nil {
		// ignore unmarshalling error but log for debugging
	}
	_ = json.Unmarshal([]byte(analysis.Insights), &result.Insights)

	return result, nil
}
