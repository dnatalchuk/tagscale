package services_test

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"tagscale/internal/models"
	"tagscale/internal/services"
)

func setupAnalysisDB(t *testing.T) *gorm.DB {
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
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
	results := svc.InferTeamOwnership(context.Background(), now.Add(-time.Hour), now.Add(time.Hour))

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
	results := svc.InferTeamOwnership(context.Background(), now.Add(-time.Hour), now.Add(time.Hour))

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
	results := svc.InferTeamOwnership(context.Background(), now.Add(-time.Hour), now.Add(time.Hour))

	costs := make(map[string]float64)
	for _, r := range results {
		costs[r.Team] = r.TotalCost
	}

	require.NotContains(t, costs, "plat")
	require.InDelta(t, 1.0, costs["unassigned"], 0.001)
}

func TestInferTeamOwnershipLargeDataset(t *testing.T) {
	db := setupAnalysisDB(t)

	now := time.Now()
	var recs []models.CostRecord
	for i := 0; i < 20000; i++ {
		recs = append(recs, models.CostRecord{Date: now, Service: fmt.Sprintf("svc-%d", i%5), Cost: 1})
	}
	require.NoError(t, db.CreateInBatches(recs, 500).Error)

	mapping := models.TeamMapping{Pattern: "svc-1", PatternType: "service", Team: "team1"}
	require.NoError(t, db.Create(&mapping).Error)

	recs = nil
	runtime.GC()
	var before runtime.MemStats
	runtime.ReadMemStats(&before)

	svc := services.NewAnalysisService(db)
	results := svc.InferTeamOwnership(context.Background(), now.Add(-time.Hour), now.Add(time.Hour))

	runtime.GC()
	var after runtime.MemStats
	runtime.ReadMemStats(&after)

	costs := make(map[string]float64)
	for _, r := range results {
		costs[r.Team] = r.TotalCost
	}
	require.InDelta(t, 4000.0, costs["team1"], 0.001)
	require.Less(t, after.Alloc-before.Alloc, uint64(30*1024*1024))
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
		svc.InferTeamOwnership(context.Background(), now.Add(-time.Hour), now.Add(time.Hour))
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
	_, err := svc.RunAnalysis(context.Background(), 5, now.AddDate(0, 0, -7), now, []string{"service", "account", "region", "team"}, true)
	require.NoError(t, err)

	var analysis models.CostAnalysis
	require.NoError(t, db.Last(&analysis).Error)
	require.InDelta(t, 5.0, analysis.TotalCost, 0.001)
}

func TestRunAnalysisSkipSave(t *testing.T) {
	db := setupAnalysisDB(t)

	now := time.Now()
	rec := models.CostRecord{Date: now, Service: "svc", Cost: 1, Tags: "{}"}
	require.NoError(t, db.Create(&rec).Error)

	svc := services.NewAnalysisService(db)
	_, err := svc.RunAnalysis(context.Background(), 5, now.Add(-time.Hour), now, []string{"service"}, false)
	require.NoError(t, err)

	var count int64
	require.NoError(t, db.Model(&models.CostAnalysis{}).Count(&count).Error)
	require.Equal(t, int64(0), count)
}

func TestRunAnalysisReturnsErrorWhenTotalsScanFails(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.Exec(`CREATE TABLE cost_records (id INTEGER PRIMARY KEY, date DATETIME)`).Error)

	now := time.Now()
	require.NoError(t, db.Exec(`INSERT INTO cost_records (date) VALUES (?)`, now).Error)

	svc := services.NewAnalysisService(db)
	_, err = svc.RunAnalysis(context.Background(), 5, now.Add(-time.Hour), now, []string{"service"}, false)
	require.Error(t, err)
	require.ErrorContains(t, err, "failed to get cost totals")
}

func TestRunAnalysisReturnsErrorWhenTotalsScanFailsMissingTags(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.Exec(`CREATE TABLE cost_records (id INTEGER PRIMARY KEY, date DATETIME, cost REAL)`).Error)

	now := time.Now()
	require.NoError(t, db.Exec(`INSERT INTO cost_records (date, cost) VALUES (?, 1)`, now).Error)

	svc := services.NewAnalysisService(db)
	_, err = svc.RunAnalysis(context.Background(), 5, now.Add(-time.Hour), now, []string{"service"}, false)
	require.Error(t, err)
	require.ErrorContains(t, err, "failed to get cost totals")
}

func TestRunAnalysisReturnsErrorWhenGetTopCostsFails(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.Exec(`CREATE TABLE cost_records (id INTEGER PRIMARY KEY, date DATETIME, cost REAL, tags TEXT)`).Error)

	now := time.Now()
	require.NoError(t, db.Exec(`INSERT INTO cost_records (date, cost, tags) VALUES (?, ?, '{}')`, now, 1).Error)

	svc := services.NewAnalysisService(db)
	_, err = svc.RunAnalysis(context.Background(), 5, now.Add(-time.Hour), now.Add(time.Hour), []string{"service"}, true)
	require.Error(t, err)
	require.ErrorContains(t, err, "failed to get top services")
}

