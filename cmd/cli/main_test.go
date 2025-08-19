package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/costexplorer"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"tagscale/internal/config"
	"tagscale/internal/models"
	"tagscale/internal/services"
)

type mockQuietAWSClient struct{}

func (m *mockQuietAWSClient) GetCostAndUsage(ctx context.Context, startDate, endDate time.Time, nextToken *string) (*costexplorer.GetCostAndUsageOutput, error) {
	return &costexplorer.GetCostAndUsageOutput{}, nil
}

func TestCliDBPathPermissions(t *testing.T) {
	tmp := t.TempDir()
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmp)
	defer os.Setenv("HOME", oldHome)

	path, err := cliDBPath()
	require.NoError(t, err)
	expected := filepath.Join(tmp, ".tagscale", "cli.db")
	require.Equal(t, expected, path)

	info, err := os.Stat(filepath.Dir(path))
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0o700), info.Mode().Perm())
}

func TestRunSummaryRunsMigrationsWithFlag(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "cli.db")
	os.Setenv("DATABASE_URL", "sqlite://"+dbPath)
	defer os.Unsetenv("DATABASE_URL")

	cfg, err := config.Load()
	require.NoError(t, err)
	require.NoError(t, runSummary(cfg, "30", true, true, "table", "service", 5, true, false))

	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	require.NoError(t, err)
	require.True(t, db.Migrator().HasTable(&models.CostRecord{}))
	require.True(t, db.Migrator().HasTable(&models.CostAnalysis{}))
	require.True(t, db.Migrator().HasTable(&models.TeamMapping{}))
}

func TestRunSummaryClosesDB(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "cli.db")
	os.Setenv("DATABASE_URL", "sqlite://"+dbPath)
	defer os.Unsetenv("DATABASE_URL")

	cfg, err := config.Load()
	require.NoError(t, err)
	require.NoError(t, runSummary(cfg, "30", true, true, "table", "service", 5, true, false))

	fds, err := os.ReadDir("/proc/self/fd")
	require.NoError(t, err)
	for _, fd := range fds {
		link, err := os.Readlink(filepath.Join("/proc/self/fd", fd.Name()))
		if err != nil {
			continue
		}
		require.NotContains(t, link, dbPath)
	}
}

func TestRunSummaryQuietModeNoOutput(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "cli.db")
	os.Setenv("DATABASE_URL", "sqlite://"+dbPath)
	defer os.Unsetenv("DATABASE_URL")

	cfg, err := config.Load()
	require.NoError(t, err)

	r, w, err := os.Pipe()
	require.NoError(t, err)
	old := os.Stdout
	os.Stdout = w

	err = runSummary(cfg, "30", true, true, "table", "service", 5, true, false)
	w.Close()
	os.Stdout = old
	require.NoError(t, err)

	out, err := io.ReadAll(r)
	require.NoError(t, err)
	require.Empty(t, strings.TrimSpace(string(out)))
}

func TestRunScanQuietModeNoOutput(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "cli.db")
	os.Setenv("DATABASE_URL", "sqlite://"+dbPath)
	defer os.Unsetenv("DATABASE_URL")

	cfg, err := config.Load()
	require.NoError(t, err)

	mockClient := &mockQuietAWSClient{}
	origFactory := awsClientFactory
	awsClientFactory = func(ctx context.Context, region, profile string) (services.CostExplorerAPI, error) {
		return mockClient, nil
	}
	defer func() { awsClientFactory = origFactory }()

	r, w, err := os.Pipe()
	require.NoError(t, err)
	old := os.Stdout
	os.Stdout = w

	err = runScan(context.Background(), cfg, "1", true, true, "", "", "table", 30, true, false)
	w.Close()
	os.Stdout = old
	require.NoError(t, err)

	out, err := io.ReadAll(r)
	require.NoError(t, err)
	require.Empty(t, strings.TrimSpace(string(out)))
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

