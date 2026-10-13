# Polish loop 进度（清零观感审查遗留项）

分支：`polish-loop`（自 main=41f237f）。纪律：每项一个改进，全绿才 commit。
来源：观感 loop 审查报告的「留给后续」五项，用户要求全部清零、不留阻塞。

## 清单

- [x] 3xx→黄 与 空 stage→红 语义修正（1xx/3xx 信息性中性色；空 stage 未开始中性色）
- [x] 五个颜色映射 switch 上收 kit：`kit.Level`/`kit.LevelColor` 单点触主题
- [x] `rest.panel()`/`overview.card()` 合一为 `kit.Card`（材质归 kit，Padding/Gap 调用方配）
- [x] 页头 15 处重复抽取 `kit.PageHeader(c, title, caption, trailing)`
- [x] 过渡动画细节：模块页切换/连接成功外壳 8px 上浮淡入、错误态 4px 淡入；mygo 自动尊重系统「减少动态效果」

## 每轮结果

| 轮 | 内容 | 结果 |
|---|---|---|
| 1 | `kit.Level`（Neutral/Success/Warning/Danger）+ `LevelColor`；rest/recovery/shards/snapshots/settings 五个域函数改为返回 Level，不再各自触 Theme；HTTP 1xx/3xx→Neutral、空恢复 stage→Neutral | 260 tests green, vet 干净, d292766 |
| 2 | `kit.Card` 统一 overview.card/rest.panel 的分组卡片材质（Surface+圆角+1px 边），4+4 调用点迁移 | 260 tests green, e34c89c |
| 3 | `kit.PageHeader` 抽取 15 处页头（13 模块 + templates/settings 未连接态），净删 46 行；顺带清掉两处未用变量 | 260 tests green, acaefa5 |
| 4 | 过渡动画：`pageEnter`（模块页 Key 稳定，同页刷新不重放）用于模块切换与连接成功；StateText 错误列 4px 淡入 | 260 tests green, 49b1c03 |

## 总结（循环完成）

五项全部完成；`go build` + `go tool mygo vet` + `CGO_ENABLED=0 go test ./...` 全绿
（260 tests / 20 packages），浅色+深色 11 张截图已刷新。表格行过渡受 mygo List
限制（文档明确行不参与 Transition），属框架边界，非本应用可解。
