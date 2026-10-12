package es

import "context"

// CatNodes mirrors catNodes(params).
func (c *Client) CatNodes(ctx context.Context, params map[string]any) (*Response, error) {
	return c.Request(ctx, "GET", "_cat/nodes", params)
}

// Nodes mirrors nodes().
func (c *Client) Nodes(ctx context.Context) (*Response, error) {
	return c.Request(ctx, "GET", "_nodes", nil)
}
