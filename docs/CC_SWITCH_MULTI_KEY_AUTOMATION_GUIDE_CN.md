# 用 CC Switch 管理多个 API Key 并实现自动化

本文介绍如何把多个 sub2api API Key 导入 CC Switch（以下简称 CCS），把客户端配置、用量巡检和故障切换拆开管理。目标是让 Claude Code、Codex 或 Gemini CLI 在不反复手改环境变量的前提下，使用不同用途的 Key，并在主线路不可用时有明确的兜底路径。

本文中的 Key 均指 sub2api 签发给客户端的 API Key，不是上游 Claude、OpenAI 或 Gemini 账号的密钥。

## 1. 先理解分层

```text
Claude Code / Codex / Gemini CLI
              |
              v
       CC Switch Provider
       Key-A / Key-B / Key-C
              |
              v
            sub2api
   分组 -> 上游账号池 -> 模型服务
```

每层只负责一件事：

| 层级 | 负责内容 | 不负责内容 |
| --- | --- | --- |
| CC Switch | 保存多套客户端配置、切换 Provider、读取 Key 用量 | 调度 sub2api 内的上游账号 |
| sub2api API Key | 权限边界、额度、限流、归属分组 | 在客户端本地切换配置 |
| sub2api 分组与账号池 | 模型路由、账号负载均衡、服务端容灾 | 替用户保管客户端本地配置 |

不要把同一个 Key 反复改绑到不同分组。正在运行的客户端会立即受到影响，也难以判断一笔用量属于哪种用途。正确方式是为每个用途创建独立 Key。

## 2. 推荐的 Key 规划

从三个用途开始即可，不建议一开始创建大量 Key：

| Key 名称示例 | 绑定分组 | 用途 | 优先级 |
| --- | --- | --- | --- |
| `ccs-claude-primary` | Claude 主分组 | 日常 Claude Code 开发 | 1 |
| `ccs-claude-backup` | Claude 备用分组 | 主线路不可用时备用 | 2 |
| `ccs-codex-primary` | OpenAI 分组 | Codex 开发任务 | 独立 |
| `ccs-gemini-primary` | Gemini 分组 | Gemini CLI 任务 | 独立 |

为每个 Key 分别设置合理的总额度、RPM 和并发限制。这样某个客户端失控时，影响只会停留在该 Key，不会耗尽其他工作流的额度。

## 3. 在 sub2api 准备 Key

1. 在“分组管理”中确认每个分组的平台、模型范围和价格符合预期。
2. 在“账号管理”中为每个分组至少配置一个可用上游账号；生产环境建议同一分组有多个账号。
3. 打开“API Keys”，按上表创建独立 Key，并给出可识别的名称。
4. 为每个 Key 绑定正确分组。Claude、OpenAI 和 Gemini 不能靠同一个 Key 的改绑来回切换。
5. 记录 Key 的用途、创建人和轮换日期，但不要把完整 Key 写入文档、聊天记录、截图或 Git 仓库。

创建完成后，先用每个 Key 单独完成一次真实请求，确保分组、模型和余额都正常，再开始导入 CCS。

## 4. 导入到 CC Switch

sub2api 的“API Keys”页面已提供“导入到 CCS”入口。对每个 Key 重复以下操作：

1. 本机安装并启动 CC Switch，确保系统能打开 `ccswitch://` 链接。
2. 进入 sub2api 的“API Keys”。
3. 在目标 Key 的操作栏点击“导入到 CCS”。
4. 浏览器询问是否打开 CC Switch 时确认。
5. 回到 CCS，检查新建的 Provider 名称、Endpoint、模型和 Key 是否正确。
6. 立即把 Provider 重命名为清晰的名称，例如 `Claude - Primary`、`Claude - Backup`、`Codex - Primary`。多个 Key 由同一站点导入时，初始名称可能相同。
7. 对其余 Key 重复导入。

项目会根据 Key 所属分组自动生成对应客户端配置：

