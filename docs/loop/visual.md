# Visual loop 进度（原生观感重构：macOS 26/27 激进追新）

分支：`visual-loop`。纪律：每轮一个模块或一条方向线，`go build ./... && go test ./...` 全绿才 commit；遵循 mygo-maintenance skill；每轮开始先确认 mygo 已是最新版本。
截图：`SHOTS_DIR=../es-native-design/shots/ui go test -run TestRenderSmokeScreens .`（浅色+深色）。
实机包：每完成 3 个模块 `go tool mygo build -platform darwin/universal` → `build/darwin-universal/Seismic.app`（用户 rsync 到 Mac 目检，批量反馈不阻塞循环）。

## 方向（已拍板）

- **材质**：main.go 开启 `WindowOptions.Vibrancy`；侧边栏 vibrancy + 标题栏材质融合
- **配色**：删除 teal accent 覆盖，完全跟随系统 accent/深色模式；健康色语义保留、色值向系统观感校准并去重
- **控件/字体/间距**：清理硬编码 FontSize/pixel padding → 主题 metrics；toolbar 与标题栏融合；密度向系统 App 看齐
- **交互**：右键菜单用系统 ContextMenu、常用快捷键、过渡动画细节
- **mygo 0.4.8+ 新能力可用则用**

## 清单（13 模块 × 5 维度 + 壳层）

| 模块 | 材质 | 配色 | 控件 | 间距 | 交互 |
|---|---|---|---|---|---|
| 壳层（app/main/kit） | ✅ | ✅ | ✅ | ✅ | ✅ |
| overview | ✅ | ✅ | ✅ | ✅ | ✅ |
| indices | ✅ | ✅ | ✅ | ✅ | ✅ |
| search | ✅ | ✅ | ✅ | ✅ | ✅ |
| rest | ✅ | ✅ | ✅ | ✅ | ✅ |
| nodes | ✅ | ✅ | ✅ | ✅ | ✅ |
| shards | ✅ | ✅ | ✅ | ✅ | ✅ |
| recovery | ✅ | ✅ | ✅ | ✅ | ✅ |
| snapshots | ✅ | ✅ | ✅ | ✅ | ✅ |
| repositories | ✅ | ✅ | ✅ | ✅ | ✅ |
| templates | ✅ | ✅ | ✅ | ✅ | ✅ |
| settings | ✅ | ✅ | ✅ | ✅ | ✅ |
| users | ✅ | ✅ | ✅ | ✅ | ✅ |
| roles | ✅ | ✅ | ✅ | ✅ | ✅ |

## 每轮结果

