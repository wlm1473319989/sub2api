# 官方上游第二批引入待办

更新时间：2026-07-07

当前本地基线分支：`sync/openai-codex-fixes`

当前官方基线：`upstream/main` at `44ab690a`

## 标注规则

- `[ ]` 未开始
- `[~]` 审计中或部分引入
- `[x]` 已完成并验证
- `[!]` 暂缓，需要产品或架构决策

每完成一项，在对应行把状态改成 `[x]`，并补充本地提交号、测试命令或暂缓原因。

## 第一批已完成基线

- [x] `fd64d07e` Codex function_call 续链非法 `item_*` id 清理。本地提交：`6aa6b756`
- [x] `4dd3aee5` OpenAI responses 使用映射后的 billing model 计费。本地提交：`b10a106d`
- [x] `f881ff7c` 支持非 `/v1` OpenAI models URL。本地提交：`bd30f724`
- [x] `2fb212b7` 区分 `/v1/responses/compact` 入站端点。本地提交：`227f2e53`
- [x] `75e30894` 从 raw request path 归一化入站端点。本地提交：`aefd1093`
- [x] `438f17be` 修复 compact JSON/SSE 判断导致的用量漏记。本地提交：`9f6fce97`
- [x] `867616fc` 过滤历史消息里上游不接受的 web search 块。本地提交：`2e1253d7`
- [x] `40c563c4` 请求体解析失败时记录真实错误原因。本地提交：`2eea44d6`

验证记录：

```powershell
docker run --rm -v sub2api-go-mod-cache:/go/pkg/mod -v sub2api-go-build-cache:/root/.cache/go-build -v C:\project\sub2api:/workspace -w /workspace/backend golang:1.26.4-alpine go test ./internal/handler ./internal/service -run "TestNormalizeInboundEndpoint|TestDeriveUpstreamEndpoint|TestResponsesSubpathSuffix|TestGetUpstreamEndpoint_FullFlow|TestLogRequestBodyParseFailure|TestDescribeInvalidJSON|TestParseGatewayRequest_InvalidJSONErrorIsDiagnostic|TestFilterWebSearchHistoryBlocks|TestFilterCodexInput_StripsFunctionCallItemID|TestOpenAIGatewayServiceRecordUsage_ResponsesMappedBillingModelHonorsBillingModelSource|TestOpenAIBuildUpstreamRequest.*Compact|TestOpenAIResponses_CompactUnauthorizedLogsFailed"
```

## 第二批优先级 A

这些提交优先逐个审计。原则：保留本地架构，只引入明确 bugfix，不整包引入官方大功能。

- [ ] `0da1fe28` 修复 text-only `/v1/responses` 被误判为图片计费  
  风险：中。涉及本地图片计费、`image_output_accounting`。  
  完成记录：

- [ ] `df51edfb` OAuth Codex 保留 instructions，同时保留 developer input  
  风险：中。影响 Codex 请求转换和 developer/system 输入合并。  
  完成记录：

- [ ] `ae5e980d` `/v1/chat/completions` 也执行 `codex_cli_only` 限制  
  风险：低中。权限策略修复，需要确认本地 chat bridge 是否同语义。  
  完成记录：

- [ ] `65fa7289` OpenAI chat transport error 支持 failover  
  风险：中。影响故障切换和错误分类。  
  完成记录：

- [ ] `dbdbfb11` chat bridge 避免注入默认 Codex instructions  
  风险：中。影响模型行为，需核对本地默认 instructions 策略。  
  完成记录：

- [ ] `2b49d662` 去重 passthrough function call args  
  风险：中。影响工具调用兼容，需覆盖单 chunk 上游响应。  
  完成记录：

- [ ] `01127820` Codex Spark 剥离 `image_generation` 工具，修复 502  
  风险：中。和本地 image bridge、Spark 策略交叉。  
  完成记录：

- [ ] `cc7612bd` 识别 OpenAI overloaded 错误码  
  风险：低中。影响调度、限流、账号暂停判断。  
  完成记录：

- [ ] `29122e30` 避免单 chunk 上游重复 tool_call arguments  
  风险：低中。API compat 修复。  
  完成记录：

