package clock

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/scottwis/persistent"
)

type Clock interface {
	Now() time.Time
	NewTimer(d time.Duration) Timer
	AfterFunc(d time.Duration, f func()) Timer
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

func NewFakeClock(now time.Time) *FakeClock {
	return &FakeClock{
		now: now,
	}
}
