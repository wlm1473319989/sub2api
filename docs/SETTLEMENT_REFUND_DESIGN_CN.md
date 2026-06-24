# 结算单退款详细设计

## 1. 背景

当前退款能力以支付订单为入口：

- 用户侧：`POST /api/v1/payment/orders/:id/refund-preview`、`POST /api/v1/payment/orders/:id/refund-request`
- 管理端：`POST /api/v1/admin/payment/orders/:id/refund-preview`、`POST /api/v1/admin/payment/orders/:id/refund`

这套入口适合余额充值订单，但订阅退款存在语义偏差：订阅退款真正要退的是当前订阅权益的剩余价值，而不是任意一张订单金额。当前实现虽然已经能在订单退款预览中展示结算残值，但退款对象仍然是 `PaymentOrder`，容易产生几个问题：

- 管理员需要先定位订单，不能直接从订阅或结算链退款。
- 退款金额容易被理解为订单金额，而不是当前结算头的残值。
- 非支付来源的订阅，例如兑换码和后台分配，不应该走支付订单退款。
- 当残值大于单张订单可退金额时，前端无法看清每张订单退多少、差额如何处理。
- 订单退款和结算链退款逻辑分散，后续维护容易出现口径不一致。

目标是新增“结算单退款”作为订阅退款的主业务入口：以当前有效结算头为退款对象，以残值为唯一业务退款金额来源。支付订单只在支付购买来源下作为支付通道凭据使用。

### 1.1 本轮确认结论

- 残值总金额取当前有效结算头的 `after_settlement_value`，中文显示为“结算后权益价值”；它不是订单金额，也不是前端提交金额。
- 剩余刀数由当前订阅的有效期、额度快照、已用刀数和窗口起点计算，按日、周、月三组剩余额度取最小值。
- 用户提交退款时不传退款金额，只传 `preview_id`、`preview_token`、退款原因，以及在需要人工补退时传收款信息。
- 如果残值大于当前结算链订单可通过支付网关退款的金额，网关只退可退部分，超出部分进入 `manual_transfer_amount`，不能尝试向网关发起超额退款。
- 可用于网关退款的订单只能从当前结算链回溯得到，不能拿用户其他无关订单抵扣退款。
- `suspended` 是系统已有订阅状态，不是本方案新增状态；现有订阅校验链路已经会拒绝 `suspended`，因此可以复用为“退款处理中，权益已冻结”。

## 2. 目标与非目标

### 2.1 目标

- 用户可以从当前订阅发起退款预览和退款申请。
- 退款预览必须有时效性：预览创建后 2 分钟内必须提交，超时后作废并要求重新预览。
- 用户提交退款申请后，订阅立即冻结，不能继续使用。
- 管理端可以查看、处理和完成结算单退款申请。
- 退款预览展示当前结算头、残值、订单分摊、网关可退金额、人工补退金额。
- 退款金额由当前有效结算头残值计算得出，前端和接口都不允许手工覆盖。
- 支付购买来源按结算链相关支付订单分摊网关退款，并同步写订单退款状态。
- 残值超过支付网关可退金额时，超出部分走人工转账，管理员上传转账凭证，用户可见。
- 兑换码和后台分配来源只执行权益回收和结算链退款记录，不调用支付网关。
- 退款完成后新增 `refund` 结算单，订阅状态变为 `refunded`。
- 现有订单退款接口保持兼容，但订阅订单退款内部复用结算单退款口径。

### 2.2 非目标

- v1 不允许用户或管理员手工修改残值退款金额。
- v1 不退款历史结算单，只允许退款当前有效结算头。
- v1 不拿用户无关订单抵扣退款，只允许使用当前结算链引用过的支付订单。
- v1 不支持强制绕过结算头校验、预览有效期校验或订阅冻结校验。
- 余额充值订单退款不纳入结算单退款设计。

## 3. 核心口径

### 3.1 结算头

`subscription_settlement_orders` 中 `status = effective` 的当前有效记录代表用户当前订阅权益的结算头。

结算单通过 `prev_settlement_id` 串联。购买、续费、升级会形成新的权益头；退款和撤销是终态动作。

只允许退款当前有效结算头。退款申请提交后，退款完成前不立即关闭结算头；最终退款成功时才新增 `refund` 结算单并关闭原结算头。

### 3.2 残值总金额来源

订阅退款金额不直接使用订单金额，而是使用当前结算头的权益价值：

```text
business_value_basis = settlementResidualBasisValue(
  head,
  activeSubscription,
  head.after_settlement_value
)
```

口径说明：

