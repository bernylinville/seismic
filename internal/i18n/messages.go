package i18n

import "fmt"

// formatArg renders an argument for placeholder substitution.
func formatArg(a any) string {
	switch v := a.(type) {
	case string:
		return v
	case fmt.Stringer:
		return v.String()
	default:
		return fmt.Sprint(v)
	}
}

var en = map[string]string{
	// navigation
	"nav.overview":             "Overview",
	"nav.indices":              "Indices",
	"nav.indexTemplates":       "Index Templates",
	"nav.settings":             "Settings",
	"nav.shards":               "Shards",
	"nav.shardRecovery":        "Shard Recovery",
	"nav.nodes":                "Nodes",
	"nav.search":               "Search",
	"nav.rest":                 "REST",
	"nav.snapshotRepositories": "Snapshot Repositories",
	"nav.snapshots":            "Snapshots",
	"nav.users":                "Users",
	"nav.roles":                "Roles",

	// sidebar groups
	"sidebar.cluster":  "Cluster",
	"sidebar.data":     "Data",
	"sidebar.ops":      "Operations",
	"sidebar.security": "Security",

	// toolbar
	"toolbar.filterIndices":   "Filter indices",
	"toolbar.refresh":         "Refresh",
	"toolbar.settings":        "Settings",
	"toolbar.clusterSelector": "Cluster",

	// status
	"status.health":     "Health",
	"status.docs":       "Docs",
	"status.size":       "Size",
	"status.shards":     "Shards",
	"status.unassigned": "Unassigned",
	"status.updatedAgo": "Updated {0} ago",

	// connection
	"conn.name":         "Name",
	"conn.uri":          "URI",
	"conn.authType":     "Auth type",
	"conn.username":     "Username",
	"conn.password":     "Password",
	"conn.apiKey":       "API key",
	"conn.region":       "Region",
	"conn.accessKey":    "Access key",
	"conn.secretKey":    "Secret key",
	"conn.sessionToken": "Session token",
	"conn.test":         "Test",
	"conn.save":         "Save",
	"conn.delete":       "Delete",
	"conn.connecting":   "Connecting…",
	"conn.success":      "Connected!",
	"conn.failed":       "Connection failed",

	// common
	"common.create":  "Create",
	"common.delete":  "Delete",
	"common.edit":    "Edit",
	"common.close":   "Close",
	"common.open":    "Open",
	"common.refresh": "Refresh",
	"common.cancel":  "Cancel",
	"common.confirm": "Confirm",
	"common.search":  "Search",
	"common.format":  "Format",
	"common.copy":    "Copy",
	"common.export":  "Export",

	// welcome
	"welcome.title":    "Seismic",
	"welcome.subtitle": "A native macOS client for Elasticsearch and OpenSearch",
	"welcome.addFirst": "Add your first connection to get started",

	// rest
	"rest.newTab":     "New tab",
	"rest.history":    "History",
	"rest.saved":      "Saved",
	"rest.send":       "Send",
	"rest.formatJson": "Format JSON",
	"rest.response":   "Response",
	"rest.statusCode": "Status code",

	// search
	"search.query":   "Query",
	"search.results": "Results",
	"search.indices": "Indices",
	"search.size":    "Size",
	"search.from":    "From",
}

var zh = map[string]string{
	// navigation
	"nav.overview":             "概览",
	"nav.indices":              "索引",
	"nav.indexTemplates":       "索引模板",
	"nav.settings":             "设置",
	"nav.shards":               "分片",
	"nav.shardRecovery":        "分片恢复",
	"nav.nodes":                "节点",
	"nav.search":               "搜索",
	"nav.rest":                 "REST",
	"nav.snapshotRepositories": "快照仓库",
	"nav.snapshots":            "快照",
	"nav.users":                "用户",
	"nav.roles":                "角色",

	// sidebar groups
	"sidebar.cluster":  "集群",
	"sidebar.data":     "数据",
	"sidebar.ops":      "运维",
	"sidebar.security": "安全",

	// toolbar
	"toolbar.filterIndices":   "筛选索引",
	"toolbar.refresh":         "刷新",
	"toolbar.settings":        "设置",
	"toolbar.clusterSelector": "集群",

	// status
	"status.health":     "健康状态",
	"status.docs":       "文档数",
	"status.size":       "大小",
	"status.shards":     "分片",
	"status.unassigned": "未分配",
	"status.updatedAgo": "{0}前更新",

	// connection
	"conn.name":         "名称",
	"conn.uri":          "地址",
	"conn.authType":     "认证方式",
	"conn.username":     "用户名",
	"conn.password":     "密码",
	"conn.apiKey":       "API 密钥",
	"conn.region":       "区域",
	"conn.accessKey":    "Access Key",
	"conn.secretKey":    "Secret Key",
	"conn.sessionToken": "会话令牌",
	"conn.test":         "测试",
	"conn.save":         "保存",
	"conn.delete":       "删除",
	"conn.connecting":   "连接中…",
	"conn.success":      "连接成功！",
	"conn.failed":       "连接失败",

	// common
	"common.create":  "创建",
	"common.delete":  "删除",
	"common.edit":    "编辑",
	"common.close":   "关闭",
	"common.open":    "打开",
	"common.refresh": "刷新",
	"common.cancel":  "取消",
	"common.confirm": "确认",
	"common.search":  "搜索",
	"common.format":  "格式化",
	"common.copy":    "复制",
	"common.export":  "导出",

	// welcome
	"welcome.title":    "Seismic",
	"welcome.subtitle": "原生 macOS Elasticsearch / OpenSearch 客户端",
	"welcome.addFirst": "添加第一个连接以开始",

	// rest
	"rest.newTab":     "新建标签页",
	"rest.history":    "历史",
	"rest.saved":      "已保存",
	"rest.send":       "发送",
	"rest.formatJson": "格式化 JSON",
	"rest.response":   "响应",
	"rest.statusCode": "状态码",

	// search
	"search.query":   "查询",
	"search.results": "结果",
	"search.indices": "索引",
	"search.size":    "条数",
	"search.from":    "起始位置",
}
