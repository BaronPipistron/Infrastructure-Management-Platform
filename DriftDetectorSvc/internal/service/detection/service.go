package detection

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"drift-detector-svc/internal/cooldown"
	"drift-detector-svc/internal/detectors"
	"drift-detector-svc/internal/domain"

	"go.uber.org/zap"
)

var ErrCycleAlreadyRunning = errors.New("detection cycle already running")

type inventoryClient interface {
	FetchActualState(ctx context.Context) (domain.ActualState, error)
}

type parserClient interface {
	FetchDesiredState(ctx context.Context) (domain.DesiredState, error)
}

type reconcilerClient interface {
	SendReconcile(ctx context.Context, command domain.ReconcileCommand) (domain.ReconcileAccepted, error)
}

type cooldownStore interface {
	CanSend(key string, now time.Time, cooldownDuration time.Duration) (bool, time.Duration)
	MarkSent(key string, at time.Time)
}

type Service struct {
	inventory         inventoryClient
	parser            parserClient
	reconciler        reconcilerClient
	registry          *detectors.Registry
	cooldownStore     cooldownStore
	reconcileCooldown time.Duration
	enabledComponents []domain.Component
	log               *zap.SugaredLogger

	mu         sync.RWMutex
	running    bool
	ready      bool
	lastResult domain.DetectionCycleResult
	hasResult  bool
}

func NewService(
	inventory inventoryClient,
	parser parserClient,
	reconciler reconcilerClient,
	registry *detectors.Registry,
	cooldownStore cooldownStore,
	reconcileCooldown time.Duration,
	enabledComponents []domain.Component,
	log *zap.SugaredLogger,
) *Service {
	return &Service{
		inventory:         inventory,
		parser:            parser,
		reconciler:        reconciler,
		registry:          registry,
		cooldownStore:     cooldownStore,
		reconcileCooldown: reconcileCooldown,
		enabledComponents: append([]domain.Component{}, enabledComponents...),
		log:               log,
	}
}

func (s *Service) RunCycle(ctx context.Context, trigger string) (domain.DetectionCycleResult, error) {
	if !s.beginCycle() {
		return domain.DetectionCycleResult{}, ErrCycleAlreadyRunning
	}
	defer s.finishCycle()

	result, err := s.runCycle(ctx, trigger)
	s.persistResult(result, err == nil)
	return result, err
}

func (s *Service) IsReady() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.ready
}

func (s *Service) LastCycleResult() (domain.DetectionCycleResult, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if !s.hasResult {
		return domain.DetectionCycleResult{}, false
	}

	return s.lastResult.Clone(), true
}

func (s *Service) beginCycle() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.running {
		return false
	}

	s.running = true
	return true
}

func (s *Service) finishCycle() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.running = false
}

func (s *Service) persistResult(result domain.DetectionCycleResult, markReady bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.lastResult = result.Clone()
	s.hasResult = true
	if markReady {
		s.ready = true
	}
}

