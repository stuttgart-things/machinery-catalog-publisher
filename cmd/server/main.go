// Command machinery-catalog-publisher streams live resource status from
// machinery and publishes it as Backstage Resource entities to an S3/MinIO
// bucket, which the Git-side Backstage Location points at. It is deployment-
// mode: one config.yaml (ConfigMap) + one connection secret (Secret) → one
// sync. Health is exposed via /metrics and /healthz instead of a CR status.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/stuttgart-things/machinery-catalog-publisher/internal/config"
	"github.com/stuttgart-things/machinery-catalog-publisher/internal/metrics"
	"github.com/stuttgart-things/machinery-catalog-publisher/internal/publisher"
	"github.com/stuttgart-things/machinery-catalog-publisher/internal/sink"
	"github.com/stuttgart-things/machinery-catalog-publisher/internal/source"
)

// Build metadata. Overridden at link time via -ldflags="-X main.version=...".
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})))

	cfg, err := config.Load()
	if err != nil {
		slog.Error("config", "err", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	src, err := source.Dial(cfg.Source.MachineryAddr, cfg.Source.Kinds)
	if err != nil {
		slog.Error("dial machinery", "err", err)
		os.Exit(1)
	}
	defer src.Close()

	snk, err := sink.NewS3(ctx, cfg.S3, cfg.Sink.Bucket)
	if err != nil {
		slog.Error("init s3 sink", "err", err)
		os.Exit(1)
	}

	stats := &metrics.Stats{}
	stats.SetHealthy(true) // until the first resync proves otherwise

	// Metrics/health server — the deployment-mode stand-in for CR status.
	metricsSrv := &http.Server{
		Addr:              cfg.MetricsAddr,
		Handler:           stats.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
	}
	go func() {
		slog.Info("metrics listening", "addr", cfg.MetricsAddr)
		if err := metricsSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("metrics serve", "err", err)
		}
	}()

	pub := publisher.New(src, snk, publisher.Options{
		Owner:           cfg.Owner,
		EntityNamespace: cfg.Sink.EntityNamespace,
		KeyPrefix:       cfg.Sink.KeyPrefix,
		Layout:          cfg.Sink.Layout,
		Resync:          cfg.Interval,
		Now:             func() string { return time.Now().UTC().Format(time.RFC3339) },
	}, stats)

	slog.Info("publisher starting",
		"version", version, "commit", commit, "date", date,
		"machinery", cfg.Source.MachineryAddr, "kinds", cfg.Source.Kinds,
		"bucket", cfg.Sink.Bucket, "endpoint", cfg.S3.Endpoint,
		"layout", cfg.Sink.Layout, "interval", cfg.Interval.String())

	runErr := pub.Run(ctx)

	shutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = metricsSrv.Shutdown(shutCtx)

	if runErr != nil && !errors.Is(runErr, context.Canceled) {
		slog.Error("publisher stopped", "err", runErr)
		os.Exit(1)
	}
	slog.Info("stopped")
}
