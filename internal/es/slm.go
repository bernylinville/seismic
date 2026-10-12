package es

import "context"

// CatSlmPolicies mirrors catSlmPolicies().
func (c *Client) CatSlmPolicies(ctx context.Context) (*Response, error) {
	return c.Request(ctx, "GET", "_slm/policy", nil)
}

// SlmGetPolicy mirrors slmGetPolicy({policy}).
func (c *Client) SlmGetPolicy(ctx context.Context, policy string) (*Response, error) {
	return c.Request(ctx, "GET", "_slm/policy/"+policy, nil)
}

// SlmPutPolicy mirrors slmPutPolicy({policy, body}).
func (c *Client) SlmPutPolicy(ctx context.Context, policy string, body any) (*Response, error) {
	return c.Request(ctx, "PUT", "_slm/policy/"+policy, body)
}

// SlmDeletePolicy mirrors slmDeletePolicy({policy}).
func (c *Client) SlmDeletePolicy(ctx context.Context, policy string) (*Response, error) {
	return c.Request(ctx, "DELETE", "_slm/policy/"+policy, nil)
}

// SlmExecutePolicy mirrors slmExecutePolicy({policy}).
func (c *Client) SlmExecutePolicy(ctx context.Context, policy string) (*Response, error) {
	return c.Request(ctx, "POST", "_slm/policy/"+policy+"/_execute", nil)
}

// SlmGetStatus mirrors slmGetStatus().
func (c *Client) SlmGetStatus(ctx context.Context) (*Response, error) {
	return c.Request(ctx, "GET", "_slm/status", nil)
}
