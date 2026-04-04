package scheduler

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestSchedulerRunsTaskPeriodically(t *testing.T) {
	t.Parallel()

	var calls int32
	task := func(ctx context.Context) error {
		atomic.AddInt32(&calls, 1)
		return nil
	}

	s := New(20*time.Millisecond, task, zap.NewNop().Sugar())
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan struct{})
	go func() {
		defer close(done)
		s.Start(ctx)
	}()

	time.Sleep(70 * time.Millisecond)
	cancel()
	<-done

	require.GreaterOrEqual(t, atomic.LoadInt32(&calls), int32(2))
}

func TestSchedulerDisabledWhenIntervalIsZero(t *testing.T) {
	t.Parallel()

	var calls int32
	task := func(ctx context.Context) error {
		atomic.AddInt32(&calls, 1)
		return nil
	}

	s := New(0, task, zap.NewNop().Sugar())
	s.Start(context.Background())
	require.Equal(t, int32(0), atomic.LoadInt32(&calls))
}
