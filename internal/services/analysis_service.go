// internal/services/analysis_service.go
package services

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"tagscale/internal/models"

	"golang.org/x/sync/errgroup"
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

func (s *AnalysisService) RunAnalysis(limit int, startDate, endDate time.Time, groupBy []string, save bool) (AnalysisResult, error) {
	var result AnalysisResult
	// Get total cost
	var totalCostDB sql.NullFloat64
	db := s.db.Model(&models.CostRecord{}).
		Where("date >= ? AND date <= ?", startDate, endDate).
		Select("SUM(cost)").
		Scan(&totalCostDB)
	if db.Error != nil {
		return result, fmt.Errorf("failed to get total cost: %w", db.Error)
	}

	totalCost := float64(0)
	if totalCostDB.Valid {
		totalCost = totalCostDB.Float64
	}

	// Get untagged cost (assuming empty or "{}" tags means untagged)
	var untaggedCostDB sql.NullFloat64
	db = s.db.Model(&models.CostRecord{}).
		Where("date >= ? AND date <= ? AND (tags = '' OR tags = '{}')", startDate, endDate).
		Select("SUM(cost)").
		Scan(&untaggedCostDB)
	if db.Error != nil {
		return result, fmt.Errorf("failed to get untagged cost: %w", db.Error)
	}

	untaggedCost := float64(0)
	if untaggedCostDB.Valid {
		untaggedCost = untaggedCostDB.Float64
	}

	untaggedPercent := float64(0)
	if totalCost > 0 {
		untaggedPercent = (untaggedCost / totalCost) * 100
	}

	var (
		topServices []models.CostSummary
		topAccounts []models.CostSummary
		topRegions  []models.CostSummary
		costByTeam  []models.CostSummary
	)

	g := new(errgroup.Group)

	for _, gb := range groupBy {
		gb := gb // capture loop variable
		switch gb {
		case "service", "account", "region":
			g.Go(func() error {
				top, err := s.GetTopCosts(limit, gb, startDate, endDate)
				if err != nil {
					var label string
					switch gb {
					case "service":
						label = "services"
					case "account":
						label = "accounts"
					case "region":
						label = "regions"
					}
					return fmt.Errorf("failed to get top %s: %w", label, err)
				}
				switch gb {
				case "service":
					topServices = top
				case "account":
					topAccounts = top
				case "region":
					topRegions = top
				}
				return nil
			})
		case "team":
			// handled after waiting
		}
	}

	if err := g.Wait(); err != nil {
		return result, err
	}

	// Infer team ownership if requested
	for _, gb := range groupBy {
		if gb == "team" {
			costByTeam = s.InferTeamOwnership(startDate, endDate)
			sort.Slice(costByTeam, func(i, j int) bool { return costByTeam[i].TotalCost > costByTeam[j].TotalCost })
			if len(costByTeam) > limit {
				costByTeam = costByTeam[:limit]
			}
			break
		}
	}

	// Generate insights
	insights := s.GenerateInsights(totalCost, untaggedPercent, topServices)

	result = AnalysisResult{
		TotalCost:       totalCost,
		UntaggedCost:    untaggedCost,
		UntaggedPercent: untaggedPercent,
		TopServices:     topServices,
		TopAccounts:     topAccounts,
		TopRegions:      topRegions,
		CostByTeam:      costByTeam,
		Insights:        insights,
	}

	if !save {
		return result, nil
	}

	topServicesJSON, err := json.Marshal(topServices)
	if err != nil {
		return AnalysisResult{}, fmt.Errorf("failed to marshal top services: %w", err)
	}
	topAccountsJSON, err := json.Marshal(topAccounts)
	if err != nil {
		return AnalysisResult{}, fmt.Errorf("failed to marshal top accounts: %w", err)
	}
	topRegionsJSON, err := json.Marshal(topRegions)
	if err != nil {
		return AnalysisResult{}, fmt.Errorf("failed to marshal top regions: %w", err)
	}
	costByTeamJSON, err := json.Marshal(costByTeam)
	if err != nil {
		return AnalysisResult{}, fmt.Errorf("failed to marshal cost by team: %w", err)
	}
	insightsJSON, err := json.Marshal(insights)
	if err != nil {
		return AnalysisResult{}, fmt.Errorf("failed to marshal insights: %w", err)
	}

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
		return AnalysisResult{}, fmt.Errorf("failed to save analysis: %w", err)
	}

	return result, nil
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
	// Get team mappings ordered by priority so higher priority mappings win
	var teamMappings []models.TeamMapping
	s.db.Order("priority DESC").Find(&teamMappings)

	var simpleIDs []uint
	compiled := make([]compiledTeamMapping, 0, len(teamMappings))
	for _, m := range teamMappings {
		cm := compiledTeamMapping{TeamMapping: m}
		switch m.PatternType {
		case "service":
			if isRegex(m.Pattern) {
				if re, err := regexp.Compile(m.Pattern); err == nil {
					cm.pattern = re
				}
				compiled = append(compiled, cm)
			} else {
				simpleIDs = append(simpleIDs, m.ID)
			}
		case "resource_name":
			if re, err := regexp.Compile(m.Pattern); err == nil {
				cm.pattern = re
			}
			compiled = append(compiled, cm)
		case "tag":
			simpleIDs = append(simpleIDs, m.ID)
		}
	}

	teamCosts := make(map[string]float64)

	// Aggregate costs using SQL for simple mappings
	if len(simpleIDs) > 0 {
		query := `
SELECT
    COALESCE((
        SELECT tm.team
        FROM team_mappings tm
        WHERE tm.id IN ? AND (
            (tm.pattern_type = 'service' AND cr.service = tm.pattern) OR
            (tm.pattern_type = 'tag' AND EXISTS (SELECT 1 FROM json_each(cr.tags) WHERE value = tm.pattern))
        )
        ORDER BY tm.priority DESC
        LIMIT 1
    ), 'unassigned') AS team,
    SUM(cr.cost) AS total_cost
FROM cost_records cr
WHERE cr.date >= ? AND cr.date <= ?
GROUP BY team`

		var sqlResults []models.CostSummary
		if err := s.db.Raw(query, simpleIDs, startDate, endDate).Scan(&sqlResults).Error; err == nil {
			for _, r := range sqlResults {
				teamCosts[r.Team] += r.TotalCost
			}
		}
	} else {
		// No simple mappings, calculate total cost for unassigned baseline
		var total float64
		s.db.Model(&models.CostRecord{}).
			Where("date >= ? AND date <= ?", startDate, endDate).
			Select("SUM(cost)").Scan(&total)
		teamCosts["unassigned"] = total
	}

	// Process records that require in-memory evaluation
	if len(compiled) > 0 {
		var remaining []models.CostRecord
		if len(simpleIDs) > 0 {
			sub := `
NOT EXISTS (
    SELECT 1
    FROM team_mappings tm
    WHERE tm.id IN ? AND (
        (tm.pattern_type = 'service' AND cost_records.service = tm.pattern) OR
        (tm.pattern_type = 'tag' AND EXISTS (SELECT 1 FROM json_each(cost_records.tags) WHERE value = tm.pattern))
    )
)`
			s.db.Where("date >= ? AND date <= ?", startDate, endDate).
				Where(sub, simpleIDs).Find(&remaining)
		} else {
			s.db.Where("date >= ? AND date <= ?", startDate, endDate).Find(&remaining)
		}

		for _, record := range remaining {
			team := s.inferTeamFromRecord(record, compiled)
			teamCosts[team] += record.Cost
			teamCosts["unassigned"] -= record.Cost
		}
	}

	var results []models.CostSummary
	for team, cost := range teamCosts {
		if cost == 0 {
			continue
		}
		results = append(results, models.CostSummary{Team: team, TotalCost: cost})
	}

	return results
}

// isRegex reports whether the pattern contains regex metacharacters.
func isRegex(p string) bool {
	return strings.ContainsAny(p, ".*[]^$+?|()")
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

	if err := json.Unmarshal([]byte(analysis.TopServices), &result.TopServices); err != nil {
		return AnalysisResult{}, fmt.Errorf("failed to unmarshal top services: %w", err)
	}
	if err := json.Unmarshal([]byte(analysis.TopAccounts), &result.TopAccounts); err != nil {
		return AnalysisResult{}, fmt.Errorf("failed to unmarshal top accounts: %w", err)
	}
	if err := json.Unmarshal([]byte(analysis.TopRegions), &result.TopRegions); err != nil {
		return AnalysisResult{}, fmt.Errorf("failed to unmarshal top regions: %w", err)
	}
	if err := json.Unmarshal([]byte(analysis.CostByTeam), &result.CostByTeam); err != nil {
		return AnalysisResult{}, fmt.Errorf("failed to unmarshal cost by team: %w", err)
	}
	if err := json.Unmarshal([]byte(analysis.Insights), &result.Insights); err != nil {
		return AnalysisResult{}, fmt.Errorf("failed to unmarshal insights: %w", err)
	}

	return result, nil
}
