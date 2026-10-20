// Package order owns orders and queries Inventory through HTTP.
package order

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Order struct {
	ID       string `json:"id"`
	ItemID   string `json:"item_id"`
	Quantity int    `json:"quantity"`
	Status   string `json:"status"`
}

type Service struct {
	inventoryURL string
	client       *http.Client
	mu           sync.RWMutex
	orders       map[string]Order
	nextID       uint64
}

func NewHandler(inventoryURL string, timeout time.Duration) http.Handler {
	s := &Service{
		inventoryURL: strings.TrimRight(inventoryURL, "/"),
		client:       &http.Client{Timeout: timeout},
		orders:       make(map[string]Order),
	}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /orders", s.create)
	mux.HandleFunc("GET /orders/{id}", s.get)
	return mux
}

func (s *Service) create(w http.ResponseWriter, r *http.Request) {
	var input struct {
		ItemID   string `json:"item_id"`
		Quantity int    `json:"quantity"`
	}
	r.Body = http.MaxBytesReader(w, r.Body, 4096)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "provide a JSON object with item_id and quantity"})
		return
	}
	if decoder.Decode(&struct{}{}) != io.EOF || strings.TrimSpace(input.ItemID) == "" || input.Quantity <= 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "provide one JSON object with a nonempty item_id and positive integer quantity"})
		return
	}

	query := url.Values{"item_id": {input.ItemID}, "quantity": {strconv.Itoa(input.Quantity)}}
	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, s.inventoryURL+"/inventory/availability?"+query.Encode(), nil)
	if err != nil {
		dependencyError(w)
		return
	}
	response, err := s.client.Do(req)
	if err != nil {
		dependencyError(w)
		return
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		dependencyError(w)
		return
	}
	// A pointer distinguishes an explicit false from a missing or null field.
	var availability struct {
		ItemID    string `json:"item_id"`
		Quantity  int    `json:"quantity"`
		Available *bool  `json:"available"`
	}
	result := json.NewDecoder(io.LimitReader(response.Body, 4096))
	if err := result.Decode(&availability); err != nil || availability.Available == nil || availability.ItemID != input.ItemID || availability.Quantity != input.Quantity {
		dependencyError(w)
		return
	}
	if result.Decode(&struct{}{}) != io.EOF {
		dependencyError(w)
		return
	}
	if !*availability.Available {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "requested item or quantity is unavailable"})
		return
	}

	s.mu.Lock()
	s.nextID++
	created := Order{ID: fmt.Sprintf("order-%d", s.nextID), ItemID: input.ItemID, Quantity: input.Quantity, Status: "created"}
	s.orders[created.ID] = created
	s.mu.Unlock()
	w.Header().Set("Location", "/orders/"+created.ID)
	writeJSON(w, http.StatusCreated, created)
}

func (s *Service) get(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	found, ok := s.orders[r.PathValue("id")]
	s.mu.RUnlock()
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "order not found"})
		return
	}
	writeJSON(w, http.StatusOK, found)
}

func dependencyError(w http.ResponseWriter) {
	writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "inventory service unavailable; please try again later"})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(value)
}
