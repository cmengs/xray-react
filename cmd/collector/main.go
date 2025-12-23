package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/google/uuid"
	"github.com/cmengs/xray-react/internal/collector"
	"github.com/cmengs/xray-react/internal/storage"
)

func main() {
	// Read config from env
	token := os.Getenv("COLLECTOR_API_TOKEN")
	if token == "" {
		log.Println("Warning: COLLECTOR_API_TOKEN not set — collector will allow unauthenticated traffic for protected routes")
	}

	// Initialize storage
	store, err := storage.NewSQLiteStore("./collector.db")
	if err != nil {
		log.Fatalf("failed to open sqlite store: %v", err)
	}

	col := collector.NewCollector(store)

	mux := http.NewServeMux()

	// Ingest handler — each inbound request gets a connection_id and events saved with it
	mux.HandleFunc("/ingest", func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		connectionID := uuid.New().String()
		// track active connection
		col.AddConnection(connectionID)
		defer col.RemoveConnection(connectionID)

		// read payload
		payload := ""
		if r.ContentLength > 0 {
			buf := make([]byte, r.ContentLength)
			_, _ = r.Body.Read(buf)
			payload = string(buf)
		}

		// save the event along with connection id
		if err := col.SaveEvent(ctx, payload, connectionID); err != nil {
			http.Error(w, "failed to save event", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Connection-Id", connectionID)
		w.Write([]byte(`{"status":"ok"}`))
	})

	// Simple API route to check active connections
	mux.HandleFunc("/api/active_connections", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		count := col.ActiveConnectionsCount()
		w.Write([]byte(`{"active_connections":` + string(intToBytes(count)) + `}`))
	})

	// Wrap mux with scoped token middleware — only protect ingest and /api routes
	protectedPrefixes := []string{"/ingest", "/api"}
	handler := ScopedTokenMiddleware(token, protectedPrefixes)(mux)

	addr := ":8080"
	log.Printf("collector listening on %s", addr)
	if err := http.ListenAndServe(addr, handler); err != nil {
		log.Fatalf("server error: %v", err)
	}
}

// ScopedTokenMiddleware returns middleware that enforces a bearer token only
// for requests whose path matches one of the provided prefixes. If token is
// empty the middleware allows requests (useful for local/dev).
func ScopedTokenMiddleware(token string, protectedPrefixes []string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// If token not configured, allow everything
			if token == "" {
				next.ServeHTTP(w, r)
				return
			}

			// Check if path needs protection
			for _, p := range protectedPrefixes {
				if strings.HasPrefix(r.URL.Path, p) {
					auth := r.Header.Get("Authorization")
					if !strings.HasPrefix(auth, "Bearer ") {
						http.Error(w, "unauthorized", http.StatusUnauthorized)
						return
					}
					provided := strings.TrimPrefix(auth, "Bearer ")
					if provided != token {
						http.Error(w, "forbidden", http.StatusForbidden)
						return
					}
					break
				}
			}

			next.ServeHTTP(w, r)
		})
	}
}

// intToBytes converts a small int to its ASCII bytes (very small helper to avoid imports)
func intToBytes(i int) []byte {
	// supports only non-negative small ints
	if i == 0 {
		return []byte("0")
	}
	buf := make([]byte, 0, 4)
	for i > 0 {
		d := i % 10
		buf = append([]byte{byte('0' + d)}, buf...)
		i = i / 10
	}
	return buf
}
