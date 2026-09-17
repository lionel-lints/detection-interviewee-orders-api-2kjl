package main

import (
	"context"
	"log"
	"net/http"
	"os"

	"orders-api/internal/app"
)

func main() {
	dsn := os.Getenv("ORDERS_DSN")
	if dsn == "" {
		dsn = "file:orders.db?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)"
	}

	ctx := context.Background()
	a, err := app.New(ctx, dsn)
	if err != nil {
		log.Fatalf("start: %v", err)
	}
	defer a.Close()

	addr := ":8080"
	log.Printf("orders-api listening on %s", addr)
	if err := http.ListenAndServe(addr, a.Handler); err != nil {
		log.Fatal(err)
	}
}
