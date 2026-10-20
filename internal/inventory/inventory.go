// Package inventory owns stock data and exposes read-only availability queries.
package inventory

import (
	"encoding/json"
	"net/http"
	"strconv"
)

type Availability struct {
	ItemID    string `json:"item_id"`
	Quantity  int    `json:"quantity"`
	Available bool   `json:"available"`
}

// NewHandler copies the supplied stock so callers cannot mutate service data.
// Stock stays unchanged: checking availability does not reserve inventory.
func NewHandler(stock map[string]int) http.Handler {
	items := make(map[string]int, len(stock))
	for id, quantity := range stock {
		items[id] = quantity
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /inventory/availability", func(w http.ResponseWriter, r *http.Request) {
		id := r.URL.Query().Get("item_id")
		quantity, err := strconv.Atoi(r.URL.Query().Get("quantity"))
		w.Header().Set("Content-Type", "application/json")
		if id == "" || err != nil || quantity <= 0 {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "item_id and a positive integer quantity are required"})
			return
		}
		json.NewEncoder(w).Encode(Availability{ItemID: id, Quantity: quantity, Available: items[id] >= quantity})
	})
	return mux
}
