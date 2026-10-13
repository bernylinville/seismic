# 术语表（GLOSSARY）

本文件只放领域词汇，不放实现细节。术语在设计与代码中冲突时，以本表为准并立即修订。

## 连接与集群

- **Connection（连接）**：用户保存的一个 ES 集群入口——名称、URI、认证方式与凭据引用。切换连接即切换全局上下文。
- **Cluster（集群）**：一个 Connection 连上后呈现的真实 Elasticsearch/OpenSearch 集群，有版本、健康、UUID。一个 Cluster 对应一个 Connection；Connection 未连通时只有元数据。
- **AuthType（认证方式)**：`none | basic | apiKey | awsIAM` 四选一。凭据本体存 Keychain，Connection 元数据只存引用。
- **Distribution（发行版）**：`elasticsearch | opensearch`，由 ping 响应推断；决定 API 分支。
- **BuildFlavor（构建风味）**：serverless 等特殊部署标记，限制可用路由。

## 数据对象

- **Index（索引）**：ES 文档存储单元。健康取集群分片视角（green/yellow/red）。
- **Index Template（索引模板）**：新建索引的模板（legacy 与 composable 两种；UI 分别称「旧版模板」「新版模板」）。
- **Shard（分片）**：索引的分片，主分片（primary）与副本（replica）；分片有未分配（unassigned）状态。
- **Shard Recovery（分片恢复）**：分片数据恢复事件（store/replica/snapshot 等）。
- **Node（节点）**：集群成员，有角色、堆内存、磁盘水位。
- **Snapshot（快照）**：某时刻索引数据的备份，属某个 Repository。
- **Repository（快照仓库）**：快照存储位置（fs/s3/url…）。
- **Mapping（映射）**：索引字段定义；编辑器智能补全的数据来源。

## 操作

- **Search（搜索）**：对索引发 Query DSL 查询并浏览结果。
- **REST Console（控制台）**：任意方法+路径+body 的原始请求工具；有 Tab、历史、保存的查询，按集群独立。UI 名「控制台」，不再带 REST 前缀。
- **Query History（查询历史）**：REST 控制台发出的请求记录，自动记录，按集群分区存 SQLite。
- **Saved Query（保存的查询）**：用户显式命名的 REST 请求。

## 版本与兼容

- **Version Branch（版本分支）**：按集群 major/minor 版本切换 API 形状的行为（如 `_security` vs `_xpack/security`）。
- **SLM**：Snapshot Lifecycle Management，定时自动做快照；UI 称「自动快照」。ES ≥ 7.4 才有。

## UI

- **Module（模块）**：侧栏导航的功能单元，共 13 个：概览、索引、索引模板、设置、分片、分片恢复、节点、搜索、控制台、快照仓库、快照、用户、角色。
- **Health Dot（健康点）**：绿/黄/红三色圆点，全 app 唯一的健康语义色。
