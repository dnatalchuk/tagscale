package services_test

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"tagscale/internal/models"
	"tagscale/internal/services"
)

func setupAnalysisDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.CostRecord{}, &models.TeamMapping{}, &models.CostAnalysis{}))
	return db
}

func TestInferTeamOwnershipRegex(t *testing.T) {
	db := setupAnalysisDB(t)

	now := time.Now()
	recs := []models.CostRecord{
		{Date: now, Service: "AmazonEC2", ResourceID: "i-1", Cost: 1},
		{Date: now, Service: "AmazonS3", ResourceID: "bucket-1", Cost: 2},
		{Date: now, Service: "Other", ResourceID: "res", Cost: 3},
	}
	require.NoError(t, db.Create(&recs).Error)

	mappings := []models.TeamMapping{
		{Pattern: "^AmazonEC2$", PatternType: "service", Team: "compute"},
		{Pattern: "^bucket-", PatternType: "resource_name", Team: "storage"},
	}
	require.NoError(t, db.Create(&mappings).Error)

	svc := services.NewAnalysisService(db)
	results := svc.InferTeamOwnership(now.Add(-time.Hour), now.Add(time.Hour))

	// Convert results to map for easy lookup
	costs := make(map[string]float64)
	for _, r := range results {
		costs[r.Team] = r.TotalCost
	}

	require.InDelta(t, 1.0, costs["compute"], 0.001)
	require.InDelta(t, 2.0, costs["storage"], 0.001)
	require.InDelta(t, 3.0, costs["unassigned"], 0.001)
}

func TestInferTeamOwnershipTagExact(t *testing.T) {
	db := setupAnalysisDB(t)

	now := time.Now()
	tagJSON, _ := json.Marshal(map[string]string{"Team": "platform"})
	recs := []models.CostRecord{
		{Date: now, Tags: string(tagJSON), Cost: 1},
	}
	require.NoError(t, db.Create(&recs).Error)

	mappings := []models.TeamMapping{
		{Pattern: "platform", PatternType: "tag", Team: "plat"},
	}
	require.NoError(t, db.Create(&mappings).Error)

	svc := services.NewAnalysisService(db)
	results := svc.InferTeamOwnership(now.Add(-time.Hour), now.Add(time.Hour))

	costs := make(map[string]float64)
	for _, r := range results {
		costs[r.Team] = r.TotalCost
	}

	require.InDelta(t, 1.0, costs["plat"], 0.001)
}

func TestInferTeamOwnershipTagPartial(t *testing.T) {
	db := setupAnalysisDB(t)

	now := time.Now()
	tagJSON, _ := json.Marshal(map[string]string{"Team": "platform"})
	recs := []models.CostRecord{
		{Date: now, Tags: string(tagJSON), Cost: 1},
	}
	require.NoError(t, db.Create(&recs).Error)

	mappings := []models.TeamMapping{
		{Pattern: "plat", PatternType: "tag", Team: "plat"},
	}
	require.NoError(t, db.Create(&mappings).Error)

	svc := services.NewAnalysisService(db)
	results := svc.InferTeamOwnership(now.Add(-time.Hour), now.Add(time.Hour))

	costs := make(map[string]float64)
	for _, r := range results {
		costs[r.Team] = r.TotalCost
	}

	require.NotContains(t, costs, "plat")
	require.InDelta(t, 1.0, costs["unassigned"], 0.001)
}

func BenchmarkInferTeamOwnership(b *testing.B) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		b.Fatalf("failed to open db: %v", err)
	}
	if err := db.AutoMigrate(&models.CostRecord{}, &models.TeamMapping{}); err != nil {
		b.Fatalf("migrate: %v", err)
	}

	// Create a large number of cost records
	now := time.Now()
	var recs []models.CostRecord
	for i := 0; i < 10000; i++ {
		recs = append(recs, models.CostRecord{
			Date:       now,
			Service:    fmt.Sprintf("svc-%d", i%10),
			ResourceID: fmt.Sprintf("res-%d", i),
			Cost:       1,
		})
	}
	if err := db.CreateInBatches(recs, 100).Error; err != nil {
		b.Fatalf("create records: %v", err)
	}

	mappings := []models.TeamMapping{
		{Pattern: "^svc-0$", PatternType: "service", Team: "team0"},
		{Pattern: "^res-", PatternType: "resource_name", Team: "teamres"},
	}
	if err := db.Create(&mappings).Error; err != nil {
		b.Fatalf("create mappings: %v", err)
	}

	svc := services.NewAnalysisService(db)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		svc.InferTeamOwnership(now.Add(-time.Hour), now.Add(time.Hour))
	}
}

func TestRunAnalysisHonorsRange(t *testing.T) {
	db := setupAnalysisDB(t)

	now := time.Now().Truncate(24 * time.Hour)
	recs := []models.CostRecord{
		{Date: now.AddDate(0, 0, -1), Service: "In", Cost: 5, Tags: "{}"},
		{Date: now.AddDate(0, 0, -10), Service: "Out", Cost: 20, Tags: "{}"},
	}
	require.NoError(t, db.Create(&recs).Error)

	svc := services.NewAnalysisService(db)
	require.NoError(t, svc.RunAnalysis(5, now.AddDate(0, 0, -7), now))

	var analysis models.CostAnalysis
	require.NoError(t, db.Last(&analysis).Error)
	require.InDelta(t, 5.0, analysis.TotalCost, 0.001)
}
