package grainactor

import (
	"time"

	"github.com/asynkron/protoactor-go/actor"
	"github.com/asynkron/protoactor-go/scheduler"
)

type scheduleAfter struct {
	name  string
	delay time.Duration
	fn    func(Context)
}

type scheduleEvery struct {
	name     string
	interval time.Duration
	fn       func(Context)
}

type cancelTimer struct {
	name string
}

type timerFire struct {
	name string
}

func (*timerFire) NotInfluenceReceiveTimeout() {}

func (a *BaseActor) receiveScheduleAfter(ctx actor.Context, msg *scheduleAfter) {
	if msg == nil || msg.fn == nil {
		return
	}
	a.cancelTimer(msg.name)
	a.timerHandlers[msg.name] = msg.fn
	a.timerEvery[msg.name] = 0
	a.scheduleOnce(ctx, msg.name, msg.delay)
}

func (a *BaseActor) receiveScheduleEvery(ctx actor.Context, msg *scheduleEvery) {
	if msg == nil || msg.fn == nil {
		return
	}
	a.cancelTimer(msg.name)
	a.timerHandlers[msg.name] = msg.fn
	a.timerEvery[msg.name] = msg.interval
	a.scheduleRepeat(ctx, msg.name, msg.interval)
}

func (a *BaseActor) receiveCancelTimer(msg *cancelTimer) {
	if msg == nil {
		return
	}
	a.cancelTimer(msg.name)
}

func (a *BaseActor) receiveTimerFire(msg *timerFire) {
	if msg == nil {
		return
	}
	fn, ok := a.timerHandlers[msg.name]
	if !ok || fn == nil {
		return
	}
	if a.ctx != nil {
		fn(a.ctx)
	}
	if a.timerEvery[msg.name] == 0 {
		delete(a.timerHandlers, msg.name)
		delete(a.timerCancels, msg.name)
	}
}

func (a *BaseActor) scheduleOnce(ctx actor.Context, name string, delay time.Duration) {
	if delay <= 0 || ctx == nil {
		return
	}
	timerName := name
	s := scheduler.NewTimerScheduler(ctx)
	a.timerCancels[timerName] = s.SendOnce(delay, ctx.Self(), &timerFire{name: timerName})
}

func (a *BaseActor) scheduleRepeat(ctx actor.Context, name string, interval time.Duration) {
	if interval <= 0 || ctx == nil {
		return
	}
	timerName := name
	s := scheduler.NewTimerScheduler(ctx)
	a.timerCancels[timerName] = s.SendRepeatedly(interval, interval, ctx.Self(), &timerFire{name: timerName})
}

func (a *BaseActor) cancelTimer(name string) {
	if cancel, ok := a.timerCancels[name]; ok && cancel != nil {
		cancel()
	}
	delete(a.timerCancels, name)
	delete(a.timerHandlers, name)
	delete(a.timerEvery, name)
}

func (a *BaseActor) clearTimers() {
	for name := range a.timerCancels {
		a.cancelTimer(name)
	}
}