func (s *Service) runCycle(ctx context.Context, trigger string) (domain.DetectionCycleResult, error) {
	startedAt := time.Now().UTC()
	cycleID := fmt.Sprintf("%d", startedAt.UnixNano())
	result := domain.DetectionCycleResult{
		CycleID:       cycleID,
		Trigger:       trigger,
		StartedAt:     startedAt,
		Warnings:      []string{},
		ErrorMessages: []string{},
	}

	s.log.Infow("detection cycle started",
		"cycle_id", cycleID,
		"trigger", trigger,
		"enabled_components", s.enabledComponents,
	)

	inventoryFetchStarted := time.Now()
	actualState, err := s.inventory.FetchActualState(ctx)
	result.StageTimings.InventoryFetchMs = time.Since(inventoryFetchStarted).Milliseconds()
	if err != nil {
		result.Stats.Errors++
		result.ErrorMessages = append(result.ErrorMessages, fmt.Sprintf("inventory fetch failed: %v", err))
		s.finishResult(&result)
		s.log.Errorw("detection cycle failed: inventory fetch failed", "cycle_id", cycleID, "error", err)
		return result, fmt.Errorf("fetch actual state from inventory: %w", err)
	}

	parserFetchStarted := time.Now()
	desiredState, err := s.parser.FetchDesiredState(ctx)
	result.StageTimings.ParserFetchMs = time.Since(parserFetchStarted).Milliseconds()
	if err != nil {
		result.Stats.Errors++
		result.ErrorMessages = append(result.ErrorMessages, fmt.Sprintf("parser fetch failed: %v", err))
		s.finishResult(&result)
		s.log.Errorw("detection cycle failed: parser fetch failed", "cycle_id", cycleID, "error", err)
		return result, fmt.Errorf("fetch desired state from parser: %w", err)
	}

	result.InventoryMarkedPartial = actualState.Metadata.IsPartial
	result.ParserReady = desiredState.Metadata.Ready
	if actualState.Metadata.IsPartial {
		result.Partial = true
		result.Warnings = append(result.Warnings, "inventory metadata marked response as partial")
	}
	if !desiredState.Metadata.Ready {
		result.Partial = true
		if strings.TrimSpace(desiredState.Metadata.ReadyReason) != "" {
			result.Warnings = append(result.Warnings, "parser not ready: "+desiredState.Metadata.ReadyReason)
		} else {
			result.Warnings = append(result.Warnings, "parser not ready")
		}
	}

	hostsByID, hostsByFQDN := indexActualHosts(actualState.Hosts)
	result.Stats.DesiredHosts = len(desiredState.Hosts)
	compareTimer := time.Now()
	var compareDuration time.Duration
	var dispatchDuration time.Duration

	for _, desiredHost := range desiredState.Hosts {
		actualHost, found := lookupActualHost(desiredHost, hostsByID, hostsByFQDN)
		if !found {
			result.Partial = true
			result.Stats.SkippedHostsNoActualHost++
			warning := fmt.Sprintf("actual host is missing for desired host_id=%s fqdn=%s", desiredHost.HostID, desiredHost.FQDN)
			result.Warnings = append(result.Warnings, warning)
			s.log.Warnw("actual host missing, skip host", "cycle_id", cycleID, "host_id", desiredHost.HostID, "fqdn", desiredHost.FQDN)
			continue
		}

		result.Stats.ComparedHosts++
		hostSkippedNoData := false

		for _, component := range s.enabledComponents {
			detector, found := s.registry.Get(component)
			if !found {
				result.Stats.Errors++
				result.ErrorMessages = append(result.ErrorMessages, "detector not found for component: "+string(component))
				s.log.Errorw("detector is not registered", "cycle_id", cycleID, "component", component)
				continue
			}

			detectionResult := detector.Detect(cycleID, desiredHost, actualHost)
			if !detectionResult.Applicable {
				continue
			}

			if detectionResult.MissingActualData {
				result.Partial = true
				if !hostSkippedNoData {
					result.Stats.SkippedHostsNoActualData++
					hostSkippedNoData = true
				}

				if strings.TrimSpace(detectionResult.SkipReason) != "" {
					result.Warnings = append(result.Warnings, fmt.Sprintf("%s on %s skipped: %s", component, desiredHost.FQDN, detectionResult.SkipReason))
				}
				s.log.Warnw("skip detection due to missing actual data",
					"cycle_id", cycleID,
					"component", component,
					"host_id", desiredHost.HostID,
					"fqdn", desiredHost.FQDN,
					"reason", detectionResult.SkipReason,
				)
				continue
			}

			if !detectionResult.Drift {
				continue
			}

			result.Stats.DriftsFound++
			if detectionResult.ReconcileCommand == nil {
				result.Stats.Errors++
				result.ErrorMessages = append(result.ErrorMessages, fmt.Sprintf("detector %s reported drift without reconcile command", component))
				s.log.Errorw("drift without reconcile command",
					"cycle_id", cycleID,
					"component", component,
					"host_id", desiredHost.HostID,
					"fqdn", desiredHost.FQDN,
				)
				continue
			}

			command := *detectionResult.ReconcileCommand
			key := cooldown.BuildKey(command.Component, command.FQDN)
			now := time.Now().UTC()
			canSend, remaining := s.cooldownStore.CanSend(key, now, s.reconcileCooldown)
			if !canSend {
				result.Stats.ReconcileSuppressed++
				s.log.Infow("reconcile suppressed by cooldown",
					"cycle_id", cycleID,
					"component", command.Component,
					"host_id", command.HostID,
					"fqdn", command.FQDN,
					"remaining", remaining.String(),
				)
				continue
			}

			compareDuration += time.Since(compareTimer)
			sendStarted := time.Now()
			accepted, sendErr := s.reconciler.SendReconcile(ctx, command)
			sendDuration := time.Since(sendStarted)
			dispatchDuration += sendDuration
			compareTimer = time.Now()
			if sendErr != nil {
				result.Stats.Errors++
				result.ErrorMessages = append(result.ErrorMessages, fmt.Sprintf("reconcile send failed for %s on %s: %v", command.Component, command.FQDN, sendErr))
				s.log.Errorw("failed to send reconcile request",
					"cycle_id", cycleID,
					"component", command.Component,
					"host_id", command.HostID,
					"fqdn", command.FQDN,
					"error", sendErr,
				)
				continue
			}

			s.cooldownStore.MarkSent(key, now)
			result.Stats.ReconcileSent++
			s.log.Infow("reconcile request accepted",
				"cycle_id", cycleID,
				"component", command.Component,
				"host_id", command.HostID,
				"fqdn", command.FQDN,
				"request_id", accepted.RequestID,
			)
		}
	}

	compareDuration += time.Since(compareTimer)
	result.StageTimings.DriftComparisonMs = compareDuration.Milliseconds()
	result.StageTimings.ReconcileDispatchMs = dispatchDuration.Milliseconds()

	s.finishResult(&result)
	s.log.Infow("detection cycle completed",
		"cycle_id", cycleID,
		"trigger", trigger,
		"duration_ms", result.DurationMs,
		"inventory_fetch_ms", result.StageTimings.InventoryFetchMs,
		"parser_fetch_ms", result.StageTimings.ParserFetchMs,
		"drift_comparison_ms", result.StageTimings.DriftComparisonMs,
		"reconcile_dispatch_ms", result.StageTimings.ReconcileDispatchMs,
		"partial", result.Partial,
		"desired_hosts", result.Stats.DesiredHosts,
		"compared_hosts", result.Stats.ComparedHosts,
		"drifts_found", result.Stats.DriftsFound,
		"reconcile_sent", result.Stats.ReconcileSent,
		"reconcile_suppressed", result.Stats.ReconcileSuppressed,
		"errors", result.Stats.Errors,
	)

	return result, nil
}