| 轮 | 内容 | 结果 |
|---|---|---|
| 1 | 壳层材质+配色：main.go 开启 `VibrancySidebar`（app.go 既有条件分支首次生效，welcome 页也坐材质上）；删 teal accent 覆盖（app.go 156-159），完全跟随系统 accent；健康色去重——app.go `healthColor`/deps.go `healthDotColor`（死代码）删除，`kit.HealthColor` 单源改映射主题语义色 `Success/Warning/Danger`（自动跟随深浅模式），connect 表单测试结果色与 kit.StateText 错误色同步换语义色 | 259 tests green, vet 干净 |
| 2 | overview 全五维：kit 新增字号阶梯 `SizeCaption/SizeBody/SizeTitle/SizeLarge`（Rem 相对推导，跟随系统字体设置，对齐 macOS 11/13/17/26）；四张卡抽 `card()` helper——Surface 填充 + Space(3) 圆角 + 1px 边（系统设置分组卡片观感）；全部 pixel padding/gap 换 `t.Space(n)`；标题 19→17、正文 12→13、caption 12→11 对齐系统字阶；⌘R 刷新快捷键（页内作用域） | 259 tests green, vet 干净 |
| 3 | indices 全五维：表格单元格去掉 8 处 `FontSize(12)` 走主题默认 13；操作错误色 `#d64545`→`t.Danger`；页头/表格/详情面板 padding 全换 `t.Space(n)`、页头标题/计数走字阶；健康点 7px→Space(1.75)；⌘R 刷新；顺带删 support.go 的 `stateText`/`errMessage`（kit.StateText/kit.ErrMessage 的重复实现，含最后一处壳层硬编码红） | 259 tests green, vet 干净 |
| 4 | search 全五维：页头对齐全局模式（裸 20px 标题+整页 Padding(16) → 标题 17+副标题 caption+标准页头/内容边距）；3 处错误红→`t.Danger`；表格 4 处 FontSize(12) 拆除走默认；查询区/结果区 gap 全换 Space；编辑器容器圆角 6→`t.Radius`；详情面板走 Space + mono 字号新增 `kit.SizeMono`（代码文本统一定义）；⌘R/⌘↩ 均重第一页跑搜索（新增副标题字符串 zh/en） | 259 tests green, vet 干净 |
| 5 | rest 全五维：页头对齐全局模式（副标题「直接调用集群 REST API」）；请求/响应/历史/保存查询四个面板抽 `panel()` helper 换 overview 同款卡片材质（Surface+Space(3) 圆角）；`statusColor` 硬编码健康色→主题语义（Success/Warning/Danger，需 *ui.Theme 参数）；响应正文 mono 走 `kit.SizeMono`；历史行加右键菜单（重新发送/填入标签）；⌘↩ 发送、⌘T 新建标签 | 259 tests green, vet 干净 |
| 6 | nodes 全五维：与 indices 同构同套路——页头/表格/详情全换 Space 与字阶；表格 9 处 FontSize(12) 拆除；详情错误红→`t.Danger`；⌘R；详情 JSON 与 indices 详情统一为 mono+`kit.SizeMono`（两处 JSON 面板观感一致） | 259 tests green, vet 干净 |
| 7 | shards 全五维：`stateColor` 硬编码→主题语义（STARTED Success/UNASSIGNED Danger/其余 Warning，加 *ui.Theme 参数）；页头/表格/reroute 条全换 Space 与字阶；表格 8 处 FontSize(12) 拆除；状态点 7px→Space(1.75)；操作错误→`t.Danger`；⌘R | 259 tests green, vet 干净 |
| 8 | recovery 全五维：`stageColor` 硬编码→主题语义（DONE Success/失败 Danger/进行中 Warning）；页头/表格全换 Space 与字阶；表格 7 处 FontSize(12) 拆除；阶段点→Space(1.75)；⌘R | 259 tests green, vet 干净 |
| 9 | snapshots 全五维：`stateColor`/`failColor` 硬编码→主题语义（加 *ui.Theme 参数）；页头/表格/详情/SLM 区/三个对话框按钮行全换 Space 与字阶；表格+SLM 表共 9 处 FontSize(12) 拆除；6 处错误文案→`t.Danger`；⌘R 刷新（repos+snaps+slm 同按钮逻辑） | 259 tests green, vet 干净 |
| 10 | repositories 全五维：页头/表格/对话框全换 Space 与字阶；2 处错误红→`t.Danger`；表格 2 处 FontSize(12) 拆除；⌘R；新增 `kit.DangerButton`（主题 Danger 填充的破坏性确认按钮，mygo v0.4.8 无内建变体）并用于删除仓库模态（原为蓝色 PrimaryButton） | 259 tests green, vet 干净 |
| 11 | templates 全五维：未连接态页头对齐全局模式（原裸 20px）；页头/Tabs/表格/详情/两对话框全换 Space 与字阶；3 处错误红→`t.Danger`；详情 JSON→mono+`kit.SizeMono`；删除确认按钮→`kit.DangerButton`（原为普通灰按钮）；⌘R | 259 tests green, vet 干净 |
| 12 | settings 全五维：未连接态页头对齐全局模式；页头/表格/编辑对话框全换 Space 与字阶；`sectionColor` 硬编码→主题语义（transient Warning/持久 Success）；错误/警告→`t.Danger`；「确认修改」（集群级危险操作）→`kit.DangerButton`；⌘R | 259 tests green, vet 干净 |
| 13 | users 全五维：页头/表格/三个对话框全换 Space 与字阶（对话框标题 16→17 统一）；操作消息错误色→`t.Danger`；表格 7 处 FontSize(12) 拆除；删除按钮硬编码 `Background(#d64545)+白字`→`kit.DangerButton`（悬念清单项兑现）；⌘R | 259 tests green, vet 干净 |
| 14 | roles 全五维：与 users 同构同套路——页头/表格/详情/两对话框全换 Space 与字阶；详情 JSON→mono+`kit.SizeMono`；操作消息错误色→`t.Danger`；删除按钮硬编码→`kit.DangerButton`（最后一个兑现）；⌘R | 259 tests green, vet 干净 |
| 15 | editor 调色板校准（悬念清单最后一项）：语法高亮 key 色旧 teal `#0e8c7f`→`t.Accent`（跟随系统 accent 与深浅模式，全链路无 teal 残留）；str 已是 `t.Success`、num/lit 保留明暗双档手调值（主题无数值/字面量语义位）；状态栏 7 处 `FontSize(12)`→`kit.SizeCaption`、`Padding(6,10)/Gap(12)`→Space 单位 | 259 tests green, vet 干净 |
| 16 | 壳层 控件/间距/交互（清单收官）：statusbar `Height(26)/Padding(12,8,0,8)/Gap(14)`→Space、6 处 `FontSize(12)`→SizeCaption、健康点 8px→Space(2)；welcome 标题 30→SizeLarge(26)、Gap/字号全走主题；connectionsList/connectForm 的 Padding/Gap/Radius→Space/t.Radius/t.Space(3)；交互：⌘↩=保存并连接（欢迎页默认动作）、已存连接行右键菜单（连接/删除…）+删除确认模态（`kit.DangerButton`，`conn.Store.Delete`）；新增 Tester 回归测试覆盖 ⌘↩ 与右键删除全流程 | 260 tests green, vet 干净 |

