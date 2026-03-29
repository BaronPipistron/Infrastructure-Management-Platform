package app

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"inventory-svc/internal/bootstrap/selfprovisioning"
	"inventory-svc/internal/config"
	"inventory-svc/internal/logger"
	"inventory-svc/internal/scheduler"
	"inventory-svc/internal/service/inventory"
	"inventory-svc/internal/sources"
	"inventory-svc/internal/sources/cadvisor"
	"inventory-svc/internal/store/memory"
	httpapi "inventory-svc/internal/transport/http"

	"go.uber.org/zap"
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

	log.Infow("appsettings loaded",
		"path", cfgFilePath,
		"http_port", cfg.Server.Port,
		"sync_interval", cfg.Sync.Interval.String(),
	)

	selfProvisioningPath, err := config.ResolvePath(cfgFilePath, cfg.Bootstrap.SelfProvisioningPath)
	if err != nil {
		return fmt.Errorf("resolve selfProvisioning path: %w", err)
	}

	hostSeeds, err := selfprovisioning.Load(selfProvisioningPath)
	if err != nil {
		return fmt.Errorf("load selfProvisioning: %w", err)
	}
	log.Infow("selfProvisioning loaded", "path", selfProvisioningPath, "hosts", len(hostSeeds))

	store := memory.NewStore()
	hostSources := buildSources(cfg, log)
	inventorySvc := inventory.NewService(store, hostSources, hostSeeds, log)

	log.Info("running initial sync")
	if err := inventorySvc.Sync(ctx); err != nil {
		return fmt.Errorf("initial sync failed: %w", err)
	}
	inventorySvc.SetReady(true)

	handler := httpapi.NewHandler(inventorySvc)
	router := httpapi.NewRouter(handler)

	httpServer := &http.Server{
		Addr:              fmt.Sprintf(":%d", cfg.Server.Port),
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
	}

	syncScheduler := scheduler.New(cfg.Sync.Interval, inventorySvc.Sync, log)
	go syncScheduler.Start(ctx)

	serverErrCh := make(chan error, 1)
	go func() {
		log.Infow("http server started", "addr", httpServer.Addr)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErrCh <- err
		}
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		log.Info("shutdown signal received")
		return httpServer.Shutdown(shutdownCtx)
	case err := <-serverErrCh:
		return fmt.Errorf("http server error: %w", err)
	}
}

func buildSources(cfg config.Config, log *zap.SugaredLogger) []sources.HostSource {
	return []sources.HostSource{
		cadvisor.NewSource(cfg.Sources.CAdvisor, log),
		sources.PlaceholderSource{SourceName: "netbox", IsOn: cfg.Sources.NetBox.IsEnabled},
		sources.PlaceholderSource{SourceName: "otherSources", IsOn: cfg.Sources.OtherSources.IsEnabled},
	}
}