- `head.after_settlement_value` 是主要总金额来源，中文含义是“结算后权益价值”。
- `after_settlement_value` 在购买、续费、升级创建结算单时写入，表示该结算头之后用户持有的订阅权益业务价值。
- 如果结算头缺失有效的 `after_settlement_value`，后端才按现有兜底逻辑使用订阅套餐快照价格或传入 fallback。
- 订单 `amount` 和 `pay_amount` 不能直接作为残值总金额，只能用于计算支付通道的可退能力和网关实退金额。

### 3.3 剩余刀数与单位刀成本

残值计算复用现有 `CalculateUpgradeResidual` 口径。

核心公式：

```text
theoretical_full_max_knives = min(
  full_daily_family_capacity,
  full_weekly_family_capacity,
  full_monthly_family_capacity
)

residual_quota_knives = min(
  residual_daily_family_capacity,
  residual_weekly_family_capacity,
  residual_monthly_family_capacity
)

unit_cost = business_value_basis / theoretical_full_max_knives
refund_residual_value = unit_cost * residual_quota_knives
```

字段来源：

- 总金额取 `head.after_settlement_value`，即“结算后权益价值”。
- 有效期和天数取当前订阅 `starts_at`、`expires_at`。
- 已用刀数取当前订阅 `daily_used_knives`、`weekly_used_knives`、`monthly_used_knives`。
- 当前额度窗口取 `daily_window_start`、`weekly_window_start`、`monthly_window_start`。
- 每日、每周、每月额度取当前订阅快照 `daily_quota_knives`、`weekly_quota_knives`、`monthly_quota_knives`。

剩余刀数不是简单的“套餐总刀数减已用刀数”，而是分别计算日、周、月额度族在剩余有效期内还能使用多少刀，然后取最小值。这样可以避免某个额度族已经接近耗尽时仍然高估残值。

### 3.4 退款残值

最终业务退款金额：

```text
refund_residual_value = settlementResidualValue(
  activeSubscription,
  business_value_basis
)
```

该值是结算单退款的唯一业务退款金额。前端展示、提交申请、结算链写入都以它为准。

### 3.5 支付网关退款金额

当结算链里存在支付购买来源时，残值需要分摊到具体支付订单，再折算为实际网关退款金额：

```text
gateway_refund_amount = calculateGatewayRefundAmount(
  order.amount,
  order.pay_amount,
  allocated_refund_value,
  order.currency
)
```

其中：

- `order.amount` 是业务订单金额。
- `order.pay_amount` 是用户通过支付渠道实际支付金额。
- `allocated_refund_value` 是本次分摊给该订单的业务退款金额。
- 网关退款金额只用于支付通道调用。
- 结算链记录仍以 `refund_residual_value` 为准。

支付网关退款必须封顶，不能超过订单剩余可退实付金额。当前 `calculateGatewayRefundAmount` 只做比例换算，不负责封顶；结算单退款服务必须在调用它之前先计算并限制每张订单的可退上限。

### 3.6 字段中文对照

| 字段 | 中文含义 |
| --- | --- |
| `after_settlement_value` | 结算后权益价值 |
| `refund_residual_value` | 退款残值 |
| `carry_in_residual_value` | 带入残值 |
| `action_delta_value` | 本次结算变动值 |
| `writeoff_value` | 核销金额 |
| `expected_settlement_id` | 预期结算头 ID |
| `preview_expires_at` | 预览失效时间 |
| `gateway_refund_amount` | 网关实退金额 |
| `manual_transfer_amount` | 人工补退金额 |
| `manual_transfer_proof_url` | 人工转账凭证图片 |

## 4. 退款生命周期

### 4.1 状态流转

建议新增退款申请状态机：

```text
previewed
  -> expired
  -> submitted
  -> gateway_processing
  -> manual_pending
  -> completed
  -> failed
  -> cancelled
```

状态说明：

- `previewed`：只生成预览，订阅仍可使用。
- `expired`：预览超过 2 分钟未提交，作废。
- `submitted`：用户已提交申请，订阅已冻结。
- `gateway_processing`：管理员正在处理网关退款。
- `manual_pending`：存在人工补退金额，等待管理员转账或上传凭证。
- `completed`：网关退款和人工补退都完成，订阅最终变为 `refunded`。
- `failed`：处理失败，需要管理员介入。
- `cancelled`：管理员取消申请，只允许在未发生任何网关退款和人工转账前取消。

### 4.2 预览有效期

用户点击退款申请时，后端创建预览单：

- `preview_issued_at = now`
- `preview_expires_at = now + 2 minutes`
- 返回 `preview_id` 和不可猜测的 `preview_token`

提交申请时必须满足：

- `preview_id` 存在，属于当前用户和当前订阅。
- `preview_token` 校验通过。
- 当前服务端时间不晚于 `preview_expires_at`。
- 当前订阅仍是 `active`。
- 当前 effective settlement head 仍等于 `expected_settlement_id`。
- 后端重新计算残值、订单分摊和人工补退金额，结果必须与预览一致，允许货币精度误差。

