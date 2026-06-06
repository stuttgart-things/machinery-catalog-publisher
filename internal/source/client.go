// Package source is a thin gRPC client for machinery's ResourceService.
// It owns no business logic — it hands raw ResourceEvents to the publisher.
package source

import (
	"context"
	"fmt"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	rs "github.com/stuttgart-things/maschinist/resourceservice"
)

// Client wraps a ResourceServiceClient and the kind selector to query.
type Client struct {
	conn *grpc.ClientConn
	svc  rs.ResourceServiceClient
	kind string // "*" for all, or comma-separated kinds
}

// Dial connects to machinery. The link is in-cluster and plaintext (h2c);
// TLS, if ever wanted, terminates at the Gateway, not here.
func Dial(addr string, kinds []string) (*Client, error) {
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("dial machinery %s: %w", addr, err)
	}
	return &Client{conn: conn, svc: rs.NewResourceServiceClient(conn), kind: joinKinds(kinds)}, nil
}

// Watch replays the cache as ADDED then streams live deltas. The caller loops
// on Recv; on stream error the publisher reconnects with backoff.
func (c *Client) Watch(ctx context.Context) (grpc.ServerStreamingClient[rs.ResourceEvent], error) {
	return c.svc.WatchResources(ctx, &rs.ResourceRequest{Count: -1, Kind: c.kind})
}

// Snapshot is the periodic full resync — heals any delta missed during a
// stream drop without waiting for the next MODIFIED.
func (c *Client) Snapshot(ctx context.Context) ([]*rs.ResourceStatus, error) {
	resp, err := c.svc.GetResources(ctx, &rs.ResourceRequest{Count: -1, Kind: c.kind})
	if err != nil {
		return nil, err
	}
	return resp.GetResources(), nil
}

// Close releases the underlying connection.
func (c *Client) Close() error { return c.conn.Close() }

func joinKinds(kinds []string) string {
	if len(kinds) == 0 {
		return "*"
	}
	out := kinds[0]
	for _, k := range kinds[1:] {
		out += "," + k
	}
	return out
}
