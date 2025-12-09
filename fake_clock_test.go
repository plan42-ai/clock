package clock_test

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/debugging-sucks/clock"
	"github.com/stretchr/testify/require"
)

var theMostImportantDateEver = time.Date(1980, 8, 19, 0, 0, 0, 0, time.UTC)

func TestAdvance(t *testing.T) {
	t.Parallel()
	c := clock.NewFakeClock(theMostImportantDateEver)
	c.Advance(time.Hour * 24)
	require.Equal(t, theMostImportantDateEver.Add(time.Hour*24), c.Now())
}

func TestTimer(t *testing.T) {
	t.Parallel()
	c := clock.NewFakeClock(theMostImportantDateEver)
	timer := c.NewTimer(time.Hour * 25)
	c.Advance(time.Hour * 24)
	ensureNotTriggered(t, timer)
	c.Advance(time.Hour)
	ensureTriggered(t, timer)
}

func TestAfterFunc(t *testing.T) {
	t.Parallel()
	c := clock.NewFakeClock(theMostImportantDateEver)
	run := atomic.Bool{}
	timer := c.AfterFunc(
		time.Hour*25,
		func() {
			run.Store(true)
		},
	)
	c.Advance(time.Hour * 24)
	ensureNotTriggered(t, timer)
	require.False(t, run.Load())
	c.Advance(time.Hour)
	timer.Stop()
	time.Sleep(50 * time.Millisecond)
	require.True(t, run.Load())
}

func TestAfterFuncCanceled(t *testing.T) {
	t.Parallel()
	c := clock.NewFakeClock(theMostImportantDateEver)
	run := atomic.Bool{}
	timer := c.AfterFunc(
		time.Hour*25,
		func() {
			run.Store(true)
		},
	)
	c.Advance(time.Hour * 24)
	ensureNotTriggered(t, timer)
	timer.Stop()
	c.Advance(time.Hour)
	time.Sleep(50 * time.Millisecond)
	require.False(t, run.Load())
}

func ensureTriggered(t *testing.T, timer clock.Timer) {
	select {
	case <-timer.C():
	default:
		require.Fail(t, "Timer should have triggered")
	}
}

func ensureNotTriggered(t *testing.T, timer clock.Timer) {
	select {
	case <-timer.C():
		require.Fail(t, "Timer should not have triggered")
	default:
	}
}

func TestStop(t *testing.T) {
	t.Parallel()
	c := clock.NewFakeClock(theMostImportantDateEver)
	timer := c.NewTimer(time.Hour)
	stopped := timer.Stop()
	require.True(t, stopped)
	c.Advance(time.Hour * 2)
	ensureNotTriggered(t, timer)
}

func TestReset(t *testing.T) {
	t.Parallel()
	c := clock.NewFakeClock(theMostImportantDateEver)
	timer := c.NewTimer(time.Hour)
	c.Advance(time.Hour * 2)
	ensureTriggered(t, timer)
	reset := timer.Reset(time.Hour * 4)
	require.False(t, reset)
	c.Advance(time.Hour * 3)
	ensureNotTriggered(t, timer)
	c.Advance(time.Hour * 2)
	ensureTriggered(t, timer)
}

func TestResetAfterFunc(t *testing.T) {
	t.Parallel()
	ch := make(chan struct{})
	clk := clock.NewFakeClock(theMostImportantDateEver)
	fakeTimer := clk.AfterFunc(
		time.Second,
		func() {
			close(ch)
		},
	)
	clk.Advance(time.Hour)
	realTimer := time.NewTimer(time.Second)
	select {
	case <-ch:
	case <-realTimer.C:
		require.Fail(t, "after func didn't execute the first time")
	}
	realTimer.Reset(time.Second)
	ch = make(chan struct{})
	fakeTimer.Reset(time.Second)
	clk.Advance(time.Hour)

	select {
	case <-ch:
	case <-realTimer.C:
		require.Fail(t, "after func didn't execute the second time")
	}
}

func TestResetNegative(t *testing.T) {
	t.Parallel()
	c := clock.NewFakeClock(theMostImportantDateEver)
	timer := c.NewTimer(time.Hour)
	c.Advance(time.Hour * 2)
	ensureTriggered(t, timer)
	reset := timer.Reset(-time.Hour * 4)
	require.False(t, reset)
	ensureTriggered(t, timer)
}

func TestTimeout(t *testing.T) {
	t.Parallel()
	c := clock.NewFakeClock(theMostImportantDateEver)
	ctx, cancel := c.WithTimeout(context.Background(), time.Second)
	defer cancel()
	c.Advance(time.Hour)

	realTimeout, realCancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer realCancel()

	select {
	case <-ctx.Done():
		// Our fake timeout triggered after we advanced time in the fake clock
		// Check to make sure the context reports the correct error.
		require.ErrorIs(t, ctx.Err(), context.DeadlineExceeded)
	case <-realTimeout.Done():
		// 250 ms of real time elapsed, and our fake time context didn't respond to fake
		// time being advanced. this means there's a bug in our implementation.
		require.Fail(t, "test context did not timeout")
	}
}

func TestParentCanceled(t *testing.T) {
	t.Parallel()
	c := clock.NewFakeClock(theMostImportantDateEver)
	parent, cancelParent := context.WithCancel(context.Background())
	defer cancelParent()
	child, cancelChild := c.WithTimeout(parent, time.Hour)
	defer cancelChild()
	cancelParent()

	realTimeout, realCancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer realCancel()

	select {
	case <-child.Done():
		// Our fake timeout was canceled after the parent context was canceled
		require.ErrorIs(t, child.Err(), context.Canceled)
	case <-realTimeout.Done():
		// 250 ms of real time elapsed, and our fake time context didn't respond to the
		// parent context being canceled. This means there's a big in our implementation.
		require.Fail(t, "cancellation did not propagate from parent context to child context")
	}
}
