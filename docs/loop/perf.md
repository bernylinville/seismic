# Perf loop 进度（大集群档：5k–50k 索引、10k hits、MB 级 JSON）

分支：`perf-loop`。纪律：每轮一个独立改进，`go build ./... && go test ./...` 全绿才 commit。

## 待办（按序）

- [x] 0. 升级 mygo v0.4.5→最新（go get @latest；vet；migrate-ui 有提示则执行；全测试）
- [ ] 1. internal/es/client.go 响应护栏：Request 的 io.ReadAll 加 io.LimitReader（64MB 起步），超限清晰报错
- [ ] 2. 减拷贝：json.Decoder 直解 resp.Body，消除 ReadAll+Unmarshal 双份内存
- [ ] 3. 表格过滤/排序缓存：indices_state.go 与 shards 的 visibleRows() 改脏标记，数据变化时算一次
- [ ] 4. 编辑器防护：文档大小上限、Validate 防抖、超大文档降级提示
- [ ] 5. REST 控制台大响应：>1MB 跳过全量 json.Indent，分块或按需格式化
- [ ] 6. 请求取消：UI 层传真实 ctx，切页/切连接/重查询时取消在途请求
- [ ] 7. mapping 拉取异步化 + 按 pattern 缓存，移出搜索点击同步路径（ui/search/search.go:352-411）
- [ ] 8. 造数+benchmark：es8 灌 5k 索引 + 10k hits；Go benchmark（visibleRows、tokenizer、client 解码）+ smoke 阈值断言

## 已完成

- [x] 基线收尾（round 0）：`go test ./...` 238 通过；i18n/kit.Tr 迁移 + skills 配置已 commit（ad53111）

## 每轮结果

| 轮 | 内容 | 结果 |
|---|---|---|
| 0 | 基线提交 | 238 tests green, commit ad53111 |
| 0' | mygo v0.4.5→v0.4.8，migrate-ui 迁移 editor.go Key API（4 处） | vet 干净, 238 tests green, commit 45a7211 |
