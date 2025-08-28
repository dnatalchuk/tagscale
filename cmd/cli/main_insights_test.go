package main

import (
	"bytes"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"tagscale/internal/models"
	"tagscale/internal/services"
)

func TestPrintAnalysisNoInsightsHeader(t *testing.T) {
	result := services.AnalysisResult{
		StartDate:       time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC),
		EndDate:         time.Date(2023, 1, 31, 0, 0, 0, 0, time.UTC),
		TotalCost:       100,
		UntaggedCost:    10,
		UntaggedPercent: 10,
		TopServices: []models.CostSummary{
			{Service: "AmazonEC2", TotalCost: 50, Percentage: 50},
		},
		Insights: []string{},
	}

	var buf bytes.Buffer

	printAnalysis(&buf, result, []string{"service"}, false, false)

	output := buf.String()

	require.NotContains(t, output, "Insights:")
}
