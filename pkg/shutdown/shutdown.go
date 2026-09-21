// Package shutdown implements an ordered, phased process-shutdown manager.
//
// The pattern (and motivation for not just using the canonical
// "srv.Shutdown(ctx)" idiom) is described at length in CodeX's
// "Graceful Shutdown in Go: Why Most Implementations Are Wrong"
// (https://medium.com/codex/...-323ff193f1f8). In short:
//
//   - Each phase gets its own timeout budget (not a shared one), so a
//     long-running phase never starves later phases.
//   - Phases execute in registration order — concrete composition roots
//     register them in the right shape:
//
//     Phase 1: Stop accepting new work       (flip readiness, sleep,
//                                              close listeners)
//     Phase 2: Drain in-flight work          (gRPC GracefulStop, message
//                                              consumer drain)
//     Phase 3: Flush and close resources     (metrics flush, DB Close,
//                                              tracer Stop)
//
//   - Errors are aggregated via errors.Join rather than silently dropped.
//     A failed phase is logged but does not block subsequent phases —
//     the next phase still runs with its own budget so DB connections
//     still get closed even if the gRPC drain timed out.
//
// The Manager type below is intentionally tiny: no goroutines, no fan-out,
// no panic recovery. Each phase is a plain func(context.Context) error
// that the manager invokes synchronously inside a per-phase
// context.WithTimeout. If a phase needs concurrent fan-out internally
// (e.g. closing several connection pools in parallel), it does so inside
// its own fn — the manager does not impose a structure on phase
// internals.
package shutdown

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	logger "github.com/astronautsid/astro-golibs/logger"
)

// Phase is a single shutdown step. Implementations SHOULD respect the
// passed context's deadline and return a wrapped error on failure so the
// aggregated errors.Join output remains scannable.
type Phase func(ctx context.Context) error

// phaseEntry records a Phase and its per-phase timeout budget.
type phaseEntry struct {
	name    string
	timeout time.Duration
	fn      Phase
}

// Manager orchestrates ordered, phased shutdown. Registrations are
// goroutine-safe so a composition root can register from any setup
// helper; Run is intended to be invoked once from main after a shutdown
// signal arrives.
type Manager struct {
	mu     sync.Mutex
	phases []phaseEntry
}

// NewManager constructs an empty Manager.
func NewManager() *Manager {
	return &Manager{}
}

// Register appends a phase to the end of the execution list. Phases run
// in registration order; the composition root SHOULD register them in
// "stop accepting → drain → flush/close" order so dependencies are still
// alive when earlier phases need them.
//
// timeout MUST be > 0. A zero or negative timeout would make the phase
// return immediately with context.DeadlineExceeded, which is almost
// never what the caller wants.
func (m *Manager) Register(name string, timeout time.Duration, fn Phase) {
	if timeout <= 0 {
		panic(fmt.Sprintf("shutdown: phase %q registered with non-positive timeout %s", name, timeout))
	}
	if fn == nil {
		panic(fmt.Sprintf("shutdown: phase %q registered with nil fn", name))
	}
	m.mu.Lock()
	m.phases = append(m.phases, phaseEntry{name: name, timeout: timeout, fn: fn})
	m.mu.Unlock()
}

// Run executes every registered phase sequentially, each inside its own
// context.WithTimeout. Phases run regardless of earlier phase failures
// — a Mongo close that times out does not prevent the Datadog tracer
// flush from happening.
//
// The returned error is errors.Join of all per-phase errors (nil on a
// fully successful shutdown). Each phase's name is also written to log
// at Info on start, Error on failure, and Info on success, so operators
// have a clear timeline of what ran and which budgets were exceeded.
func (m *Manager) Run(log logger.Logger) error {
	m.mu.Lock()
	phases := append([]phaseEntry(nil), m.phases...)
	m.mu.Unlock()

	var errs []error
	for _, p := range phases {
		log.Info(fmt.Sprintf("shutdown phase %q starting (budget=%s)", p.name, p.timeout))
		start := time.Now()
		ctx, cancel := context.WithTimeout(context.Background(), p.timeout)
		err := p.fn(ctx)
		cancel()
		elapsed := time.Since(start)
		if err != nil {
			log.Error(fmt.Sprintf("shutdown phase %q failed after %s: %v", p.name, elapsed, err))
			errs = append(errs, fmt.Errorf("shutdown phase %q: %w", p.name, err))
			continue
		}
		log.Info(fmt.Sprintf("shutdown phase %q completed in %s", p.name, elapsed))
	}
	return errors.Join(errs...)
}
