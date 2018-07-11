package rpc

import ()

// 事件
type Event struct {
	EType EVENTSTATUS
	Conn  *Connection
	Pack  *Pack
}

// 创建新事件
func NewEvent(eType EVENTSTATUS, c *Connection, p *Pack) *Event {
	return &Event{
		EType: eType,
		Conn:  c,
		Pack:  p,
	}
}

func NewEventTask(event *Event, f func(e *Event, err error), err error) *EventTask {
	return &EventTask{Event: event, Err: err, f: f}
}

type EventTask struct {
	Event *Event
	Err   error
	f     func(e *Event, err error)
}

func (et EventTask) Run() {

	et.f(et.Event, et.Err)

}
