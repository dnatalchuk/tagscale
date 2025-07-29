package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"tagscale/internal/models"
)

func TestRunSummaryRunsMigrationsWithDB(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "cli.db")
	os.Setenv("DATABASE_URL", "sqlite://"+dbPath)
	defer os.Unsetenv("DATABASE_URL")

	runSummary(true)

	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	require.NoError(t, err)
	require.True(t, db.Migrator().HasTable(&models.CostRecord{}))
	require.True(t, db.Migrator().HasTable(&models.CostAnalysis{}))
	require.True(t, db.Migrator().HasTable(&models.TeamMapping{}))
}
