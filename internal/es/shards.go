package es

import "context"

// CatShards mirrors catShards(params, filter).
func (c *Client) CatShards(ctx context.Context, params map[string]any, filter string) (*Response, error) {
	query := ""
	if filter != "" {
		query = filter + "*"
	}
	return c.Request(ctx, "GET", "_cat/shards/"+query, params)
}

// CatShardsInto streams the cat shards reply straight into out.
func (c *Client) CatShardsInto(ctx context.Context, params map[string]any, filter string, out any) (*Response, error) {
	query := ""
	if filter != "" {
		query = filter + "*"
	}
	return c.RequestJSON(ctx, "GET", "_cat/shards/"+query, params, out)
}
