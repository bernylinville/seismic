# 0001 · 用 mygo 纯 Go 自绘 native UI 全量重写 elasticvue

日期：2026-10-12 · 状态：已接受

## 背景

elasticvue 是 Vue 3 + Quasar 纯前端应用，浏览器直连 ES（受 CORS 限制，桌面版靠 Tauri 壳绕过）。目标是 macOS 原生客户端。候选方案：

1. **mygo webview + 现有 Vue 前端**：成本最低，但不是重写，观感仍是网页，等于换 Tauri。
2. **mygo native UI（纯 Go 自绘，Metal）**：全部 UI 用 Go 重写。
3. **Swift/AppKit 原生**：脱离 Go 生态，团队栈不符。

## 决策

选 2：mygo native UI 模板，全 app 单窗口自绘。

## 理由

- mygo 官方定位 native UI 适合 tools/inspectors/utilities，正是 ES 客户端品类。
- Go `net/http` 直连 ES，无 CORS（浏览器版最大痛点消失），无需任何后端。
- 无 cgo：Linux 可交叉编译 darwin/universal `.app`/`.dmg`。
- UI 方向 A「原生工具流」（见 es-native-design/design-notes.md Round 01）：vibrancy 侧栏 + 系统主题 token + 原生表格，与 mygo 组件能力一一对应。

## 后果

- 无 CodeMirror 等价物：JSON 编辑器自建（ADR 0003）。
- 无 Keychain API：凭据经 `security` CLI 存取（ADR 0004 候选，实现时定稿）。
- 自绘组件的可达性/字体回退依赖 mygo 框架（v0.4.5 起已内建 accessibility 与主题跟随）。
- 放弃 web 资产复用，13 模块 UI 全部重写成本已由用户确认接受（全量一次交付）。
