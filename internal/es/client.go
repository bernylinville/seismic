// Package es is the Elasticsearch REST client, a port of elasticvue's
// src/services/ElasticsearchAdapter.ts and src/helpers/elasticsearchAdapter.ts.
package es

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// AuthType mirrors elasticvue AuthType (src/store/connection.ts).
type AuthType string

// maxResponseBytes caps response bodies read into memory. ES JSON replies
// (cat APIs, search, mappings) stay far below this even on large clusters;
// anything bigger is almost certainly a mis-sized request and would otherwise
// balloon memory without a usable error.
const maxResponseBytes = 64 << 20 // 64 MB


const (
	AuthNone   AuthType = "none"
	AuthBasic  AuthType = "basicAuth"
	AuthAPIKey AuthType = "apiKey"
	AuthAWSIAM AuthType = "awsIAM"
)

// Auth holds the credentials for one connection. Secrets arrive from the
// connection store (Keychain on macOS) and never leave the client.
type Auth struct {
	Type AuthType

	Username string // basic
	Password string // basic (or raw API key when username empty)

	AccessKeyID     string // awsIAM
	SecretAccessKey string // awsIAM
	SessionToken    string // awsIAM
	Region          string // awsIAM
}

// buildAuthHeader mirrors buildFetchAuthHeader: username+password → Basic,
// username only → Basic, password only → ApiKey (the elasticvue convention
// where a lone "password" is an API key).
func buildAuthHeader(username, password string) string {
	switch {
	case username != "" && password != "":
		return "Basic " + base64.StdEncoding.EncodeToString([]byte(username+":"+password))
	case username != "":
		return "Basic " + base64.StdEncoding.EncodeToString([]byte(username))
	case password != "":
		return "ApiKey " + password
	}
	return ""
}

// clusterAuthHeader mirrors clusterAuthHeader: the Authorization header for
// the connection's auth, empty when none or when awsIAM (signed per request).
func clusterAuthHeader(a Auth) string {
	switch a.Type {
	case AuthBasic:
		return buildAuthHeader(a.Username, a.Password)
	case AuthAPIKey:
		return buildAuthHeader("", a.Password)
	default: // AuthNone, AuthAWSIAM
		return ""
	}
}

// Client talks to one cluster. Version is the live cluster version
// ("8.14.1"), set after a successful ping; version-dependent paths are
// computed from it at call time, as elasticvue reads its connection store.
type Client struct {
	uri        string // base URI, always with trailing slash
	http       *http.Client
	auth       Auth
	authHeader string
	aws        *sigv4Signer

	// Version of the connected cluster, e.g. "8.14.1". Empty until ping.
	Version string
	// Distribution: "elasticsearch" (default) or "opensearch".
	Distribution string
	// BuildFlavor is non-empty for special deployments (e.g. "serverless").
	BuildFlavor string
}

// New builds a client for uri with the given auth, verifying TLS as usual.
func New(uri string, auth Auth) (*Client, error) {
	return NewWithTLS(uri, auth, nil)
}

// NewWithTLS builds a client with custom TLS settings; a nil config keeps
// system verification. Self-signed clusters (the ES docker default) pass a
// config with InsecureSkipVerify set by the caller's explicit choice.
func NewWithTLS(uri string, auth Auth, tlsCfg *tls.Config) (*Client, error) {
	if !strings.HasSuffix(uri, "/") {
		uri += "/"
	}
	c := &Client{
		uri:  uri,
		auth: auth,
		http: &http.Client{Timeout: 60 * time.Second},
	}
	if tlsCfg != nil {
		c.http.Transport = &http.Transport{TLSClientConfig: tlsCfg.Clone()}
	}
	c.authHeader = clusterAuthHeader(auth)
	if auth.Type == AuthAWSIAM {
		var err error
		c.aws, err = newSigV4Signer(auth.AccessKeyID, auth.SecretAccessKey, auth.SessionToken, auth.Region, "es")
		if err != nil {
			return nil, err
		}
	}
	return c, nil
}

// HTTPError is a non-2xx response; Body carries the ES error JSON for display.
type HTTPError struct {
	Status string
	Code   int
	Body   []byte
}

func (e *HTTPError) Error() string {
	if len(e.Body) > 512 {
		return fmt.Sprintf("%s: %s…", e.Status, e.Body[:512])
	}
	return fmt.Sprintf("%s: %s", e.Status, e.Body)
}

// Response is a successful ES reply; Body is the raw payload.
type Response struct {
	StatusCode int
	Header     http.Header
	Body       []byte
}

// JSON unmarshals the body into v.
func (r *Response) JSON(v any) error { return json.Unmarshal(r.Body, v) }

const maxIndicesPerRequest = 16

