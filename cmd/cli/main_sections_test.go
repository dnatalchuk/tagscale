package main

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"

	"tagscale/internal/models"
	"tagscale/internal/services"
)

func TestPrintAnalysisSkipsEmptySections(t *testing.T) {
	result := services.AnalysisResult{
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
