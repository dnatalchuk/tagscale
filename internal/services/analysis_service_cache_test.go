package services

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"tagscale/internal/models"
)

// setupAnalysisDBCache is a copy of setupAnalysisDB from services_test package
// but scoped to the services package so we can access unexported fields.
func setupAnalysisDBCache(t *testing.T) *gorm.DB {
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.CostRecord{}, &models.TeamMapping{}))
	return db
}

func TestCompiledRegexCaching(t *testing.T) {
	db := setupAnalysisDBCache(t)
	now := time.Now()
	recs := []models.CostRecord{
		{Date: now, Service: "AmazonEC2", Cost: 1},
		{Date: now, Service: "AmazonS3", Cost: 2},
	}
	require.NoError(t, db.Create(&recs).Error)

	mapping := models.TeamMapping{Pattern: "^AmazonEC2$", PatternType: "service", Team: "compute"}
	require.NoError(t, db.Create(&mapping).Error)

	svc := NewAnalysisService(db)
	svc.InferTeamOwnership(context.Background(), now.Add(-time.Hour), now.Add(time.Hour))

	first := svc.compiledMappings[mapping.ID]
	require.NotNil(t, first)

	svc.InferTeamOwnership(context.Background(), now.Add(-time.Hour), now.Add(time.Hour))
	second := svc.compiledMappings[mapping.ID]
	require.Equal(t, first, second)
}

func TestCompiledRegexCacheInvalidatedOnChange(t *testing.T) {
	db := setupAnalysisDBCache(t)
	now := time.Now()
	recs := []models.CostRecord{
		{Date: now, Service: "AmazonEC2", Cost: 1},
		{Date: now, Service: "AmazonS3", Cost: 2},
	}
	require.NoError(t, db.Create(&recs).Error)

	mapping := models.TeamMapping{Pattern: "^AmazonEC2$", PatternType: "service", Team: "compute"}
	require.NoError(t, db.Create(&mapping).Error)

	svc := NewAnalysisService(db)
	// Initial population
	svc.InferTeamOwnership(context.Background(), now.Add(-time.Hour), now.Add(time.Hour))
	first := svc.compiledMappings[mapping.ID]
	require.NotNil(t, first)

	// Update mapping pattern
	require.NoError(t, db.Model(&models.TeamMapping{}).Where("id = ?", mapping.ID).Update("pattern", "^AmazonS3$").Error)

	svc.InferTeamOwnership(context.Background(), now.Add(-time.Hour), now.Add(time.Hour))
	second := svc.compiledMappings[mapping.ID]
	require.NotNil(t, second)
	require.NotEqual(t, first, second)

	// Verify that new mapping is used
	results := svc.InferTeamOwnership(context.Background(), now.Add(-time.Hour), now.Add(time.Hour))
	costs := make(map[string]float64)
	for _, r := range results {
		costs[r.Team] = r.TotalCost
	}
	require.InDelta(t, 2.0, costs["compute"], 0.001)
}
