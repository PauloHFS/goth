package sse

import (
	"context"
	"sync"
)

type Broker interface {
	Broadcast(event, data string)
	BroadcastToUser(userID int64, event, data string)
	Shutdown()
}

type clientRegistration struct {
	UserID int64
	Chan   chan string
}

type targetedMessage struct {
	UserID int64
	Event  string
	Data   string
}

type broker struct {
	Users     map[int64][]chan string
	Mu        sync.Mutex
	NewClient chan clientRegistration
	Closing   chan clientRegistration
	Message   chan targetedMessage
	Wg        sync.WaitGroup
	Ctx       context.Context
	Cancel    context.CancelFunc
}

func NewBroker(ctx context.Context) Broker {
	brokerCtx, cancel := context.WithCancel(ctx)
	b := &broker{
		Users:     make(map[int64][]chan string),
		Mu:        sync.Mutex{},
		NewClient: make(chan clientRegistration),
		Closing:   make(chan clientRegistration),
		Message:   make(chan targetedMessage),
		Ctx:       brokerCtx,
		Cancel:    cancel,
	}
	go b.listen()
	return b
}

func (b *broker) listen() {
	b.Wg.Add(1)
	defer b.Wg.Done()

	for {
		select {
		case <-b.Ctx.Done():
			b.Mu.Lock()
			for _, channels := range b.Users {
				for _, ch := range channels {
					close(ch)
				}
			}
			b.Users = make(map[int64][]chan string)
			b.Mu.Unlock()
			return

		case reg := <-b.NewClient:
			b.Mu.Lock()
			b.Users[reg.UserID] = append(b.Users[reg.UserID], reg.Chan)
			b.Mu.Unlock()

		case reg := <-b.Closing:
			b.Mu.Lock()
			clients := b.Users[reg.UserID]
			for i, ch := range clients {
				if ch == reg.Chan {
					b.Users[reg.UserID] = append(clients[:i], clients[i+1:]...)
					break
				}
			}
			if len(b.Users[reg.UserID]) == 0 {
				delete(b.Users, reg.UserID)
			}
			b.Mu.Unlock()

		case tm := <-b.Message:
			b.Mu.Lock()
			msg := tm.Event + "\ndata: " + tm.Data + "\n\n"

			if tm.UserID == 0 {
				// Broadcast to all users
				for _, channels := range b.Users {
					for _, ch := range channels {
						// Non-blocking send: drop message for slow clients
						select {
						case ch <- msg:
						default:
							// Client is slow, drop message to prevent blocking other clients
							// Could add logging here if needed
						}
					}
				}
			} else {
				// Targeted message to specific user
				if channels, ok := b.Users[tm.UserID]; ok {
					for _, ch := range channels {
						// Non-blocking send: drop message for slow clients
						select {
						case ch <- msg:
						default:
							// Client is slow, drop message
						}
					}
				}
			}
			b.Mu.Unlock()
		}
	}
}

func (b *broker) Broadcast(event, data string) {
	select {
	case <-b.Ctx.Done():
		return
	case b.Message <- targetedMessage{UserID: 0, Event: event, Data: data}:
	}
}

func (b *broker) BroadcastToUser(userID int64, event, data string) {
	select {
	case <-b.Ctx.Done():
		return
	case b.Message <- targetedMessage{UserID: userID, Event: event, Data: data}:
	}
}

func (b *broker) Shutdown() {
	b.Cancel()
	b.Wg.Wait()
}

func RegisterClient(b Broker, userID int64, ch chan string) {
	if bb, ok := b.(*broker); ok {
		bb.NewClient <- clientRegistration{UserID: userID, Chan: ch}
	}
}

func UnregisterClient(b Broker, userID int64, ch chan string) {
	if bb, ok := b.(*broker); ok {
		bb.Closing <- clientRegistration{UserID: userID, Chan: ch}
	}
}
