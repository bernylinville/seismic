# 0004 · 凭据存 macOS Keychain（security CLI），元数据存 JSON

日期：2026-10-12 · 状态：已接受

## 背景

elasticvue 把集群凭据明文存浏览器 localStorage。mygo 无 Keychain API（有 PathUserData/SQLite）。用户在做集中凭据治理，明文倒退不可接受。候选：

1. 明文 JSON（对齐 elasticvue）
2. SQLite 存全部
3. Keychain 存敏感凭据 + JSON 存连接元数据

## 决策

选 3：密码/API key/AWS secret 通过 `security add-generic-password/find-generic-password/delete-generic-password` 存取（服务名 `com.bernylinville.seismic`，account = connection UUID）；连接元数据（名称、URI、AuthType、用户名、region 等）存 `PathUserData/connections.json`。

## 后果

- 非 macOS 平台 keychain 退化为文件存 `PathUserData/credentials.json`（0600），构建标签隔离；macOS 是验收平台。
- `security` CLI 每次调用有子进程开销：连接建立时读一次、保存时写一次，频率可忽略。
- 备份/迁移连接时凭据不随 connections.json 走，属预期行为。
