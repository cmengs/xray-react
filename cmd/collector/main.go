package main

import (
	"context"
	"encoding/json"
	_ "fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	_ "time"

	"github.com/cmengs/xray-react/collector/internal/storage"
)

func main() {
	ctx := context.Background()
	dbPath := os.Getenv("COLLECTOR_DB_PATH")
	if dbPath == "" {
		dbPath = "./data/collector.db"
	}

	store, err := storage.NewSQLiteStore(dbPath)
	if err != nil {
		log.Fatalf("failed to open store: %v", err)
	}
	defer store.Close()

	http.HandleFunc("/events", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "POST":
			handlePost(ctx, store, w, r)
		case "GET":
			handleList(ctx, store, w, r)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})

	addr := ":8080"
	log.Printf("collector listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, nil))
}

func handlePost(ctx context.Context, s *storage.SQLiteStore, w http.ResponseWriter, r *http.Request) {
	var req struct {
		Type    string `json:"type"`
		Payload string `json:"payload"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json: "+err.Error(), http.StatusBadRequest)
		return
	}
	if req.Type == "" {
		req.Type = "default"
	}
	id, err := s.InsertEvent(ctx, req.Type, req.Payload)
	if err != nil {
		http.Error(w, "insert failed: "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{"id": id})
}

func handleList(ctx context.Context, s *storage.SQLiteStore, w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	typ := q.Get("type")
	pageStr := q.Get("page")
	perPageStr := q.Get("per_page")

	page := 1
	perPage := 25
	if pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
			page = p
		}
	}
	if perPageStr != "" {
		if pp, err := strconv.Atoi(perPageStr); err == nil && pp > 0 {
			perPage = pp
		}
	}
	offset := (page - 1) * perPage

	list, err := s.ListEvents(ctx, typ, perPage, offset)
	if err != nil {
		http.Error(w, "list failed: "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"page":     page,
		"per_page": perPage,
		"items":    list,
	})
}
