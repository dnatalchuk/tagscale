package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

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

	runSummary(true, true, "table", "service", 5)

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

func TestCLIGroupByValidation(t *testing.T) {
	cli := NewCLI()
	summaryCmd, _, err := cli.Find([]string{"summary"})
	require.NoError(t, err)
	summaryCmd.Run = func(cmd *cobra.Command, args []string) {}

	// valid group-by option
	cli.SetArgs([]string{"summary", "--group-by", "account"})
	require.NoError(t, cli.Execute())

	// invalid group-by option
	cli.SetArgs([]string{"summary", "--group-by", "department"})
	var outBuf, errBuf bytes.Buffer
	cli.SetOut(&outBuf)
	cli.SetErr(&errBuf)
	err = cli.Execute()
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid group-by value")
	combined := outBuf.String() + errBuf.String()
	require.Contains(t, combined, "Usage:")
}

func TestRunSummaryGroupByLimit(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "cli.db")
	os.Setenv("DATABASE_URL", "sqlite://"+dbPath)
	defer os.Unsetenv("DATABASE_URL")

	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.CostRecord{}, &models.CostAnalysis{}, &models.TeamMapping{}))

	yesterday := time.Now().AddDate(0, 0, -1)
	recs := []models.CostRecord{
		{Date: yesterday, Service: "AmazonEC2", Account: "1111", Region: "us-east-1", Cost: 100, Tags: "{}", ResourceID: "i-1"},
		{Date: yesterday, Service: "AmazonS3", Account: "2222", Region: "us-west-2", Cost: 50, Tags: "{}", ResourceID: "bucket-1"},
	}
	require.NoError(t, db.Create(&recs).Error)
	mappings := []models.TeamMapping{
		{Pattern: "^AmazonEC2$", PatternType: "service", Team: "compute"},
		{Pattern: "^bucket-", PatternType: "resource_name", Team: "storage"},
	}
	require.NoError(t, db.Create(&mappings).Error)

	cases := []struct {
		group   string
		limit   int
		include []string
		exclude []string
	}{
		{"service", 1, []string{"AmazonEC2"}, []string{"AmazonS3"}},
		{"account", 2, []string{"1111", "2222"}, nil},
		{"region", 1, []string{"us-east-1"}, []string{"us-west-2"}},
		{"team", 2, []string{"compute", "storage"}, nil},
	}

	for _, tt := range cases {
		t.Run(fmt.Sprintf("%s-%d", tt.group, tt.limit), func(t *testing.T) {
			r, w, err := os.Pipe()
			require.NoError(t, err)
			old := os.Stdout
			os.Stdout = w

			runSummary(true, false, "table", tt.group, tt.limit)

			w.Close()
			os.Stdout = old
			out, err := io.ReadAll(r)
			require.NoError(t, err)
			output := string(out)

			for _, inc := range tt.include {
				require.Contains(t, output, inc)
			}
			for _, exc := range tt.exclude {
				require.NotContains(t, output, exc)
			}
		})
	}
}
