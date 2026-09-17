// Package api serves the HTTP interface.
package api

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"orders-api/internal/broker"
	"orders-api/internal/store"
)

// requestBudget enforces invariant 3: POST /orders p99 under 150ms. A request
// that exceeds it is not worth holding a connection open for.
const requestBudget = 150 * time.Millisecond

// API holds the handler dependencies.
type API struct {
	store  *store.Store
	broker *broker.Broker
}

// New builds the HTTP handler.
func New(s *store.Store, b *broker.Broker) http.Handler {
	a := &API{store: s, broker: b}

	mux := http.NewServeMux()
	mux.HandleFunc("POST /orders", a.createOrder)
	mux.HandleFunc("GET /orders/{id}", a.getOrder)
	mux.HandleFunc("POST /orders/{id}/cancel", a.cancelOrder)

	return withBudget(requestBudget, mux)
}

func withBudget(d time.Duration, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), d)
		defer cancel()
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

type createRequest struct {
	MerchantID  string `json:"merchant_id"`
	AmountCents int64  `json:"amount_cents"`
}

func (a *API) createOrder(w http.ResponseWriter, r *http.Request) {
	var req createRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	if req.MerchantID == "" {
		http.Error(w, "merchant_id is required", http.StatusBadRequest)
		return
	}
	if req.AmountCents <= 0 {
		http.Error(w, "amount_cents must be positive", http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	o := store.Order{
		ID:          newID(),
		MerchantID:  req.MerchantID,
		AmountCents: req.AmountCents,
		Status:      "created",
		CreatedAt:   time.Now().UTC(),
	}

	err := a.store.WithTx(ctx, func(s *store.Store) error {
		return s.InsertOrder(ctx, o)
	})
	if err != nil {
		http.Error(w, "could not create order", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusCreated, o)
}

func (a *API) getOrder(w http.ResponseWriter, r *http.Request) {
	o, err := a.store.GetOrder(r.Context(), r.PathValue("id"))
	if errors.Is(err, store.ErrNotFound) {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, "could not fetch order", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, o)
}

func (a *API) cancelOrder(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := r.PathValue("id")

	var o store.Order
	err := a.store.WithTx(ctx, func(s *store.Store) error {
		var err error
		o, err = s.GetOrder(ctx, id)
		if err != nil {
			return err
		}
		if o.Status == "cancelled" {
			return nil
		}
		return s.SetStatus(ctx, id, "cancelled")
	})
	if errors.Is(err, store.ErrNotFound) {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, "could not cancel order", http.StatusInternalServerError)
		return
	}

	a.broker.Publish(ctx, o.MerchantID, broker.Event{
		Type:       "order.cancelled",
		OrderID:    o.ID,
		MerchantID: o.MerchantID,
	})

	w.WriteHeader(http.StatusNoContent)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func newID() string {
	var b [12]byte
	rand.Read(b[:])
	return "ord_" + hex.EncodeToString(b[:])
}
