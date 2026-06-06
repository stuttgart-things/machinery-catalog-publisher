// Package publisher runs the reconcile loop: stream machinery deltas to the
// sink, with a periodic full resync as a safety net. The loop is single-
// goroutine, so the in-memory state map needs no locking.
package publisher

import (
	"context"
	"io"
	"log/slog"
	"time"

	rs "github.com/stuttgart-things/maschinist/resourceservice"
	"google.golang.org/grpc"

	"github.com/stuttgart-things/machinery-catalog-publisher/internal/entity"
	"github.com/stuttgart-things/machinery-catalog-publisher/internal/metrics"
)

// Layout constants.
const (
	LayoutPerResource = "PerResource"
	LayoutAggregate   = "Aggregate"
)

// Source streams and snapshots machinery resources.
type Source interface {
	Watch(context.Context) (grpc.ServerStreamingClient[rs.ResourceEvent], error)
	Snapshot(context.Context) ([]*rs.ResourceStatus, error)
}

// Sink persists rendered entities.
type Sink interface {
	Put(ctx context.Context, key string, body []byte) error
	Delete(ctx context.Context, key string) error
}

// Options configure rendering and cadence.
type Options struct {
	Owner           string
	EntityNamespace string
	KeyPrefix       string
	Layout          string
	Resync          time.Duration
	Now             func() string // RFC3339 clock, injectable for tests
}

// Publisher supervises one source→sink sync.
type Publisher struct {
	src   Source
	snk   Sink
	opt   Options
	stats *metrics.Stats
	state map[string]*rs.ResourceStatus // entity.Key → latest status
}

// New constructs a Publisher with an empty state map.
func New(src Source, snk Sink, opt Options, stats *metrics.Stats) *Publisher {
	if opt.Now == nil {
		opt.Now = func() string { return time.Now().UTC().Format(time.RFC3339) }
	}
	return &Publisher{src: src, snk: snk, opt: opt, stats: stats, state: map[string]*rs.ResourceStatus{}}
}

// Run drives the loop until ctx is cancelled. It does an initial resync (which
// also writes the placeholder objects), then consumes the watch stream with a
// background resync ticker that heals anything a dropped stream missed.
func (p *Publisher) Run(ctx context.Context) error {
	ticker := time.NewTicker(p.opt.Resync)
	defer ticker.Stop()

	p.resync(ctx) // initial fill + placeholders

	stream, err := p.src.Watch(ctx)
	if err != nil {
		slog.Warn("initial watch failed; relying on resync ticker", "err", err)
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			p.resync(ctx)
		default:
			if stream == nil {
				stream = p.reconnect(ctx)
				continue
			}
			ev, err := stream.Recv()
			if err != nil {
				if err == io.EOF || ctx.Err() != nil {
					return ctx.Err()
				}
				slog.Warn("watch stream ended; reconnecting", "err", err)
				stream = nil
				continue
			}
			p.apply(ctx, ev)
		}
	}
}

func (p *Publisher) reconnect(ctx context.Context) grpc.ServerStreamingClient[rs.ResourceEvent] {
	select {
	case <-ctx.Done():
		return nil
	case <-time.After(2 * time.Second): // simple backoff
	}
	s, err := p.src.Watch(ctx)
	if err != nil {
		slog.Error("reconnect failed", "err", err)
		p.stats.Errored()
		return nil
	}
	return s
}

// apply handles one watch event and updates the sink.
func (p *Publisher) apply(ctx context.Context, ev *rs.ResourceEvent) {
	r := ev.GetResource()
	if r == nil {
		return
	}
	key := entity.Key(r, p.opt.KeyPrefix)

	if ev.GetType() == rs.EventType_DELETED {
		delete(p.state, key)
		if p.opt.Layout == LayoutAggregate {
			p.publishAggregate(ctx)
			return
		}
		if err := p.snk.Delete(ctx, key); err != nil {
			slog.Error("sink delete", "key", key, "err", err)
			p.stats.Errored()
		}
		return
	}

	p.state[key] = r
	if p.opt.Layout == LayoutAggregate {
		p.publishAggregate(ctx)
		return
	}
	p.putOne(ctx, r, key)
}

// resync fetches the full snapshot, rewrites every object, and (PerResource)
// deletes objects whose resources have disappeared.
func (p *Publisher) resync(ctx context.Context) {
	all, err := p.src.Snapshot(ctx)
	if err != nil {
		slog.Error("resync snapshot", "err", err)
		p.stats.Errored()
		p.stats.SetHealthy(false)
		return
	}

	next := make(map[string]*rs.ResourceStatus, len(all))
	for _, r := range all {
		next[entity.Key(r, p.opt.KeyPrefix)] = r
	}

	if p.opt.Layout == LayoutAggregate {
		p.state = next
		p.publishAggregate(ctx)
	} else {
		for key, r := range next {
			p.putOne(ctx, r, key)
		}
		for key := range p.state { // delete vanished resources
			if _, ok := next[key]; !ok {
				if err := p.snk.Delete(ctx, key); err != nil {
					slog.Error("sink delete (resync)", "key", key, "err", err)
					p.stats.Errored()
				}
			}
		}
		p.state = next
	}

	p.stats.Synced(time.Now().Unix())
	p.stats.SetHealthy(true)
	slog.Info("resync complete", "objects", len(next), "layout", p.opt.Layout)
}

func (p *Publisher) putOne(ctx context.Context, r *rs.ResourceStatus, key string) {
	body, err := entity.Render(r, p.renderOpts())
	if err != nil {
		slog.Error("render", "key", key, "err", err)
		p.stats.Errored()
		return
	}
	if err := p.snk.Put(ctx, key, body); err != nil {
		slog.Error("sink put", "key", key, "err", err)
		p.stats.Errored()
		return
	}
	p.stats.Published(1)
}

func (p *Publisher) publishAggregate(ctx context.Context) {
	resources := make([]*rs.ResourceStatus, 0, len(p.state))
	for _, r := range p.state {
		resources = append(resources, r)
	}
	entity.Sort(resources) // deterministic object body
	body, err := entity.RenderAll(resources, p.renderOpts())
	if err != nil {
		slog.Error("render aggregate", "err", err)
		p.stats.Errored()
		return
	}
	key := entity.AggregateKey(p.opt.KeyPrefix)
	if err := p.snk.Put(ctx, key, body); err != nil {
		slog.Error("sink put aggregate", "key", key, "err", err)
		p.stats.Errored()
		return
	}
	p.stats.Published(1)
}

func (p *Publisher) renderOpts() entity.Options {
	return entity.Options{
		Owner:           p.opt.Owner,
		EntityNamespace: p.opt.EntityNamespace,
		LastUpdated:     p.opt.Now(),
	}
}