如果任一条件不满足，提交失败，前端必须提示“本次退款预览已失效，请重新计算”。

#### 4.2.1 预览新鲜度指纹

2 分钟 TTL 只解决时间窗口问题，仍需要防止预览后业务数据变化。预览记录建议保存以下只读快照或 hash：

- `expected_settlement_id`
- `subscription_status`
- `subscription_starts_at`
- `subscription_expires_at`
- `daily_used_knives`
- `weekly_used_knives`
- `monthly_used_knives`
- `daily_window_start`
- `weekly_window_start`
- `monthly_window_start`
- `refund_residual_value`
- `gateway_refundable_total`
- `manual_transfer_amount`
- `allocation_fingerprint`

提交时后端重新加载订阅、结算头和订单可退状态，重新计算残值和分摊。如果上述任一关键值变化，返回 `SETTLEMENT_REFUND_PREVIEW_STALE`，要求用户重新预览。

### 4.3 提交即冻结

用户点击确认提交后，后端必须在同一事务中完成：

1. 锁定退款预览记录。
2. 重新校验预览未过期、未过时。
3. 将退款申请状态改为 `submitted`。
4. 将订阅状态从 `active` 改为 `suspended`。
5. 记录冻结时间 `frozen_at`。
6. 清理订阅缓存。

`suspended` 是现有订阅状态，不是本方案新增状态。它以前的作用是通用“暂停/冻结订阅”，现有使用校验已经会拒绝 `suspended` 订阅。因此这里复用它表达“退款处理中，权益已冻结”。前端中文建议显示为“退款处理中（已冻结）”，避免和普通暂停混淆。

现有代码依据：

- `backend/internal/domain/constants.go` 已定义 `SubscriptionStatusSuspended = "suspended"`。
- `ValidateAndCheckLimits` 会对 `suspended` 返回 `SUBSCRIPTION_SUSPENDED`，中间件热路径不能继续使用。
- `ValidateSubscription` 也会拒绝 `suspended`，普通订阅有效性校验不能通过。

本方案不改变 `suspended` 的底层语义，只新增退款场景下的业务原因。为了避免和其他暂停原因混淆，退款申请表必须记录 `frozen_at`、`original_subscription_status`、`original_subscription_expires_at`，并通过退款申请状态判断是否属于“退款处理中冻结”。

冻结期间：

- 用户不能继续使用该订阅。
- 用户不能续费、升级或再次申请退款。
- 结算头暂不关闭，直到退款最终完成并写入 `refund` 结算单。
- 如果管理员在未发生任何退款付款前取消申请，订阅可从 `suspended` 恢复为 `active`；如果原到期时间已经过期，则恢复为 `expired`。

### 4.4 完成退款

退款完成时，后端执行：

1. 确认所有网关退款分摊已成功或无需网关退款。
2. 如果 `manual_transfer_amount > 0`，确认管理员已上传人工转账凭证。
3. 将订阅状态从 `suspended` 改为 `refunded`，`expires_at` 设置为完成退款时间。
4. 创建 `refund` 结算单，关闭原 effective head。
5. 将退款申请状态改为 `completed`。
6. 清理订阅缓存并写审计日志。

## 5. 退款边界

### 5.1 可退款对象

必须满足：

- 订阅存在，且状态为 `active`。
- 用户存在当前 effective settlement head。
- `head.after_user_subscription_id == active_subscription.id`。
- 预览和提交里的 `expected_settlement_id` 等于当前 head ID。
- head 的 `action_type` 不是 `refund` 或 `revoke`。
- 当前不存在未完成的退款申请。
- 计算出的 `refund_residual_value > 0`。

### 5.2 不可退款场景

以下场景直接拒绝：

- 没有 active subscription。
- 没有 effective settlement head。
- 当前结算头与 active subscription 不匹配。
- 预览超过 2 分钟未提交。
- 预览后发生续费、升级、撤销、退款、用量变化或订单退款状态变化，导致预览过时。
- 当前结算头来源不支持退款。
- 当前结算头已经是退款或撤销结果。
- 计算出的残值小于或等于 0。
- 支付购买来源缺失可用支付订单，且未进入人工补退流程。
- 支付来源对应 provider 不支持退款时，对应金额只能进入人工补退。
- 支付订单状态不允许退款时，对应金额不能走网关退款。

### 5.3 来源边界

按 `action_source + trigger_ref_type` 分派。

#### 支付购买

```text
action_source = user_purchase
trigger_ref_type = payment_order
trigger_ref_id = payment_order.id
```

行为：

- 沿当前结算链查找相关支付订单。
- 校验订单属于同一用户。
- 校验订单状态允许退款。
- 校验 provider 支持退款。
- 按残值分摊计算每张订单的业务退款金额和网关实退金额。
- 管理员执行网关退款。
- 更新支付订单退款状态。
- 如网关可退金额不足，剩余部分进入人工补退。

