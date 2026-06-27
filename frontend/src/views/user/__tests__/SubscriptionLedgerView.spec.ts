import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import SubscriptionLedgerView from '../SubscriptionLedgerView.vue'

const showError = vi.hoisted(() => vi.fn())
const getSubscriptionLedger = vi.hoisted(() => vi.fn())
const i18nMessages = vi.hoisted<Record<string, string>>(() => ({
  'subscriptionLedger.summary.totalOrders': '结算单数',
  'subscriptionLedger.summary.totalChains': '支付链数',
  'subscriptionLedger.summary.currentStatus': '当前状态',
  'subscriptionLedger.summary.totalWriteoff': '累计核销',
  'subscriptionLedger.columns.time': '时间',
  'subscriptionLedger.columns.action': '动作',
  'subscriptionLedger.columns.plan': '套餐快照',
  'subscriptionLedger.columns.values': '结算值',
  'subscriptionLedger.columns.status': '状态',
  'subscriptionLedger.columns.chain': '链路',
  'subscriptionLedger.actions.purchase': '购买',
  'subscriptionLedger.actions.revoke': '撤销',
  'subscriptionLedger.sources.subscription_assign': '管理员分配',
  'subscriptionLedger.sources.admin_revoke': '管理员撤销',
  'subscriptionLedger.status.effective': '当前节点',
  'subscriptionLedger.status.closed': '已闭合',
  'subscriptionLedger.chain.title': '支付链 {index}',
  'subscriptionLedger.chain.head': '链头',
  'subscriptionLedger.chain.current': '当前链',
  'subscriptionLedger.chain.closed': '已结束链',
  'subscriptionLedger.chain.currentNode': '当前节点',
  'subscriptionLedger.chain.previous': '上一笔',
  'subscriptionLedger.chain.nodes': '{count} 笔',
  'subscriptionLedger.values.carryIn': '承接剩余',
  'subscriptionLedger.values.delta': '本次变动',
  'subscriptionLedger.values.after': '结算后',
  'subscriptionLedger.values.writeoff': '核销',
  'subscriptionLedger.subscriptionNo': '订阅',
  'subscriptionLedger.chainStart': '起点 #{id}',
  'userSubscriptions.status.active': '有效',
  'userSubscriptions.status.revoked': '已撤销',
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string, params?: Record<string, unknown>) => {
        const message = i18nMessages[key] ?? key
        if (params) {
          return Object.entries(params).reduce(
            (text, [name, value]) => text.replace(`{${name}}`, String(value)),
            message,
          )
        }
        return message
      },
    }),
  }
})

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showError,
  }),
}))

vi.mock('@/api/subscriptions', () => ({
  default: {
    getSubscriptionLedger,
  },
}))

vi.mock('@/utils/format', () => ({
  formatCurrency: (amount: number) => `$${amount.toFixed(2)}`,
  formatDateTime: () => '2099-01-01 10:00:00',
}))

const AppLayoutStub = { template: '<div><slot /></div>' }
const IconStub = { template: '<i />' }

describe('SubscriptionLedgerView', () => {
  beforeEach(() => {
    showError.mockReset()
    getSubscriptionLedger.mockReset()
  })

  it('renders revoke settlement chain with localized labels', async () => {
    getSubscriptionLedger.mockResolvedValue([
      {
        id: 1,
        user_id: 7,
        action_type: 'purchase',
        action_source: 'subscription_assign',
        status: 'closed',
        trigger_ref_type: 'admin_assignment',
        operator_user_id: 1,
        carry_in_residual_value: 0,
        action_delta_value: 120,
        after_settlement_value: 120,
        writeoff_value: 0,
        after_user_subscription_id: 9,
        after_plan_id: 2,
        after_plan_name_snapshot: 'Starter',
        after_subscription_status: 'active',
        effective_at: '2099-01-01T00:00:00Z',
        created_at: '2099-01-01T00:00:00Z',
        updated_at: '2099-01-01T00:00:00Z',
      },
      {
        id: 2,
        user_id: 7,
        prev_settlement_id: 1,
        action_type: 'revoke',
        action_source: 'admin_revoke',
        status: 'effective',
        trigger_ref_type: 'direct_action',
        operator_user_id: 1,
        carry_in_residual_value: 42,
        action_delta_value: 0,
        after_settlement_value: 0,
        writeoff_value: 42,
        after_user_subscription_id: 9,
        after_plan_id: 2,
        after_plan_name_snapshot: 'Starter',
        after_subscription_status: 'revoked',
        effective_at: '2099-01-02T00:00:00Z',
        created_at: '2099-01-02T00:00:00Z',
        updated_at: '2099-01-02T00:00:00Z',
      },
    ])

    const wrapper = mount(SubscriptionLedgerView, {
      global: {
        stubs: {
          AppLayout: AppLayoutStub,
          Icon: IconStub,
        },
      },
    })

    await flushPromises()

    expect(wrapper.text()).toContain('撤销')
    expect(wrapper.text()).toContain('管理员撤销')
    expect(wrapper.text()).toContain('已撤销')
    expect(wrapper.text()).toContain('$42.00')
    expect(wrapper.text()).toContain('支付链 1')
    expect(wrapper.text()).toContain('链头 #1')
    expect(wrapper.text()).toContain('上一笔 #1')
    expect(wrapper.text()).toContain('当前节点')
    expect(wrapper.text()).not.toContain('subscriptionLedger.actions.revoke')
  })
})