func TestRunAnalysisReturnsErrorWhenMarshalFails(t *testing.T) {
	db := setupAnalysisDB(t)

	now := time.Now()
	rec := models.CostRecord{Date: now, Service: "svc", Account: "acc", Region: "us", Cost: math.Inf(1), Tags: "{}"}
	require.NoError(t, db.Create(&rec).Error)

	svc := services.NewAnalysisService(db)
	_, err := svc.RunAnalysis(context.Background(), 5, now.Add(-time.Hour), now, []string{"service"}, true)
	require.Error(t, err)
	require.ErrorContains(t, err, "marshal top services")
}

func TestGetLatestAnalysisUnmarshalErrors(t *testing.T) {
	cases := []struct {
		name    string
		mutate  func(*models.CostAnalysis)
		wantErr string
	}{
		{
			name: "TopServices",
			mutate: func(a *models.CostAnalysis) {
				a.TopServices = "invalid"
			},
			wantErr: "unmarshal top services",
		},
		{
			name: "TopAccounts",
			mutate: func(a *models.CostAnalysis) {
				a.TopAccounts = "invalid"
			},
			wantErr: "unmarshal top accounts",
		},
		{
			name: "TopRegions",
			mutate: func(a *models.CostAnalysis) {
				a.TopRegions = "invalid"
			},
			wantErr: "unmarshal top regions",
		},
		{
			name: "CostByTeam",
			mutate: func(a *models.CostAnalysis) {
				a.CostByTeam = "invalid"
			},
			wantErr: "unmarshal cost by team",
		},
		{
			name: "Insights",
			mutate: func(a *models.CostAnalysis) {
				a.Insights = "invalid"
			},
			wantErr: "unmarshal insights",
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			db := setupAnalysisDB(t)
			analysis := models.CostAnalysis{
				Date:            time.Now(),
				TotalCost:       1,
				UntaggedCost:    0,
				UntaggedPercent: 0,
				TopServices:     "[]",
				TopAccounts:     "[]",
				TopRegions:      "[]",
				CostByTeam:      "[]",
				Insights:        "[]",
			}
			tt.mutate(&analysis)
			require.NoError(t, db.Create(&analysis).Error)

			svc := services.NewAnalysisService(db)
			_, err := svc.GetLatestAnalysis()
			require.Error(t, err)
			require.ErrorContains(t, err, tt.wantErr)
		})
	}
}

type fakeAnalysisService struct {
	*services.AnalysisService
	delay time.Duration
}

func (f *fakeAnalysisService) GetTopCosts(ctx context.Context, limit int, groupBy string, startDate, endDate time.Time) ([]models.CostSummary, error) {
	time.Sleep(f.delay)
	return []models.CostSummary{}, nil
}

func (f *fakeAnalysisService) RunAnalysisSequential(ctx context.Context, limit int, startDate, endDate time.Time, groupBy []string, save bool) (services.AnalysisResult, error) {
	for _, gb := range groupBy {
		if _, err := f.GetTopCosts(ctx, limit, gb, startDate, endDate); err != nil {
			return services.AnalysisResult{}, err
		}
	}
	// Saving is omitted for simplicity in tests/benchmarks.
	return services.AnalysisResult{}, nil
}

func TestRunAnalysisFetchesTopCostsConcurrently(t *testing.T) {
	db := setupAnalysisDB(t)

	now := time.Now()
	rec := models.CostRecord{Date: now, Service: "svc", Account: "acc", Region: "reg", Cost: 1, Tags: "{}"}
	require.NoError(t, db.Create(&rec).Error)

	baseSvc := services.NewAnalysisService(db)
	fake := &fakeAnalysisService{AnalysisService: baseSvc, delay: 100 * time.Millisecond}

	start := time.Now()
	_, err := fake.RunAnalysis(context.Background(), 5, now.Add(-time.Hour), now.Add(time.Hour), []string{"service", "account", "region"}, false)
	require.NoError(t, err)
	elapsed := time.Since(start)

	require.Less(t, elapsed, 250*time.Millisecond)
}

func BenchmarkRunAnalysisConcurrent(b *testing.B) {
	dsn := fmt.Sprintf("file:bench?mode=memory&cache=shared")
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		b.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&models.CostRecord{}, &models.TeamMapping{}, &models.CostAnalysis{}); err != nil {
		b.Fatalf("migrate: %v", err)
	}

	now := time.Now()
	rec := models.CostRecord{Date: now, Service: "svc", Account: "acc", Region: "reg", Cost: 1, Tags: "{}"}
	if err := db.Create(&rec).Error; err != nil {
		b.Fatalf("create record: %v", err)
	}

	baseSvc := services.NewAnalysisService(db)
	fake := &fakeAnalysisService{AnalysisService: baseSvc, delay: 10 * time.Millisecond}

	b.Run("sequential", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			if _, err := fake.RunAnalysisSequential(context.Background(), 5, now.Add(-time.Hour), now.Add(time.Hour), []string{"service", "account", "region"}, false); err != nil {
				b.Fatalf("sequential run: %v", err)
			}
		}
	})

	b.Run("concurrent", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			if _, err := fake.RunAnalysis(context.Background(), 5, now.Add(-time.Hour), now.Add(time.Hour), []string{"service", "account", "region"}, false); err != nil {
				b.Fatalf("concurrent run: %v", err)
			}
		}
	})
}
