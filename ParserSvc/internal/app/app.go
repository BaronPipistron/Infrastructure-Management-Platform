package app

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"time"

	"parser-svc/internal/config"
	"parser-svc/internal/domain"
	"parser-svc/internal/loader"
	"parser-svc/internal/logger"
	"parser-svc/internal/mapper"
	"parser-svc/internal/parser"
	"parser-svc/internal/service/desiredstate"
	"parser-svc/internal/store/memory"
	httpapi "parser-svc/internal/transport/http"
)

func Run(ctx context.Context, configPath string) error {
	cfg, cfgFilePath, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("load appconfig: %w", err)
	}

	log, err := logger.New(cfg.Logging.Level)
	if err != nil {
		return fmt.Errorf("build logger: %w", err)
	}
	defer func() { _ = log.Sync() }()

	log.Infow("application startup",
		"config_path", cfgFilePath,
		"http_host", cfg.Server.Host,
		"http_port", cfg.Server.Port,
		"manifest_mode", cfg.Manifests.Mode,
		"manifest_path", cfg.Manifests.Path,
	)

	resolvedManifestPath, err := config.ResolvePath(cfgFilePath, cfg.Manifests.Path)
	if err != nil {
		return fmt.Errorf("resolve manifests path: %w", err)
	}
	log.Infow("manifest path resolved", "path", resolvedManifestPath)

	files, err := loader.DiscoverFiles(cfg.Manifests.Mode, resolvedManifestPath)
	if err != nil {
		return fmt.Errorf("discover manifests: %w", err)
	}

	for _, ignored := range files.IgnoredFiles {
		log.Infow("manifest ignored (unsupported extension)", "path", ignored)
	}
	log.Infow("manifest discovery completed",
		"json_files", len(files.JSONFiles),
		"ignored_files", len(files.IgnoredFiles),
	)

	snapshot := loadDesiredState(files.JSONFiles, cfg.Manifests.Mode, resolvedManifestPath, len(files.IgnoredFiles), log)

	store := memory.NewStore()
	store.Replace(snapshot)
	store.SetReady(snapshot.Metadata.Ready)

	svc := desiredstate.NewService(store)
	handler := httpapi.NewHandler(svc)
	router := httpapi.NewRouter(handler)

	httpServer := &http.Server{
		Addr:              fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port),
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
	}

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

func loadDesiredState(jsonFiles []string, manifestMode, manifestPath string, ignoredFiles int, log loggerContract) domain.Snapshot {
	startedAt := time.Now().UTC()
	brokenFiles := 0
	loadedFiles := 0

	hostsByID := make(map[string]domain.DesiredHost)
	warningsCount := 0

	for _, filePath := range jsonFiles {
		log.Infow("loading manifest file", "path", filePath)

		fileData, err := loader.LoadFile(filePath)
		if err != nil {
			brokenFiles++
			log.Warnw("failed to read manifest file", "path", filePath, "error", err)
			continue
		}

		manifest, err := parser.ParseStructurizrManifest(fileData)
		if err != nil {
			brokenFiles++
			log.Warnw("failed to parse manifest json", "path", filePath, "error", err)
			continue
		}

		mapResult := mapper.MapManifest(manifest, filePath)
		for _, warning := range mapResult.Warnings {
			warningsCount++
			log.Warnw("manifest mapping warning",
				"file", warning.File,
				"node_id", warning.NodeID,
				"host_id", warning.HostID,
				"workload", warning.Workload,
				"warning", warning.Message,
			)
		}

		for _, host := range mapResult.Hosts {
			if _, exists := hostsByID[host.HostID]; exists {
				log.Warnw("duplicate host_id detected, replacing previous host", "host_id", host.HostID, "file", filePath)
			}
			hostsByID[host.HostID] = host
		}

		loadedFiles++
		log.Infow("manifest file loaded",
			"path", filePath,
			"hosts_added", len(mapResult.Hosts),
			"workloads_added", mapResult.Workloads,
		)
	}

	hosts := make([]domain.DesiredHost, 0, len(hostsByID))
	for _, host := range hostsByID {
		hosts = append(hosts, host.Clone())
	}
	sort.Slice(hosts, func(i, j int) bool { return hosts[i].HostID < hosts[j].HostID })

	workloadsTotal := 0
	for _, host := range hosts {
		workloadsTotal += len(host.Workloads)
	}

	ready := loadedFiles > 0
	readyReason := ""
	if !ready {
		readyReason = "no valid manifest files were loaded"
		log.Warnw("desired state is not ready", "reason", readyReason)
	}

	snapshot := domain.Snapshot{
		State: domain.DesiredState{Hosts: hosts},
		Metadata: domain.SnapshotMetadata{
			LoadedAt:       time.Now().UTC(),
			ManifestMode:   manifestMode,
			ManifestPath:   manifestPath,
			FilesTotal:     len(jsonFiles) + ignoredFiles,
			FilesLoaded:    loadedFiles,
			FilesBroken:    brokenFiles,
			FilesIgnored:   ignoredFiles,
			HostsTotal:     len(hosts),
			WorkloadsTotal: workloadsTotal,
			Ready:          ready,
			ReadyReason:    readyReason,
		},
	}

	log.Infow("desired state build completed",
		"duration_ms", time.Since(startedAt).Milliseconds(),
		"files_total", snapshot.Metadata.FilesTotal,
		"files_loaded", loadedFiles,
		"files_broken", brokenFiles,
		"files_ignored", ignoredFiles,
		"hosts_total", len(hosts),
		"workloads_total", workloadsTotal,
		"warnings_total", warningsCount,
		"ready", ready,
	)

	return snapshot
}

type loggerContract interface {
	Infow(msg string, keysAndValues ...interface{})
	Warnw(msg string, keysAndValues ...interface{})
}
