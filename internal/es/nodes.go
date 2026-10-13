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

// CatNodesInto streams the cat nodes reply straight into out.
func (c *Client) CatNodesInto(ctx context.Context, params map[string]any, out any) (*Response, error) {
	return c.RequestJSON(ctx, "GET", "_cat/nodes", params, out)
}

// NodesInto streams the _nodes reply straight into out.
func (c *Client) NodesInto(ctx context.Context, out any) (*Response, error) {
	return c.RequestJSON(ctx, "GET", "_nodes", nil, out)
}