订单在这里不是退款对象，只是支付凭据和网关可退能力来源。

#### 兑换码

```text
action_source = exchange_code
trigger_ref_type = redeem_code
```

行为：

- 不调用支付网关。
- 用户提交后冻结当前订阅。
- 管理端完成后回收当前订阅权益。
- 写入退款结算单。
- `refund_mode = entitlement_only`。

#### 后台分配

```text
action_source = subscription_assign
trigger_ref_type = admin_assignment
```

行为：

- 不调用支付网关。
- 用户提交后冻结当前订阅。
- 管理端完成后回收当前订阅权益。
- 写入退款结算单。
- `refund_mode = entitlement_only`。

#### 后台撤销

```text
action_source = admin_revoke
trigger_ref_type = direct_action
```

行为：

- 不允许退款。
- 撤销是核销或行政关闭，不产生可退款权益。

## 6. 订单分摊与人工补退

### 6.1 订单查找范围

允许查找的订单必须来自当前结算链：

- 从当前 effective head 开始，通过 `prev_settlement_id` 向前回溯。
- 只取 `action_source = user_purchase` 且 `trigger_ref_type = payment_order` 的结算单。
- 只取 `trigger_ref_id` 对应的 `PaymentOrder`。
- 不允许拿用户其他充值订单、其他订阅订单或无关历史订单抵扣退款。

这样可以保证退款来源和权益来源一致。

明确禁止：

- 使用同一用户的其他订阅订单。
- 使用同一用户的余额充值订单。
- 使用其他用户订单。
- 使用历史上未进入当前结算链的支付订单。

原因是支付网关退款必须基于原支付单，业务账务也必须和权益来源一致。拿无关订单退款会导致支付流水、订阅权益和结算链三者不一致。

### 6.2 分摊顺序

按结算链从新到旧分摊：

1. 当前 head 引用的支付订单优先。
2. 如果当前订单可退不足，继续使用前一个结算节点引用的支付订单。
3. 直到 `refund_residual_value` 全部分摊完，或当前结算链所有可用支付订单耗尽。

### 6.3 单订单封顶

每张订单的分摊必须同时满足：

- 业务退款金额不超过订单剩余可退业务金额。
- 网关实退金额不超过订单剩余可退实付金额。
- 订单 provider 必须支持退款。
- 订单状态必须允许退款。
- 币种必须与本次退款币种一致。

建议计算：

```text
remaining_order_value = max(order.amount - order.refund_amount, 0)
remaining_pay_value = max(order.pay_amount - order.gateway_refunded_amount, 0)

allocated_refund_value = min(remaining_residual, remaining_order_value)
gateway_refund_amount = calculateGatewayRefundAmount(
  order.amount,
  order.pay_amount,
  allocated_refund_value,
  order.currency
)

if gateway_refund_amount > remaining_pay_value:
  allocated_refund_value = reverseCalculateBusinessAmount(remaining_pay_value)
  gateway_refund_amount = remaining_pay_value
```

如果当前 `PaymentOrder` 没有独立记录历史网关实退金额，v1 应在新的退款分摊表里记录本次和累计网关退款金额，不能只依赖 `payment_orders.refund_amount` 推断。

如果 `refund_residual_value` 大于所有可用订单的剩余可退实付金额：

- 网关退款金额只能等于订单剩余可退实付金额合计。
- 超出部分必须写入 `manual_transfer_amount`。
- 后端不能把超出部分传给支付网关；大多数网关会拒绝超过原实付或超过剩余可退金额的退款。
- 前端必须在提交前展示该差额，并收集用户收款信息。

### 6.4 预览展示

残值预览必须展示：

- 总残值 `refund_residual_value`。
- 网关可退合计 `gateway_refundable_total`。
- 人工补退金额 `manual_transfer_amount`。
- 每张订单的订单号、支付方式、订单金额、实付金额、已退金额、可退上限、本次分摊金额、网关实退金额、不可退原因。

示例：

```json
{
  "refund_residual_value": 168.50,
  "gateway_refundable_total": 99.00,
  "manual_transfer_amount": 69.50,
  "allocations": [
    {
      "payment_order_id": 1001,
      "order_amount": 99.00,
      "pay_amount": 99.00,
      "already_refunded_amount": 0,
      "refundable_order_amount": 99.00,
      "allocated_refund_value": 99.00,
      "gateway_refund_amount": 99.00,
      "currency": "CNY",
      "refund_channel_available": true
    }
  ]
}
```

### 6.5 人工补退

如果 `manual_transfer_amount > 0`：

- 前端在用户提交申请前弹窗收集收款方式。
- 支持用户上传收款二维码图片，或填写收款账号、姓名、备注。
- 提交时必须带上人工收款信息，否则后端拒绝提交。
- 管理员完成线下转账后上传转账凭证图片。
- 用户退款详情页可查看人工补退金额、状态和转账凭证。

