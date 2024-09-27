package clock

import (
	"sync"
	"time"
	"unsafe"

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
}

func (f *FakeClock) Now() time.Time {
	f.mux.Lock()
	defer f.mux.Unlock()
	return f.now
}

func (f *FakeClock) NewTimer(d time.Duration) Timer {
	f.mux.Lock()
	defer f.mux.Unlock()

	timer := &FakeTimer{
		clock:   f,
		c:       make(chan time.Time, 1),
		cancel:  make(chan any),
		trigger: f.now.Add(d),
	}

	if timer.trigger.Before(f.now) {
		timer.c <- timer.trigger
	} else {
		f.pendingTimers = f.pendingTimers.Add(timer)
	}

	return timer
}

func (f *FakeClock) AfterFunc(d time.Duration, fn func()) Timer {
	timer := f.NewTimer(d).(*FakeTimer)
	go func() {
		select {
		case <-timer.c:
			fn()
		case <-timer.cancel:
		}
	}()
	return timer
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
		timer.c <- timer.trigger
	}
}

type FakeTimer struct {
	clock   *FakeClock
	c       chan time.Time
	cancel  chan any
	trigger time.Time
}

func (f *FakeTimer) Less(other *FakeTimer) bool {
	if !f.trigger.Before(other.trigger) {
		return false
	}
	if uintptr(unsafe.Pointer(f)) >= uintptr(unsafe.Pointer(other)) {
		return false
	}
	return true
}

func (f *FakeTimer) Stop() bool {
	f.clock.mux.Lock()
	defer f.clock.mux.Unlock()
	return f.stop()
}

func (f *FakeTimer) C() <-chan time.Time {
	return f.c
}

func (f *FakeTimer) Reset(d time.Duration) bool {
	f.clock.mux.Lock()
	defer f.clock.mux.Unlock()
	stopped := f.stop()
	f.trigger = f.clock.now.Add(d)
	if f.trigger.Before(f.clock.now) {
		f.c <- f.trigger
	} else {
		f.clock.pendingTimers = f.clock.pendingTimers.Add(f)
	}
	return stopped
}

func (f *FakeTimer) stop() bool {
	if f.clock.pendingTimers.Contains(f) {
		f.clock.pendingTimers = f.clock.pendingTimers.Remove(f)
		close(f.cancel)
		return true
	}
	return false
}

func NewFakeClock(now time.Time) *FakeClock {
	return &FakeClock{
		now: now,
	}
}
