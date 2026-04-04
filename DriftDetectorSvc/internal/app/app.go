package app

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"drift-detector-svc/internal/clients/inventory"
	"drift-detector-svc/internal/clients/parser"
	"drift-detector-svc/internal/clients/reconciler"
	"drift-detector-svc/internal/config"
	"drift-detector-svc/internal/cooldown"
	"drift-detector-svc/internal/detectors"
	"drift-detector-svc/internal/domain"
	"drift-detector-svc/internal/logger"
	"drift-detector-svc/internal/scheduler"
	"drift-detector-svc/internal/service/detection"
	httpapi "drift-detector-svc/internal/transport/http"
)

func Run(ctx context.Context, configPath string) error {
	cfg, cfgFilePath, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("load appsettings: %w", err)
	}

	log, err := logger.New(cfg.Logging.Level)
	if err != nil {
		return fmt.Errorf("build logger: %w", err)
	}
	defer func() { _ = log.Sync() }()

	enabledComponents, err := parseEnabledComponents(cfg.Detection.EnabledComponents)
	if err != nil {
		return fmt.Errorf("parse enabled components: %w", err)
	}

	log.Infow("appsettings loaded",
		"path", cfgFilePath,
		"http_host", cfg.Server.Host,
		"http_port", cfg.Server.Port,
		"detection_interval", cfg.Detection.Interval.String(),
		"enabled_components", enabledComponents,
		"reconcile_cooldown", cfg.AntiSpam.ReconcileCooldown.String(),
	)

	inventoryClient := inventory.NewClient(cfg.Clients.Inventory, log)
	parserClient := parser.NewClient(cfg.Clients.Parser, log)
	reconcilerClient := reconciler.NewClient(cfg.Clients.Reconciler, log)

	registry := detectors.NewRegistry(
		detectors.NewNodeExporterDetector(),
		detectors.NewCadvisorDetector(),
	)
	cooldownStore := cooldown.NewStore()

	detectionService := detection.NewService(
		inventoryClient,
		parserClient,
		reconcilerClient,
		registry,
		cooldownStore,
		cfg.AntiSpam.ReconcileCooldown,
		enabledComponents,
		log,
	)

	if _, err := detectionService.RunCycle(ctx, "startup"); err != nil {
		log.Warnw("initial detection cycle failed", "error", err)
	}

	handler := httpapi.NewHandler(detectionService)
	router := httpapi.NewRouter(handler)

	httpServer := &http.Server{
		Addr:              fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port),
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
	}

	detectionScheduler := scheduler.New(cfg.Detection.Interval, func(taskCtx context.Context) error {
		_, runErr := detectionService.RunCycle(taskCtx, "scheduler")
		if errors.Is(runErr, detection.ErrCycleAlreadyRunning) {
			log.Infow("scheduled cycle skipped because another cycle is running")
			return nil
		}
		return runErr
	}, log)
	go detectionScheduler.Start(ctx)

	serverErrCh := make(chan error, 1)
	go func() {
		log.Infow("http server started", "addr", httpServer.Addr)
		if serveErr := httpServer.ListenAndServe(); serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
			serverErrCh <- serveErr
		}
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		log.Info("shutdown signal received")
		return httpServer.Shutdown(shutdownCtx)
	case serveErr := <-serverErrCh:
		return fmt.Errorf("http server error: %w", serveErr)
	}
}

func parseEnabledComponents(values []string) ([]domain.Component, error) {
	result := make([]domain.Component, 0, len(values))
	for _, item := range values {
		component, err := domain.ParseComponent(item)
		if err != nil {
			return nil, err
		}
		result = append(result, component)
	}

	return result, nil
}