人工补退不调用支付网关，也不能写入任何支付订单退款金额。它只作为本次结算单退款申请的一部分记录和展示。

### 6.6 提交退款所需参数

用户提交退款申请需要的参数只用于确认预览和补齐人工收款信息，不能包含退款金额：

| 参数 | 必填 | 来源 | 说明 |
| --- | --- | --- | --- |
| `subscription_id` | 是 | URL | 当前用户订阅 ID |
| `preview_id` | 是 | 预览响应 | 本次预览单 ID |
| `preview_token` | 是 | 预览响应 | 不可猜测 token，后端只存 hash |
| `reason` | 否 | 用户输入 | 退款原因 |
| `manual_transfer.receiver_type` | 条件必填 | 用户输入 | `manual_transfer_required = true` 时必填 |
| `manual_transfer.receiver_name` | 条件必填 | 用户输入 | 收款人名称 |
| `manual_transfer.receiver_account` | 条件必填 | 用户输入 | 收款账号，可与二维码二选一 |
| `manual_transfer.receiver_qr_image_url` | 条件必填 | 用户上传 | 收款二维码，可与账号二选一 |
| `manual_transfer.remark` | 否 | 用户输入 | 补充说明 |

后端必须忽略或拒绝请求体里的任何退款金额字段。实际退款金额全部由提交时重新计算得到。

### 6.7 前端预览必须可核对

残值预览弹窗不能只展示一个总金额，至少要让用户和管理员提前看清：

- 本次残值如何拆成网关退款和人工补退。
- 每张订单本次退多少。
- 每张订单为什么只能退这么多。
- 是否存在不可走网关的差额。
- 差额是否需要用户上传收款二维码或填写收款账号。

如果 `manual_transfer_amount > 0`，前端在确认提交前必须弹出收款信息表单。用户不提供收款方式时不能提交申请。

## 7. 后端接口设计

### 7.1 用户侧 API

```http
POST /api/v1/subscriptions/:id/settlement-refund/preview
POST /api/v1/subscriptions/:id/settlement-refund/submit
GET  /api/v1/subscription-refund-requests/:id
```

`:id` 是 `user_subscriptions.id`，不是订单 ID，也不是结算单 ID。

#### 预览请求

```json
{
  "reason": "不再需要使用"
}
```

#### 预览响应

```json
{
  "preview_id": 9001,
  "preview_token": "opaque-token",
  "preview_issued_at": "2026-06-24T12:00:00+08:00",
  "preview_expires_at": "2026-06-24T12:02:00+08:00",
  "subscription_id": 456,
  "user_id": 789,
  "settlement_id": 123,
  "expected_settlement_id": 123,
  "action_source": "user_purchase",
  "trigger_ref_type": "payment_order",
  "trigger_ref_id": 1001,
  "refund_mode": "hybrid",
  "refund_residual_value": 168.50,
  "gateway_refundable_total": 99.00,
  "manual_transfer_amount": 69.50,
  "manual_transfer_required": true,
  "currency": "CNY",
  "after_submit_subscription_status": "suspended",
  "after_complete_subscription_status": "refunded",
  "allocations": []
}
```

`refund_mode` 取值：

- `gateway_refund`：全部可通过支付网关退款。
- `manual_transfer`：全部需要人工转账。
- `hybrid`：部分网关退款，部分人工补退。
- `entitlement_only`：只回收权益，不产生真实付款退款。

#### 提交请求

```json
{
  "preview_id": 9001,
  "preview_token": "opaque-token",
  "reason": "不再需要使用",
  "manual_transfer": {
    "receiver_type": "wechat_qr",
    "receiver_name": "张三",
    "receiver_account": "",
    "receiver_qr_image_url": "uploads/refund/qr/9001.png",
    "remark": "请转到微信收款码"
  }
}
```

当预览里的 `manual_transfer_required = false` 时，`manual_transfer` 可省略。

#### 提交响应

```json
{
  "success": true,
  "refund_request_id": 9001,
  "subscription_id": 456,
  "subscription_status": "suspended",
  "refund_status": "submitted",
  "refund_residual_value": 168.50,
  "gateway_refundable_total": 99.00,
  "manual_transfer_amount": 69.50,
  "currency": "CNY"
}
```

### 7.2 管理端 API

```http
GET  /api/v1/admin/subscription-refund-requests
GET  /api/v1/admin/subscription-refund-requests/:id
POST /api/v1/admin/subscription-refund-requests/:id/process-gateway-refunds
POST /api/v1/admin/subscription-refund-requests/:id/manual-transfer-proof
POST /api/v1/admin/subscription-refund-requests/:id/complete
POST /api/v1/admin/subscription-refund-requests/:id/cancel
```

