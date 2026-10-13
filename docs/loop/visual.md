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
| 壳层（app/main/kit） | ✅ | ✅ | ⬜ | ⬜ | ⬜ |
| overview | ⬜ | ⬜ | ⬜ | ⬜ | ⬜ |
| indices | ⬜ | ⬜ | ⬜ | ⬜ | ⬜ |
| search | ⬜ | ⬜ | ⬜ | ⬜ | ⬜ |
| rest | ⬜ | ⬜ | ⬜ | ⬜ | ⬜ |
| nodes | ⬜ | ⬜ | ⬜ | ⬜ | ⬜ |
| shards | ⬜ | ⬜ | ⬜ | ⬜ | ⬜ |
| recovery | ⬜ | ⬜ | ⬜ | ⬜ | ⬜ |
| snapshots | ⬜ | ⬜ | ⬜ | ⬜ | ⬜ |
| repositories | ⬜ | ⬜ | ⬜ | ⬜ | ⬜ |
| templates | ⬜ | ⬜ | ⬜ | ⬜ | ⬜ |
| settings | ⬜ | ⬜ | ⬜ | ⬜ | ⬜ |
| users | ⬜ | ⬜ | ⬜ | ⬜ | ⬜ |
| roles | ⬜ | ⬜ | ⬜ | ⬜ | ⬜ |

## 每轮结果

| 轮 | 内容 | 结果 |
|---|---|---|
| 1 | 壳层材质+配色：main.go 开启 `VibrancySidebar`（app.go 既有条件分支首次生效，welcome 页也坐材质上）；删 teal accent 覆盖（app.go 156-159），完全跟随系统 accent；健康色去重——app.go `healthColor`/deps.go `healthDotColor`（死代码）删除，`kit.HealthColor` 单源改映射主题语义色 `Success/Warning/Danger`（自动跟随深浅模式），connect 表单测试结果色与 kit.StateText 错误色同步换语义色 | 259 tests green, vet 干净 |

## 截图变化点

- 轮 1（全部 11 张刷新）：侧边栏选中项 teal→系统蓝 accent；健康点/健康大字 teal 绿→主题 Success 绿（深色模式下自动换深色变体）；浅色截图侧边栏/标题栏/状态栏呈材质底（Tester 以中性色占位，真实毛玻璃需实机）；深色 10-indices-dark 全页深色化正确

## 悬念（后续轮处理）

- editor.go:43 语法高亮 key 色仍为旧 teal `#0e8c7f`——editor 模块轮统一校准调色板
- 全模块散落 `#d64545/#2f9e63/#d9a406` 错误/健康色硬编码（见各模块轮）
- users/roles 删除按钮 `Background(#d64545)` 自绘破坏性按钮——控件轮换系统语义（Danger 变体或 Confirm 按钮）
