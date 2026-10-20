package main

import (
	"log"
	"net/http"
	"os"
	"time"

	"example.com/online-store/internal/inventory"
)

func main() {
	addr := os.Getenv("INVENTORY_ADDR")
	if addr == "" {
		addr = "127.0.0.1:8081"
	}
	server := &http.Server{
		Addr:              addr,
		Handler:           inventory.NewHandler(map[string]int{"item-1": 10, "item-2": 0, "item-3": 5}),
		ReadHeaderTimeout: 5 * time.Second,
	}
	log.Printf("Inventory Service listening on http://%s", addr)
	log.Fatal(server.ListenAndServe())
}