行为说明：

- `process-gateway-refunds` 按退款分摊表逐笔调用支付网关，必须幂等。
- `manual-transfer-proof` 上传人工转账凭证，记录图片地址、管理员 ID、上传时间、备注。
- `complete` 校验所有网关退款成功、人工补退凭证齐全，然后写入 `refund` 结算单并把订阅改为 `refunded`。
- `cancel` 仅允许在没有任何网关退款成功、没有人工转账凭证时执行。

## 8. 后端服务设计

新增独立服务，建议命名为 `SettlementRefundService`。核心输入输出：

```go
type SettlementRefundPreviewInput struct {
    SubscriptionID int64
    UserID         int64
    Reason         string
}

type SettlementRefundSubmitInput struct {
    SubscriptionID  int64
    UserID          int64
    PreviewID       int64
    PreviewToken    string
    Reason          string
    ManualTransfer  *ManualTransferInput
}

type SettlementRefundAdminProcessInput struct {
    RefundRequestID int64
    OperatorUserID  int64
}
```

推荐内部拆分：

- `loadSettlementRefundContext`
- `validateCurrentSettlementHead`
- `calculateSettlementRefundPreview`
- `findRefundablePaymentOrdersFromSettlementChain`
- `allocateRefundAcrossOrders`
- `validatePreviewFreshness`
- `submitAndFreezeSubscription`
- `processGatewayRefundAllocations`
- `recordManualTransferProof`
- `completeSettlementRefund`
- `createRefundSettlementOrder`

### 8.1 统一上下文

内部上下文应包含：

- 当前订阅。
- 当前结算头。
- 结算残值。
- 来源类型。
- 当前结算链相关支付订单列表。
- provider instance，可空。
- 每张订单的退款分摊。
- 网关可退合计。
- 人工补退金额。
- 币种。

### 8.2 用户提交流程

1. 加载预览记录并校验 `preview_token`。
2. 校验预览未超过 2 分钟。
3. 重新加载订阅、当前结算头、订单可退状态。
4. 重新计算残值和分摊结果，确认与预览一致。
5. 如需要人工补退，校验用户已提供收款信息。
6. 在 DB 事务内：
   - 更新退款申请状态为 `submitted`。
   - 将订阅状态从 `active` 更新为 `suspended`。
   - 保存冻结前订阅快照，用于未付款取消时恢复。
7. 清理订阅缓存。
8. 返回提交成功。

### 8.3 管理端网关退款流程

1. 加载退款申请和分摊明细。
2. 只处理状态为待处理的网关退款分摊。
3. 对每张订单加载原始 provider instance 和支付交易号。
4. 调用支付网关退款，金额使用分摊记录里的 `gateway_refund_amount`。
5. 网关成功后更新分摊状态和支付订单退款状态。
6. 任一订单失败时记录失败原因，不回滚已成功的网关退款。
7. 如果仍有人工补退金额，申请进入 `manual_pending`。
8. 如果无需人工补退且全部网关退款成功，可进入 `completed` 前置状态，等待 `complete` 写结算单。

### 8.4 人工补退流程

1. 管理员查看申请中的 `manual_transfer_amount` 和用户收款信息。
2. 管理员线下转账。
3. 管理员上传凭证图片和备注。
4. 后端保存 `manual_transfer_proof_url`、`manual_transfer_proof_uploaded_at`、`manual_transfer_operator_user_id`。
5. 用户详情页展示凭证。
6. 后端允许执行 `complete`。

### 8.5 完成流程

1. 校验申请状态不是 `completed`、`cancelled`。
2. 校验订阅当前为 `suspended`，且仍属于申请用户。
3. 校验所有网关分摊都成功或无需网关退款。
4. 校验人工补退凭证齐全或无需人工补退。
5. 在 DB 事务内：
   - 将订阅状态更新为 `refunded`。
   - 设置订阅 `expires_at = completed_at`。
   - 创建 `subscription_settlement_orders` 的 `refund` 记录。
   - 更新退款申请为 `completed`。
6. 清理订阅缓存。

## 9. 数据模型

### 9.1 新增 `subscription_refund_requests`

建议字段：

- `id`
- `user_id`
- `subscription_id`
- `settlement_id`
- `expected_settlement_id`
- `status`
- `refund_mode`
- `currency`
- `reason`
- `refund_residual_value`
- `gateway_refundable_total`
- `manual_transfer_amount`
- `preview_token_hash`
- `preview_issued_at`
- `preview_expires_at`
- `submitted_at`
- `frozen_at`
- `completed_at`
- `cancelled_at`
- `original_subscription_status`
- `original_subscription_expires_at`
- `manual_receiver_type`
- `manual_receiver_name`
- `manual_receiver_account`
- `manual_receiver_qr_image_url`
- `manual_transfer_proof_url`
- `manual_transfer_proof_uploaded_at`
- `manual_transfer_operator_user_id`
- `admin_note`
- `created_at`
- `updated_at`

