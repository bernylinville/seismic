package es

import (
	"context"
	"strings"
)

// cleanJoin mirrors the adapter's cleanIndexName(indices.join(',')) used by
// every multi-index route.
func cleanJoin(indices []string) string {
	return CleanIndexName(strings.Join(indices, ","))
}

// chunked mirrors callInChunks: split indices in batches of 16 and run the
// call serially per batch, collecting the responses in order.
func (c *Client) chunked(ctx context.Context, indices []string,
	call func(ctx context.Context, batch []string) (*Response, error)) ([]*Response, error) {

	responses := make([]*Response, 0, len(ChunkIndices(indices)))
	for _, batch := range ChunkIndices(indices) {
		resp, err := call(ctx, batch)
		if err != nil {
			return nil, err
		}
		responses = append(responses, resp)
	}
	return responses, nil
}

// CatIndices mirrors catIndices(params, filter).
func (c *Client) CatIndices(ctx context.Context, params map[string]any, filter string) (*Response, error) {
	query := ""
	if filter != "" {
		query = filter + "*"
	}
	return c.Request(ctx, "GET", "_cat/indices/"+query, params)
}

// Template mirrors template().
func (c *Client) Template(ctx context.Context) (*Response, error) {
	return c.Request(ctx, "GET", "_template", nil)
}

// IndexTemplate mirrors indexTemplate().
func (c *Client) IndexTemplate(ctx context.Context) (*Response, error) {
	return c.Request(ctx, "GET", "_index_template", nil)
}

// IndexGetAlias mirrors indexGetAlias({index}).
func (c *Client) IndexGetAlias(ctx context.Context, index string) (*Response, error) {
	return c.Request(ctx, "GET", CleanIndexName(index)+"/_alias", nil)
}

// IndexAddAlias mirrors indexAddAlias({index, alias}).
func (c *Client) IndexAddAlias(ctx context.Context, index, alias string) (*Response, error) {
	return c.Request(ctx, "PUT", CleanIndexName(index)+"/_alias/"+alias, nil)
}

// IndexDeleteAlias mirrors indexDeleteAlias({index, alias}).
func (c *Client) IndexDeleteAlias(ctx context.Context, index, alias string) (*Response, error) {
	return c.Request(ctx, "DELETE", CleanIndexName(index)+"/_alias/"+alias, nil)
}

// IndexCreate mirrors indexCreate({index, body}); body may be nil.
func (c *Client) IndexCreate(ctx context.Context, index string, body map[string]any) (*Response, error) {
	return c.Request(ctx, "PUT", CleanIndexName(index), body)
}

// DeleteByQuery mirrors deleteByQuery({index}): match_all with refresh.
func (c *Client) DeleteByQuery(ctx context.Context, index string) (*Response, error) {
	body := map[string]any{"query": map[string]any{"match_all": map[string]any{}}}
	return c.Request(ctx, "POST", CleanIndexName(index)+"/_delete_by_query?refresh=true", body)
}

// IndexGet mirrors indexGet(params): index or index list joined with ','.
func (c *Client) IndexGet(ctx context.Context, index []string) (*Response, error) {
	return c.Request(ctx, "GET", cleanJoin(index), nil)
}

// IndexStats mirrors indexStats({index}).
func (c *Client) IndexStats(ctx context.Context, index string) (*Response, error) {
	return c.Request(ctx, "GET", CleanIndexName(index)+"/_stats", nil)
}

// IndexDelete mirrors indexDelete({indices}), chunked past 16 indices.
func (c *Client) IndexDelete(ctx context.Context, indices []string) ([]*Response, error) {
	return c.chunked(ctx, indices, func(ctx context.Context, batch []string) (*Response, error) {
		return c.Request(ctx, "DELETE", cleanJoin(batch), nil)
	})
}

// IndexRefresh mirrors indexRefresh({indices}), chunked past 16 indices.
func (c *Client) IndexRefresh(ctx context.Context, indices []string) ([]*Response, error) {
	return c.chunked(ctx, indices, func(ctx context.Context, batch []string) (*Response, error) {
		return c.Request(ctx, "POST", cleanJoin(batch)+"/_refresh", nil)
	})
}

