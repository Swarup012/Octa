// Solo - Personal AI Agent
// License: MIT

package scheduler

import (
	"context"
	"sync"
	"time"

	"github.com/Swarup012/solo/pkg/logger"
)

// Job describes a periodic background task registered with the Scheduler.
type Job struct {
	Name     string
	Interval time.Duration
	Fn       func(ctx context.Context)

	lastRun time.Time
}

// Scheduler drives all registered jobs from a single goroutine.
type Scheduler struct {
	mu     sync.Mutex
	jobs   []*Job
	stopCh chan struct{}
	wg     sync.WaitGroup
}

// New creates a new Scheduler.
func New() *Scheduler {
	return &Scheduler{
		stopCh: make(chan struct{}),
	}
}

// Register adds a named job that will run every interval.
func (s *Scheduler) Register(name string, interval time.Duration, fn func(ctx context.Context)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.jobs = append(s.jobs, &Job{
		Name:     name,
		Interval: interval,
		Fn:       fn,
	})
	logger.DebugCF("scheduler", "Job registered", map[string]any{
		"job":      name,
		"interval": interval.String(),
	})
}

// Run starts the scheduler loop in a background goroutine.
func (s *Scheduler) Run(ctx context.Context) {
	s.wg.Add(1)
	go s.loop(ctx)
}

func (s *Scheduler) loop(ctx context.Context) {
	defer s.wg.Done()

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-s.stopCh:
			return
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			s.mu.Lock()
			due := make([]*Job, 0)
			for _, j := range s.jobs {
				if j.lastRun.IsZero() || now.Sub(j.lastRun) >= j.Interval {
					due = append(due, j)
				}
			}
			s.mu.Unlock()

			for _, j := range due {
				select {
				case <-s.stopCh:
					return
				case <-ctx.Done():
					return
				default:
				}

				logger.DebugCF("scheduler", "Running job", map[string]any{"job": j.Name})
				j.Fn(ctx)

				s.mu.Lock()
				j.lastRun = now
				s.mu.Unlock()
			}
		}
	}
}

// Stop signals the scheduler to stop and waits for the goroutine to exit.
func (s *Scheduler) Stop() {
	close(s.stopCh)
	s.wg.Wait()
}