索引建议：

- `user_id`
- `subscription_id`
- `settlement_id`
- `status`
- `preview_expires_at`
- 对 `subscription_id` 建未完成申请唯一约束，避免同一订阅重复申请。

### 9.2 新增 `subscription_refund_allocations`

建议字段：

- `id`
- `refund_request_id`
- `payment_order_id`
- `payment_provider_instance_id`
- `order_amount`
- `order_pay_amount`
- `already_refunded_amount`
- `refundable_order_amount`
- `allocated_refund_value`
- `gateway_refund_amount`
- `currency`
- `status`
- `gateway_refund_trade_no`
- `failed_reason`
- `processed_at`
- `created_at`
- `updated_at`

状态建议：

- `pending`
- `processing`
- `succeeded`
- `failed`
- `skipped`

### 9.3 凭证图片存储

当前项目没有明确的通用附件表。v1 建议使用最小实现：

- 管理端上传凭证图片后，后端保存文件到受控上传目录或对象存储。
- 在 `subscription_refund_requests.manual_transfer_proof_url` 记录可访问地址或受控文件 key。
- 用户访问详情时，由后端校验用户归属后返回凭证 URL。
- 图片限制：只允许图片 MIME，限制大小，记录上传管理员和上传时间。

## 10. 结算单写入规则

退款最终完成后新增结算单：

```text
action_type = refund
action_source = 原 head.action_source
trigger_ref_type = 原 head.trigger_ref_type
trigger_ref_id = 原 head.trigger_ref_id
carry_in_residual_value = refund_residual_value
action_delta_value = -refund_residual_value
after_settlement_value = 0
refund_residual_value = refund_residual_value
writeoff_value = 0
after_subscription_status = refunded
after_user_subscription_id = 当前订阅 ID
```

退款单应成为新的结算链终态节点。原 head 被关闭，退款单成为最终记录。

提交申请冻结订阅时不写 `refund` 结算单，因为此时付款退款和人工补退还没有完成。

## 11. 兼容现有订单退款

保留现有订单退款接口，避免破坏用户端和后台订单管理页面。

兼容规则：

- 余额充值订单继续走原余额退款逻辑。
- 订阅订单退款不再自行决定退款金额。
- 订阅订单退款需要确认该订单属于当前结算链。
- 如果订单不是当前结算链来源，则拒绝退款，并提示应从当前订阅结算单退款。
- 订阅订单退款预览和执行复用新的结算单退款服务。
- 旧入口不能绕过 2 分钟预览有效期、提交冻结和订单分摊封顶。

这样可以保证旧入口可用，但业务口径统一。

## 12. 前端设计

### 12.1 用户侧

当前订阅页面新增“申请退款”按钮。

展示条件：

- 订阅状态为 `active`。
- 存在当前 effective settlement head。
- 当前没有未完成退款申请。

退款预览弹窗展示：

- 当前套餐。
- 订阅到期时间。
- 当前结算单 ID。
- `after_settlement_value`，显示为“结算后权益价值”。
- 当前剩余刀数。
- 当前残值。
- 退款模式。
- 每张订单退款分摊。
- 网关可退合计。
- 人工补退金额。
- 预览倒计时。

交互规则：

- 预览倒计时从后端 `preview_expires_at` 开始。
- 倒计时结束后禁用确认按钮。
- 超时后用户必须重新点击“重新计算退款预览”。
- 如果人工补退金额大于 0，确认提交前弹窗收集收款二维码或收款账号。
- 提交成功后订阅状态展示为“退款处理中（已冻结）”。
- 用户详情页展示网关退款明细、人工补退状态和管理员上传的转账凭证。

### 12.2 管理端

管理端新增退款申请列表和详情。

列表展示：

- 申请 ID。
- 用户。
- 订阅。
- 状态。
- 残值。
- 网关可退金额。
- 人工补退金额。
- 创建时间。
- 提交时间。

详情展示：

- 用户信息。
- 订阅快照。
- 结算头信息。
- 残值计算结果。
- 每张订单分摊。
- 网关退款处理按钮。
- 人工收款信息。
- 人工转账凭证上传入口。
- 完成和取消操作。

## 13. 错误码建议

