package services_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"tagscale/internal/models"
	"tagscale/internal/services"
)

func setupAnalysisDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.CostRecord{}, &models.TeamMapping{}))
	return db
}

func TestInferTeamOwnershipRegex(t *testing.T) {
	db := setupAnalysisDB(t)

	recs := []models.CostRecord{
		{Service: "AmazonEC2", ResourceID: "i-1", Cost: 1},
		{Service: "AmazonS3", ResourceID: "bucket-1", Cost: 2},
		{Service: "Other", ResourceID: "res", Cost: 3},
	}
	require.NoError(t, db.Create(&recs).Error)

	mappings := []models.TeamMapping{
		{Pattern: "^AmazonEC2$", PatternType: "service", Team: "compute"},
		{Pattern: "^bucket-", PatternType: "resource_name", Team: "storage"},
	}
	require.NoError(t, db.Create(&mappings).Error)

	svc := services.NewAnalysisService(db)
	results := svc.InferTeamOwnership()

	// Convert results to map for easy lookup
	costs := make(map[string]float64)
	for _, r := range results {
		costs[r.Team] = r.TotalCost
	}

	require.InDelta(t, 1.0, costs["compute"], 0.001)
	require.InDelta(t, 2.0, costs["storage"], 0.001)
	require.InDelta(t, 3.0, costs["unassigned"], 0.001)
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
	var recs []models.CostRecord
	for i := 0; i < 10000; i++ {
		recs = append(recs, models.CostRecord{
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
		svc.InferTeamOwnership()
	}
}