## 截图变化点

- 轮 1（全部 11 张刷新）：侧边栏选中项 teal→系统蓝 accent；健康点/健康大字 teal 绿→主题 Success 绿（深色模式下自动换深色变体）；浅色截图侧边栏/标题栏/状态栏呈材质底（Tester 以中性色占位，真实毛玻璃需实机）；深色 10-indices-dark 全页深色化正确
- 轮 2（全部 11 张刷新，变化集中在 01-overview）：四卡由透明+边框变 Surface 填充圆角分组卡片；页标题 19→17、卡内正文字号对齐系统阶梯（caption 11 / body 13 / large 26）；卡间距/内距换主题 Space 单位，密度略收紧
- 轮 3（全部 11 张刷新，变化集中在 02-indices/10-indices-dark/11-indices-en）：表格正文 12→主题默认 13（更接近系统表格）；操作错误红改主题 Danger（深色下自动变亮红）；间距换 Space 单位
- 轮 4（全部 11 张刷新，变化集中在 03-search）：页头从裸 20px 标题变为「标题+副标题」全局统一模式；表格正文走默认 13；错误文案主题红；编辑器外框圆角随主题
- 轮 5（全部 11 张刷新，变化集中在 04-rest）：页头统一模式+副标题；四面板变 Surface 填充圆角卡片（与 overview 一致）；状态码颜色走主题语义（浅色 2xx 绿/4xx 黄/5xx 红，深色自动变体）
- 轮 6（全部 11 张刷新，变化集中在 05-nodes）：表格正文走默认 13；详情面板 JSON 换 mono 等宽字体（indices 详情同步）
- 轮 7（全部 11 张刷新，变化集中在 06-shards）：表格正文走默认 13；状态点颜色走主题语义（深色自动变体）；reroute 条间距随主题
- 轮 8（全部 11 张刷新；recovery 页不在截图清单，变化同 shards 套路——表格正文/阶段点/间距全部主题化）
- 轮 9（全部 11 张刷新，变化集中在 07-snapshots）：错误文案主题红；表格/SLM 正文走默认 13；详情与 SLM 区间距随主题（截图里 mock server 对 _snapshot 返回数组导致的 decode 错误为既有测试数据形态，非本轮回归）
- 轮 10（全部 11 张刷新；repositories 页不在截图清单——同 snapshots 套路，删除确认按钮变系统式红色破坏性按钮）
- 轮 11（全部 11 张刷新；templates 页不在截图清单——同套路，详情 JSON 变 mono、删除确认变红色破坏性按钮）
- 轮 12（全部 11 张刷新；09-settings 无数据行可显——页头与空态走新字阶，transient/持久分区色待实机数据验证）
- 轮 13（全部 11 张刷新；08-users 表格正文走默认 13，其余对话框变化需交互触发）
- 轮 14（全部 11 张刷新；roles 页不在截图清单——同 users 套路）
- 轮 15（全部 11 张刷新，变化集中在 03-search 的编辑器）：JSON 键名高亮 teal→系统蓝 accent（与侧边栏选中色同源）；编辑器下方状态栏字号 12→11 caption、间距随主题（「JSON 有效」仍为主题绿）
- 轮 16（全部 11 张刷新，变化在连接后的壳层）：底部状态栏字号 12→11 caption、高度/内距/间距换主题单位（视觉密度略收）；欢迎页不在截图清单（标题 30→26、连接卡片圆角/间距随主题，需实机目检）

