package collector

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/cmengs/xray-react/collector/internal/parser"
	"github.com/cmengs/xray-react/collector/internal/storage"
	"github.com/prometheus/client_golang/prometheus"
)

type Collector struct {
	store storage.Store

	// Prometheus metrics
	connectionsTotal  *prometheus.CounterVec
	connectionsActive *prometheus.GaugeVec
	connectionBytes   *prometheus.CounterVec
	connectionErrors  *prometheus.CounterVec
	authFailures      *prometheus.CounterVec
}

func NewCollector(store storage.Store) *Collector {
	c := &Collector{
		store: store,
		connectionsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "proxy_connections_total",
				Help: "Total number of processed connection events",
			},
			[]string{"protocol", "status"},
		),
		connectionsActive: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "proxy_connections_active",
				Help: "Currently active connections by protocol",
			},
			[]string{"protocol"},
		),
		connectionBytes: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "proxy_bytes_total",
				Help: "Total bytes transferred",
			},
			[]string{"protocol", "direction"},
		),
		connectionErrors: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "proxy_connection_errors_total",
				Help: "Connection errors by protocol and reason",
			},
			[]string{"protocol", "reason"},
		),
		authFailures: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "proxy_auth_failures_total",
				Help: "Authentication failures by protocol and user/id where available",
			},
			[]string{"protocol", "user_or_id"},
		),
	}

	prometheus.MustRegister(c.connectionsTotal, c.connectionsActive, c.connectionBytes, c.connectionErrors, c.authFailures)
	return c
}

// Expected JSON format (simple PoC)
type IngestEvent struct {
	Timestamp  time.Time              `json:"timestamp"`
	SrcIP      string                 `json:"src_ip"`
	SrcPort    int                    `json:"src_port"`
	DstIP      string                 `json:"dst_ip"`
	DstPort    int                    `json:"dst_port"`
	Protocol   string                 `json:"protocol"`
	User       string                 `json:"user,omitempty"`
	BytesUp    int64                  `json:"bytes_up,omitempty"`
	BytesDown  int64                  `json:"bytes_down,omitempty"`
	Status     string                 `json:"status,omitempty"` // established, closed, failed
	Reason     string                 `json:"reason,omitempty"` // auth_failed, timeout, etc
	DurationMs int64                  `json:"duration_ms,omitempty"`
	Meta       map[string]interface{} `json:"meta,omitempty"`
}

func (c *Collector) HandleIngest(req *http.Request, w http.ResponseWriter) error {
	body, err := io.ReadAll(req.Body)
	if err != nil {
		http.Error(w, "failed read body", http.StatusBadRequest)
		return err
	}
	var ev IngestEvent
	if err := json.Unmarshal(body, &ev); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return err
	}

	// Normalize protocol using parser helper
	proto := parser.NormalizeProtocol(ev.Protocol)

	// Extract and canonicalize protocol-specific meta
	meta := parser.ExtractMeta(ev.Meta, proto)
	// allow overriding user from meta if present
	if u, ok := meta["user"].(string); ok && u != "" {
		ev.User = u
	}

	// Update metrics
	status := ev.Status
	if status == "" {
		status = "established"
	}
	c.connectionsTotal.WithLabelValues(proto, status).Inc()
	if status == "established" {
		c.connectionsActive.WithLabelValues(proto).Inc()
	} else if status == "closed" || status == "failed" {
		// decrease active (best-effort PoC)
		c.connectionsActive.WithLabelValues(proto).Dec()
	}
	if ev.BytesUp > 0 {
		c.connectionBytes.WithLabelValues(proto, "up").Add(float64(ev.BytesUp))
	}
	if ev.BytesDown > 0 {
		c.connectionBytes.WithLabelValues(proto, "down").Add(float64(ev.BytesDown))
	}
	if ev.Reason != "" && status == "failed" {
		c.connectionErrors.WithLabelValues(proto, ev.Reason).Inc()
		// handle auth failures
		if ev.Reason == "auth_failed" {
			userOrID := ev.User
			if userOrID == "" {
				// try from meta keys commonly present
				if v, ok := meta["id"].(string); ok {
					userOrID = v
				} else if v, ok := meta["uuid"].(string); ok {
					userOrID = v
				}
			}
			if userOrID == "" {
				userOrID = "unknown"
			}
			c.authFailures.WithLabelValues(proto, userOrID).Inc()
		}
	}

	// Persist event
	storeEv := storage.ConnectionEvent{
		Timestamp:  ev.Timestamp,
		SrcIP:      ev.SrcIP,
		SrcPort:    ev.SrcPort,
		DstIP:      ev.DstIP,
		DstPort:    ev.DstPort,
		Protocol:   proto,
		User:       ev.User,
		BytesUp:    ev.BytesUp,
		BytesDown:  ev.BytesDown,
		Status:     status,
		Reason:     ev.Reason,
		DurationMs: ev.DurationMs,
		Meta:       meta,
	}
	if err := c.store.Insert(storeEv); err != nil {
		// Write response but continue
		http.Error(w, fmt.Sprintf("store insert error: %v", err), http.StatusInternalServerError)
		return err
	}

	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"result":"ok"}`))
	return nil
}
