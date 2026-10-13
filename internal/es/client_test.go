package es

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func newTestClient(t *testing.T, auth Auth, handler http.HandlerFunc) (*Client, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	c, err := New(srv.URL, auth)
	if err != nil {
		t.Fatal(err)
	}
	return c, srv
}

func TestClientAuthHeaders(t *testing.T) {
	cases := []struct {
		name       string
		auth       Auth
		want       string
		wantPrefix string
	}{
		{"none", Auth{Type: AuthNone}, "", ""},
		{"basic", Auth{Type: AuthBasic, Username: "elastic", Password: "pass"}, "Basic " + mustB64("elastic:pass"), ""},
		{"basic username only", Auth{Type: AuthBasic, Username: "elastic"}, "Basic " + mustB64("elastic"), ""},
		{"api key", Auth{Type: AuthAPIKey, Password: "apikey123"}, "ApiKey apikey123", ""},
		{"aws iam uses signer", Auth{Type: AuthAWSIAM, AccessKeyID: "AKID", SecretAccessKey: "SECRET", Region: "us-east-1"}, "", "AWS4-HMAC-SHA256 Credential=AKID/"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var got string
			c, _ := newTestClient(t, tc.auth, func(w http.ResponseWriter, r *http.Request) {
				got = r.Header.Get("Authorization")
				w.WriteHeader(200)
			})
			if _, err := c.Request(context.Background(), http.MethodGet, "", nil); err != nil {
				t.Fatal(err)
			}
			if tc.want != "" && got != tc.want {
				t.Errorf("Authorization = %q, want %q", got, tc.want)
			}
			if tc.wantPrefix != "" && !strings.HasPrefix(got, tc.wantPrefix) {
				t.Errorf("Authorization = %q, want prefix %q", got, tc.wantPrefix)
			}
		})
	}
}

func mustB64(s string) string {
	return base64.StdEncoding.EncodeToString([]byte(s))
}
func TestClientGetParamsBecomeQuery(t *testing.T) {
	c, _ := newTestClient(t, Auth{Type: AuthNone}, func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if q.Get("bytes") != "b" || q.Get("h") != "index,health" {
			t.Errorf("query = %v", r.URL.RawQuery)
		}
		if r.URL.Path != "/_cat/indices" {
			t.Errorf("path = %q", r.URL.Path)
		}
		w.WriteHeader(200)
	})
	params := map[string]any{"bytes": "b", "h": "index,health"}
	if _, err := c.Request(context.Background(), http.MethodGet, "_cat/indices", params); err != nil {
		t.Fatal(err)
	}
}

func TestClientPostBodyIsJSON(t *testing.T) {
	c, _ := newTestClient(t, Auth{Type: AuthNone}, func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("Content-Type = %q", r.Header.Get("Content-Type"))
		}
		body, _ := io.ReadAll(r.Body)
		var m map[string]any
		if err := json.Unmarshal(body, &m); err != nil {
			t.Fatalf("body not JSON: %s", body)
		}
		if m["query"].(map[string]any)["match_all"] == nil {
			t.Errorf("body = %s", body)
		}
		w.WriteHeader(200)
	})
	body := map[string]any{"query": map[string]any{"match_all": map[string]any{}}}
	if _, err := c.Request(context.Background(), http.MethodPost, "_search", body); err != nil {
		t.Fatal(err)
	}
}

func TestClientRawStringBody(t *testing.T) {
	c, _ := newTestClient(t, Auth{Type: AuthNone}, func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if string(body) != `{"a":1}` {
			t.Errorf("body = %q", body)
		}
		w.WriteHeader(200)
	})
	if _, err := c.Request(context.Background(), http.MethodPost, "_bulk", `{"a":1}`); err != nil {
		t.Fatal(err)
	}
}

func TestClientHeadReturnsBool(t *testing.T) {
	c := testHeadClient(t, 404)
	resp, err := c.Request(context.Background(), http.MethodHead, "missing-index", nil)
	if err != nil {
		t.Fatal(err)
	}
	if string(resp.Body) != "false" {
		t.Errorf("HEAD 404 body = %q", resp.Body)
	}
}