| sub2api 分组平台 | CC Switch 客户端 | 导入 Endpoint |
| --- | --- | --- |
| Anthropic | Claude | `https://api.example.com` |
| OpenAI | Codex | `https://api.example.com` |
| Gemini | Gemini | `https://api.example.com` |
| Antigravity | Claude 或 Gemini | `https://api.example.com/antigravity` |

其中 `https://api.example.com` 应替换为你的 sub2api 对外 API 地址。使用页面的“导入到 CCS”按钮时，无需手动拼接 URL；站点会读取 `api_base_url`，未配置时使用当前站点地址。

导入配置会包含 Key 用量探测：CCS 通过 `GET /v1/usage` 请求读取该 Key 的可用状态和剩余额度，默认自动刷新间隔为 30 秒。这个探测仅用于可视化与健康判断，不会消耗模型额度。

## 5. 配置自动化策略

### 策略 A：同平台主备切换

适用于同一客户端需要两条线路的场景，例如 Claude 主分组与 Claude 备用分组。

1. 在 CCS 中将 `Claude - Primary` 与 `Claude - Backup` 都配置为 Claude Provider。
2. 将主 Provider 设为日常启用项，备用 Provider 作为故障兜底。
3. 打开 CCS 当前版本提供的 Provider 健康检查、优先级、自动切换或 Failover 功能；不同版本的名称可能不同。
4. 优先级按“主 -> 备”设置，并让健康判断同时考虑请求失败和 `/v1/usage` 显示的无效/额度耗尽状态。
5. 为备用 Key 设置较小但足够的额度，防止主线路故障后备用线路被意外长期消耗。

CCS 版本如果没有 Provider 自动切换能力，仍可以保留两套 Provider 并手动一键切换；不要伪造不存在的自动化规则。此时应启用下面的策略 C，让服务端承担真实的自动容灾。

### 策略 B：按客户端隔离 Key

不要让 Claude Code、Codex 和 Gemini CLI 共用一个“万能 Key”。在 CCS 中分别选择对应的 Provider：

| 客户端 | 选择的 Provider | 建议 Key |
| --- | --- | --- |
| Claude Code | Claude Provider | `ccs-claude-primary` |
| Codex | Codex Provider | `ccs-codex-primary` |
| Gemini CLI | Gemini Provider | `ccs-gemini-primary` |

这种隔离让你可以独立设置每个客户端的模型范围、额度和限流，也使审计记录更清晰。

### 策略 C：让 sub2api 做服务端容灾

客户端换 Key 只能处理客户端配置或 Key 级故障；上游账号失效、限流和线路波动应优先交给 sub2api 的账号池和分组容灾处理。

管理员可按以下方式设置：

1. 为主分组配置多个同平台上游账号，让 sub2api 自动在可用账号间调度。
2. 在主分组的备用分组配置中指定同平台备用分组，并配置熔断条件和冷却时间。
3. 只对明确允许承担备用费用的 API Key 启用付费备用切换。
4. 观察错误请求和账号状态，确认 401、403、429 等异常会按你的策略触发重试、降级或熔断。

这样即使 CCS 仍然使用 `ccs-claude-primary`，sub2api 也能在后端完成账号级或分组级的自动切换。客户端 Key 越稳定，排障和审计越简单。

## 6. 首次验证清单

完成导入和自动化设置后，逐项验证：

1. 在 CCS 中选中主 Provider，执行一个最小请求，确认请求进入预期客户端和分组。
2. 在 sub2api “使用记录”中按 API Key 筛选，确认用量记在 `ccs-*-primary` 而非其他 Key。
3. 验证用量接口可访问：

   ```bash
   curl -H "Authorization: Bearer $SUB2API_KEY" \
     "$SUB2API_BASE_URL/v1/usage"
   ```

   返回应能体现 Key 的有效状态及剩余额度或配额。

4. 在维护窗口内临时禁用主 Key 或使其额度归零，确认 CCS 的主备策略是否切到备用 Provider。
5. 恢复主 Key，确认 CCS 是否会按预期回切；如果不自动回切，保持手动回切比频繁抖动更安全。
6. 模拟一个上游账号异常，确认 sub2api 的账号池或备用分组能够接管，而不是要求每台客户端手动改 Key。

