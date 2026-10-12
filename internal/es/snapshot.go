package es

import "context"

// CatRepositories mirrors catRepositories(params).
func (c *Client) CatRepositories(ctx context.Context, params map[string]any) (*Response, error) {
	return c.Request(ctx, "GET", "_snapshot", params)
}

// CatSnapshots mirrors catSnapshots({repository}).
func (c *Client) CatSnapshots(ctx context.Context, repository string) (*Response, error) {
	return c.Request(ctx, "GET", "_snapshot/"+repository+"/_all", nil)
}

// SnapshotCreateRepository mirrors snapshotCreateRepository({repository, body}).
func (c *Client) SnapshotCreateRepository(ctx context.Context, repository string, body any) (*Response, error) {
	return c.Request(ctx, "PUT", "_snapshot/"+repository, body)
}

// SnapshotDeleteRepository mirrors snapshotDeleteRepository({repository}).
func (c *Client) SnapshotDeleteRepository(ctx context.Context, repository string) (*Response, error) {
	return c.Request(ctx, "DELETE", "_snapshot/"+repository, nil)
}

// SnapshotCreate mirrors snapshotCreate({repository, snapshot, body}).
func (c *Client) SnapshotCreate(ctx context.Context, repository, snapshot string, body any) (*Response, error) {
	return c.Request(ctx, "PUT", "_snapshot/"+repository+"/"+snapshot, body)
}

// SnapshotDelete mirrors snapshotDelete({repository, snapshot}).
func (c *Client) SnapshotDelete(ctx context.Context, repository, snapshot string) (*Response, error) {
	return c.Request(ctx, "DELETE", "_snapshot/"+repository+"/"+snapshot, nil)
}

// SnapshotRestore mirrors snapshotRestore({repository, snapshot, body}).
func (c *Client) SnapshotRestore(ctx context.Context, repository, snapshot string, body any) (*Response, error) {
	return c.Request(ctx, "POST", "_snapshot/"+repository+"/"+snapshot+"/_restore", body)
}

// GetSnapshot mirrors getSnapshot({repository, snapshot}).
func (c *Client) GetSnapshot(ctx context.Context, repository, snapshot string) (*Response, error) {
	return c.Request(ctx, "GET", "_snapshot/"+repository+"/"+snapshot, nil)
}