func testHeadClient(t *testing.T, code int) *Client {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(code)
	}))
	t.Cleanup(srv.Close)
	c, err := New(srv.URL, Auth{Type: AuthNone})
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestClientErrorCarriesBody(t *testing.T) {
	c, _ := newTestClient(t, Auth{Type: AuthNone}, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(400)
		w.Write([]byte(`{"error":{"type":"index_not_found_exception"}}`))
	})
	_, err := c.Request(context.Background(), http.MethodGet, "nope/_stats", nil)
	httpErr, ok := err.(*HTTPError)
	if !ok {
		t.Fatalf("err = %v, want *HTTPError", err)
	}
	if httpErr.Code != 400 {
		t.Errorf("code = %d", httpErr.Code)
	}
	if len(httpErr.Body) == 0 || httpErr.Error() == "" {
		t.Error("empty error body")
	}
}

func TestChunkIndices(t *testing.T) {
	var indices []string
	for i := range 35 {
		indices = append(indices, itoa(i))
	}
	chunks := ChunkIndices(indices)
	if len(chunks) != 3 || len(chunks[0]) != 16 || len(chunks[2]) != 3 {
		t.Errorf("chunks = %d lens %d %d %d", len(chunks), len(chunks[0]), len(chunks[1]), len(chunks[2]))
	}
}

func itoa(i int) string {
	return string(rune('a' + i))
}

func TestClientResponseSizeLimit(t *testing.T) {
	c, _ := newTestClient(t, Auth{Type: AuthNone}, func(w http.ResponseWriter, r *http.Request) {
		w.Write(bytes.Repeat([]byte("x"), maxResponseBytes+1))
	})
	_, err := c.Request(context.Background(), http.MethodGet, "_cat/indices", nil)
	if err == nil || !strings.Contains(err.Error(), "exceeds the 64 MB limit") {
		t.Fatalf("err = %v, want size-limit error", err)
	}
}

func TestClientResponseAtLimitOK(t *testing.T) {
	c, _ := newTestClient(t, Auth{Type: AuthNone}, func(w http.ResponseWriter, r *http.Request) {
		w.Write(bytes.Repeat([]byte("x"), maxResponseBytes))
	})
	resp, err := c.Request(context.Background(), http.MethodGet, "_cat/indices", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Body) != maxResponseBytes {
		t.Errorf("body = %d bytes, want %d", len(resp.Body), maxResponseBytes)
	}
}

func TestClientRequestJSONDecodes(t *testing.T) {
	c, _ := newTestClient(t, Auth{Type: AuthNone}, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`[{"index":"a"},{"index":"b"}]`))
	})
	var rows []struct {
		Index string `json:"index"`
	}
	resp, err := c.RequestJSON(context.Background(), http.MethodGet, "_cat/indices", nil, &rows)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 || rows[0].Index != "a" {
		t.Errorf("rows = %+v", rows)
	}
	if resp.StatusCode != 200 {
		t.Errorf("status = %d", resp.StatusCode)
	}
	if resp.Body != nil {
		t.Errorf("Body should stay nil on streamed replies, got %d bytes", len(resp.Body))
	}
}

func TestClientRequestJSONErrorCarriesBody(t *testing.T) {
	c, _ := newTestClient(t, Auth{Type: AuthNone}, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(400)
		w.Write([]byte(`{"error":{"type":"search_phase_execution"}}`))
	})
	var v map[string]any
	_, err := c.RequestJSON(context.Background(), http.MethodGet, "_search", nil, &v)
	httpErr, ok := err.(*HTTPError)
	if !ok {
		t.Fatalf("err = %v, want *HTTPError", err)
	}
	if httpErr.Code != 400 || len(httpErr.Body) == 0 {
		t.Errorf("httpErr = %+v", httpErr)
	}
}

func TestClientRequestJSONOversize(t *testing.T) {
	c, _ := newTestClient(t, Auth{Type: AuthNone}, func(w http.ResponseWriter, r *http.Request) {
		// A valid JSON string value that stays open past the cap, so the
		// decoder itself never completes: only the byte counter can tell.
		w.Write([]byte(`{"pad":"`))
		w.Write(bytes.Repeat([]byte("x"), maxResponseBytes))
		w.Write([]byte(`"}`))
	})
	var v map[string]any
	_, err := c.RequestJSON(context.Background(), http.MethodGet, "_search", nil, &v)
	if err == nil || !strings.Contains(err.Error(), "exceeds the 64 MB limit") {
		t.Fatalf("err = %v, want size-limit error", err)
	}
}

func TestClientRequestJSONDecodeError(t *testing.T) {
	c, _ := newTestClient(t, Auth{Type: AuthNone}, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`not json`))
	})
	var v map[string]any
	_, err := c.RequestJSON(context.Background(), http.MethodGet, "_search", nil, &v)
	if err == nil || !strings.Contains(err.Error(), "decode") {
		t.Fatalf("err = %v, want decode error", err)
	}
}
