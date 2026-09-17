// Package store is the persistence layer. SQLite locally, Postgres in production.
package store

import (
	"context"
	"database/sql"
	"errors"
	"time"

	_ "modernc.org/sqlite"
)

// ErrNotFound is returned when an order does not exist.
var ErrNotFound = errors.New("order not found")

// Order is a merchant order.
type Order struct {
	ID          string    `json:"id"`
	MerchantID  string    `json:"merchant_id"`
	AmountCents int64     `json:"amount_cents"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"created_at"`
}

const schema = `
CREATE TABLE IF NOT EXISTS orders (
	id           TEXT PRIMARY KEY,
	merchant_id  TEXT NOT NULL,
	amount_cents INTEGER NOT NULL,
	status       TEXT NOT NULL,
	created_at   TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS orders_by_merchant ON orders(merchant_id);
`

// Store talks to the database. A Store returned by WithTx is scoped to that
// transaction; otherwise it uses the connection pool.
type Store struct {
	db *sql.DB
	tx *sql.Tx
}

// Open connects to the database and applies the schema.
func Open(dsn string) (*Store, error) {
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, err
	}
	return &Store{db: db}, nil
}

// DB exposes the underlying pool for callers that need it.
func (s *Store) DB() *sql.DB { return s.db }

// Close releases the connection pool.
func (s *Store) Close() error { return s.db.Close() }

// WithTx runs fn inside a transaction, rolling back if it returns an error or
// panics. The Store handed to fn is scoped to the transaction.
func (s *Store) WithTx(ctx context.Context, fn func(*Store) error) (err error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback()
			panic(p)
		}
		if err != nil {
			_ = tx.Rollback()
			return
		}
		tx.Commit()
	}()
	return fn(&Store{db: s.db, tx: tx})
}

func (s *Store) exec(ctx context.Context, q string, args ...any) (sql.Result, error) {
	if s.tx != nil {
		return s.tx.ExecContext(ctx, q, args...)
	}
	return s.db.ExecContext(ctx, q, args...)
}

func (s *Store) queryRow(ctx context.Context, q string, args ...any) *sql.Row {
	if s.tx != nil {
		return s.tx.QueryRowContext(ctx, q, args...)
	}
	return s.db.QueryRowContext(ctx, q, args...)
}

// InsertOrder writes a new order.
func (s *Store) InsertOrder(ctx context.Context, o Order) error {
	_, err := s.exec(ctx,
		`INSERT INTO orders (id, merchant_id, amount_cents, status, created_at) VALUES (?, ?, ?, ?, ?)`,
		o.ID, o.MerchantID, o.AmountCents, o.Status, o.CreatedAt.Format(time.RFC3339Nano))
	return err
}

// GetOrder fetches an order by id.
func (s *Store) GetOrder(ctx context.Context, id string) (Order, error) {
	var o Order
	var created string
	err := s.queryRow(ctx,
		`SELECT id, merchant_id, amount_cents, status, created_at FROM orders WHERE id = ?`, id).
		Scan(&o.ID, &o.MerchantID, &o.AmountCents, &o.Status, &created)
	if errors.Is(err, sql.ErrNoRows) {
		return Order{}, ErrNotFound
	}
	if err != nil {
		return Order{}, err
	}
	o.CreatedAt, _ = time.Parse(time.RFC3339Nano, created)
	return o, nil
}

// SetStatus updates an order's status.
func (s *Store) SetStatus(ctx context.Context, id, status string) error {
	res, err := s.exec(ctx, `UPDATE orders SET status = ? WHERE id = ?`, status, id)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// CountOrders returns the number of orders stored.
func (s *Store) CountOrders(ctx context.Context) (int, error) {
	var n int
	err := s.queryRow(ctx, `SELECT COUNT(*) FROM orders`).Scan(&n)
	return n, err
}
