package store

import (
	"context"
	"errors"
	"testing"
	"time"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	s, err := Open("file:" + t.TempDir() + "/test.db")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestInsertAndGetOrder(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	want := Order{ID: "ord_1", MerchantID: "acme-superstore", AmountCents: 1999, Status: "created", CreatedAt: time.Now().UTC()}
	if err := s.InsertOrder(ctx, want); err != nil {
		t.Fatalf("insert: %v", err)
	}

	got, err := s.GetOrder(ctx, "ord_1")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.ID != want.ID || got.MerchantID != want.MerchantID || got.AmountCents != want.AmountCents {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestGetOrderNotFound(t *testing.T) {
	_, err := newTestStore(t).GetOrder(context.Background(), "nope")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("got %v, want ErrNotFound", err)
	}
}

func TestSetStatus(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	s.InsertOrder(ctx, Order{ID: "ord_2", MerchantID: "m", AmountCents: 1, Status: "created", CreatedAt: time.Now()})

	if err := s.SetStatus(ctx, "ord_2", "cancelled"); err != nil {
		t.Fatalf("set status: %v", err)
	}
	got, _ := s.GetOrder(ctx, "ord_2")
	if got.Status != "cancelled" {
		t.Fatalf("status = %q, want cancelled", got.Status)
	}
}

func TestWithTxRollsBackOnError(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	wantErr := errors.New("boom")
	err := s.WithTx(ctx, func(tx *Store) error {
		if err := tx.InsertOrder(ctx, Order{ID: "ord_3", MerchantID: "m", AmountCents: 1, Status: "created", CreatedAt: time.Now()}); err != nil {
			return err
		}
		return wantErr
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("got %v, want %v", err, wantErr)
	}

	if _, err := s.GetOrder(ctx, "ord_3"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("order survived a rolled-back transaction")
	}
}

func TestWithTxCommits(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	err := s.WithTx(ctx, func(tx *Store) error {
		return tx.InsertOrder(ctx, Order{ID: "ord_4", MerchantID: "m", AmountCents: 1, Status: "created", CreatedAt: time.Now()})
	})
	if err != nil {
		t.Fatalf("with tx: %v", err)
	}
	if _, err := s.GetOrder(ctx, "ord_4"); err != nil {
		t.Fatalf("committed order missing: %v", err)
	}
}
