package rest

import "seismic/internal/ui/kit"

func init() {
	kit.RegisterLang(map[string]string{
		"新建标签": "New tab",
		"关闭标签": "Close tab",
		"直接调用集群 REST API": "Call the cluster REST API directly",
		"重新发送": "Resend",
		"填入标签": "Load into tab",
		"路径":   "Path",
		"发送":   "Send",
		"{0} 请求不带正文；参数写在路径里，如 _cat/indices?v=true": "{0} sends no body; put params in the path, e.g. _cat/indices?v=true",
		"查询名称":        "Query name",
		"保存查询":        "Save query",
		"发送中…":        "Sending…",
		"还没发过请求":      "No request sent yet",
		"耗时 {0} ms":   "{0} ms",
		"历史":          "History",
		"清空历史":        "Clear history",
		"发过的请求会出现在这里": "Requests you send appear here",
		"保存的查询":       "Saved queries",
		"保存常用请求，一键再发": "Save frequent requests to resend in one click",
		"删除":          "Delete",
		"响应 {0} 超过 1 MB，跳过格式化，原样显示": "Response {0} exceeds 1 MB; shown raw without formatting",
	})
}
