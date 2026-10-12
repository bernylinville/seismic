# 0005 · 全量一次交付与严格验收边界

日期：2026-10-12 · 状态：已接受

## 背景

用户在盘问中明确选择：13 模块全量、一次性全兼容（awsIAM/OpenSearch/serverless）、严格一次性交付（开发期间无阶段性 build）、本地 Docker ES 8.x + 7.x 双版本功能验收 + MacBook Air M4 视觉验收。

## 决策

按上述边界执行。仓库内以测试与 `mygo build` 保持持续可用，但对用户只做一次终验。

## 后果

- 问题暴露最晚，返工风险由用户知情接受。
- OpenSearch/serverless 分支没有真实实例验收：以 elasticvue 源码行为为准移植 + httptest 固定响应测试，交付说明中如实标注「未对真实 OpenSearch/serverless 实例验证」。
- Docker ES 8.x 与 7.17 双容器是功能验收的唯一真实环境（ES 9 以 httptest 分支测试覆盖）。
