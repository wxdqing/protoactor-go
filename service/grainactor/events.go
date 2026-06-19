package grainactor

type eventRegister struct {
	name    string
	handler func(Context, any)
}

type eventUnregister struct {
	name string
}

type eventEmit struct {
	name    string
	payload any
}

func (*eventEmit) NotInfluenceReceiveTimeout() {}

func (a *BaseActor) receiveEventRegister(msg *eventRegister) {
	if msg == nil || msg.handler == nil || msg.name == "" {
		return
	}
	a.eventHandlers[msg.name] = append(a.eventHandlers[msg.name], msg.handler)
}

func (a *BaseActor) receiveEventUnregister(msg *eventUnregister) {
	if msg == nil || msg.name == "" {
		return
	}
	delete(a.eventHandlers, msg.name)
}

func (a *BaseActor) receiveEventEmit(msg *eventEmit) {
	if msg == nil || msg.name == "" || a.ctx == nil {
		return
	}
	for _, handler := range a.eventHandlers[msg.name] {
		if handler != nil {
			handler(a.ctx, msg.payload)
		}
	}
}

func (a *BaseActor) clearEvents() {
	a.eventHandlers = make(map[string][]func(Context, any))
}
