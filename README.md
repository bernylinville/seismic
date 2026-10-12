# Seismic

macOS 原生 Elasticsearch 客户端，[mygo](https://mygo.egoist.dev) 纯 Go 自绘 native UI。elasticvue 的原生重写，功能对等（13 模块），MIT。

## 特性

- 纯 Go、无 cgo、无 webview：Metal 自绘 UI，即开即用
- `net/http` 直连 Elasticsearch（7/8/9）、OpenSearch、serverless，无 CORS
- 认证：none / basic / API key / AWS IAM（手写 SigV4）
- 凭据入 macOS Keychain，连接元数据本地 JSON
- REST 历史 / 保存查询 / 表格偏好入 SQLite（按集群分区）
- JSON 编辑器：语法高亮、格式化、校验、结构补全、mapping 智能补全
- 中英双语

## 开发

```sh
go tool mygo dev      # 开发（改 .go 热重启）
go test ./...         # 全部测试（view 测试无窗口）
go tool mygo build    # 打包当前平台
go tool mygo build -platform darwin/universal   # 交叉出 .app/.dmg
```

## 目录

- `internal/es` — Elasticsearch REST 客户端（认证、版本分支、全端点）
- `internal/conn` — 连接存储（JSON 元数据 + Keychain 凭据）
- `internal/db` — SQLite 本地数据
- `internal/i18n` — 中英文案
- `internal/ui` — 各模块视图
- `GLOSSARY.md`、`docs/adr/` — 领域词汇与架构决策

## 致谢

源自 [elasticvue](https://github.com/elasticvue/elasticvue)（MIT, © 2017 Carsten König）。
