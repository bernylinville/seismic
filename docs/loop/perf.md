# Perf loop 进度（大集群档：5k–50k 索引、10k hits、MB 级 JSON）

分支：`perf-loop`。纪律：每轮一个独立改进，`go build ./... && go test ./...` 全绿才 commit。

## 待办（按序）

- [x] 0. 升级 mygo v0.4.5→最新（go get @latest；vet；migrate-ui 有提示则执行；全测试）
- [x] 1. internal/es/client.go 响应护栏：Request 的 io.ReadAll 加 io.LimitReader（64MB 起步），超限清晰报错
- [x] 2. 减拷贝：json.Decoder 直解 resp.Body，消除 ReadAll+Unmarshal 双份内存
- [x] 3. 表格过滤/排序缓存：indices_state.go 与 shards 的 visibleRows() 改脏标记，数据变化时算一次
- [x] 4. 编辑器防护：文档大小上限、Validate 防抖、超大文档降级提示
- [x] 5. REST 控制台大响应：>1MB 跳过全量 json.Indent，分块或按需格式化
- [x] 6. 请求取消：UI 层传真实 ctx，切页/切连接/重查询时取消在途请求
- [x] 7. mapping 拉取异步化 + 按 pattern 缓存，移出搜索点击同步路径（ui/search/search.go:352-411）
- [x] 8. 造数+benchmark：es8 灌 5k 索引 + 10k hits；Go benchmark（visibleRows、tokenizer、client 解码）+ smoke 阈值断言

## 已完成

- [x] 基线收尾（round 0）：`go test ./...` 238 通过；i18n/kit.Tr 迁移 + skills 配置已 commit（ad53111）

## 每轮结果

| 轮 | 内容 | 结果 |
|---|---|---|
| 0 | 基线提交 | 238 tests green, commit ad53111 |
| 0' | mygo v0.4.5→v0.4.8，migrate-ui 迁移 editor.go Key API（4 处） | vet 干净, 238 tests green, commit 45a7211 |
| 1 | 响应护栏：maxResponseBytes=64MB，LimitReader(lim+1) 探测超限，错误提示收窄请求；边界测试 ×2 | 240 tests green, commit c993bb0 |
| 2 | RequestJSON 流式直解（计数 limitReader，超限检查先于解码错误）；Into 变体 ×17；迁移全部 25 个 resp.JSON/resp.Body 站点（search/indices/shards/nodes/recovery/snapshots/repos/overview/deps/roles/users/settings/templates/conn）；顺带消除请求体发送时的整串拷贝 | 244 tests green, vet 干净, commit f16f15c |
| 3 | visibleRows 缓存：输入快照比对（filter/hide/sort 值比较 + rows 切片头身份比较），indices/shards/nodes 三表同改（nodes 与任务点名的两处同模式）；每帧 2-3 次调用只算一次；缓存复用/失效测试 ×3 | 247 tests green, vet 干净, commit ac12995 |
| 4 | 编辑器三层防护：8MiB 硬上限（SetText 拒载/插入拒增长+状态栏提示）；1MiB 降级（停逐键校验/高亮/补全，打开时强制校验一次）；64KiB 起防抖（200ms 一次）。框架限制记录：v0.4.8 文本输入绑定为 string 每帧物化，buffer 版 TextAreaBuffer 需整体替换自定义编辑器 | 251 tests green, vet 干净, commit 2ae6e5f |
| 5 | REST 大响应分级：>1MiB 跳过全量 json.Indent（省 1.5-2× 缩进副本），原样显示 + 双语提示行；小响应照常缩进；lang 主线程捕获避免 worker 竞态 | 252 tests green, vet 干净, commit 4fc120e |
| 6 | Load(key, mark, fn(ctx))：33 站点 15 模块全迁移；同 key 单飞取消（重查询不堆积）+ 切连接 cancelAllLoads；被取代结果丢弃、被取消结果以「已取消」落地；注册表互斥锁保护。设计取舍：切页不取消（模块状态持久，返回时数据已就绪；在途量由 per-key 槽位约束）。既有问题记录：无窗口测试模式 a.update 内联在 worker 跑，race detector 会报（生产走 Window.Update 主线程，非本轮引入） | 254 tests green, vet 干净, commit f0fb5f4 |
| 7 | mapping 按 cluster+pattern 缓存（16 槽插入序逐出）：切回旧 pattern 即时命中；集群维度键隔离防串集群；失败清 mappedKey 可重试；换 pattern 自动取消在途 wildcard 拉取（复用轮 6 槽位）。注：拉取本就经 h.Load 在 worker，本轮消掉的是重复的 MB 级 _mapping 往返 | 255 tests green, vet 干净, commit 11d6854 |
| 8 | 造数+benchmark 体系：`cmd/perfseed`（5k 轻索引 + 20k 字段宽 mapping（_mapping 响应 0.5MB）+ 10k hits；ES8 三个坑全踩平后幂等：`destructive_requires_name` 改显式名批量删除、索引模板 1 主 0 副防 `max_shards_per_node` 配额占满、`total_fields.limit` 按字段数放宽且样例文档只用已映射字段）；benchmark ×6：visibleRows 冷 114µs/147KB vs 缓存命中 4.5ns/0B、Tokenize1MB 19.3ms、Validate1MB 27.4ms、RequestJSON 10k hits 18ms/8.7MB/op vs RequestRaw 2.2ms/4.7MB/op；阈值断言 ×4：5k 索引首屏 <2s（实测 0.43s）、10k hits 解码+建表 <2s、3MB JSON 打开 <1s（实测 0.02s）、流式分配 ≤5×payload（实测 4.2×，原始断言「流式≈裸 Unmarshal」被实测推翻：json.Decoder 内部缓冲倍增累计 ≈2×payload，TotalAlloc 上流式必然高于 Unmarshal，改为锁「无叠加整份拷贝」的诚实预算）；env 门控真实集群 acceptance（SEISMIC_ES8*，无 env 自动跳过）对 docker es8 实跑：5k cat/10k hits/宽 mapping 合计 0.34s，阈值 2s/3s/2s 全过。live 验证备注：宿主 9200 被占用，es8 经 `!override` 端口覆盖起在 9299（堆 1g） | 259 tests green, vet 干净, 同提交 |

