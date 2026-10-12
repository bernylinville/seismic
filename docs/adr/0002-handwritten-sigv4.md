# 0002 · AWS SigV4 手写移植 aws4fetch，不引 aws-sdk-go

日期：2026-10-12 · 状态：已接受

## 背景

awsIAM 认证需要 SigV4 请求签名。elasticvue 前端用 aws4fetch（约 300 行 JS 库）。Go 侧选择：

1. **aws-sdk-go v2 signer**：官方维护，但引入重依赖（credentials/signer 子包），与"无重依赖小二进制"目标冲突。
2. **手写移植 aws4fetch**：单一用途、可测试、与上游行为逐字对齐。

## 决策

选 2：按 aws4fetch 算法（canonical request → string-to-sign → signing key → Authorization header）手写 Go 实现，支持 session token 与 x-amz-date；用 AWS 官方测试向量与 httptest 双重验证。

## 后果

- 依赖树保持干净（仅 mygo + sqlite 预编译库）。
- SigV4 边角（chunked upload、双跳签名）不支持：客户端场景只发普通 JSON 请求，不受影响；若未来需要再评估。
- 签名正确性责任在本仓库，测试向量必须齐（es 连接的 region/service 从 URI 推断，与 aws4fetch 相同）。
