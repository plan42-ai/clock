package clock_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/plan42-ai/clock"
	"github.com/stretchr/testify/require"
)

func TestParseDurationWithDays(t *testing.T) {
	t.Parallel()

	dur, err := clock.ParseDurationWithDays("1.5d")
	require.NoError(t, err)
	require.Equal(t, 36*time.Hour, dur)
}

func TestParseDurationWithDaysInvalid(t *testing.T) {
	t.Parallel()

	_, err := clock.ParseDurationWithDays("2x")
	require.Error(t, err)
}

func TestParseTimeOrDurationWithDuration(t *testing.T) {
	t.Parallel()

	now := time.Date(2024, 7, 10, 12, 0, 0, 0, time.UTC)
	timeValue, err := clock.ParseTimeOrRelativeDuration("-1.5h", now)
	require.NoError(t, err)
	require.True(t, now.Add(-90*time.Minute).Equal(timeValue))
}

func TestParseTimeOrDurationWithTime(t *testing.T) {
	t.Parallel()

	expected := time.Date(2024, 7, 10, 10, 0, 0, 0, time.UTC)
	timeValue, err := clock.ParseTimeOrRelativeDuration(expected.Format(time.RFC3339), time.Now())
	require.NoError(t, err)
	require.True(t, expected.Equal(timeValue))
}

func TestParseTimeOrDurationWithNanoTime(t *testing.T) {
	t.Parallel()

	expected := time.Date(2024, 7, 10, 10, 0, 0, 123456000, time.UTC)
	timeValue, err := clock.ParseTimeOrRelativeDuration(expected.Format(time.RFC3339Nano), time.Now())
	require.NoError(t, err)
	require.True(t, expected.Equal(timeValue))
}

func TestParseTimeOrDurationInvalid(t *testing.T) {
	t.Parallel()

	_, err := clock.ParseTimeOrRelativeDuration("invalid", time.Now())
	require.Error(t, err)
}

func TestParseTimeOrRelativeDuration_AllLayouts(t *testing.T) {
	t.Parallel()

	layouts := []string{
		time.RFC3339,
		time.RFC3339Nano,
		"2006-01-02",
		"2006/01/02",
		"01-02-2006",
		"01/02/2006",
		"2006.01.02",
		"01.02.2006",
		"2006-01-02 15:04:05",
		"2006-01-02 03:04:05 PM",
		"2006-01-02 15:04:05 MST",
		"2006-01-02 03:04:05 PM MST",
		"2006-01-02 15:04:05 -0700",
		"2006-01-02 03:04:05 PM -0700",
		"2006-01-02 15:04:05Z07:00",
		"2006-01-02 15:04:05.999999999Z07:00",
		"2006/01/02 15:04:05 MST",
		"2006/01/02 03:04:05 PM MST",
		"2006/01/02 15:04:05 -0700",
		"2006/01/02 03:04:05 PM -0700",
		"2006/01/02 15:04:05Z07:00",
		"2006/01/02 15:04:05.999999999Z07:00",
		"2006.01.02 15:04:05",
		"2006.01.02 03:04:05 PM",
		"2006.01.02 15:04:05 MST",
		"2006.01.02 03:04:05 PM MST",
		"2006.01.02 15:04:05 -0700",
		"2006.01.02 03:04:05 PM -0700",
		"2006.01.02 15:04:05Z07:00",
		"2006.01.02 15:04:05.999999999Z07:00",
		"01-02-2006 15:04:05",
		"01-02-2006 03:04:05 PM",
		"01-02-2006 15:04:05 MST",
		"01-02-2006 03:04:05 PM MST",
		"01-02-2006 15:04:05 -0700",
		"01-02-2006 03:04:05 PM -0700",
		"01-02-2006 15:04:05Z07:00",
		"01/02/2006 15:04:05",
		"01/02/2006 03:04:05 PM",
		"01/02/2006 15:04:05 MST",
		"01/02/2006 03:04:05 PM MST",
		"01/02/2006 15:04:05 -0700",
		"01/02/2006 03:04:05 PM -0700",
		"01/02/2006 15:04:05Z07:00",
	}

	// fixed sample time used to produce deterministic formatted strings
	sample := time.Date(2024, 7, 10, 10, 0, 0, 123456000, time.UTC)

	for i, layout := range layouts {
		i, layout := i, layout // capture
		t.Run(
			fmt.Sprintf("layout_%02d_%s", i, layout), func(t *testing.T) {
				t.Parallel()
				str := sample.Format(layout)

				// expected using the stdlib parse for that layout
				expected, perr := time.Parse(layout, str)
				require.NoError(t, perr, "time.Parse failed for layout %q and string %q", layout, str)

				parsed, err := clock.ParseTimeOrRelativeDuration(str, time.Now())
				require.NoError(t, err, "ParseTimeOrRelativeDuration failed for layout %q and string %q", layout, str)
				require.Truef(
					t,
					expected.Equal(parsed),
					"parsed time does not match expected for layout %q and string %q: expected %v, got %v",
					layout,
					str,
					expected,
					parsed,
				)
			},
		)
	}
}
