// Package app wires the service together.
package app

import (
	"context"
	"net/http"

	"orders-api/internal/api"
	"orders-api/internal/broker"
	"orders-api/internal/fulfilment"
	"orders-api/internal/store"
)

// App is the assembled service.
type App struct {
	Store      *store.Store
	Broker     *broker.Broker
	Fulfilment *fulfilment.Consumer
	Handler    http.Handler
}

// New builds the service against the given database DSN.
func New(ctx context.Context, dsn string) (*App, error) {
	s, err := store.Open(dsn)
	if err != nil {
		return nil, err
	}

	b := broker.New()
	f := fulfilment.New()
	b.Start(ctx, f.Handle)

	return &App{
		Store:      s,
		Broker:     b,
		Fulfilment: f,
		Handler:    api.New(s, b),
	}, nil
}

// Close shuts the service down.
func (a *App) Close() {
	a.Broker.Stop()
	a.Store.Close()
}
