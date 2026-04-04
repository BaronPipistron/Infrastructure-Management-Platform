package detection

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"drift-detector-svc/internal/cooldown"
	"drift-detector-svc/internal/detectors"
	"drift-detector-svc/internal/domain"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

type inventoryClientStub struct {
	state domain.ActualState
	err   error
}

func (s *inventoryClientStub) FetchActualState(ctx context.Context) (domain.ActualState, error) {
	if s.err != nil {
		return domain.ActualState{}, s.err
	}
	return s.state, nil
}

type parserClientStub struct {
	state domain.DesiredState
	err   error
}

func (s *parserClientStub) FetchDesiredState(ctx context.Context) (domain.DesiredState, error) {
	if s.err != nil {
		return domain.DesiredState{}, s.err
	}
	return s.state, nil
}

type reconcilerClientStub struct {
	mu       sync.Mutex
	commands []domain.ReconcileCommand
	err      error
}

func (s *reconcilerClientStub) SendReconcile(ctx context.Context, command domain.ReconcileCommand) (domain.ReconcileAccepted, error) {
	if s.err != nil {
		return domain.ReconcileAccepted{}, s.err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.commands = append(s.commands, command)
	return domain.ReconcileAccepted{RequestID: "req-001", Component: command.Component, FQDN: command.FQDN}, nil
}

func (s *reconcilerClientStub) commandsCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.commands)
}

func TestRunCycleSendsReconcileAndSuppressesByCooldown(t *testing.T) {
	t.Parallel()

	inventory := &inventoryClientStub{
		state: domain.ActualState{
			Hosts: []domain.ActualHost{
				{
					HostID: "srv-001",
					FQDN:   "srv-001.example.local",
					SourceStatus: map[string]domain.ActualSourceStatus{
						"cadvisor": {Source: "cadvisor", Enabled: true, Status: "ok"},
					},
				},
			},
		},
	}
	parser := &parserClientStub{
		state: domain.DesiredState{
			Metadata: domain.DesiredMetadata{Ready: true},
			Hosts: []domain.DesiredHost{
				{
					HostID: "srv-001",
					FQDN:   "srv-001.example.local",
					Workloads: []domain.DesiredWorkload{
						{Name: "node_exporter", Enabled: true, DeploymentMode: "container"},
					},
				},
			},
		},
	}
	reconciler := &reconcilerClientStub{}

	svc := NewService(
		inventory,
		parser,
		reconciler,
		detectors.NewRegistry(detectors.NewNodeExporterDetector()),
		cooldown.NewStore(),
		time.Hour,
		[]domain.Component{domain.ComponentNodeExporter},
		zap.NewNop().Sugar(),
	)

	first, err := svc.RunCycle(context.Background(), "test")
	require.NoError(t, err)
	require.Equal(t, 1, first.Stats.DriftsFound)
	require.Equal(t, 1, first.Stats.ReconcileSent)
	require.Equal(t, 0, first.Stats.ReconcileSuppressed)
	require.Equal(t, 1, reconciler.commandsCount())
	require.True(t, svc.IsReady())

	second, err := svc.RunCycle(context.Background(), "test")
	require.NoError(t, err)
	require.Equal(t, 1, second.Stats.DriftsFound)
	require.Equal(t, 0, second.Stats.ReconcileSent)
	require.Equal(t, 1, second.Stats.ReconcileSuppressed)
	require.Equal(t, 1, reconciler.commandsCount())
}

func TestRunCyclePartialWhenActualHostMissing(t *testing.T) {
	t.Parallel()

	inventory := &inventoryClientStub{
		state: domain.ActualState{
			Hosts: []domain.ActualHost{},
		},
	}
	parser := &parserClientStub{
		state: domain.DesiredState{
			Metadata: domain.DesiredMetadata{Ready: true},
			Hosts: []domain.DesiredHost{
				{
					HostID: "srv-001",
					FQDN:   "srv-001.example.local",
					Workloads: []domain.DesiredWorkload{
						{Name: "cadvisor", Enabled: true, DeploymentMode: "container"},
					},
				},
			},
		},
	}

	svc := NewService(
		inventory,
		parser,
		&reconcilerClientStub{},
		detectors.NewRegistry(detectors.NewCadvisorDetector()),
		cooldown.NewStore(),
		time.Minute,
		[]domain.Component{domain.ComponentCadvisor},
		zap.NewNop().Sugar(),
	)

	result, err := svc.RunCycle(context.Background(), "test")
	require.NoError(t, err)
	require.True(t, result.Partial)
	require.Equal(t, 1, result.Stats.SkippedHostsNoActualHost)
	require.Equal(t, 0, result.Stats.DriftsFound)
}

func TestRunCycleReturnsErrorWhenInventoryFails(t *testing.T) {
	t.Parallel()

	inventory := &inventoryClientStub{err: errors.New("inventory unavailable")}
	parser := &parserClientStub{state: domain.DesiredState{Metadata: domain.DesiredMetadata{Ready: true}}}

	svc := NewService(
		inventory,
		parser,
		&reconcilerClientStub{},
		detectors.NewRegistry(detectors.NewNodeExporterDetector()),
		cooldown.NewStore(),
		time.Minute,
		[]domain.Component{domain.ComponentNodeExporter},
		zap.NewNop().Sugar(),
	)

	_, err := svc.RunCycle(context.Background(), "test")
	require.Error(t, err)
	require.False(t, svc.IsReady())
}

func TestRunCycleRejectsParallelExecution(t *testing.T) {
	t.Parallel()

	inventory := &inventoryBlockingStub{
		block:  make(chan struct{}),
		called: make(chan struct{}),
	}
	parser := &parserClientStub{
		state: domain.DesiredState{
			Metadata: domain.DesiredMetadata{Ready: true},
			Hosts:    []domain.DesiredHost{},
		},
	}

	svc := NewService(
		inventory,
		parser,
		&reconcilerClientStub{},
		detectors.NewRegistry(detectors.NewNodeExporterDetector()),
		cooldown.NewStore(),
		time.Minute,
		[]domain.Component{domain.ComponentNodeExporter},
		zap.NewNop().Sugar(),
	)

	firstDone := make(chan struct{})
	go func() {
		defer close(firstDone)
		_, _ = svc.RunCycle(context.Background(), "test")
	}()

	// Wait for the first cycle to enter inventory call.
	inventory.waitUntilCalled()

	_, err := svc.RunCycle(context.Background(), "test")
	require.ErrorIs(t, err, ErrCycleAlreadyRunning)

	close(inventory.block)
	<-firstDone
}

type inventoryBlockingStub struct {
	block      chan struct{}
	called     chan struct{}
	calledOnce sync.Once
}

func (s *inventoryBlockingStub) FetchActualState(ctx context.Context) (domain.ActualState, error) {
	s.calledOnce.Do(func() {
		close(s.called)
	})

	<-s.block
	return domain.ActualState{}, nil
}

func (s *inventoryBlockingStub) waitUntilCalled() {
	<-s.called
}
