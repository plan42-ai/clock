package clock_test

import (
	"testing"
	"time"

	"github.com/agile-security/clock"
	"github.com/stretchr/testify/require"
)

var theMostImportantDateEver = time.Date(1980, 8, 19, 0, 0, 0, 0, time.UTC)

func TestAdvance(t *testing.T) {
	c := clock.NewFakeClock(theMostImportantDateEver)
	c.Advance(time.Hour * 24)
	require.Equal(t, theMostImportantDateEver.Add(time.Hour*24), c.Now())
}

func TestTimer(t *testing.T) {
	c := clock.NewFakeClock(theMostImportantDateEver)
	timer := c.NewTimer(time.Hour * 25)
	c.Advance(time.Hour * 24)
	ensureNotTriggered(t, timer)
	c.Advance(time.Hour)
	ensureTriggered(t, timer)
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
	c := clock.NewFakeClock(theMostImportantDateEver)
	timer := c.NewTimer(time.Hour)
	stopped := timer.Stop()
	require.True(t, stopped)
	c.Advance(time.Hour * 2)
	ensureNotTriggered(t, timer)
}

func TestReset(t *testing.T) {
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

func TestResetNegative(t *testing.T) {
	c := clock.NewFakeClock(theMostImportantDateEver)
	timer := c.NewTimer(time.Hour)
	c.Advance(time.Hour * 2)
	ensureTriggered(t, timer)
	reset := timer.Reset(-time.Hour * 4)
	require.False(t, reset)
	ensureTriggered(t, timer)
}
