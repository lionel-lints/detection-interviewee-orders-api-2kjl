// Command loadgen replays production-shaped traffic against the service and
// reports what came out the other end.
//
// It runs everything in process against a throwaway database, so it is safe to
// run as often as you like.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"orders-api/internal/app"
)

const (
	workers  = 32
	duration = 15 * time.Second

	// targetRate compresses Monday-peak traffic into the run. Production peak is
	// roughly 40 orders/sec; we replay at 600 so a 15 second run says something
	// about a system that has to hold up for hours.
	targetRate = 600
)

type profile struct {
	Merchants []struct {
		ID     string `json:"id"`
		Weight int    `json:"weight"`
	} `json:"merchants"`
	CancelRate float64 `json:"cancel_rate"`
}

type counters struct {
	created201 atomic.Int64
	created5xx atomic.Int64
	cancel204  atomic.Int64
	cancel5xx  atomic.Int64
}

func main() {
	prof := loadProfile("testdata/traffic.json")

	dir, err := os.MkdirTemp("", "orders-load")
	if err != nil {
		log.Fatal(err)
	}
	defer os.RemoveAll(dir)
	dsn := "file:" + filepath.Join(dir, "load.db") + "?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)"

	ctx := context.Background()
	a, err := app.New(ctx, dsn)
	if err != nil {
		log.Fatal(err)
	}

	srv := httptest.NewServer(a.Handler)
	defer srv.Close()

	client := &http.Client{
		Timeout:   5 * time.Second,
		Transport: &http.Transport{MaxIdleConnsPerHost: workers * 2},
	}

	var c counters
	deadline := time.Now().Add(duration)

	fmt.Printf("replaying %s of peak traffic at %d orders/sec across %d merchants...\n\n",
		duration, targetRate, len(prof.Merchants))

	// Pace the run so it reflects production load rather than how fast this
	// laptop can saturate a socket.
	tokens := make(chan struct{}, targetRate)
	go func() {
		tick := time.NewTicker(time.Second / targetRate)
		defer tick.Stop()
		for time.Now().Before(deadline) {
			<-tick.C
			select {
			case tokens <- struct{}{}:
			default:
			}
		}
		close(tokens)
	}()

	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(seed int64) {
			defer wg.Done()
			rng := rand.New(rand.NewSource(seed))
			recent := make([]string, 0, 32)

			for range tokens {
				if time.Now().After(deadline) {
					return
				}
				merchant := pick(rng, prof)
				id, ok := postOrder(client, srv.URL, merchant, rng, &c)
				if ok {
					recent = append(recent, id)
					if len(recent) > 32 {
						recent = recent[1:]
					}
				}
				if len(recent) > 0 && rng.Float64() < prof.CancelRate {
					i := rng.Intn(len(recent))
					cancelOrder(client, srv.URL, recent[i], &c)
					recent = append(recent[:i], recent[i+1:]...)
				}
			}
		}(int64(i) + 1)
	}
	wg.Wait()

	stats := a.Broker.Stats()

	// Let the log drain before reconciling, so a backlog is not mistaken for a loss.
	a.Broker.Stop()

	rows, err := a.Store.CountOrders(ctx)
	if err != nil {
		log.Fatal(err)
	}
	counts := a.Fulfilment.Counts()
	a.Store.Close()

	fmt.Println("event log")
	fmt.Printf("  %-10s %12s %8s %12s\n", "partition", "processed", "backlog", "max lag")
	for _, s := range stats {
		fmt.Printf("  %-10d %12d %8d %12s\n", s.Partition, s.Processed, s.Depth, s.MaxLag.Round(time.Millisecond))
	}

	fmt.Println("\nrequests")
	fmt.Printf("  POST /orders          201: %-10d 500: %d\n", c.created201.Load(), c.created5xx.Load())
	fmt.Printf("  POST /orders/{id}/cancel  204: %-6d 500: %d\n", c.cancel204.Load(), c.cancel5xx.Load())

	fmt.Println("\ndatabase")
	fmt.Printf("  orders stored:        %-10d (%+d vs 201 responses)\n", rows, rows-int(c.created201.Load()))

	fmt.Println("\nfulfilment")
	cancelled := counts["order.cancelled"]
	fmt.Printf("  order.cancelled:      %-10d (%+d vs 204 responses)\n", cancelled, cancelled-int(c.cancel204.Load()))
	for typ, n := range counts {
		if typ != "order.cancelled" {
			fmt.Printf("  %-21s %d\n", typ+":", n)
		}
	}
	fmt.Printf("  total applied:        %d\n", a.Fulfilment.Processed())
}

func postOrder(client *http.Client, base, merchant string, rng *rand.Rand, c *counters) (string, bool) {
	body, _ := json.Marshal(map[string]any{
		"merchant_id":  merchant,
		"amount_cents": 500 + rng.Int63n(50000),
	})
	resp, err := client.Post(base+"/orders", "application/json", bytes.NewReader(body))
	if err != nil {
		return "", false
	}
	defer resp.Body.Close()

	switch {
	case resp.StatusCode == http.StatusCreated:
		c.created201.Add(1)
		var o struct {
			ID string `json:"id"`
		}
		json.NewDecoder(resp.Body).Decode(&o)
		return o.ID, true
	case resp.StatusCode >= 500:
		c.created5xx.Add(1)
	}
	return "", false
}

func cancelOrder(client *http.Client, base, id string, c *counters) {
	req, _ := http.NewRequest(http.MethodPost, base+"/orders/"+id+"/cancel", nil)
	resp, err := client.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	switch {
	case resp.StatusCode == http.StatusNoContent:
		c.cancel204.Add(1)
	case resp.StatusCode >= 500:
		c.cancel5xx.Add(1)
	}
}

func pick(rng *rand.Rand, p profile) string {
	total := 0
	for _, m := range p.Merchants {
		total += m.Weight
	}
	n := rng.Intn(total)
	for _, m := range p.Merchants {
		n -= m.Weight
		if n < 0 {
			return m.ID
		}
	}
	return p.Merchants[0].ID
}

func loadProfile(path string) profile {
	f, err := os.ReadFile(path)
	if err != nil {
		log.Fatalf("read %s: %v (run make load from the repo root)", path, err)
	}
	var p profile
	if err := json.Unmarshal(f, &p); err != nil {
		log.Fatal(err)
	}
	return p
}
