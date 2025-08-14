// internal/services/analysis_service.go
package services

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
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
	TopAccounts     []models.CostSummary
	TopRegions      []models.CostSummary
	CostByTeam      []models.CostSummary
	Insights        []string
}

func NewAnalysisService(db *gorm.DB) *AnalysisService {
	return &AnalysisService{db: db}
}

func (s *AnalysisService) RunAnalysis(limit int, startDate, endDate time.Time) error {
	// Get total cost
	var totalCost float64
	s.db.Model(&models.CostRecord{}).
		Where("date >= ? AND date <= ?", startDate, endDate).
		Select("SUM(cost)").
		Scan(&totalCost)

	// Get untagged cost (assuming empty or "{}" tags means untagged)
	var untaggedCost float64
	s.db.Model(&models.CostRecord{}).
		Where("date >= ? AND date <= ? AND (tags = '' OR tags = '{}')", startDate, endDate).
		Select("SUM(cost)").
		Scan(&untaggedCost)

	untaggedPercent := float64(0)
	if totalCost > 0 {
		untaggedPercent = (untaggedCost / totalCost) * 100
	}

	// Get top services
	topServices, err := s.GetTopCosts(limit, "service", startDate, endDate)
	if err != nil {
		return fmt.Errorf("failed to get top services: %w", err)
	}
	topServicesJSON, err := json.Marshal(topServices)
	if err != nil {
		return fmt.Errorf("failed to marshal top services: %w", err)
	}

	// Get top accounts
	topAccounts, err := s.GetTopCosts(limit, "account", startDate, endDate)
	if err != nil {
		return fmt.Errorf("failed to get top accounts: %w", err)
	}
	topAccountsJSON, err := json.Marshal(topAccounts)
	if err != nil {
		return fmt.Errorf("failed to marshal top accounts: %w", err)
	}

	// Get top regions
	topRegions, err := s.GetTopCosts(limit, "region", startDate, endDate)
	if err != nil {
		return fmt.Errorf("failed to get top regions: %w", err)
	}
	topRegionsJSON, err := json.Marshal(topRegions)
	if err != nil {
		return fmt.Errorf("failed to marshal top regions: %w", err)
	}

	// Infer team ownership
	costByTeam := s.InferTeamOwnership(startDate, endDate)
	sort.Slice(costByTeam, func(i, j int) bool { return costByTeam[i].TotalCost > costByTeam[j].TotalCost })
	if len(costByTeam) > limit {
		costByTeam = costByTeam[:limit]
	}
	costByTeamJSON, err := json.Marshal(costByTeam)
	if err != nil {
		return fmt.Errorf("failed to marshal cost by team: %w", err)
	}

	// Generate insights
	insights := s.GenerateInsights(totalCost, untaggedPercent, topServices)
	insightsJSON, err := json.Marshal(insights)
	if err != nil {
		return fmt.Errorf("failed to marshal insights: %w", err)
	}

	// Save analysis
	analysis := models.CostAnalysis{
		Date:            endDate,
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

func (s *AnalysisService) GetTopCosts(limit int, groupBy string, startDate, endDate time.Time) ([]models.CostSummary, error) {
	var results []models.CostSummary

	query := s.db.Model(&models.CostRecord{}).
		Select(fmt.Sprintf("%s, SUM(cost) as total_cost", groupBy)).
		Where("date >= ? AND date <= ?", startDate, endDate).
		Group(groupBy).
		Order("total_cost DESC").
		Limit(limit)

	if err := query.Scan(&results).Error; err != nil {
		return nil, fmt.Errorf("failed to get top costs: %w", err)
	}

	return results, nil
}

func (s *AnalysisService) InferTeamOwnership(startDate, endDate time.Time) []models.CostSummary {
	var costRecords []models.CostRecord
	s.db.Where("date >= ? AND date <= ?", startDate, endDate).Find(&costRecords)

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
	// Tags are stored as a JSON object string. Unmarshal once per record
	// to avoid repeated parsing and to enable exact value comparisons
	// against TeamMapping.Pattern when PatternType is "tag".
	var tags map[string]string
	if err := json.Unmarshal([]byte(record.Tags), &tags); err != nil {
		tags = map[string]string{}
	}

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
			for _, v := range tags {
				if v == mapping.Pattern {
					return mapping.Team
				}
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
	if err := s.db.Order("id desc").First(&analysis).Error; err != nil {
		return AnalysisResult{}, fmt.Errorf("failed to fetch latest analysis: %w", err)
	}

	result := AnalysisResult{
		TotalCost:       analysis.TotalCost,
		UntaggedCost:    analysis.UntaggedCost,
		UntaggedPercent: analysis.UntaggedPercent,
	}

	_ = json.Unmarshal([]byte(analysis.TopServices), &result.TopServices)
	_ = json.Unmarshal([]byte(analysis.TopAccounts), &result.TopAccounts)
	_ = json.Unmarshal([]byte(analysis.TopRegions), &result.TopRegions)
	_ = json.Unmarshal([]byte(analysis.CostByTeam), &result.CostByTeam)
	_ = json.Unmarshal([]byte(analysis.Insights), &result.Insights)

	return result, nil
}
