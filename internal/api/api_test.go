package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"orders-api/internal/api"
	"orders-api/internal/broker"
	"orders-api/internal/store"
)

func newTestAPI(t *testing.T) http.Handler {
	t.Helper()
	s, err := store.Open("file:" + t.TempDir() + "/test.db")
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	b := broker.New()
	b.Start(context.Background(), func(context.Context, broker.Event) {})
	t.Cleanup(func() {
		b.Stop()
		s.Close()
	})
	return api.New(s, b)
}

func createOrder(t *testing.T, h http.Handler, merchant string, amount int64) *httptest.ResponseRecorder {
	t.Helper()
	body, _ := json.Marshal(map[string]any{"merchant_id": merchant, "amount_cents": amount})
	req := httptest.NewRequest(http.MethodPost, "/orders", bytes.NewReader(body))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	return w
}

func TestCreateOrder(t *testing.T) {
	h := newTestAPI(t)

	w := createOrder(t, h, "acme-superstore", 2500)
	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201", w.Code)
	}

	var o store.Order
	if err := json.Unmarshal(w.Body.Bytes(), &o); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if o.ID == "" {
		t.Fatal("no order id returned")
	}
	if o.Status != "created" {
		t.Fatalf("status = %q, want created", o.Status)
	}
}

func TestCreateOrderValidation(t *testing.T) {
	h := newTestAPI(t)

	if w := createOrder(t, h, "", 2500); w.Code != http.StatusBadRequest {
		t.Fatalf("missing merchant: status = %d, want 400", w.Code)
	}
	if w := createOrder(t, h, "acme-superstore", 0); w.Code != http.StatusBadRequest {
		t.Fatalf("zero amount: status = %d, want 400", w.Code)
	}
}

func TestGetOrder(t *testing.T) {
	h := newTestAPI(t)

	var created store.Order
	json.Unmarshal(createOrder(t, h, "northwind-goods", 100).Body.Bytes(), &created)

	req := httptest.NewRequest(http.MethodGet, "/orders/"+created.ID, nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	var got store.Order
	json.Unmarshal(w.Body.Bytes(), &got)
	if got.ID != created.ID {
		t.Fatalf("got %q, want %q", got.ID, created.ID)
	}
}

func TestGetOrderNotFound(t *testing.T) {
	h := newTestAPI(t)
	req := httptest.NewRequest(http.MethodGet, "/orders/ord_missing", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", w.Code)
	}
}

func TestCancelOrderNotFound(t *testing.T) {
	h := newTestAPI(t)
	req := httptest.NewRequest(http.MethodPost, "/orders/ord_missing/cancel", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", w.Code)
	}
}
