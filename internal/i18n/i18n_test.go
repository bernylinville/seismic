package i18n

import (
	"sort"
	"testing"
)

func TestKeySetsIdentical(t *testing.T) {
	var enKeys, zhKeys []string
	for k := range en {
		enKeys = append(enKeys, k)
	}
	for k := range zh {
		zhKeys = append(zhKeys, k)
	}
	sort.Strings(enKeys)
	sort.Strings(zhKeys)

	if len(enKeys) != len(zhKeys) {
		t.Fatalf("key count mismatch: en=%d zh=%d", len(enKeys), len(zhKeys))
	}
	for i := range enKeys {
		if enKeys[i] != zhKeys[i] {
			t.Fatalf("key mismatch at %d: en=%q zh=%q", i, enKeys[i], zhKeys[i])
		}
	}

	// Every registered language map must have the full key set too.
	for lang, m := range maps {
		if len(m) != len(enKeys) {
			t.Errorf("lang %q has %d keys, want %d", lang, len(m), len(enKeys))
		}
		for _, k := range enKeys {
			if m[k] == "" {
				t.Errorf("lang %q missing key %q", lang, k)
			}
		}
	}
}

func TestRequiredKeys(t *testing.T) {
	required := []string{
		"nav.overview", "nav.indices", "nav.indexTemplates", "nav.settings",
		"nav.shards", "nav.shardRecovery", "nav.nodes", "nav.search", "nav.rest",
		"nav.snapshotRepositories", "nav.snapshots", "nav.users", "nav.roles",
		"sidebar.cluster", "sidebar.data", "sidebar.ops", "sidebar.security",
		"toolbar.filterIndices", "toolbar.refresh", "toolbar.settings", "toolbar.clusterSelector",
		"status.health", "status.docs", "status.size", "status.shards",
		"status.unassigned", "status.updatedAgo",
		"conn.name", "conn.uri", "conn.authType", "conn.username", "conn.password",
		"conn.apiKey", "conn.region", "conn.accessKey", "conn.secretKey",
		"conn.sessionToken", "conn.test", "conn.save", "conn.delete",
		"conn.connecting", "conn.success", "conn.failed",
		"common.create", "common.delete", "common.edit", "common.close", "common.open",
		"common.refresh", "common.cancel", "common.confirm", "common.search",
		"common.format", "common.copy", "common.export",
		"welcome.title", "welcome.subtitle", "welcome.addFirst",
		"rest.newTab", "rest.history", "rest.saved", "rest.send",
		"rest.formatJson", "rest.response", "rest.statusCode",
		"search.query", "search.results", "search.indices", "search.size", "search.from",
	}
	for _, k := range required {
		if _, ok := en[k]; !ok {
			t.Errorf("en missing required key %q", k)
		}
		if _, ok := zh[k]; !ok {
			t.Errorf("zh missing required key %q", k)
		}
	}
}

func TestPlaceholderSubstitution(t *testing.T) {
	cases := []struct {
		lang Lang
		key  string
		args []any
		want string
	}{
		{En, "status.updatedAgo", []any{"5s"}, "Updated 5s ago"},
		{Zh, "status.updatedAgo", []any{"5秒"}, "5秒前更新"},
		{En, "missing.key", []any{"x"}, "missing.key"},
	}
	for _, c := range cases {
		if got := T(c.lang, c.key, c.args...); got != c.want {
			t.Errorf("T(%q, %q, %v) = %q, want %q", c.lang, c.key, c.args, got, c.want)
		}
	}

	// Multiple placeholders and non-string args via a synthetic message.
	if got := substitute("{0}-{1}-{2}-{3}", 1, "a", 2.5, true); got != "1-a-2.5-true" {
		t.Errorf("substitute multi-arg = %q", got)
	}
	// Out-of-range and malformed placeholders are left as-is.
	if got := substitute("{0} {5} {x} {}", "A"); got != "A {5} {x} {}" {
		t.Errorf("substitute edge = %q", got)
	}
	// Unclosed brace kept verbatim.
	if got := substitute("{0", "A"); got != "{0" {
		t.Errorf("substitute unclosed = %q", got)
	}
}

func TestFallback(t *testing.T) {
	const missing = "no.such.key"
	if got := T(Zh, missing); got != missing {
		t.Errorf("T(Zh, missing) = %q, want key itself", got)
	}
	if got := T(En, missing); got != missing {
		t.Errorf("T(En, missing) = %q, want key itself", got)
	}
	// Unknown language falls back to en map.
	if got := T(Lang("fr"), "nav.indices"); got != en["nav.indices"] {
		t.Errorf("T(fr, nav.indices) = %q, want %q", got, en["nav.indices"])
	}
}

func TestTranslationsPresent(t *testing.T) {
	for k, v := range en {
		if v == "" {
			t.Errorf("en key %q has empty value", k)
		}
		if zh[k] == "" {
			t.Errorf("zh key %q has empty value", k)
		}
		if v == zh[k] && k != "welcome.title" && k != "nav.rest" {
			// Not a hard error; flag for review.
			t.Logf("note: en and zh identical for %q: %q", k, v)
		}
	}
}
