// Command perfseed populates a docker/compose.yml ES8 cluster with the
// large-cluster perf fixture: thousands of light indices, one wide-mapping
// index, and one index with a fixed number of bulk-loaded hits.
//
//	docker compose -f docker/compose.yml up -d
//	go run ./cmd/perfseed -url http://127.0.0.1:9200 -user elastic -pass {{PASSWORD}}
//
// Idempotent: deletes the perf-* fixture before rebuilding it.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"seismic/internal/es"
)

func main() {
	url := flag.String("url", "http://127.0.0.1:9200", "cluster base URL")
	user := flag.String("user", "elastic", "basic-auth user (\"\" for none)")
	pass := flag.String("pass", "seismic8test", "basic-auth password")
	indices := flag.Int("indices", 5000, "number of light perf-idx-* indices")
	bigmapFields := flag.Int("bigmap-fields", 20000, "fields in the perf-bigmap mapping")
	docs := flag.Int("docs", 10000, "documents in the perf-hits index")
	flag.Parse()

	auth := es.Auth{Type: es.AuthNone}
	if *user != "" {
		auth = es.Auth{Type: es.AuthBasic, Username: *user, Password: *pass}
	}
	client, err := es.New(*url, auth)
	if err != nil {
		fail("connect: %v", err)
	}

	reset(client)
	seedLightIndices(client, *indices)
	seedBigMapping(client, *bigmapFields)
	seedHits(client, *docs)
	fmt.Println("perf fixture ready")
}

func fail(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "perfseed: "+format+"\n", args...)
	os.Exit(1)
}

func ctx() context.Context { return context.Background() }

// reset deletes any existing perf fixture indices. ES 8 defaults to
// action.destructive_requires_name=true and rejects wildcard deletes, so
// list the matching indices first and delete them by explicit name.
func reset(c *es.Client) {
	start := time.Now()
	var rows []struct {
		Index string `json:"index"`
	}
	if _, err := c.RequestJSON(ctx(), "GET",
		"_cat/indices/perf-*?format=json&h=index", nil, &rows); err != nil {
		fail("reset list: %v", err)
	}
	const batch = 50 // 显式名拼接，控制 URL 长度
	deleted := 0
	for i := 0; i < len(rows); i += batch {
		j := i + batch
		if j > len(rows) {
			j = len(rows)
		}
		names := make([]string, 0, j-i)
		for _, r := range rows[i:j] {
			if strings.HasPrefix(r.Index, "perf-") {
				names = append(names, r.Index)
			}
		}
		if len(names) == 0 {
			continue
		}
		if _, err := c.Request(ctx(), "DELETE",
			strings.Join(names, ",")+"?ignore_unavailable=true", nil); err != nil {
			fail("reset delete: %v", err)
		}
		deleted += len(names)
	}
	fmt.Printf("reset (%d indices)  %s\n", deleted, time.Since(start).Round(time.Millisecond))
}

func seedLightIndices(c *es.Client, n int) {
	start := time.Now()
	// 单节点集群：模板固定 1 主 0 副，避免 5k 副本占满 max_shards_per_node 配额。
	tpl := `{"index_patterns":["perf-idx-*"],"priority":100,` +
		`"template":{"settings":{"number_of_shards":1,"number_of_replicas":0}}}`
	if _, err := c.Request(ctx(), "PUT", "_index_template/perf-seed", tpl); err != nil {
		fail("index template: %v", err)
	}
	const batch = 500
	var body strings.Builder
	for i := 0; i < n; i++ {
		fmt.Fprintf(&body, "{\"index\":{\"_index\":\"perf-idx-%04d\"}}\n", i)
		body.WriteString("{\"level\":\"info\",\"msg\":\"seed\"}\n")
		if (i+1)%batch == 0 || i == n-1 {
			if _, err := c.Request(ctx(), "POST", "_bulk", body.String()); err != nil {
				fail("bulk light indices: %v", err)
			}
			body.Reset()
		}
	}
	fmt.Printf("%d light indices %s\n", n, time.Since(start).Round(time.Millisecond))
}

func seedBigMapping(c *es.Client, fields int) {
	start := time.Now()
	var body strings.Builder
	// total_fields.limit 默认 1000，宽 mapping 需按字段数放宽（留余量）。
	fmt.Fprintf(&body,
		`{"settings":{"number_of_replicas":0,"mapping":{"total_fields":{"limit":%d}}},"mappings":{"properties":{`,
		fields+8)
	for i := 0; i < fields; i++ {
		if i > 0 {
			body.WriteByte(',')
		}
		if i%2 == 0 {
			fmt.Fprintf(&body, "\"f_%04d\":{\"type\":\"keyword\"}", i)
		} else {
			fmt.Fprintf(&body, "\"f_%04d\":{\"type\":\"long\"}", i)
		}
	}
	body.WriteString(`}}}`)
	if _, err := c.Request(ctx(), "PUT", "perf-bigmap", body.String()); err != nil {
		fail("big mapping: %v", err)
	}
	// 样例文档只用已映射字段（f_0000 keyword / f_0001 long），不新增字段。
	doc := map[string]any{"f_0000": "info", "f_0001": 42}
	if _, err := c.Request(ctx(), "POST", "perf-bigmap/_doc", doc); err != nil {
		fail("big mapping doc: %v", err)
	}
	fmt.Printf("big mapping (%d fields, %s) %s\n", fields, mbSize(body.Len()), time.Since(start).Round(time.Millisecond))
}

func seedHits(c *es.Client, docs int) {
	start := time.Now()
	mapping := `{"settings":{"number_of_replicas":0},"mappings":{"properties":{` +
		`"@timestamp":{"type":"date"},"level":{"type":"keyword"},` +
		`"host":{"type":"keyword"},"service":{"type":"keyword"},` +
		`"cpu":{"type":"float"},"msg":{"type":"text"}}}}`
	if _, err := c.Request(ctx(), "PUT", "perf-hits", mapping); err != nil {
		fail("hits mapping: %v", err)
	}
	const batch = 1000
	var body strings.Builder
	levels := []string{"info", "warn", "error"}
	for i := 0; i < docs; i++ {
		fmt.Fprintf(&body, "{\"index\":{\"_index\":\"perf-hits\"}}\n")
		fmt.Fprintf(&body,
			`{"@timestamp":"2026-10-%02dT%02d:%02d:%02dZ","level":%q,"host":"node-%d","service":"svc-%d","cpu":%.2f,"msg":"hit %d for the 10k-render benchmark"}`,
			1+i%28, i%24, i%60, i%60, levels[i%3], i%7, i%13, float64(i%100)/7.0, i)
		body.WriteByte('\n')
		if (i+1)%batch == 0 || i == docs-1 {
			if _, err := c.Request(ctx(), "POST", "_bulk?refresh=false", body.String()); err != nil {
				fail("bulk hits: %v", err)
			}
			body.Reset()
		}
	}
	if _, err := c.Request(ctx(), "POST", "perf-hits/_refresh", nil); err != nil {
		fail("refresh: %v", err)
	}
	fmt.Printf("%d hits            %s\n", docs, time.Since(start).Round(time.Millisecond))
}

func mbSize(n int) string { return fmt.Sprintf("%.1f MB", float64(n)/(1<<20)) }
