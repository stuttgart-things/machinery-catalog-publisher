// Package metrics exposes the publisher's health as Prometheus text and a
// /healthz probe — the deployment-mode replacement for a CR status subresource.
package metrics

import (
	"fmt"
	"net/http"
	"sync/atomic"
)

// Stats holds the counters surfaced at /metrics and /healthz. Counters are
// updated from the single publisher goroutine and read from HTTP handlers, so
// every field is atomic.
type Stats struct {
	lastSyncUnix     atomic.Int64
	objectsPublished atomic.Int64
	syncErrors       atomic.Int64
	healthy          atomic.Bool
}

// Published records n successful object writes.
func (s *Stats) Published(n int) { s.objectsPublished.Add(int64(n)) }

// Errored records a single sync error.
func (s *Stats) Errored() { s.syncErrors.Add(1) }

// Synced records the unix timestamp of a completed resync.
func (s *Stats) Synced(unix int64) { s.lastSyncUnix.Store(unix) }

// SetHealthy flips the /healthz state (false once a resync fails).
func (s *Stats) SetHealthy(v bool) { s.healthy.Store(v) }

// Handler serves /metrics (Prometheus text) and /healthz.
func (s *Stats) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		if s.healthy.Load() {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("ok\n"))
			return
		}
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte("degraded\n"))
	})
	mux.HandleFunc("GET /metrics", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4")
		fmt.Fprintf(w, "# HELP publisher_last_sync_timestamp_seconds Unix time of the last successful resync.\n")
		fmt.Fprintf(w, "# TYPE publisher_last_sync_timestamp_seconds gauge\n")
		fmt.Fprintf(w, "publisher_last_sync_timestamp_seconds %d\n", s.lastSyncUnix.Load())
		fmt.Fprintf(w, "# HELP publisher_objects_published_total Objects written to the sink.\n")
		fmt.Fprintf(w, "# TYPE publisher_objects_published_total counter\n")
		fmt.Fprintf(w, "publisher_objects_published_total %d\n", s.objectsPublished.Load())
		fmt.Fprintf(w, "# HELP publisher_sync_errors_total Sink/source errors observed.\n")
		fmt.Fprintf(w, "# TYPE publisher_sync_errors_total counter\n")
		fmt.Fprintf(w, "publisher_sync_errors_total %d\n", s.syncErrors.Load())
	})
	return mux
}
