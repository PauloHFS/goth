package sse

import (
	"context"
	"testing"
	"time"
)

func TestBroker_Broadcast(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	broker := NewBroker(ctx)
	defer broker.Shutdown()

	ch := make(chan string, 10)
	RegisterClient(broker, 1, ch)
	defer UnregisterClient(broker, 1, ch)

	broker.Broadcast("test_event", "test_data")

	select {
	case msg := <-ch:
		if msg != "test_event\ndata: test_data\n\n" {
			t.Errorf("unexpected message: %s", msg)
		}
	case <-time.After(1 * time.Second):
		t.Error("timeout waiting for message")
	}
}

func TestBroker_BroadcastToUser(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	broker := NewBroker(ctx)
	defer broker.Shutdown()

	ch := make(chan string, 10)
	RegisterClient(broker, 1, ch)
	defer UnregisterClient(broker, 1, ch)

	broker.BroadcastToUser(1, "test_event", "test_data")

	select {
	case msg := <-ch:
		if msg != "test_event\ndata: test_data\n\n" {
			t.Errorf("unexpected message: %s", msg)
		}
	case <-time.After(1 * time.Second):
		t.Error("timeout waiting for message")
	}
}

func TestBroker_BroadcastToUser_OtherUser(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	broker := NewBroker(ctx)
	defer broker.Shutdown()

	ch := make(chan string, 10)
	RegisterClient(broker, 1, ch)
	defer UnregisterClient(broker, 1, ch)

	broker.BroadcastToUser(2, "test_event", "test_data")

	select {
	case msg := <-ch:
		t.Errorf("unexpected message received: %s", msg)
	case <-time.After(100 * time.Millisecond):
	}
}

func TestBroker_Broadcast_AllUsers(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	broker := NewBroker(ctx)
	defer broker.Shutdown()

	ch1 := make(chan string, 10)
	ch2 := make(chan string, 10)
	RegisterClient(broker, 1, ch1)
	RegisterClient(broker, 2, ch2)
	defer UnregisterClient(broker, 1, ch1)
	defer UnregisterClient(broker, 2, ch2)

	broker.Broadcast("test_event", "test_data")

	select {
	case msg := <-ch1:
		if msg != "test_event\ndata: test_data\n\n" {
			t.Errorf("unexpected message on ch1: %s", msg)
		}
	case <-time.After(1 * time.Second):
		t.Error("timeout waiting for message on ch1")
	}

	select {
	case msg := <-ch2:
		if msg != "test_event\ndata: test_data\n\n" {
			t.Errorf("unexpected message on ch2: %s", msg)
		}
	case <-time.After(1 * time.Second):
		t.Error("timeout waiting for message on ch2")
	}
}

func TestBroker_Shutdown(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	broker := NewBroker(ctx)

	ch := make(chan string, 10)
	RegisterClient(broker, 1, ch)

	broker.Shutdown()

	select {
	case _, ok := <-ch:
		if ok {
			t.Error("channel should be closed after shutdown")
		}
	case <-time.After(1 * time.Second):
		t.Error("timeout waiting for channel close")
	}

	cancel()
}

func TestBroker_MultipleClientsSameUser(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	broker := NewBroker(ctx)
	defer broker.Shutdown()

	ch1 := make(chan string, 10)
	ch2 := make(chan string, 10)
	RegisterClient(broker, 1, ch1)
	RegisterClient(broker, 1, ch2)
	defer UnregisterClient(broker, 1, ch1)
	defer UnregisterClient(broker, 1, ch2)

	broker.BroadcastToUser(1, "test_event", "test_data")

	select {
	case msg := <-ch1:
		if msg != "test_event\ndata: test_data\n\n" {
			t.Errorf("unexpected message on ch1: %s", msg)
		}
	case <-time.After(1 * time.Second):
		t.Error("timeout waiting for message on ch1")
	}

	select {
	case msg := <-ch2:
		if msg != "test_event\ndata: test_data\n\n" {
			t.Errorf("unexpected message on ch2: %s", msg)
		}
	case <-time.After(1 * time.Second):
		t.Error("timeout waiting for message on ch2")
	}
}

func TestBroker_UnregisterClient(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	broker := NewBroker(ctx)
	defer broker.Shutdown()

	ch := make(chan string, 10)
	RegisterClient(broker, 1, ch)
	UnregisterClient(broker, 1, ch)

	broker.BroadcastToUser(1, "test_event", "test_data")

	select {
	case msg := <-ch:
		t.Errorf("unexpected message after unregister: %s", msg)
	case <-time.After(100 * time.Millisecond):
	}
}
