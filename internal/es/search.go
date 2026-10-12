package es

import (
	"context"
	"net/url"
)

// Search mirrors search(params, searchIndex?, queryParams?): indices are
// joined with ','; empty index list searches all; queryParams become the
// query string.
func (c *Client) Search(ctx context.Context, params any, searchIndex []string, queryParams map[string]string) (*Response, error) {
	index := ""
	if len(searchIndex) == 1 {
		index = searchIndex[0]
	} else if len(searchIndex) > 1 {
		index = joinComma(searchIndex)
	}

	path := "_search"
	if index != "" {
		path = CleanIndexName(index) + "/_search"
	}

	if len(queryParams) > 0 {
		q := url.Values{}
		for k, v := range queryParams {
			q.Set(k, v)
		}
		path += "?" + q.Encode()
	}

	return c.Request(ctx, "POST", path, params)
}

// Test mirrors test(): ping the cluster, then prove search rights with a
// no-match pattern; returns the ping (cluster info) response.
func (c *Client) Test(ctx context.Context) (*Response, error) {
	info, err := c.Ping(ctx)
	if err != nil {
		return nil, err
	}
	_, err = c.Search(ctx, map[string]any{}, []string{"no_indices_match_this_pattern*"}, map[string]string{
		"allow_no_indices":   "true",
		"ignore_unavailable": "true",
		"size":               "0",
	})
	if err != nil {
		return nil, err
	}
	return info, nil
}
