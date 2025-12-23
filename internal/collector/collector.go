package collector

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
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
	nodeStats         map[string]*NodeStats
	// optional config
	sampleDir string
}

type NodeStats struct {
	BytesUp      int64
	BytesDown    int64
	ProtocolFreq map[string]int
	LastSeen     time.Time
}

func NewCollector(s *storage.SQLiteStore) *Collector {
	c := &Collector{
		store:             s,
		activeConnections: make(map[string]time.Time),
		nodeStats:         make(map[string]*NodeStats),
		sampleDir:         "data/packets",
	}
	go c.periodicLogAndFlush(30 * time.Second)
	return c
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

func (c *Collector) periodicLogAndFlush(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for range ticker.C {
		c.mu.Lock()
		totalNodes := len(c.nodeStats)
		// Build top nodes by bytes
		type kv struct {
			Node  string
			Bytes int64
		}
		var arr []kv
		var totalUp, totalDown int64
		for node, ns := range c.nodeStats {
			totalUp += ns.BytesUp
			totalDown += ns.BytesDown
			arr = append(arr, kv{Node: node, Bytes: ns.BytesUp + ns.BytesDown})
		}
		c.mu.Unlock()

		// sort top
		sort.Slice(arr, func(i, j int) bool { return arr[i].Bytes > arr[j].Bytes })

		topN := 5
		if len(arr) < topN {
			topN = len(arr)
		}
		topSummary := arr[:topN]

		log.Printf("[collector] nodes=%d total_up=%d total_down=%d top=%v", totalNodes, totalUp, totalDown, topSummary)
	}
}

func (c *Collector) appendSample(nodeID, payload string) {
	if nodeID == "" || payload == "" {
		return
	}
	// ensure dir exists
	_ = os.MkdirAll(c.sampleDir, 0o755)
	fname := filepath.Join(c.sampleDir, nodeID+".log")
	f, err := os.OpenFile(fname, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		// optionally record metric or log the error
		return
	}
	defer f.Close()
	ts := time.Now().UTC().Format(time.RFC3339)
	// write a small line: timestamp + payload newline
	fmt.Fprintf(f, "%s %s\n", ts, payload)
}

func (c *Collector) updateNodeStats(nodeID string, proto string, up, down int64) {
	if nodeID == "" {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	ns := c.nodeStats[nodeID]
	if ns == nil {
		ns = &NodeStats{
			ProtocolFreq: make(map[string]int),
		}
		c.nodeStats[nodeID] = ns
	}
	ns.BytesUp += up
	ns.BytesDown += down
	if proto == "" {
		proto = "unknown"
	}
	ns.ProtocolFreq[proto]++
	ns.LastSeen = time.Now().UTC()
}
