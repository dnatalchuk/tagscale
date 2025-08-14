package main

import (
	"context"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"tagscale/internal/config"
	"tagscale/internal/services"
)

func TestBackgroundWorkersStopOnContextCancel(t *testing.T) {
	cfg := &config.Config{DataCollectionInterval: 1, AnalysisInterval: 1}

	costSvc := services.NewCostService(nil, nil)
	analysisSvc := services.NewAnalysisService(nil)
	notificationSvc := services.NewNotificationService(&config.Config{}, nil)

	ctx, cancel := context.WithCancel(context.Background())

	baseline := runtime.NumGoroutine()
	startBackgroundWorkers(ctx, costSvc, analysisSvc, notificationSvc, cfg)

	require.Eventually(t, func() bool {
		return runtime.NumGoroutine() >= baseline+3
	}, time.Second, 10*time.Millisecond)

	cancel()

	require.Eventually(t, func() bool {
		return runtime.NumGoroutine() <= baseline+1
	}, time.Second, 10*time.Millisecond)
}
