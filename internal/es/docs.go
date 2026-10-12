package es

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// encodeURIComponent mirrors JS encodeURIComponent: A-Za-z0-9 and
// -_.!~*'() pass through, everything else becomes %XX per UTF-8 byte.
func encodeURIComponent(s string) string {
	var b strings.Builder
	for i := range len(s) {
		c := s[i]
		if c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' ||
			strings.IndexByte("-_.!~*'()", c) >= 0 {
			b.WriteByte(c)
		} else {
			fmt.Fprintf(&b, "%%%02X", c)
		}
	}
	return b.String()
}

// Index mirrors index({index, type, id, routing, params}): PUT a document
// with refresh=true (and routing when set).
func (c *Client) Index(ctx context.Context, index, typ, id, routing string, params any) (*Response, error) {
	path := CleanIndexName(index) + "/" + typ + "/" + encodeURIComponent(id) + "?refresh=true"
	if routing != "" {
		path += "&routing=" + routing
	}
	return c.Request(ctx, "PUT", path, params)
}

// Get mirrors get({index, type, id, routing}).
func (c *Client) Get(ctx context.Context, index, typ, id, routing string) (*Response, error) {
	params := map[string]any{}
	if routing != "" {
		params["routing"] = routing
	}
	return c.Request(ctx, "GET", CleanIndexName(index)+"/"+typ+"/"+encodeURIComponent(id), params)
}

// Delete mirrors delete({index, type, id, routing}).
func (c *Client) Delete(ctx context.Context, index, typ, id, routing string) (*Response, error) {
	path := CleanIndexName(index) + "/" + typ + "/" + encodeURIComponent(id) + "?refresh=true"
	if routing != "" {
		path += "&routing=" + routing
	}
	return c.Request(ctx, "DELETE", path, nil)
}

// DocsBulkDelete mirrors docsBulkDelete(documents): each document is an
// elasticvue row id of the form "_index####_type####_id"; only _index and
// _id are used, joined into a refresh=true bulk body with CRLF lines.
func (c *Client) DocsBulkDelete(ctx context.Context, documents []string) (*Response, error) {
	var b strings.Builder
	for _, doc := range documents {
		parts := strings.Split(doc, "####") // _index####_type####_id
		var index, id string
		if len(parts) > 0 {
			index = parts[0]
		}
		if len(parts) > 2 {
			id = parts[2]
		}
		line, err := json.Marshal(map[string]any{
			"delete": map[string]any{"_index": index, "_id": id},
		})
		if err != nil {
			return nil, err
		}
		b.Write(line)
		b.WriteString("\r\n")
	}
	return c.Request(ctx, "POST", "_bulk?refresh=true", b.String())
}