## 变更总览（loop 收官）

16 轮全部完成，13 模块 × 5 维度 + 壳层清单全 ✅，`go test ./...` 260 绿，浅色+深色 11 张截图每轮刷新。

- **材质**：`main.go` 开启 `WindowOptions.Vibrancy(VibrancySidebar)`，透明根页面让侧边栏/工具栏/状态栏坐上系统毛玻璃，标题栏随之融合（round 1）
- **配色**：teal accent 全链路清零——app.go 覆盖（r1）与 editor 语法高亮 key 色（r15）都改跟随系统 accent/深浅模式；健康色去重为 `kit.HealthColor`→主题 `Success/Warning/Danger`（r1）；全部模块错误红/状态色硬编码换主题语义（r3–r9）；破坏性按钮统一 `kit.DangerButton`（r10 起五个模块+连接删除 r16）
- **控件/字体**：kit 字号阶梯 `SizeCaption/SizeBody/SizeTitle/SizeLarge/SizeMono`（Rem 推导，跟随系统字体设置）替换全应用约 90 处硬编码 `FontSize`；表格单元行走主题默认 13；`panel()/card()` 卡片材质 helper 统一分组观感（r2 起）
- **间距**：pixel padding/gap 全部换 `t.Space(n)`（含壳层状态栏/欢迎页/连接表单，r16）；密度向系统 App 收紧
- **交互**：⌘R 全部 13 页刷新；⌘↩ 搜索/rest 发送/欢迎页连接；⌘T rest 新建标签；右键菜单：索引行、用户行、角色行、rest 历史、已存连接（r3–r5、r13–r14、r16）
- **交付节奏**：每 3 个模块产出 darwin/universal 实机包（r4/r8/r14 补 r10/r12/r14 节点+最终包见下）；实机批量反馈不阻塞
- **已知残留**（有意保留）：editor 画布绘制常量（gutterFont/fontSize 等自绘 metric，非主题覆盖）；固定宽度（侧边栏 220、表单 360、搜索框 200）为布局决定；07-snapshots 截图里 mock server 对 `_snapshot` 返回数组导致的 decode 错误为既有测试数据形态

## 悬念（后续轮处理）

- ~~editor.go:43 语法高亮 key 色仍为旧 teal `#0e8c7f`~~（轮 15 清零：→`t.Accent`）
- ~~全模块散落 `#d64545/#2f9e63/#d9a406` 错误/健康色硬编码~~（各模块轮已清零）
- ~~users/roles 删除按钮 `Background(#d64545)` 自绘破坏性按钮~~（轮 10/13/14 已换 `kit.DangerButton`）

悬念清单已全部清零；壳层 控件/间距/交互 在轮 16 收官，清单全 ✅。