// chunk mirrors the adapter's chunk() for callInChunks.
func chunk[T any](in []T, size int) [][]T {
	out := make([][]T, 0, (len(in)+size-1)/size)
	for i := 0; i < len(in); i += size {
		end := i + size
		if end > len(in) {
			end = len(in)
		}
		out = append(out, in[i:end])
	}
	return out
}

// ChunkIndices splits index lists the way callInChunks does.
func ChunkIndices(indices []string) [][]string { return chunk(indices, maxIndicesPerRequest) }

// Request performs one ES call, mirroring the adapter's request():
//
//   - GET: params (map[string]any) becomes the query string;
//   - PUT/POST: params is the JSON body (a string is sent verbatim);
//   - HEAD: returns ok as a bool body;
//   - non-2xx answers come back as *HTTPError.
func (c *Client) Request(ctx context.Context, method, path string, params any) (*Response, error) {
	req, err := c.buildRequest(ctx, method, path, params)
	if err != nil {
		return nil, err
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// Read at most maxResponseBytes+1 so an oversized body is detected by the
	// extra byte instead of buffering it all first.
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxResponseBytes {
		return nil, errResponseTooLarge(method, path)
	}

	if method == http.MethodHead {
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return &Response{StatusCode: resp.StatusCode, Header: resp.Header, Body: []byte("true")}, nil
		}
		return &Response{StatusCode: resp.StatusCode, Header: resp.Header, Body: []byte("false")}, nil
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, &HTTPError{Status: resp.Status, Code: resp.StatusCode, Body: data}
	}
	return &Response{StatusCode: resp.StatusCode, Header: resp.Header, Body: data}, nil
}

// RequestJSON performs one ES call and, on 2xx, streams the body straight
// into out with a json.Decoder: the raw JSON is never buffered alongside the
// decoded value, halving peak memory on MB-scale replies (10k-hit searches,
// whole-cluster mappings). The returned Response carries status and headers
// only (Body stays nil); non-2xx replies still come back as *HTTPError with
// the raw error body. HEAD is not supported.
func (c *Client) RequestJSON(ctx context.Context, method, path string, params, out any) (*Response, error) {
	req, err := c.buildRequest(ctx, method, path, params)
	if err != nil {
		return nil, err
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		data, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes+1))
		if err != nil {
			return nil, err
		}
		return nil, &HTTPError{Status: resp.Status, Code: resp.StatusCode, Body: data}
	}

	lr := &limitReader{r: resp.Body, left: maxResponseBytes + 1}
	err = json.NewDecoder(lr).Decode(out)
	if lr.read > maxResponseBytes {
		return nil, errResponseTooLarge(method, path)
	}
	if err != nil {
		return nil, fmt.Errorf("es: decode %s %s reply: %w", method, path, err)
	}
	return &Response{StatusCode: resp.StatusCode, Header: resp.Header}, nil
}

// errResponseTooLarge is the guardrail message shared by both read paths.
func errResponseTooLarge(method, path string) error {
	return fmt.Errorf("es: %s %s response exceeds the %d MB limit; narrow the request (filter_path, size, or pagination)", method, path, maxResponseBytes>>20)
}

// limitReader serves at most n bytes and counts what it served, so callers
// can detect a body that reached the cap after a streaming decode.
type limitReader struct {
	r    io.Reader
	left int
	read int
}

func (l *limitReader) Read(p []byte) (int, error) {
	if l.left <= 0 {
		return 0, io.EOF
	}
	if len(p) > l.left {
		p = p[:l.left]
	}
	n, err := l.r.Read(p)
	l.left -= n
	l.read += n
	return n, err
}

// buildRequest assembles the http.Request for one ES call:
//
//   - GET: params (map[string]any) becomes the query string;
//   - PUT/POST: params is the JSON body (a string is sent verbatim).
func (c *Client) buildRequest(ctx context.Context, method, path string, params any) (*http.Request, error) {
	full := c.uri + path
	if method == http.MethodGet {
		if m, ok := params.(map[string]any); ok && len(m) > 0 {
			q := url.Values{}
			for k, v := range m {
				q.Add(k, fmt.Sprint(v))
			}
			full += "?" + q.Encode()
		}
	}

	var bodyReader io.Reader
	var rawBody []byte
	if (method == http.MethodPut || method == http.MethodPost) && params != nil {
		switch b := params.(type) {
		case string:
			rawBody = []byte(b)
		default:
			enc, err := json.Marshal(params)
			if err != nil {
				return nil, err
			}
			rawBody = enc
		}
		bodyReader = bytes.NewReader(rawBody)
	}

	req, err := http.NewRequestWithContext(ctx, method, full, bodyReader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	if len(rawBody) > 0 {
		req.Header.Set("Content-Type", "application/json")
	}

	if c.authHeader != "" {
		req.Header.Set("Authorization", c.authHeader)
	} else if c.aws != nil {
		if err := c.aws.sign(ctx, req); err != nil {
			return nil, err
		}
	}
	return req, nil
}
