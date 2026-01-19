package clock

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/scottwis/persistent"
)

var timeLayouts = []string{
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

const (
	Day         = 24 * time.Hour
	SimpleDate1 = "2006-01-02"
	SimpleDate2 = "2026/01/02"
	UsDate1     = "01-02-2006"
)

type Clock interface {
	Now() time.Time
	NewTimer(d time.Duration) Timer
	AfterFunc(d time.Duration, f func()) Timer
	WithTimeout(parent context.Context, d time.Duration) (context.Context, context.CancelFunc)
}

type Timer interface {
	Stop() bool
	C() <-chan time.Time
	Reset(d time.Duration) bool
}

type RealClock struct{}

func (r RealClock) Now() time.Time {
	return time.Now()
}

func (r RealClock) NewTimer(d time.Duration) Timer {
	return RealTimer{Timer: time.NewTimer(d)}
}

func (r RealClock) AfterFunc(d time.Duration, f func()) Timer {
	return RealTimer{Timer: time.AfterFunc(d, f)}
}

func (r RealClock) WithTimeout(parent context.Context, d time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(parent, d)
}

type RealTimer struct {
	*time.Timer
}

func (r RealTimer) C() <-chan time.Time {
	return r.Timer.C
}

func NewRealClock() *RealClock {
	return &RealClock{}
}

type FakeClock struct {
	mux           sync.Mutex
	now           time.Time
	pendingTimers *persistent.SetEx[*FakeTimer]
	nextID        atomic.Int64
}

func (f *FakeClock) Now() time.Time {
	f.mux.Lock()
	defer f.mux.Unlock()
	return f.now
}

func (f *FakeClock) NewTimer(d time.Duration) Timer {
	f.mux.Lock()
	defer f.mux.Unlock()

	ret := &FakeTimer{
		clock:   f,
		c:       make(chan time.Time, 1),
		trigger: f.now.Add(d),
		id:      f.nextID.Add(1),
	}
	return f.addTimer(ret)
}

func (f *FakeClock) AfterFunc(d time.Duration, fn func()) Timer {
	f.mux.Lock()
	defer f.mux.Unlock()
	return f.afterFunc(d, fn)
}

func (f *FakeClock) afterFunc(d time.Duration, fn func()) Timer {
	ret := &FakeTimer{
		clock:   f,
		c:       nil,
		fn:      fn,
		trigger: f.now.Add(d),
		id:      f.nextID.Add(1),
	}
	return f.addTimer(ret)
}

func (f *FakeClock) Advance(d time.Duration) {
	f.mux.Lock()
	defer f.mux.Unlock()
	if d < 0 {
		panic("time cannot move backwards")
	}
	f.now = f.now.Add(d)

	for timer, ok := f.pendingTimers.GetKthElement(0); ok && !timer.trigger.After(f.now); timer, ok = f.pendingTimers.GetKthElement(0) {
		f.pendingTimers = f.pendingTimers.Remove(timer)
		timer.fire()
	}
}

func (f *FakeClock) addTimer(t *FakeTimer) Timer {
	if !t.trigger.After(f.now) {
		t.fire()
	} else {
		f.pendingTimers = f.pendingTimers.Add(t)
	}
	return t
}

func (f *FakeClock) WithTimeout(parent context.Context, d time.Duration) (context.Context, context.CancelFunc) {
	f.mux.Lock()
	defer f.mux.Unlock()
	ctx := &FakeDeadlineContext{
		Context:  parent,
		done:     make(chan struct{}),
		deadline: f.now.Add(d),
	}

	// If the deadline is already in the past, mark the context as deadline exceeded.
	if d <= 0 {
		ctx.setErrorOnce(context.DeadlineExceeded)
		return ctx, func() {
			// already canceled
		}
	}

	// if the parent is already done, propagate its completion.
	select {
	case <-parent.Done():
		ctx.setErrorOnce(parent.Err())
		return ctx, func() {
			// already canceled}
		}
	default:
	}

	// otherwise create a fake timer that trigger's deadline exceeded when it fires
	timer := f.afterFunc(
		d, func() {
			ctx.setErrorOnce(context.DeadlineExceeded)
		},
	)

	// generate a proper cancel function
	cancel := func() {
		timer.Stop()
		ctx.setErrorOnce(context.Canceled)
	}

	// and spin up a go routine that propagates cancellation from the parent context to the new context
	go func() {
		select {
		case <-ctx.done:
			return
		case <-parent.Done():
			timer.Stop()
			ctx.setErrorOnce(parent.Err())
		}
	}()

	// finally return the new context and the cancel function
	return ctx, cancel
}

type FakeTimer struct {
	clock   *FakeClock
	c       chan time.Time
	fn      func()
	trigger time.Time
	id      int64
}

func (f *FakeTimer) Stop() bool {
	f.clock.mux.Lock()
	defer f.clock.mux.Unlock()

	if f.clock.pendingTimers.Contains(f) {
		f.clock.pendingTimers = f.clock.pendingTimers.Remove(f)
		return true
	}
	return false
}

func (f *FakeTimer) C() <-chan time.Time {
	return f.c
}

func (f *FakeTimer) Reset(d time.Duration) bool {
	f.clock.mux.Lock()
	defer f.clock.mux.Unlock()

	ret := f.clock.pendingTimers.Contains(f)
	if ret {
		f.clock.pendingTimers = f.clock.pendingTimers.Remove(f)
	}
	f.trigger = f.clock.now.Add(d)
	f.clock.addTimer(f)
	return ret
}

func (f *FakeTimer) fire() {
	if f.fn != nil {
		go f.fn()
	} else {
		f.c <- f.trigger
	}
}
func (f *FakeTimer) Less(rhs *FakeTimer) bool {
	if f.trigger.Before(rhs.trigger) {
		return true
	}
	if f.trigger.After(rhs.trigger) {
		return false
	}
	return f.id < rhs.id
}

type FakeDeadlineContext struct {
	context.Context
	done     chan struct{}
	err      atomic.Pointer[error]
	deadline time.Time
}

func (ctx *FakeDeadlineContext) Deadline() (deadline time.Time, ok bool) {
	parentDeadline, ok := ctx.Context.Deadline()
	if ok && ctx.deadline.After(parentDeadline) {
		return parentDeadline, true
	}
	return ctx.deadline, true
}

func (ctx *FakeDeadlineContext) Done() <-chan struct{} {
	return ctx.done
}

func (ctx *FakeDeadlineContext) Err() error {
	select {
	case <-ctx.Done():
		return *ctx.err.Load()
	default:
		return nil
	}
}

func (ctx *FakeDeadlineContext) setErrorOnce(err error) {
	if ctx.err.CompareAndSwap(nil, &err) {
		close(ctx.done)
	}
}

func NewFakeClock(now time.Time) *FakeClock {
	return &FakeClock{
		now: now,
	}
}

func ParseDurationWithDays(value string) (time.Duration, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return 0, fmt.Errorf("duration is required")
	}

	sign := 1.0
	if strings.HasPrefix(trimmed, "+") {
		trimmed = trimmed[1:]
	} else if strings.HasPrefix(trimmed, "-") {
		sign = -1
		trimmed = trimmed[1:]
	}

	if len(trimmed) < 2 {
		return 0, fmt.Errorf("duration %q is missing units", value)
	}

	unit := trimmed[len(trimmed)-1]
	number := trimmed[:len(trimmed)-1]
	magnitude, err := strconv.ParseFloat(number, 64)
	if err != nil {
		return 0, err
	}

	var base time.Duration
	switch unit {
	case 'd':
		base = Day
	case 'h':
		base = time.Hour
	case 'm':
		base = time.Minute
	case 's':
		base = time.Second
	default:
		return 0, fmt.Errorf("invalid duration suffix %q", string(unit))
	}

	dur := time.Duration(sign * magnitude * float64(base))
	return dur, nil
}

func ParseTimeOrRelativeDuration(value string, now time.Time) (time.Time, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return time.Time{}, fmt.Errorf("time value is required")
	}

	for _, layout := range timeLayouts {
		timestamp, err := time.Parse(layout, trimmed)
		if err == nil {
			return timestamp, nil
		}
	}

	dur, durErr := ParseDurationWithDays(trimmed)
	if durErr != nil {
		return time.Time{}, fmt.Errorf("failed to parse %q as duration or RFC3339 time", value)
	}

	return now.Add(dur), nil
}
