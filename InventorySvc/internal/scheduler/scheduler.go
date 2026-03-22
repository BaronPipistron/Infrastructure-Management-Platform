package scheduler

import (
	"context"
	"time"

	"go.uber.org/zap"
)

type Task func(ctx context.Context) error

type Scheduler struct {
	interval time.Duration
	task     Task
	log      *zap.SugaredLogger
}

func New(interval time.Duration, task Task, log *zap.SugaredLogger) *Scheduler {
	return &Scheduler{
		interval: interval,
		task:     task,
		log:      log,
	}
}

func (s *Scheduler) Start(ctx context.Context) {
	if s.interval <= 0 {
		s.log.Warn("scheduler is disabled because interval <= 0")
		return
	}

	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	s.log.Infow("scheduler started", "interval", s.interval.String())

	for {
		select {
		case <-ctx.Done():
			s.log.Info("scheduler stopped")
			return
		case <-ticker.C:
			if err := s.task(ctx); err != nil {
				s.log.Errorw("periodic sync failed", "error", err)
			}
		}
	}
}
