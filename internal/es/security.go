package es

import "context"

// securityPath prefixes p with the version-dependent security API base path
// (_xpack/security before 6.6, _security from 6.6 on).
func (c *Client) securityPath(p string) string {
	return SecurityBasePath(c.Version) + "/" + p
}

// SecurityUsers mirrors securityUsers().
func (c *Client) SecurityUsers(ctx context.Context) (*Response, error) {
	return c.Request(ctx, "GET", c.securityPath("user"), nil)
}

// SecurityRoles mirrors securityRoles().
func (c *Client) SecurityRoles(ctx context.Context) (*Response, error) {
	return c.Request(ctx, "GET", c.securityPath("role"), nil)
}

// SecurityRolePut mirrors securityRolePut({name, body}).
func (c *Client) SecurityRolePut(ctx context.Context, name string, body any) (*Response, error) {
	return c.Request(ctx, "PUT", c.securityPath("role/"+encodeURIComponent(name)), body)
}

// SecurityRoleDelete mirrors securityRoleDelete({name}).
func (c *Client) SecurityRoleDelete(ctx context.Context, name string) (*Response, error) {
	return c.Request(ctx, "DELETE", c.securityPath("role/"+encodeURIComponent(name)), nil)
}

// SecurityUserPut mirrors securityUserPut({username, body}).
func (c *Client) SecurityUserPut(ctx context.Context, username string, body any) (*Response, error) {
	return c.Request(ctx, "PUT", c.securityPath("user/"+encodeURIComponent(username)), body)
}

// SecurityUserPutPassword mirrors securityUserPutPassword({username, body}).
func (c *Client) SecurityUserPutPassword(ctx context.Context, username string, body any) (*Response, error) {
	return c.Request(ctx, "PUT", c.securityPath("user/"+encodeURIComponent(username)+"/_password"), body)
}

// SecurityUserDelete mirrors securityUserDelete({username}).
func (c *Client) SecurityUserDelete(ctx context.Context, username string) (*Response, error) {
	return c.Request(ctx, "DELETE", c.securityPath("user/"+encodeURIComponent(username)), nil)
}

// SecurityUsersInto streams the user list straight into out.
func (c *Client) SecurityUsersInto(ctx context.Context, out any) (*Response, error) {
	return c.RequestJSON(ctx, "GET", c.securityPath("user"), nil, out)
}

// SecurityRolesInto streams the role list straight into out.
func (c *Client) SecurityRolesInto(ctx context.Context, out any) (*Response, error) {
	return c.RequestJSON(ctx, "GET", c.securityPath("role"), nil, out)
}
