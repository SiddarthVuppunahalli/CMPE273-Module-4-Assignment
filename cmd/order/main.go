package main

import (
	"log"
	"net/http"
	"net/url"
	"os"
	"time"

	"example.com/online-store/internal/order"
)

func main() {
	addr := os.Getenv("ORDER_ADDR")
	if addr == "" {
		addr = "127.0.0.1:8080"
	}
	inventoryURL := os.Getenv("INVENTORY_URL")
	if inventoryURL == "" {
		inventoryURL = "http://127.0.0.1:8081"
	}
	parsed, err := url.Parse(inventoryURL)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.RawQuery != "" || parsed.Fragment != "" {
		log.Fatal("INVENTORY_URL must be an HTTP(S) base URL without a query or fragment")
	}
	server := &http.Server{
		Addr:              addr,
		Handler:           order.NewHandler(inventoryURL, 2*time.Second),
		ReadHeaderTimeout: 5 * time.Second,
	}
	log.Printf("Order Service listening on http://%s; Inventory URL: %s", addr, inventoryURL)
	log.Fatal(server.ListenAndServe())
}
