package inventory

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAvailability(t *testing.T) {
	stock := map[string]int{"item-1": 10, "item-2": 0}
	handler := NewHandler(stock)
	stock["item-1"] = 0 // The handler owns a copy.
	for _, tc := range []struct {
		name, query string
		status      int
		available   bool
	}{
		{"available", "item_id=item-1&quantity=2", 200, true},
		{"exact stock", "item_id=item-1&quantity=10", 200, true},
		{"insufficient stock", "item_id=item-1&quantity=11", 200, false},
		{"out of stock", "item_id=item-2&quantity=1", 200, false},
		{"unknown item", "item_id=unknown&quantity=1", 200, false},
		{"missing item", "quantity=1", 400, false},
		{"missing quantity", "item_id=item-1", 400, false},
		{"invalid quantity", "item_id=item-1&quantity=no", 400, false},
		{"fractional quantity", "item_id=item-1&quantity=1.5", 400, false},
		{"zero quantity", "item_id=item-1&quantity=0", 400, false},
		{"negative quantity", "item_id=item-1&quantity=-1", 400, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/inventory/availability?"+tc.query, nil))
			if response.Code != tc.status {
				t.Fatalf("status = %d, want %d", response.Code, tc.status)
			}
			if tc.status == http.StatusOK {
				var got Availability
				if err := json.Unmarshal(response.Body.Bytes(), &got); err != nil || got.Available != tc.available {
					t.Fatalf("unexpected availability: %s, error: %v", response.Body.String(), err)
				}
			}
		})
	}
}
