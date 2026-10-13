package users

import "seismic/internal/ui/kit"

func init() {
	kit.RegisterLang(map[string]string{
		"用户名":     "Username",
		"角色":      "Roles",
		"全名":      "Full name",
		"邮箱":      "Email",
		"启用":      "Enabled",
		"系统保留":    "Reserved",
		"已弃用":     "Deprecated",
		"{0} 个用户": "{0} users",
		"筛选用户…":   "Filter users…",
		"新建用户":    "New user",
		"还没有用户":   "No users yet",
		"编辑…":     "Edit…",
		"修改密码…":   "Change password…",
		"是":       "Yes",
		"否":       "No",
		"OpenSearch 不支持原生用户/角色管理": "OpenSearch does not support native user/role management",
		"编辑用户":                "Edit user",
		"用户名不能为空":             "Username is required",
		"密码不能为空":              "Password is required",
		"至少选一个角色":             "Pick at least one role",
		"逗号分隔，如 kibana_admin": "Comma-separated, e.g. kibana_admin",
		"启用该用户":               "Enable this user",
		"留空则不修改密码":            "Leave empty to keep the password",
		"修改密码":                "Change password",
		"新密码":                 "New password",
		"修改":                  "Change",
		"删了就恢复不了，确定删 {0}？":    "Delete {0}? This can't be undone.",
		"已创建用户 {0}":           "Created user {0}",
		"创建用户 {0}失败：{1}":      "Creating user {0} failed: {1}",
		"已更新用户 {0}":           "Updated user {0}",
		"更新用户 {0}失败：{1}":      "Updating user {0} failed: {1}",
		"已修改 {0} 的密码":         "Changed password of {0}",
		"修改 {0} 的密码失败：{1}":    "Changing password of {0} failed: {1}",
		"已删除用户 {0}":           "Deleted user {0}",
		"删除用户 {0}失败：{1}":      "Deleting user {0} failed: {1}",
	})
}
