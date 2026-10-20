package order

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"example.com/online-store/internal/inventory"
)

func submit(handler http.Handler, body string) *httptest.ResponseRecorder {
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/orders", strings.NewReader(body)))
	return response
}

func TestCreateAndTrackOrder(t *testing.T) {
	upstream := httptest.NewServer(inventory.NewHandler(map[string]int{"item-1": 10}))
	defer upstream.Close()
	handler := NewHandler(upstream.URL, time.Second)
	response := submit(handler, `{"item_id":"item-1","quantity":2}`)
	if response.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", response.Code, response.Body.String())
	}
	var created Order
	if err := json.Unmarshal(response.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if created.ItemID != "item-1" || created.Quantity != 2 || created.Status != "created" || response.Header().Get("Location") != "/orders/"+created.ID {
		t.Fatalf("unexpected order: %+v", created)
	}
	lookup := httptest.NewRecorder()
	handler.ServeHTTP(lookup, httptest.NewRequest(http.MethodGet, "/orders/"+created.ID, nil))
	if lookup.Code != http.StatusOK || lookup.Body.String() != response.Body.String() {
		t.Fatalf("lookup: %d %s", lookup.Code, lookup.Body.String())
	}
	unknown := httptest.NewRecorder()
	handler.ServeHTTP(unknown, httptest.NewRequest(http.MethodGet, "/orders/missing", nil))
	if unknown.Code != http.StatusNotFound {
		t.Fatalf("unknown order: %d", unknown.Code)
	}
}

func TestRejectedOrders(t *testing.T) {
	upstream := httptest.NewServer(inventory.NewHandler(map[string]int{"item-1": 10}))
	defer upstream.Close()
	handler := NewHandler(upstream.URL, time.Second)
	for _, tc := range []struct {
		body   string
		status int
	}{
		{`{"item_id":"item-1","quantity":11}`, 409},
		{`{"item_id":"unknown","quantity":1}`, 409},
		{`{"item_id":"item-1","quantity":0}`, 400},
		{`{"item_id":"item-1","quantity":-1}`, 400},
		{`{"item_id":" ","quantity":1}`, 400},
		{`{"item_id":"item-1","quantity":1.5}`, 400},
		{`{"item_id":"item-1","quantity":1,"extra":true}`, 400},
		{`{"item_id":"item-1","quantity":1} {}`, 400},
		{`null`, 400},
		{`not json`, 400},
	} {
		if got := submit(handler, tc.body); got.Code != tc.status {
			t.Errorf("body %s: status %d, want %d", tc.body, got.Code, tc.status)
		}
	}
	// Rejected requests must not create an order or consume an ID.
	got := submit(handler, `{"item_id":"item-1","quantity":1}`)
	var created Order
	json.Unmarshal(got.Body.Bytes(), &created)
	if got.Code != http.StatusCreated || created.ID != "order-1" {
		t.Fatalf("rejected requests changed order state: %s", got.Body.String())
	}
}

func TestDependencyFailures(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status int
		body   string
		delay  time.Duration
	}{
		{"server error", 500, `{}`, 0},
		{"invalid JSON", 200, `broken`, 0},
		{"missing availability", 200, `{"item_id":"item-1","quantity":1}`, 0},
		{"mismatched item", 200, `{"item_id":"other","quantity":1,"available":true}`, 0},
		{"multiple objects", 200, `{"item_id":"item-1","quantity":1,"available":true} {}`, 0},
		{"timeout", 200, `{"item_id":"item-1","quantity":1,"available":true}`, 100 * time.Millisecond},
	} {
		t.Run(tc.name, func(t *testing.T) {
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				time.Sleep(tc.delay)
				w.WriteHeader(tc.status)
				fmt.Fprint(w, tc.body)
			}))
			defer upstream.Close()
			timeout := time.Second
			if tc.delay > 0 {
				timeout = 25 * time.Millisecond
			}
			handler := NewHandler(upstream.URL, timeout)
			got := submit(handler, `{"item_id":"item-1","quantity":1}`)
			if got.Code != http.StatusServiceUnavailable {
				t.Fatalf("status %d, want 503: %s", got.Code, got.Body.String())
			}
		})
	}
}

func TestStoppedInventoryDoesNotStopOrderService(t *testing.T) {
	upstream := httptest.NewServer(inventory.NewHandler(map[string]int{"item-1": 10}))
	defer upstream.Close()
	handler := NewHandler(upstream.URL, time.Second)
	body := `{"item_id":"item-1","quantity":1}`
	if got := submit(handler, body); got.Code != http.StatusCreated {
		t.Fatalf("before failure: %d", got.Code)
	}
	upstream.Close()
	for i := 0; i < 2; i++ {
		if got := submit(handler, body); got.Code != http.StatusServiceUnavailable {
			t.Fatalf("during failure: %d", got.Code)
		}
	}
	lookup := httptest.NewRecorder()
	handler.ServeHTTP(lookup, httptest.NewRequest(http.MethodGet, "/orders/order-1", nil))
	if lookup.Code != http.StatusOK {
		t.Fatalf("stored order inaccessible after inventory failure: %d", lookup.Code)
	}
}
