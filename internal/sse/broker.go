package sse

import (
	"fmt"
	"net/http"
	"strings"
	"sync"
)

const (
	maxClientsPerResource = 100
	maxGlobalClients      = 1000
)

type Client struct {
	Events chan string
}

type Broker struct {
	clients      map[string]map[*Client]bool
	mutex        sync.RWMutex
	stop         chan struct{}
	totalClients int
}

func NewBroker() *Broker {
	return &Broker{
		clients: make(map[string]map[*Client]bool),
		stop:    make(chan struct{}),
	}
}

func (b *Broker) GetResourceKey(resourceType, resourceID string) string {
	return fmt.Sprintf("%s:%s", resourceType, resourceID)
}

func (b *Broker) Subscribe(resourceType, resourceID string) (*Client, error) {
	key := b.GetResourceKey(resourceType, resourceID)

	b.mutex.Lock()
	defer b.mutex.Unlock()

	if b.totalClients >= maxGlobalClients {
		return nil, fmt.Errorf("max global connections reached")
	}

	if b.clients[key] == nil {
		b.clients[key] = make(map[*Client]bool)
	}

	if len(b.clients[key]) >= maxClientsPerResource {
		return nil, fmt.Errorf("max connections for resource reached")
	}

	client := &Client{
		Events: make(chan string, 100),
	}

	b.clients[key][client] = true
	b.totalClients++
	return client, nil
}

func (b *Broker) Unsubscribe(client *Client, resourceType, resourceID string) {
	key := b.GetResourceKey(resourceType, resourceID)

	b.mutex.Lock()
	defer b.mutex.Unlock()

	if clients, ok := b.clients[key]; ok {
		delete(clients, client)
		close(client.Events)
		if len(clients) == 0 {
			delete(b.clients, key)
		}
		b.totalClients--
	}
}

func (b *Broker) SendHTML(resourceType, resourceID, eventType, html string) {
	key := b.GetResourceKey(resourceType, resourceID)

	b.mutex.RLock()
	defer b.mutex.RUnlock()

	var formattedData strings.Builder
	lines := strings.Split(html, "\n")
	for i, line := range lines {
		formattedData.WriteString("data: " + line)
		if i < len(lines)-1 {
			formattedData.WriteString("\n")
		}
	}

	message := fmt.Sprintf("event: %s\n%s\n\n", eventType, formattedData.String())

	for client := range b.clients[key] {
		select {
		case client.Events <- message:
		default:
		}
	}
}

func (b *Broker) SendEvaluationProgress(evaluationID, phase string, progress, total int, html string) {
	b.SendHTML("evaluation", evaluationID, "evaluation_progress", html)
}

func (b *Broker) SendEvaluationComplete(evaluationID, html string) {
	b.SendHTML("evaluation", evaluationID, "evaluation_complete", html)
}

func (b *Broker) SendEvaluationError(evaluationID, html string) {
	b.SendHTML("evaluation", evaluationID, "evaluation_error", html)
}

func (b *Broker) Shutdown() {
	close(b.stop)
}

var globalBroker *Broker

func Shutdown() {
	if globalBroker != nil {
		globalBroker.Shutdown()
	}
}

func Global() *Broker {
	if globalBroker == nil {
		globalBroker = NewBroker()
	}
	return globalBroker
}

func (b *Broker) Handler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		resourceType := r.URL.Query().Get("type")
		resourceID := r.URL.Query().Get("id")

		if resourceType == "" || resourceID == "" {
			http.Error(w, "type and id required", http.StatusBadRequest)
			return
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("X-Accel-Buffering", "no")

		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "Streaming not supported", http.StatusInternalServerError)
			return
		}

		client, err := b.Subscribe(resourceType, resourceID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusServiceUnavailable)
			return
		}
		defer b.Unsubscribe(client, resourceType, resourceID)

		fmt.Fprintf(w, ": ok\n\n")
		flusher.Flush()

		for {
			select {
			case message, ok := <-client.Events:
				if !ok {
					return
				}
				fmt.Fprint(w, message)
				flusher.Flush()
			case <-r.Context().Done():
				return
			}
		}
	}
}

type AuthHandlerFunc func(w http.ResponseWriter, r *http.Request)

func (b *Broker) AuthHandler(authFunc func(r *http.Request) (userID int64, resourceType, resourceID string)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		_, resourceType, resourceID := authFunc(r)
		if resourceType == "" || resourceID == "" {
			http.Error(w, "type and id required", http.StatusBadRequest)
			return
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("X-Accel-Buffering", "no")

		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "Streaming not supported", http.StatusInternalServerError)
			return
		}

		client, err := b.Subscribe("user:"+resourceType, resourceID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusServiceUnavailable)
			return
		}

		defer b.Unsubscribe(client, "user:"+resourceType, resourceID)

		fmt.Fprintf(w, ": ok\n\n")
		flusher.Flush()

		for {
			select {
			case message, ok := <-client.Events:
				if !ok {
					return
				}
				fmt.Fprint(w, message)
				flusher.Flush()
			case <-r.Context().Done():
				return
			}
		}
	}
}
