package main

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"

	"tagscale/internal/models"
	"tagscale/internal/services"
)

func TestPrintAnalysisNoInsightsHeader(t *testing.T) {
	result := services.AnalysisResult{
		TotalCost:       100,
		UntaggedCost:    10,
		UntaggedPercent: 10,
		TopServices: []models.CostSummary{
			{Service: "AmazonEC2", TotalCost: 50, Percentage: 50},
		},
		Insights: []string{},
	}

	var buf bytes.Buffer

	printAnalysis(&buf, result, "service", false, false)

	output := buf.String()

	require.NotContains(t, output, "Insights:")
}
