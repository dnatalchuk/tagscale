package main

import (
	"bytes"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"tagscale/internal/models"
	"tagscale/internal/services"
)

func TestPrintAnalysisSkipsEmptySections(t *testing.T) {
	result := services.AnalysisResult{
		StartDate:       time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC),
		EndDate:         time.Date(2023, 1, 31, 0, 0, 0, 0, time.UTC),
		TotalCost:       100,
		UntaggedCost:    10,
		UntaggedPercent: 10,
		TopServices: []models.CostSummary{
			{Service: "AmazonEC2", TotalCost: 50, Percentage: 50},
		},
		TopAccounts: []models.CostSummary{},
	}

	var buf bytes.Buffer

	printAnalysis(&buf, result, []string{"service", "account"}, false, false)

	output := buf.String()

	require.Contains(t, output, "Top Services:")
	require.NotContains(t, output, "Top Accounts:")
}

func TestPrintAnalysisShowsDateRange(t *testing.T) {
	result := services.AnalysisResult{
		StartDate: time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC),
		EndDate:   time.Date(2023, 1, 7, 0, 0, 0, 0, time.UTC),
	}

	var buf bytes.Buffer

	printAnalysis(&buf, result, nil, false, false)

	output := buf.String()

	require.Contains(t, output, "2023-01-01")
	require.Contains(t, output, "2023-01-07")
}
