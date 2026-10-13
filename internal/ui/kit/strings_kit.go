package kit

// Shared strings used by several modules (module names and generic table /
// button labels). Modules register their own tables on top of these.
func init() {
	RegisterLang(map[string]string{
		"概览":   "Overview",
		"节点":   "Nodes",
		"分片":   "Shards",
		"分片恢复": "Shard Recovery",
		"索引":   "Indices",
		"搜索":   "Search",
		"控制台":  "Console",
		"索引模板": "Index Templates",
		"快照":   "Snapshots",
		"快照仓库": "Snapshot Repositories",
		"设置":   "Settings",
		"用户":   "Users",
		"角色":   "Roles",

		"名称":   "Name",
		"健康":   "Health",
		"状态":   "Status",
		"刷新":   "Refresh",
		"关闭":   "Close",
		"打开":   "Open",
		"删除…":  "Delete…",
		"删除":   "Delete",
		"查看设置": "View settings",
		"保存":   "Save",
		"取消":   "Cancel",
		"操作":   "Actions",
	})
}
