package mcp

import (
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"
)

// SSEEvent represents a Server-Sent Event
type SSEEvent struct {
	Type      string                 `json:"type"`
	Data      map[string]interface{} `json:"data"`
	Timestamp time.Time              `json:"timestamp"`
}

// SSEClient represents a connected SSE client
type SSEClient struct {
	ID     string
	Events chan SSEEvent
	Done   chan bool
	UserID string
}

// SSEManager manages SSE connections
type SSEManager struct {
	clients   map[string]*SSEClient
	mu        sync.RWMutex
	broadcast chan SSEEvent
}

// NewSSEManager creates a new SSE manager
func NewSSEManager() *SSEManager {
	manager := &SSEManager{
		clients:   make(map[string]*SSEClient),
		broadcast: make(chan SSEEvent, 100),
	}

	// Start broadcast handler
	go manager.handleBroadcasts()

	return manager
}

// AddClient adds a new SSE client
func (m *SSEManager) AddClient(client *SSEClient) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.clients[client.ID] = client
	log.Printf("SSE client %s connected", client.ID)

	// Send connection event
	client.Events <- SSEEvent{
		Type: "connected",
		Data: map[string]interface{}{
			"message":  "SSE connection established",
			"clientId": client.ID,
		},
		Timestamp: time.Now(),
	}
}

// RemoveClient removes an SSE client
func (m *SSEManager) RemoveClient(clientID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if client, ok := m.clients[clientID]; ok {
		close(client.Events)
		close(client.Done)
		delete(m.clients, clientID)
		log.Printf("SSE client %s disconnected", clientID)
	}
}

// SendToClient sends an event to a specific client
func (m *SSEManager) SendToClient(clientID string, event SSEEvent) error {
	m.mu.RLock()
	client, ok := m.clients[clientID]
	m.mu.RUnlock()

	if !ok {
		return fmt.Errorf("client %s not found", clientID)
	}

	select {
	case client.Events <- event:
		return nil
	case <-time.After(5 * time.Second):
		return fmt.Errorf("timeout sending event to client %s", clientID)
	}
}

// Broadcast sends an event to all connected clients
func (m *SSEManager) Broadcast(event SSEEvent) {
	m.broadcast <- event
}

// BroadcastCrawlUpdate broadcasts a crawl update
func (m *SSEManager) BroadcastCrawlUpdate(job *CrawlJob) {
	job.mu.RLock()
	jobID := job.ID
	jobURL := job.URL
	jobStatus := job.Status
	jobProgress := job.Progress
	jobTotal := job.Total
	jobEndTime := job.EndTime
	jobStartTime := job.StartTime
	jobError := job.Error
	job.mu.RUnlock()

	event := SSEEvent{
		Type: "crawl_progress",
		Data: map[string]interface{}{
			"jobId":    jobID,
			"url":      jobURL,
			"status":   jobStatus,
			"progress": jobProgress,
			"total":    jobTotal,
		},
		Timestamp: time.Now(),
	}

	if jobStatus == "completed" {
		event.Type = "crawl_completed"
		if jobEndTime != nil {
			event.Data["duration"] = jobEndTime.Sub(jobStartTime).Seconds()
		}
	} else if jobStatus == "failed" {
		event.Type = "error"
		event.Data["error"] = jobError
	} else if jobStatus == "running" && jobProgress == 1 {
		event.Type = "crawl_started"
	}

	m.Broadcast(event)
}

// handleBroadcasts handles broadcasting events to all clients
func (m *SSEManager) handleBroadcasts() {
	for event := range m.broadcast {
		m.mu.RLock()
		clients := make([]*SSEClient, 0, len(m.clients))
		for _, client := range m.clients {
			clients = append(clients, client)
		}
		m.mu.RUnlock()

		for _, client := range clients {
			select {
			case client.Events <- event:
			default:
				// Client's channel is full, skip
				log.Printf("Skipping event for client %s (channel full)", client.ID)
			}
		}
	}
}

// FormatSSEMessage formats an event for SSE transmission
func FormatSSEMessage(event SSEEvent) (string, error) {
	data, err := json.Marshal(map[string]interface{}{
		"type":      event.Type,
		"data":      event.Data,
		"timestamp": event.Timestamp.Format(time.RFC3339),
	})
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("data: %s\n\n", string(data)), nil
}
