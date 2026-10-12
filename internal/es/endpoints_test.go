// elasticvue adapter method coverage (src/services/ElasticsearchAdapter.ts → internal/es):
//
//	ping→Ping  clusterInfo→ClusterInfo  catIndices→CatIndices  template→Template
//	indexTemplate→IndexTemplate  indexGetAlias→IndexGetAlias  indexAddAlias→IndexAddAlias
//	indexDeleteAlias→IndexDeleteAlias  indexCreate→IndexCreate  deleteByQuery→DeleteByQuery
//	indexGet→IndexGet  indexStats→IndexStats  indexDelete→IndexDelete  indexRefresh→IndexRefresh
//	indexClearCache→IndexClearCache  indexFlush→IndexFlush  indexExists→IndexExists
//	indexGetSettings→IndexGetSettings  indexPutSettings→IndexPutSettings  reindex→Reindex
//	clone→Clone  index→Index  get→Get  delete→Delete  search→Search
//	docsBulkDelete→DocsBulkDelete  test→Test  clusterHealth→ClusterHealth
//	clusterStats→ClusterStats  clusterReroute→ClusterReroute  clusterGetSettings→ClusterGetSettings
//	clusterPutSettings→ClusterPutSettings  catShards→CatShards  catRecovery→CatRecovery
//	recovery→Recovery  indexClose→IndexClose  indexOpen→IndexOpen  indexForcemerge→IndexForcemerge
//	catNodes→CatNodes  nodes→Nodes  catRepositories→CatRepositories
//	catSnapshots→CatSnapshots  snapshotCreateRepository→SnapshotCreateRepository
//	snapshotDeleteRepository→SnapshotDeleteRepository  snapshotCreate→SnapshotCreate
//	snapshotDelete→SnapshotDelete  snapshotRestore→SnapshotRestore  getSnapshot→GetSnapshot
//	catSlmPolicies→CatSlmPolicies  slmGetPolicy→SlmGetPolicy  slmPutPolicy→SlmPutPolicy
//	slmDeletePolicy→SlmDeletePolicy  slmExecutePolicy→SlmExecutePolicy  slmGetStatus→SlmGetStatus
//	securityUsers→SecurityUsers  securityRoles→SecurityRoles  securityRolePut→SecurityRolePut
//	securityRoleDelete→SecurityRoleDelete  securityUserPut→SecurityUserPut
//	securityUserPutPassword→SecurityUserPutPassword  securityUserDelete→SecurityUserDelete
package es

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

type recReq struct {
	method string
	uri    string
	body   string
}

type endpointCase struct {
	name    string
	version string
	run     func(c *Client) error
	want    []recReq
}