// IndexClearCache mirrors indexClearCache({indices}), chunked past 16 indices.
func (c *Client) IndexClearCache(ctx context.Context, indices []string) ([]*Response, error) {
	return c.chunked(ctx, indices, func(ctx context.Context, batch []string) (*Response, error) {
		return c.Request(ctx, "POST", cleanJoin(batch)+"/_cache/clear", nil)
	})
}

// IndexFlush mirrors indexFlush({indices}), chunked past 16 indices.
func (c *Client) IndexFlush(ctx context.Context, indices []string) ([]*Response, error) {
	return c.chunked(ctx, indices, func(ctx context.Context, batch []string) (*Response, error) {
		return c.Request(ctx, "POST", cleanJoin(batch)+"/_flush", nil)
	})
}

// IndexExists mirrors indexExists({index}) (HEAD).
func (c *Client) IndexExists(ctx context.Context, index string) (*Response, error) {
	return c.Request(ctx, "HEAD", CleanIndexName(index), nil)
}

// IndexGetSettings mirrors indexGetSettings({index}).
func (c *Client) IndexGetSettings(ctx context.Context, index string) (*Response, error) {
	return c.Request(ctx, "GET", CleanIndexName(index)+"/_settings", nil)
}

// IndexPutSettings mirrors indexPutSettings({indices, body}), chunked past 16.
func (c *Client) IndexPutSettings(ctx context.Context, indices []string, body map[string]any) ([]*Response, error) {
	return c.chunked(ctx, indices, func(ctx context.Context, batch []string) (*Response, error) {
		return c.Request(ctx, "PUT", cleanJoin(batch)+"/_settings", body)
	})
}

// Reindex mirrors reindex({source, dest}): async (wait_for_completion=false).
func (c *Client) Reindex(ctx context.Context, source, dest string) (*Response, error) {
	body := map[string]any{
		"source": map[string]any{"index": source},
		"dest":   map[string]any{"index": dest},
	}
	return c.Request(ctx, "POST", "_reindex?wait_for_completion=false", body)
}

// Clone mirrors clone({source, dest}).
func (c *Client) Clone(ctx context.Context, source, dest string) (*Response, error) {
	return c.Request(ctx, "POST", CleanIndexName(source)+"/_clone/"+CleanIndexName(dest), nil)
}

// IndexClose mirrors indexClose({indices}), chunked past 16 indices.
func (c *Client) IndexClose(ctx context.Context, indices []string) ([]*Response, error) {
	return c.chunked(ctx, indices, func(ctx context.Context, batch []string) (*Response, error) {
		return c.Request(ctx, "POST", cleanJoin(batch)+"/_close", nil)
	})
}

// IndexOpen mirrors indexOpen({indices}), chunked past 16 indices.
func (c *Client) IndexOpen(ctx context.Context, indices []string) ([]*Response, error) {
	return c.chunked(ctx, indices, func(ctx context.Context, batch []string) (*Response, error) {
		return c.Request(ctx, "POST", cleanJoin(batch)+"/_open", nil)
	})
}

// IndexForcemerge mirrors indexForcemerge({indices}), chunked past 16 indices.
func (c *Client) IndexForcemerge(ctx context.Context, indices []string) ([]*Response, error) {
	return c.chunked(ctx, indices, func(ctx context.Context, batch []string) (*Response, error) {
		return c.Request(ctx, "POST", cleanJoin(batch)+"/_forcemerge", nil)
	})
}

// CatIndicesInto streams the cat indices reply straight into out.
func (c *Client) CatIndicesInto(ctx context.Context, params map[string]any, filter string, out any) (*Response, error) {
	query := ""
	if filter != "" {
		query = filter + "*"
	}
	return c.RequestJSON(ctx, "GET", "_cat/indices/"+query, params, out)
}

// TemplateInto streams the legacy _template reply straight into out.
func (c *Client) TemplateInto(ctx context.Context, out any) (*Response, error) {
	return c.RequestJSON(ctx, "GET", "_template", nil, out)
}

// IndexTemplateInto streams the _index_template reply straight into out.
func (c *Client) IndexTemplateInto(ctx context.Context, out any) (*Response, error) {
	return c.RequestJSON(ctx, "GET", "_index_template", nil, out)
}

// ClusterGetSettingsInto streams the cluster settings reply straight into out.
func (c *Client) ClusterGetSettingsInto(ctx context.Context, out any) (*Response, error) {
	return c.RequestJSON(ctx, "GET", "_cluster/settings", nil, out)
}
