package es

import (
	"context"
	"os"
	"testing"
)

// Acceptance against real Elasticsearch clusters started by
// docker/compose.yml. Skipped unless the env vars point at them:
//
//	docker compose -f docker/compose.yml up -d
//	SEISMIC_ES8=http://elastic:seismic8test@127.0.0.1:9200 \
//	SEISMIC_ES7=http://127.0.0.1:9210 go test ./internal/es/ -run Acceptance -v
func es8URL(t *testing.T) string {
	url := os.Getenv("SEISMIC_ES8")
	if url == "" {
		t.Skip("SEISMIC_ES8 not set; start docker/compose.yml to run acceptance")
	}
	return url
}

func es7URL(t *testing.T) string {
	url := os.Getenv("SEISMIC_ES7")
	if url == "" {
		t.Skip("SEISMIC_ES7 not set; start docker/compose.yml to run acceptance")
	}
	return url
}

func TestAcceptanceEs8BasicAuthCluster(t *testing.T) {
	c, err := New(es8URL(t), Auth{Type: AuthBasic, Username: "elastic", Password: "seismic8test"})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()

	resp, err := c.Request(ctx, "GET", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	var info map[string]any
	if err := resp.JSON(&info); err != nil {
		t.Fatal(err)
	}
	ver := info["version"].(map[string]any)["number"].(string)
	if ver[:2] != "8." {
		t.Fatalf("es8 version = %q", ver)
	}

	// 索引生命周期：创建、索引文档、搜索、设置、删除。
	if _, err := c.IndexCreate(ctx, "seismic-acceptance", map[string]any{
		"settings": map[string]any{"number_of_shards": 1, "number_of_replicas": 0},
	}); err != nil {
		t.Fatal(err)
	}
	defer c.IndexDelete(ctx, []string{"seismic-acceptance"})

	if _, err := c.Index(ctx, "seismic-acceptance", "_doc", "1", "", map[string]any{"msg": "hello seismic"}); err != nil {
		t.Fatal(err)
	}
	if _, err := c.IndexRefresh(ctx, []string{"seismic-acceptance"}); err != nil {
		t.Fatal(err)
	}
	sr, err := c.Search(ctx, map[string]any{"query": map[string]any{"match": map[string]any{"msg": "seismic"}}}, []string{"seismic-acceptance"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	var search struct {
		Hits struct {
			Total any `json:"total"`
		} `json:"hits"`
	}
	if err := sr.JSON(&search); err != nil {
		t.Fatal(err)
	}

	if _, err := c.IndexGetSettings(ctx, "seismic-acceptance"); err != nil {
		t.Fatal(err)
	}
	if _, err := c.ClusterHealth(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := c.CatIndices(ctx, map[string]any{"format": "json", "bytes": "b"}, "seismic-acceptance"); err != nil {
		t.Fatal(err)
	}

	// 8.x 有原生 security：_security 前缀 + 用户列表。
	c.Version = ver
	users, err := c.SecurityUsers(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var userMap map[string]any
	if err := users.JSON(&userMap); err != nil {
		t.Fatal(err)
	}
	if _, ok := userMap["elastic"]; !ok {
		t.Error("elastic user missing from _security/user")
	}
}

func TestAcceptanceEs7NoAuthSecurityPath(t *testing.T) {
	c, err := New(es7URL(t), Auth{Type: AuthNone})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()

	resp, err := c.Request(ctx, "GET", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	var info map[string]any
	if err := resp.JSON(&info); err != nil {
		t.Fatal(err)
	}
	ver := info["version"].(map[string]any)["number"].(string)
	if ver[:2] != "7." {
		t.Fatalf("es7 version = %q", ver)
	}

	// 7.17 security 关闭：/_security/user 返回非 2xx → HTTPError（不是崩溃）。
	c.Version = ver
	_, err = c.SecurityUsers(ctx)
	if err == nil {
		t.Log("es7 security returned 2xx (unexpected with xpack.security.enabled=false)")
	}

	if _, err := c.CatNodes(ctx, map[string]any{"format": "json", "h": "name,version,role"}); err != nil {
		t.Fatal(err)
	}
	if _, err := c.CatShards(ctx, map[string]any{"format": "json"}, ""); err != nil {
		t.Fatal(err)
	}
	if _, err := c.CatRecovery(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := c.CatSlmPolicies(ctx); err != nil {
		t.Fatal(err) // 7.17 支持 SLM
	}
	if _, err := c.Request(ctx, "GET", "_snapshot", nil); err != nil {
		t.Fatal(err)
	}
}