func TestEndpoints(t *testing.T) {
	cases := []endpointCase{
		{name: "ping", run: func(c *Client) error {
			_, err := c.Ping(ctx())
			return err
		}, want: []recReq{{"GET", "/", ""}}},
		{name: "clusterInfo", run: func(c *Client) error {
			_, err := c.ClusterInfo(ctx())
			return err
		}, want: []recReq{{"GET", "/", ""}}},
		{name: "catIndices with filter and params", run: func(c *Client) error {
			_, err := c.CatIndices(ctx(), map[string]any{"bytes": "b", "format": "json"}, "log")
			return err
		}, want: []recReq{{"GET", "/_cat/indices/log*?bytes=b&format=json", ""}}},
		{name: "catIndices no filter", run: func(c *Client) error {
			_, err := c.CatIndices(ctx(), nil, "")
			return err
		}, want: []recReq{{"GET", "/_cat/indices/", ""}}},
		{name: "template", run: func(c *Client) error {
			_, err := c.Template(ctx())
			return err
		}, want: []recReq{{"GET", "/_template", ""}}},
		{name: "indexTemplate", run: func(c *Client) error {
			_, err := c.IndexTemplate(ctx())
			return err
		}, want: []recReq{{"GET", "/_index_template", ""}}},
		{name: "indexGetAlias", run: func(c *Client) error {
			_, err := c.IndexGetAlias(ctx(), "my-index")
			return err
		}, want: []recReq{{"GET", "/my-index/_alias", ""}}},
		{name: "indexAddAlias", run: func(c *Client) error {
			_, err := c.IndexAddAlias(ctx(), "my-index", "alias1")
			return err
		}, want: []recReq{{"PUT", "/my-index/_alias/alias1", ""}}},
		{name: "indexDeleteAlias", run: func(c *Client) error {
			_, err := c.IndexDeleteAlias(ctx(), "my-index", "alias1")
			return err
		}, want: []recReq{{"DELETE", "/my-index/_alias/alias1", ""}}},
		{name: "indexCreate", run: func(c *Client) error {
			_, err := c.IndexCreate(ctx(), "new-index", map[string]any{"settings": map[string]any{"shards": 1}})
			return err
		}, want: []recReq{{"PUT", "/new-index", `{"settings":{"shards":1}}`}}},
		{name: "deleteByQuery", run: func(c *Client) error {
			_, err := c.DeleteByQuery(ctx(), "old-index")
			return err
		}, want: []recReq{{"POST", "/old-index/_delete_by_query?refresh=true", `{"query":{"match_all":{}}}`}}},
		{name: "indexGet list", run: func(c *Client) error {
			_, err := c.IndexGet(ctx(), []string{"a", "b"})
			return err
		}, want: []recReq{{"GET", "/a,b", ""}}},
		{name: "indexStats", run: func(c *Client) error {
			_, err := c.IndexStats(ctx(), "my-index")
			return err
		}, want: []recReq{{"GET", "/my-index/_stats", ""}}},
		{name: "indexDelete chunked", run: func(c *Client) error {
			_, err := c.IndexDelete(ctx(), indices(20))
			return err
		}, want: []recReq{
			{"DELETE", "/" + joinComma(indices(16)), ""},
			{"DELETE", "/" + joinComma(indices(20)[16:]), ""},
		}},
		{name: "indexDelete single batch", run: func(c *Client) error {
			_, err := c.IndexDelete(ctx(), []string{"a", "b"})
			return err
		}, want: []recReq{{"DELETE", "/a,b", ""}}},
		{name: "indexRefresh", run: func(c *Client) error {
			_, err := c.IndexRefresh(ctx(), []string{"a", "b"})
			return err
		}, want: []recReq{{"POST", "/a,b/_refresh", ""}}},
		{name: "indexClearCache", run: func(c *Client) error {
			_, err := c.IndexClearCache(ctx(), []string{"a"})
			return err
		}, want: []recReq{{"POST", "/a/_cache/clear", ""}}},
		{name: "indexFlush", run: func(c *Client) error {
			_, err := c.IndexFlush(ctx(), []string{"a"})
			return err
		}, want: []recReq{{"POST", "/a/_flush", ""}}},
		{name: "indexExists", run: func(c *Client) error {
			_, err := c.IndexExists(ctx(), "my-index")
			return err
		}, want: []recReq{{"HEAD", "/my-index", ""}}},
		{name: "indexGetSettings", run: func(c *Client) error {
			_, err := c.IndexGetSettings(ctx(), "my-index")
			return err
		}, want: []recReq{{"GET", "/my-index/_settings", ""}}},
		{name: "indexPutSettings chunked", run: func(c *Client) error {
			_, err := c.IndexPutSettings(ctx(), indices(17), map[string]any{"index": map[string]any{"blocks": map[string]any{"write": true}}})
			return err
		}, want: []recReq{
			{"PUT", "/" + joinComma(indices(16)) + "/_settings", `{"index":{"blocks":{"write":true}}}`},
			{"PUT", "/" + indices(17)[16] + "/_settings", `{"index":{"blocks":{"write":true}}}`},
		}},
		{name: "reindex", run: func(c *Client) error {
			_, err := c.Reindex(ctx(), "src", "dst")
			return err
		}, want: []recReq{{"POST", "/_reindex?wait_for_completion=false", `{"dest":{"index":"dst"},"source":{"index":"src"}}`}}},
		{name: "clone", run: func(c *Client) error {
			_, err := c.Clone(ctx(), "src", "dst")
			return err
		}, want: []recReq{{"POST", "/src/_clone/dst", ""}}},
		{name: "indexClose", run: func(c *Client) error {
			_, err := c.IndexClose(ctx(), []string{"a"})
			return err
		}, want: []recReq{{"POST", "/a/_close", ""}}},
		{name: "indexOpen", run: func(c *Client) error {
			_, err := c.IndexOpen(ctx(), []string{"a"})
			return err
		}, want: []recReq{{"POST", "/a/_open", ""}}},
		{name: "indexForcemerge chunked", run: func(c *Client) error {
			_, err := c.IndexForcemerge(ctx(), indices(18))
			return err
		}, want: []recReq{
			{"POST", "/" + joinComma(indices(16)) + "/_forcemerge", ""},
			{"POST", "/" + joinComma(indices(18)[16:]) + "/_forcemerge", ""},
		}},
		{name: "index doc", run: func(c *Client) error {
			_, err := c.Index(ctx(), "i", "_doc", "a/b", "r1", map[string]any{"f": 1})
			return err
		}, want: []recReq{{"PUT", "/i/_doc/a%2Fb?refresh=true&routing=r1", `{"f":1}`}}},
		{name: "index doc no routing", run: func(c *Client) error {
			_, err := c.Index(ctx(), "i", "_doc", "1", "", nil)
			return err
		}, want: []recReq{{"PUT", "/i/_doc/1?refresh=true", ""}}},
		{name: "get doc", run: func(c *Client) error {
			_, err := c.Get(ctx(), "i", "_doc", "1", "r1")
			return err
		}, want: []recReq{{"GET", "/i/_doc/1?routing=r1", ""}}},
		{name: "get doc no routing", run: func(c *Client) error {
			_, err := c.Get(ctx(), "i", "_doc", "1", "")
			return err
		}, want: []recReq{{"GET", "/i/_doc/1", ""}}},
		{name: "delete doc", run: func(c *Client) error {
			_, err := c.Delete(ctx(), "i", "_doc", "1", "r1")
			return err
		}, want: []recReq{{"DELETE", "/i/_doc/1?refresh=true&routing=r1", ""}}},
		{name: "docsBulkDelete", run: func(c *Client) error {
			_, err := c.DocsBulkDelete(ctx(), []string{"idx1####_doc####1", "idx2####_doc####2"})
			return err
		}, want: []recReq{{"POST", "/_bulk?refresh=true",
			`{"delete":{"_id":"1","_index":"idx1"}}` + "\r\n" + `{"delete":{"_id":"2","_index":"idx2"}}` + "\r\n"}}},
		{name: "search with index and query params", run: func(c *Client) error {
			_, err := c.Search(ctx(), map[string]any{"query": map[string]any{"match_all": map[string]any{}}},
				[]string{"i1", "i2"}, map[string]string{"size": "0"})
			return err
		}, want: []recReq{{"POST", "/i1,i2/_search?size=0", `{"query":{"match_all":{}}}`}}},
		{name: "search no index", run: func(c *Client) error {
			_, err := c.Search(ctx(), map[string]any{}, nil, nil)
			return err
		}, want: []recReq{{"POST", "/_search", "{}"}}},
		{name: "test", run: func(c *Client) error {
			_, err := c.Test(ctx())
			return err
		}, want: []recReq{
			{"GET", "/", ""},
			{"POST", "/no_indices_match_this_pattern*/_search?allow_no_indices=true&ignore_unavailable=true&size=0", "{}"},
		}},
		{name: "clusterHealth", run: func(c *Client) error {
			_, err := c.ClusterHealth(ctx())
			return err
		}, want: []recReq{{"GET", "/_cluster/health", ""}}},
		{name: "clusterStats", run: func(c *Client) error {
			_, err := c.ClusterStats(ctx())
			return err
		}, want: []recReq{{"GET", "/_cluster/stats", ""}}},
		{name: "clusterReroute", run: func(c *Client) error {
			_, err := c.ClusterReroute(ctx(), []any{map[string]any{"move": map[string]any{"index": "i"}}})
			return err
		}, want: []recReq{{"POST", "/_cluster/reroute", `{"commands":[{"move":{"index":"i"}}]}`}}},
		{name: "clusterGetSettings", run: func(c *Client) error {
			_, err := c.ClusterGetSettings(ctx())
			return err
		}, want: []recReq{{"GET", "/_cluster/settings", ""}}},
		{name: "clusterPutSettings", run: func(c *Client) error {
			_, err := c.ClusterPutSettings(ctx(), map[string]any{"persistent": map[string]any{"k": "v"}})
			return err
		}, want: []recReq{{"PUT", "/_cluster/settings", `{"persistent":{"k":"v"}}`}}},
		{name: "catShards", run: func(c *Client) error {
			_, err := c.CatShards(ctx(), map[string]any{"format": "json"}, "log")
			return err
		}, want: []recReq{{"GET", "/_cat/shards/log*?format=json", ""}}},
		{name: "catRecovery", run: func(c *Client) error {
			_, err := c.CatRecovery(ctx())
			return err
		}, want: []recReq{{"GET", "/_cat/recovery?s=start_time_millis:desc", ""}}},
		{name: "recovery", run: func(c *Client) error {
			_, err := c.Recovery(ctx())
			return err
		}, want: []recReq{{"GET", "/_recovery", ""}}},
		{name: "catNodes", run: func(c *Client) error {
			_, err := c.CatNodes(ctx(), map[string]any{"format": "json"})
			return err
		}, want: []recReq{{"GET", "/_cat/nodes?format=json", ""}}},
		{name: "nodes", run: func(c *Client) error {
			_, err := c.Nodes(ctx())
			return err
		}, want: []recReq{{"GET", "/_nodes", ""}}},
		{name: "catRepositories", run: func(c *Client) error {
			_, err := c.CatRepositories(ctx(), map[string]any{"format": "json"})
			return err
		}, want: []recReq{{"GET", "/_snapshot?format=json", ""}}},
		{name: "catSnapshots", run: func(c *Client) error {
			_, err := c.CatSnapshots(ctx(), "repo1")
			return err
		}, want: []recReq{{"GET", "/_snapshot/repo1/_all", ""}}},
		{name: "snapshotCreateRepository", run: func(c *Client) error {
			_, err := c.SnapshotCreateRepository(ctx(), "repo1", map[string]any{"type": "fs"})
			return err
		}, want: []recReq{{"PUT", "/_snapshot/repo1", `{"type":"fs"}`}}},
		{name: "snapshotDeleteRepository", run: func(c *Client) error {
			_, err := c.SnapshotDeleteRepository(ctx(), "repo1")
			return err
		}, want: []recReq{{"DELETE", "/_snapshot/repo1", ""}}},
		{name: "snapshotCreate", run: func(c *Client) error {
			_, err := c.SnapshotCreate(ctx(), "repo1", "snap1", map[string]any{"indices": "i"})
			return err
		}, want: []recReq{{"PUT", "/_snapshot/repo1/snap1", `{"indices":"i"}`}}},
		{name: "snapshotDelete", run: func(c *Client) error {
			_, err := c.SnapshotDelete(ctx(), "repo1", "snap1")
			return err
		}, want: []recReq{{"DELETE", "/_snapshot/repo1/snap1", ""}}},
		{name: "snapshotRestore", run: func(c *Client) error {
			_, err := c.SnapshotRestore(ctx(), "repo1", "snap1", map[string]any{"indices": "i"})
			return err
		}, want: []recReq{{"POST", "/_snapshot/repo1/snap1/_restore", `{"indices":"i"}`}}},
		{name: "getSnapshot", run: func(c *Client) error {
			_, err := c.GetSnapshot(ctx(), "repo1", "snap1")
			return err
		}, want: []recReq{{"GET", "/_snapshot/repo1/snap1", ""}}},
		{name: "catSlmPolicies", run: func(c *Client) error {
			_, err := c.CatSlmPolicies(ctx())
			return err
		}, want: []recReq{{"GET", "/_slm/policy", ""}}},
		{name: "slmGetPolicy", run: func(c *Client) error {
			_, err := c.SlmGetPolicy(ctx(), "pol1")
			return err
		}, want: []recReq{{"GET", "/_slm/policy/pol1", ""}}},
		{name: "slmPutPolicy", run: func(c *Client) error {
			_, err := c.SlmPutPolicy(ctx(), "pol1", map[string]any{"schedule": "0 0 * * * ?"})
			return err
		}, want: []recReq{{"PUT", "/_slm/policy/pol1", `{"schedule":"0 0 * * * ?"}`}}},
		{name: "slmDeletePolicy", run: func(c *Client) error {
			_, err := c.SlmDeletePolicy(ctx(), "pol1")
			return err
		}, want: []recReq{{"DELETE", "/_slm/policy/pol1", ""}}},
		{name: "slmExecutePolicy", run: func(c *Client) error {
			_, err := c.SlmExecutePolicy(ctx(), "pol1")
			return err
		}, want: []recReq{{"POST", "/_slm/policy/pol1/_execute", ""}}},
		{name: "slmGetStatus", run: func(c *Client) error {
			_, err := c.SlmGetStatus(ctx())
			return err
		}, want: []recReq{{"GET", "/_slm/status", ""}}},
	}

	// security methods: path prefix depends on the cluster version.
	securityCases := []endpointCase{
		{name: "securityUsers", run: func(c *Client) error {
			_, err := c.SecurityUsers(ctx())
			return err
		}, want: []recReq{{"GET", "/user", ""}}},
		{name: "securityRoles", run: func(c *Client) error {
			_, err := c.SecurityRoles(ctx())
			return err
		}, want: []recReq{{"GET", "/role", ""}}},
		{name: "securityRolePut", run: func(c *Client) error {
			_, err := c.SecurityRolePut(ctx(), "role/1", map[string]any{"k": "v"})
			return err
		}, want: []recReq{{"PUT", "/role/role%2F1", `{"k":"v"}`}}},
		{name: "securityRoleDelete", run: func(c *Client) error {
			_, err := c.SecurityRoleDelete(ctx(), "r1")
			return err
		}, want: []recReq{{"DELETE", "/role/r1", ""}}},
		{name: "securityUserPut", run: func(c *Client) error {
			_, err := c.SecurityUserPut(ctx(), "u1", map[string]any{"password": "p"})
			return err
		}, want: []recReq{{"PUT", "/user/u1", `{"password":"p"}`}}},
		{name: "securityUserPutPassword", run: func(c *Client) error {
			_, err := c.SecurityUserPutPassword(ctx(), "u1", map[string]any{"password": "p"})
			return err
		}, want: []recReq{{"PUT", "/user/u1/_password", `{"password":"p"}`}}},
		{name: "securityUserDelete", run: func(c *Client) error {
			_, err := c.SecurityUserDelete(ctx(), "u1")
			return err
		}, want: []recReq{{"DELETE", "/user/u1", ""}}},
	}
	for _, sc := range securityCases {
		for version, prefix := range map[string]string{
			"6.5.0":  "_xpack/security",
			"8.14.1": "_security",
		} {
			cases = append(cases, endpointCase{
				name:    sc.name + " " + version,
				version: version,
				run:     sc.run,
				want:    withPrefix(sc.want, prefix),
			})
		}
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var got []recReq
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				body, _ := io.ReadAll(r.Body)
				got = append(got, recReq{r.Method, r.URL.RequestURI(), string(body)})
				w.Header().Set("Content-Type", "application/json")
				fmt.Fprint(w, `{}`)
			}))
			defer srv.Close()

			c, err := New(srv.URL, Auth{Type: AuthNone})
			if err != nil {
				t.Fatal(err)
			}
			c.Version = tc.version
			if err := tc.run(c); err != nil {
				t.Fatalf("call: %v", err)
			}
			if len(got) != len(tc.want) {
				t.Fatalf("got %d requests, want %d: %+v", len(got), len(tc.want), got)
			}
			for i, w := range tc.want {
				if got[i] != w {
					t.Errorf("request %d:\n got  %+v\n want %+v", i, got[i], w)
				}
			}
		})
	}
}

func ctx() context.Context { return context.Background() }

func indices(n int) []string {
	out := make([]string, n)
	for i := range n {
		out[i] = fmt.Sprintf("i%d", i)
	}
	return out
}

func withPrefix(reqs []recReq, prefix string) []recReq {
	out := make([]recReq, len(reqs))
	for i, r := range reqs {
		out[i] = recReq{r.method, "/" + prefix + r.uri, r.body}
	}
	return out
}