- `SETTLEMENT_HEAD_REQUIRED`：不存在当前有效结算头。
- `SETTLEMENT_HEAD_STALE`：预览结算头已不是当前 head。
- `SETTLEMENT_HEAD_SUBSCRIPTION_MISMATCH`：当前订阅和结算头不匹配。
- `SETTLEMENT_REFUND_SOURCE_INVALID`：来源不支持退款。
- `SETTLEMENT_REFUND_ZERO_RESIDUAL`：没有可退残值。
- `SETTLEMENT_REFUND_PREVIEW_EXPIRED`：退款预览已超过 2 分钟。
- `SETTLEMENT_REFUND_PREVIEW_STALE`：退款预览已过时，需要重新计算。
- `SETTLEMENT_REFUND_PREVIEW_TOKEN_INVALID`：预览 token 无效。
- `SETTLEMENT_REFUND_ALREADY_PENDING`：当前订阅已有未完成退款申请。
- `SETTLEMENT_REFUND_MANUAL_RECEIVER_REQUIRED`：需要人工补退时缺少用户收款信息。
- `SETTLEMENT_REFUND_MANUAL_PROOF_REQUIRED`：需要人工补退时缺少管理员转账凭证。
- `SETTLEMENT_REFUND_CANNOT_CANCEL_AFTER_PAYOUT`：已有退款付款，不能取消。
- `PAYMENT_REFUND_DISABLED`：支付通道未开启退款。
- `PAYMENT_ORDER_REFUND_STATUS_INVALID`：支付订单状态不可退款。
- `PAYMENT_REFUND_AMOUNT_EXCEEDED`：退款金额超过订单可退金额。
- `PAYMENT_GATEWAY_REFUND_FAILED`：网关退款失败。

## 14. 验收标准

- 用户能从当前订阅发起退款预览。
- 预览返回 `preview_expires_at`，前端显示 2 分钟倒计时。
- 2 分钟内提交成功；超过 2 分钟提交失败并要求重新预览。
- 预览后发生续费、升级、退款、用量变化或订单状态变化时，提交返回 stale。
- 提交成功后订阅立即变为 `suspended`，使用链路拒绝该订阅。
- 预览能展示每张订单退款分摊金额。
- 单张订单网关退款不超过该订单剩余可退实付金额。
- 多订单分摊总额不超过当前结算链相关订单的剩余可退金额。
- 残值大于网关可退金额时，生成 `manual_transfer_amount`。
- 需要人工补退时，用户未提交收款信息不能提交申请。
- 管理员上传人工转账凭证后，用户侧可见。
- 退款完成后订阅状态为 `refunded`。
- 退款完成后结算链新增 `refund` 节点，`after_settlement_value = 0`。
- 兑换码和后台分配来源不会调用支付网关。
- 旧订单退款入口仍可用，但订阅订单退款结果与结算单退款一致。

## 15. 实施顺序建议

按低耦合闭环拆分，每一段完成后单独提交，避免把数据模型、网关副作用和前端交互混在一起。

1. 数据模型闭环：新增退款申请表、分摊表、迁移和字段常量；测试迁移字段、索引和唯一约束。
2. 纯计算闭环：抽出结算链订单分摊 helper；测试单订单、多订单、网关封顶、币种不一致、provider 不可退、人工补退差额。
3. 预览时效闭环：实现 2 分钟 TTL、`preview_token` 生成和 hash 校验；测试过期、未过期、token 错误。
4. 预览持久化闭环：实现预览单创建、分摊明细写入、详情读取；测试写入和读取一致。
5. 残值预览服务闭环：加载当前 active 订阅、effective head、结算链订单，计算残值和分摊；测试无 head、head 过时、残值为 0、人工补退。
6. 用户提交冻结闭环：校验预览未过期且未过时，提交后同事务把订阅改为 `suspended`；测试超时失败、stale 失败、提交成功后使用链路拒绝。
7. 管理端网关退款闭环：按分摊逐笔调用支付网关并幂等更新状态；测试成功、部分失败、重复调用。
8. 人工补退闭环：用户提交收款信息，管理员上传转账凭证，用户详情可见；测试缺少收款信息、缺少凭证不能完成。
9. 完成退款闭环：确认网关和人工补退都完成后，订阅改为 `refunded`，写入 `refund` 结算单并关闭原 head；测试结算链和订阅状态。
10. API 闭环：接入用户侧和管理端路由、DTO、错误码；测试参数校验、权限和错误映射。
11. 前端用户闭环：退款预览弹窗、2 分钟倒计时、订单分摊、人工收款弹窗、提交后冻结状态展示。
12. 前端管理端闭环：申请列表、详情、网关退款处理、人工凭证上传、完成和取消操作。
13. 旧订单退款兼容闭环：订阅订单旧入口转到结算单退款口径，余额订单原逻辑不变。

每个闭环至少保留一个最小测试：

- 纯函数优先使用单元测试。
- 数据访问使用 sqlmock 或迁移静态回归测试。
- 服务流程使用 mock repository 和 mock gateway。
- 前端交互使用组件测试覆盖倒计时、按钮禁用、人工补退弹窗。
- 涉及支付网关副作用的测试必须验证幂等，不依赖真实网关。
