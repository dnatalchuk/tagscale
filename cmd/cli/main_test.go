package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"tagscale/internal/models"
)

func TestRunSummaryRunsMigrationsWithFlag(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "cli.db")
	os.Setenv("DATABASE_URL", "sqlite://"+dbPath)
	defer os.Unsetenv("DATABASE_URL")

	runSummary(true, true, "table")

	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	require.NoError(t, err)
	require.True(t, db.Migrator().HasTable(&models.CostRecord{}))
	require.True(t, db.Migrator().HasTable(&models.CostAnalysis{}))
	require.True(t, db.Migrator().HasTable(&models.TeamMapping{}))
}

func TestCLIOutputFormatValidation(t *testing.T) {
	cli := NewCLI()
	cli.AddCommand(&cobra.Command{Use: "noop", Run: func(cmd *cobra.Command, args []string) {}})

	// valid format
	cli.SetArgs([]string{"noop", "--output", "json"})
	require.NoError(t, cli.Execute())

	// invalid format
	cli.SetArgs([]string{"noop", "--output", "yaml"})
	var outBuf, errBuf bytes.Buffer
	cli.SetOut(&outBuf)
	cli.SetErr(&errBuf)
	err := cli.Execute()
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid output format")
	combined := outBuf.String() + errBuf.String()
	require.Contains(t, combined, "Usage:")
}
