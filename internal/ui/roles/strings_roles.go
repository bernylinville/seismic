package roles

import "seismic/internal/ui/kit"

func init() {
	kit.RegisterLang(map[string]string{
		"描述":      "Description",
		"索引权限":    "Index privileges",
		"集群权限":    "Cluster privileges",
		"{0} 个角色": "{0} roles",
		"筛选角色…":   "Filter roles…",
		"新建角色":    "New role",
		"还没有角色":   "No roles yet",
		"· JSON":  "· JSON",
		"OpenSearch 不支持原生用户/角色管理": "OpenSearch does not support native user/role management",
		"编辑角色":                           "Edit role",
		"名称不能为空":                         "Name is required",
		"必须是 JSON 数组":                    "Must be a JSON array",
		"逗号分隔，如 all, monitor":            "Comma-separated, e.g. all, monitor",
		"代替的用户，逗号分隔":                     "Run-as users, comma-separated",
		"JSON 数组，每项含 names 与 privileges": "JSON array; each entry has names and privileges",
		"，可用 allow_restricted_indices":   "; allow_restricted_indices available",
		"删除角色":                           "Delete role",
		"删了就恢复不了，确定删 {0}？":               "Delete {0}? This can't be undone.",
		"已创建角色 {0}":                      "Created role {0}",
		"创建角色 {0}失败：{1}":                 "Creating role {0} failed: {1}",
		"已更新角色 {0}":                      "Updated role {0}",
		"更新角色 {0}失败：{1}":                 "Updating role {0} failed: {1}",
		"已删除角色 {0}":                      "Deleted role {0}",
		"删除角色 {0}失败：{1}":                 "Deleting role {0} failed: {1}",
	})
}