func (s *Service) finishResult(result *domain.DetectionCycleResult) {
	result.FinishedAt = time.Now().UTC()
	result.DurationMs = time.Since(result.StartedAt).Milliseconds()
}

func indexActualHosts(hosts []domain.ActualHost) (map[string]domain.ActualHost, map[string]domain.ActualHost) {
	hostsByID := make(map[string]domain.ActualHost, len(hosts))
	hostsByFQDN := make(map[string]domain.ActualHost, len(hosts))

	for _, host := range hosts {
		hostID := strings.ToLower(strings.TrimSpace(host.HostID))
		if hostID != "" {
			hostsByID[hostID] = host
		}

		fqdn := strings.ToLower(strings.TrimSpace(host.FQDN))
		if fqdn != "" {
			hostsByFQDN[fqdn] = host
		}
	}

	return hostsByID, hostsByFQDN
}

func lookupActualHost(desiredHost domain.DesiredHost, hostsByID map[string]domain.ActualHost, hostsByFQDN map[string]domain.ActualHost) (domain.ActualHost, bool) {
	hostID := strings.ToLower(strings.TrimSpace(desiredHost.HostID))
	if hostID != "" {
		host, found := hostsByID[hostID]
		if found {
			return host, true
		}
	}

	fqdn := strings.ToLower(strings.TrimSpace(desiredHost.FQDN))
	if fqdn != "" {
		host, found := hostsByFQDN[fqdn]
		if found {
			return host, true
		}
	}

	return domain.ActualHost{}, false
}