- [ ] `7cbf82ed` 避免 OpenAI 上下文窗口错误误触发账号切换  
  风险：中高。影响 failover 策略和错误透传。  
  完成记录：

- [ ] `73de2ea7` Codex OAuth 多轮保留 encrypted reasoning  
  风险：中。影响续链、reasoning item 保留。  
  完成记录：

- [ ] `c797159b` `/responses/compact` 跳过 Codex image bridge 注入  
  风险：中。和已合入 compact endpoint 逻辑强相关。  
  完成记录：

- [ ] `82553c4d` OpenAI usage billing 保留 quota platform  
  风险：中高。和本地计费链路、订阅倍率、费率倍率需要核对。  
  完成记录：

## 第二批优先级 B

这些提交有价值，但改动面更宽。A 组跑通后再逐个审。

- [ ] `fcd3bc12` 无账号支持模型时返回 404 `model_not_found` 而不是 503  
  风险：高。改多处 handler 和 model availability 判断。  
  完成记录：

- [ ] `fd004bdd` `account_repo` Count 前 Clone query，避免查询状态污染  
  风险：低中。仓储层 bugfix。  
  完成记录：

- [ ] `a1b2b32e` 防止 `usage_logs` 队列溢出时静默丢记录  
  风险：中。影响用量日志可靠性和队列背压行为。  
  完成记录：

- [ ] `f3a3a086` 优化并发槽位清理  
  风险：中高。和本地并发、排队、warmup intercept 逻辑相关。  
  完成记录：

- [ ] `a5781fe3` 修复 Claude Code stream keepalive 卡住  
  风险：中。网关流式稳定性修复。  
  完成记录：

- [ ] `41cdd438` Anthropic models 接口遵守自定义模型列表  
  风险：中。影响 models 暴露语义。  
  完成记录：

- [ ] `7869b7fe` Anthropic API Key 支持 Bearer 认证方式  
  风险：中。前后端都有改动。  
  完成记录：

- [ ] `b3f79697` Anthropic `7d_oi` / Fable window 429 作为模型级限流  
  风险：中高。限流语义和账号暂停策略需要审。  
  完成记录：

- [ ] `650c50e3` Antigravity standard tier 增加 project fallback  
  风险：中高。前后端账号配置都有改动。  
  完成记录：

## 需要先做设计决策

这些提交暂不直接 cherry-pick。先明确产品语义或架构取舍，再决定是否开专题分支。

- [!] `7a38c662` + `c4128580` OpenAI count_tokens bridge 到 responses input_tokens  
  原因：本地目前 OpenAI `/messages/count_tokens` 明确返回 404，需要先决定是否改变产品语义。  
  决策记录：

- [!] `819fda34` Codex CLI 检测加固、指纹信号、账号级 app-server  
  原因：价值高但改动面很宽，涉及后端策略、设置页、账号配置。  
  决策记录：

- [!] `f385cdce` Codex image tool strip policy  
  原因：和本地图片工具策略、image bridge 策略交叉，需要独立审。  
  决策记录：

- [!] `a5638a4e` + `6bd248fd` Codex 账号导入匹配、避免 access-only 合并  
  原因：管理端导入逻辑，应单独审计数据合并规则。  
  决策记录：

- [!] `f26ca566` + `0fd2e921` OpenAI 高级调度器控制及审计修复  
  原因：体量大，且必须成组引入，不能只拿前半部分。  
  决策记录：

- [!] `5089c303` + `7650cce5` Redis SCAN 清理架构及加固  
  原因：必须成组审，不要只拿架构优化或只拿加固。  
  决策记录：

## 暂不建议进入第二批

- Grok 整个平台支持及 Grok media/video 相关提交。
- Batch image foundation 及其后续修复。
- 支付、EasyPay、CNY 汇率、订阅恢复等支付产品提交。
- 高峰倍率系列提交。
- VERSION、sponsors、README、纯 docs、纯 i18n 维护提交。

原因：这些要么体量大，要么和本地订阅、计费、分组、支付架构存在冲突，应单独开专题分支审计。