## 总结（循环完成）

清单 0–8 全部完成，阈值断言与 `go test ./...` 全绿（259 tests / 19 packages），循环到此停止。

**护栏**（防大集群打爆）
- 64MB 响应上限，Request 与 RequestJSON 双路径探测超限、报错收窄请求范围
- 编辑器三层：8MiB 硬上限（拒载/拒插入）、1MiB 降级（停逐键校验/高亮/补全）、64KiB 起 200ms 防抖
- REST 控制台 >1MiB 跳过全量 json.Indent，原样显示 + 双语提示

**减拷贝/减重复**
- json.Decoder 流式直解 resp.Body，Into 变体 ×17 覆盖全部 25 个响应站点
- visibleRows 输入快照缓存（indices/shards/nodes 三表）：命中 4.5ns/0 alloc vs 冷 114µs
- mapping 按 cluster+pattern 缓存（16 槽逐出），切回即时命中

**取消语义**
- `Load(key, mark, fn(ctx))` 全 33 站点：同 key 单飞取消旧请求、切连接 cancelAllLoads、被取代结果丢弃、被取消以「已取消」落地；切页不取消（模块状态持久的设计取舍）

**回归体系**
- `cmd/perfseed` 幂等造数 + benchmark ×6 + smoke 阈值断言 ×4 + env 门控真实集群 acceptance
- 复现：`docker compose -f docker/compose.yml up -d` → `go run ./cmd/perfseed` → `SEISMIC_ES8=... go test ./internal/es/ -run TestPerfAcceptance -count=1`

**框架层限制（app 不可解，已记录）**
- mygo v0.4.8 文本输入绑定为 string 每帧物化；buffer 版需整体替换自定义编辑器（轮 4）
- 无窗口测试模式 a.update 内联 worker，race detector 对既有测试误报；生产走 Window.Update 主线程（轮 6）

**实测教训**
- json.Decoder 流式的 TotalAlloc 并不低于裸 Unmarshal（内部缓冲倍增 ≈2×payload）；流式的价值在峰值与守卫边界，不在累计分配——断言须按此设计（轮 8）