func TestSummaryGroupByValidation(t *testing.T) {
	cli := NewCLI()
	summaryCmd, _, err := cli.Find([]string{"summary"})
	require.NoError(t, err)
	summaryCmd.RunE = func(cmd *cobra.Command, args []string) error { return nil }

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

func TestSummaryLimitValidation(t *testing.T) {
	cli := NewCLI()
	summaryCmd, _, err := cli.Find([]string{"summary"})
	require.NoError(t, err)
	summaryCmd.RunE = func(cmd *cobra.Command, args []string) error { return nil }

	// valid limit
	cli.SetArgs([]string{"summary", "--limit", "1"})
	require.NoError(t, cli.Execute())

	cases := []string{"0", "-5"}
	for _, v := range cases {
		cli.SetArgs([]string{"summary", "--limit", v})
		var outBuf, errBuf bytes.Buffer
		cli.SetOut(&outBuf)
		cli.SetErr(&errBuf)
		err = cli.Execute()
		require.Error(t, err)
		require.Contains(t, err.Error(), "limit must be greater than 0")
		combined := outBuf.String() + errBuf.String()
		require.Contains(t, combined, "Usage:")
	}
}

func TestCLIQuietVerboseConflict(t *testing.T) {
	cli := NewCLI()
	cli.AddCommand(&cobra.Command{Use: "noop", Run: func(cmd *cobra.Command, args []string) {}})
	cli.SetArgs([]string{"noop", "--quiet", "--verbose"})
	err := cli.Execute()
	require.Error(t, err)
	require.Contains(t, err.Error(), "cannot use --quiet and --verbose together")
}

func TestCLIScanInvalidTimeout(t *testing.T) {
	cases := []string{"0", "-5"}
	for _, tt := range cases {
		cli := NewCLI()
		cli.SetArgs([]string{"scan", "--timeout", tt, "--quiet"})
		err := cli.Execute()
		require.Error(t, err)
		require.Contains(t, err.Error(), "timeout must be greater than 0")
	}
}

func TestCLIVersion(t *testing.T) {
	cases := []struct {
		args     []string
		expected string
	}{
		{[]string{"version"}, Version + "\n"},
		{[]string{"version", "--output", "json"}, fmt.Sprintf("{\"version\":\"%s\"}\n", Version)},
		{[]string{"--version"}, Version + "\n"},
	}

	for _, tt := range cases {
		cli := NewCLI()
		var out bytes.Buffer
		cli.SetOut(&out)
		cli.SetArgs(tt.args)
		require.NoError(t, cli.Execute())
		require.Equal(t, tt.expected, out.String())
	}
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

	cfg, err := config.Load()
	require.NoError(t, err)

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

			require.NoError(t, runSummary(cfg, "30", true, false, "table", tt.group, tt.limit, false, false))

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

func TestRunSummaryHonorsRange(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "cli.db")
	os.Setenv("DATABASE_URL", "sqlite://"+dbPath)
	defer os.Unsetenv("DATABASE_URL")

	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.CostRecord{}, &models.CostAnalysis{}, &models.TeamMapping{}))

	now := time.Now().Truncate(24 * time.Hour)
	recs := []models.CostRecord{
		{Date: now.AddDate(0, 0, -1), Service: "InRange", Cost: 10, Tags: "{}"},
		{Date: now.AddDate(0, 0, -10), Service: "OutRange", Cost: 20, Tags: "{}"},
	}
	require.NoError(t, db.Create(&recs).Error)

	cfg, err := config.Load()
	require.NoError(t, err)

	r, w, err := os.Pipe()
	require.NoError(t, err)
	old := os.Stdout
	os.Stdout = w

	require.NoError(t, runSummary(cfg, "7", true, false, "table", "service", 5, false, false))

	w.Close()
	os.Stdout = old
	out, err := io.ReadAll(r)
	require.NoError(t, err)
	output := string(out)

	require.Contains(t, output, "InRange")
	require.NotContains(t, output, "OutRange")
}
