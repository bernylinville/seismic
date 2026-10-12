package es

import (
	"context"
	"strings"
)

// joinComma is strings.Join for index lists.
func joinComma(parts []string) string { return strings.Join(parts, ",") }

// Ping mirrors ping(): GET the cluster root.
func (c *Client) Ping(ctx context.Context) (*Response, error) {
	return c.Request(ctx, "GET", "", nil)
}

// ClusterInfo mirrors clusterInfo(): same as ping.
func (c *Client) ClusterInfo(ctx context.Context) (*Response, error) {
	return c.Ping(ctx)
}

// ClusterHealth mirrors clusterHealth().
func (c *Client) ClusterHealth(ctx context.Context) (*Response, error) {
	return c.Request(ctx, "GET", "_cluster/health", nil)
}

// ClusterStats mirrors clusterStats().
func (c *Client) ClusterStats(ctx context.Context) (*Response, error) {
	return c.Request(ctx, "GET", "_cluster/stats", nil)
}

// ClusterReroute mirrors clusterReroute(commands).
func (c *Client) ClusterReroute(ctx context.Context, commands any) (*Response, error) {
	return c.Request(ctx, "POST", "_cluster/reroute", map[string]any{"commands": commands})
}

// ClusterGetSettings mirrors clusterGetSettings().
func (c *Client) ClusterGetSettings(ctx context.Context) (*Response, error) {
	return c.Request(ctx, "GET", "_cluster/settings", nil)
}

// ClusterPutSettings mirrors clusterPutSettings(body).
func (c *Client) ClusterPutSettings(ctx context.Context, body any) (*Response, error) {
	return c.Request(ctx, "PUT", "_cluster/settings", body)
}

// CatRecovery mirrors catRecovery(): newest recoveries first.
func (c *Client) CatRecovery(ctx context.Context) (*Response, error) {
	return c.Request(ctx, "GET", "_cat/recovery?s=start_time_millis:desc", nil)
}

// Recovery mirrors recovery().
func (c *Client) Recovery(ctx context.Context) (*Response, error) {
	return c.Request(ctx, "GET", "_recovery", nil)
}
