package main

import (
	"io"
	"os"
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

	r, w, err := os.Pipe()
	require.NoError(t, err)
	old := os.Stdout
	os.Stdout = w

	printAnalysis(result, "service", false, false)

	w.Close()
	os.Stdout = old

	out, err := io.ReadAll(r)
	r.Close()
	require.NoError(t, err)
	output := string(out)

	require.NotContains(t, output, "Insights:")
}
