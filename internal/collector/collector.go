package collector

import (
	"context"
	"sync"
	"time"

	"github.com/cmengs/xray-react/collector/internal/storage"
)

// Collector is responsible for accepting events and forwarding them to storage.
// It also tracks active connections using a mutex-protected map; each connection
// is identified by a connection_id (string).
type Collector struct {
	store *storage.SQLiteStore

	mu                sync.Mutex
	activeConnections map[string]time.Time
}

func NewCollector(s *storage.SQLiteStore) *Collector {
	return &Collector{
		store:             s,
		activeConnections: make(map[string]time.Time),
	}
}

// AddConnection records a new active connection with the current timestamp.
func (c *Collector) AddConnection(connectionID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.activeConnections[connectionID] = time.Now().UTC()
}

// RemoveConnection removes a connection from the active map.
func (c *Collector) RemoveConnection(connectionID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.activeConnections, connectionID)
}

// ActiveConnectionsCount returns the number of currently tracked active connections.
func (c *Collector) ActiveConnectionsCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.activeConnections)
}

// SaveEvent persists an event payload to the configured storage and associates it
// with the provided connection_id.
func (c *Collector) SaveEvent(ctx context.Context, payload, connectionID string) error {
	count, err := c.store.InsertEvent(ctx, payload, connectionID)
	if count == 0 {
		return err
	}
	return err
}
