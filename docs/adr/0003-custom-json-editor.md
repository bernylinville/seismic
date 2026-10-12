# 0003 · 自建 JSON 编辑器：高亮 + 结构补全 + mapping 智能补全

日期：2026-10-12 · 状态：已接受

## 背景

搜索 Query DSL 与 REST 控制台需要代码编辑器。mygo 无 CodeMirror 等价物，仅有 `TextArea`/`TextBuffer` 与 `TextInputClient` 原语（examples/text-input 演示自建控件：IME、选区、Edit 菜单）。用户明确要求 M1 即达 CodeMirror 级体验。

## 决策

基于 `TextInputClient` 原语自建编辑器，分四层：

1. **tokenizer**：JSON 词法（string/number/literal/punct），增量重词法。
2. **渲染**：自绘高亮（主题 token 配色）+ 行号列 + 括号匹配 + 自动缩进。
3. **结构补全**：光标处 JSON schema 位置推断，建议 key/值结构（Ctrl+Space）。
4. **mapping 补全**：按当前搜索上下文拉取索引 mapping，缓存后对 `"field":` 位置建议真实字段名与查询子句模板。

## 理由

- elasticvue 的核心高频路径就是写查询；编辑器体验直接决定产品成败，值得投入。
- mapping 是现成的补全数据源（GET /<index>/_mapping），无需内置 schema 库。
- TextInputClient 原语已解决 IME/选区/剪贴板最难的部分，剩余是词法与绘制。

## 后果

- 这是一个独立工程量（估计占 UI 层三分之一），接受。
- 补全建议质量依赖 mapping 缓存策略：连接切换按 clusterUuid 失效。
- 不做多光标与代码折叠（用户已确认边界，见设计盘问记录）。