测试应使用额度受控的测试 Key，避免为了验证故障切换而影响正在工作的真实客户端。

## 7. 日常运维

### Key 轮换

推荐用“新增 -> 导入 -> 验证 -> 下线旧 Key”的方式轮换：

1. 新建一个替代 Key，例如 `ccs-claude-primary-2026q3`。
2. 导入 CCS，并完成一次最小请求和 `/v1/usage` 检查。
3. 将它升为主 Provider。
4. 观察一段时间的使用记录。
5. 禁用并最终删除旧 Key。

不要直接覆盖已泄露 Key 的用途，也不要在旧 Key 仍被客户端使用时立即删除。

### 监控指标

至少定期检查以下项目：

- 每个 Key 的剩余额度、RPM、并发和最近使用时间。
- 上游账号的可用状态、429/5xx 比例和冷却状态。
- 主备切换次数。频繁切换通常说明主线路、额度策略或健康阈值存在问题。
- 使用记录中是否有未知客户端或异常模型调用。

## 8. 常见问题

### 点击“导入到 CCS”没有反应

确认本机安装了 CC Switch，并且操作系统已注册 `ccswitch://` 协议。若浏览器拦截了外部协议，请允许当前站点打开 CC Switch。也可以在 sub2api 的“使用 Key”中手动复制配置后在 CCS 创建 Provider。

### 导入后 Endpoint 不正确

检查站点的 `api_base_url` 配置；留空时会使用当前站点域名。不要把管理后台路径、`/v1/models`、`/v1/usage` 或页面地址填成 API Base URL。当前导入会基于该地址访问 `${baseUrl}/v1/usage`，因此通常应填写站点根地址 `https://api.example.com`；以“使用 Key”页面给出的接入地址为准。

### CCS 显示余额或用量异常

先验证 `GET /v1/usage`：确认 Authorization 使用的是同一个 Key，反向代理没有拦截该路径，且 Key 未被禁用、过期或耗尽。CCS 的用量刷新结果不能替代实际请求成功率，应同时查看 sub2api 使用记录和错误请求。

### 主备切换后仍持续报错

先确认失败发生在哪一层：

| 现象 | 优先检查 |
| --- | --- |
| CCS 无法切换 Provider | Provider 类型、Endpoint、Key 是否导入正确，当前 CCS 版本是否支持自动切换 |
| sub2api 返回 401/403 | Key 状态、分组权限、模型可见范围 |
| sub2api 返回 429 | Key 限流、用户限流、上游账号限流和冷却策略 |
| sub2api 返回 5xx | 上游账号状态、反向代理、分组账号池和备用分组配置 |

## 9. 安全底线

- 每个自动化任务使用最小权限和独立 Key。
- 不要在命令历史、CI 日志、截图、Issue 或 Git 中暴露 Key。
- 为高风险或高成本模型单独创建 Key，并设置额度上限。
- 离职、设备丢失或疑似泄露时，先禁用对应 Key，再创建替代 Key 并更新 CCS。
- 不要把“CCS 能切换 Provider”当作服务端容灾的替代品。客户端和服务端各自的故障域不同，两层同时配置才更可靠。

## 10. 最小可用方案

若只想快速落地，按下面做：

1. 创建 `ccs-claude-primary` 和 `ccs-claude-backup` 两个 Key，分别绑定主、备 Claude 分组。
2. 在 CCS 中分别导入并重命名两套 Provider。
3. 启用 CCS 当前版本的健康检查/主备功能；没有该功能时保留备用 Provider 供一键切换。
4. 在 sub2api 主分组中配置多个账号和备用分组，让服务端处理账号级故障。
5. 用测试 Key 演练一次主 Key 禁用和一次上游账号故障，确认使用记录、CCS 状态与预期一致。

做到这五步后，多个 Key 的用途、客户端配置和服务端容灾就都有了清晰边界，后续扩展到更多团队成员或自动化任务也不会失控。
