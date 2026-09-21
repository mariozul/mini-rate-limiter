package shutdown_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	golibslogger "github.com/astronautsid/astro-golibs/logger"
	"github.com/mariozul/mini-rate-limiter/pkg/shutdown"
	"github.com/stretchr/testify/require"
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace"
)

// nopLogger absorbs log calls so tests can run silently. Implements
// astro-golibs/logger.Logger (the upstream Logger interface is wide; we
// satisfy it once here so individual tests don't have to).
type nopLogger struct{}

func (nopLogger) Errorf(string, ...interface{}) {}
func (nopLogger) Error(...interface{})          {}
func (nopLogger) Fatalf(string, ...interface{}) {}
func (nopLogger) Fatal(...interface{})          {}
func (nopLogger) Infof(string, ...interface{})  {}
func (nopLogger) Info(...interface{})           {}
func (nopLogger) Warn(...interface{})           {}
func (nopLogger) Warnf(string, ...interface{})  {}
func (nopLogger) Debugf(string, ...interface{}) {}
func (nopLogger) Debug(...interface{})          {}
func (nopLogger) WithFields(context.Context, map[string]interface{}) golibslogger.EntryLogger {
	return nopEntry{}
}
func (nopLogger) WithContext(context.Context) golibslogger.EntryLogger { return nopEntry{} }
func (nopLogger) InfoTrace(ddtrace.Span, string, ...interface{})       {}
func (nopLogger) WarnTrace(ddtrace.Span, string, ...interface{})       {}
func (nopLogger) ErrorTrace(ddtrace.Span, string, ...interface{})      {}
func (nopLogger) DebugTrace(ddtrace.Span, string, ...interface{})      {}

// nopEntry is the EntryLogger returned by WithFields / WithContext. It
// drops every call.
type nopEntry struct{}

func (nopEntry) Error(...interface{})           {}
func (nopEntry) Errorf(string, ...interface{})  {}
func (nopEntry) Fatal(...interface{})           {}
func (nopEntry) Fatalf(string, ...interface{})  {}
func (nopEntry) Info(...interface{})            {}
func (nopEntry) Infof(string, ...interface{})   {}
func (nopEntry) Warn(...interface{})            {}
func (nopEntry) Warnf(string, ...interface{})   {}
func (nopEntry) Debug(...interface{})           {}
func (nopEntry) Debugf(string, ...interface{})  {}

func TestManager_PhasesRunInRegistrationOrder(t *testing.T) {
	t.Parallel()

	var (
		mu    sync.Mutex
		order []string
	)
	record := func(name string) shutdown.Phase {
		return func(_ context.Context) error {
			mu.Lock()
			defer mu.Unlock()
			order = append(order, name)
			return nil
		}
	}

	sm := shutdown.NewManager()
	sm.Register("readiness", time.Second, record("readiness"))
	sm.Register("grpc", time.Second, record("grpc"))
	sm.Register("metrics", time.Second, record("metrics"))
	sm.Register("postgres", time.Second, record("postgres"))

	require.NoError(t, sm.Run(nopLogger{}))
	require.Equal(t, []string{"readiness", "grpc", "metrics", "postgres"}, order)
}

func TestManager_PhaseTimeoutCancelsPhaseContext(t *testing.T) {
	t.Parallel()

	sm := shutdown.NewManager()
	sm.Register("slow", 20*time.Millisecond, func(ctx context.Context) error {
		select {
		case <-time.After(time.Second):
			return errors.New("phase exceeded test budget — context was not cancelled")
		case <-ctx.Done():
			return ctx.Err()
		}
	})

	err := sm.Run(nopLogger{})
	require.Error(t, err, "phase that hits its own timeout MUST surface as an error")
	require.ErrorIs(t, err, context.DeadlineExceeded)
}

func TestManager_LaterPhasesStillRunAfterEarlierFailure(t *testing.T) {
	t.Parallel()

	earlyErr := errors.New("early phase blew up")
	var laterRan bool

	sm := shutdown.NewManager()
	sm.Register("early", time.Second, func(_ context.Context) error { return earlyErr })
	sm.Register("later", time.Second, func(_ context.Context) error {
		laterRan = true
		return nil
	})

	err := sm.Run(nopLogger{})
	require.Error(t, err)
	require.ErrorIs(t, err, earlyErr, "the aggregated error must wrap the originating phase error")
	require.True(t, laterRan, "later phase must still run even after an earlier phase failed")
}

func TestManager_AggregatesMultipleErrors(t *testing.T) {
	t.Parallel()

	errA := errors.New("phase A failed")
	errB := errors.New("phase B failed")

	sm := shutdown.NewManager()
	sm.Register("A", time.Second, func(_ context.Context) error { return errA })
	sm.Register("ok", time.Second, func(_ context.Context) error { return nil })
	sm.Register("B", time.Second, func(_ context.Context) error { return errB })

	err := sm.Run(nopLogger{})
	require.Error(t, err)
	require.ErrorIs(t, err, errA)
	require.ErrorIs(t, err, errB, "errors.Join must surface every failed phase")
}

func TestManager_EmptyRegistration_NoOp(t *testing.T) {
	t.Parallel()
	sm := shutdown.NewManager()
	require.NoError(t, sm.Run(nopLogger{}), "Run with zero phases must succeed cleanly")
}

func TestManager_NonPositiveTimeoutPanics(t *testing.T) {
	t.Parallel()
	sm := shutdown.NewManager()
	require.Panics(t, func() {
		sm.Register("bad", 0, func(_ context.Context) error { return nil })
	})
	require.Panics(t, func() {
		sm.Register("bad", -time.Second, func(_ context.Context) error { return nil })
	})
}

func TestManager_NilFnPanics(t *testing.T) {
	t.Parallel()
	sm := shutdown.NewManager()
	require.Panics(t, func() {
		sm.Register("nil", time.Second, nil)
	})
}
