import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { defineComponent } from 'vue'

import SubscriptionRefundDialog from '../SubscriptionRefundDialog.vue'

const { previewSubscriptionRefund, requestSubscriptionRefund, showSuccess, showError } = vi.hoisted(() => ({
  previewSubscriptionRefund: vi.fn(),
  requestSubscriptionRefund: vi.fn(),
  showSuccess: vi.fn(),
  showError: vi.fn(),
}))

vi.mock('@/api/subscriptions', () => ({
  default: {
    previewSubscriptionRefund,
    requestSubscriptionRefund,
  },
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showSuccess,
    showError,
  }),
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string, params?: Record<string, unknown>) =>
        key === 'userSubscriptions.refund.submittedWithId' && params?.id
          ? `submitted:${params.id}`
          : key,
    }),
  }
})

const BaseDialogStub = defineComponent({
  name: 'BaseDialog',
  props: {
    show: { type: Boolean, default: false },
    title: { type: String, default: '' },
    width: { type: String, default: 'wide' },
  },
  emits: ['close'],
  template: '<div v-if="show"><slot /><slot name="footer" /></div>',
})

function buildSubscription(overrides: Record<string, unknown> = {}) {
  return {
    id: 22,
    user_id: 1,
    plan_id: 7,
    plan_name_snapshot: 'Starter Plan',
    status: 'active',
    starts_at: '2099-01-01T00:00:00Z',
    expires_at: '2099-01-31T00:00:00Z',
    daily_usage_usd: 0,
    weekly_usage_usd: 0,
    monthly_usage_usd: 0,
    daily_quota_knives: 100,
    weekly_quota_knives: null,
    monthly_quota_knives: null,
    daily_used_knives: 10,
    weekly_used_knives: 0,
    monthly_used_knives: 0,
    daily_window_start: '2099-01-01T00:00:00Z',
    weekly_window_start: null,
    monthly_window_start: null,
    created_at: '2099-01-01T00:00:00Z',
    updated_at: '2099-01-01T00:00:00Z',
    ...overrides,
  } as any
}

function buildPreview(overrides: Record<string, unknown> = {}) {
  return {
    preview_id: 9001,
    preview_token: 'preview-token',
    preview_issued_at: '2099-01-15T00:00:00Z',
    preview_expires_at: '2099-01-15T00:02:00Z',
    preview_ttl_seconds: 120,
    subscription_id: 22,
    user_id: 1,
    settlement_id: 33,
    expected_settlement_id: 33,
    action_source: 'user_purchase',
    trigger_ref_type: 'payment_order',
    trigger_ref_id: 1001,
    plan_name: 'Starter Plan',
    subscription_expires_at: '2099-01-31T00:00:00Z',
    after_settlement_value: 168.5,
    theoretical_full_max_knives: 300,
    residual_quota_knives: 120,
    unit_cost: 1.40416667,
    refund_mode: 'hybrid',
    refund_residual_value: 168.5,
    gateway_refundable_total: 99,
    manual_transfer_amount: 69.5,
    manual_transfer_required: true,
    currency: 'CNY',
    after_submit_subscription_status: 'suspended',
    after_complete_subscription_status: 'refunded',
    allocations: [
      {
        payment_order_id: 1001,
        order_amount: 99,
        order_pay_amount: 99,
        pay_amount: 99,
        payment_type: 'alipay',
        payment_provider_key: 'alipay',
        payment_provider_instance_id: 8,
        already_refunded_amount: 0,
        refundable_order_amount: 99,
        allocated_refund_value: 99,
        gateway_refund_amount: 99,
        currency: 'CNY',
        status: 'pending',
        refund_channel_available: true,
      },
    ],
    ...overrides,
  } as any
}

describe('SubscriptionRefundDialog', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    previewSubscriptionRefund.mockReset()
    requestSubscriptionRefund.mockReset()
    showSuccess.mockReset()
    showError.mockReset()
  })

  it('renders preview details and allocation breakdown', async () => {
    previewSubscriptionRefund.mockResolvedValue(buildPreview())

    const wrapper = mount(SubscriptionRefundDialog, {
      props: {
        show: true,
        subscription: buildSubscription(),
      },
      global: {
        stubs: {
          BaseDialog: BaseDialogStub,
          Icon: true,
        },
      },
    })

    const previewButton = wrapper.findAll('button').find((node) => node.text().includes('userSubscriptions.refund.preview'))
    expect(previewButton).toBeDefined()
    await previewButton!.trigger('click')
    await flushPromises()

    expect(previewSubscriptionRefund).toHaveBeenCalledWith(22, '')
    expect(wrapper.text()).toContain('168.5000')
    expect(wrapper.text()).toContain('1.4042')
    expect(wrapper.text()).toContain('Starter Plan')
    expect(wrapper.text()).toContain('userSubscriptions.refund.afterSettlementValue')
    expect(wrapper.text()).toContain('userSubscriptions.refund.residualKnives')
    expect(wrapper.text()).toContain('userSubscriptions.refund.refundMode')
    expect(wrapper.text()).toContain('alipay / alipay')
    expect(wrapper.text()).toContain('userSubscriptions.refund.allocatedRefundValue')
    expect(wrapper.text()).toContain('userSubscriptions.refund.calculationTitle')
    expect(wrapper.text()).toContain('userSubscriptions.refund.calculationResidualFormula')
    expect(wrapper.text()).toContain('userSubscriptions.refund.calculationManualFormula')
  })

  it('disables submit when preview expired', async () => {
    previewSubscriptionRefund.mockResolvedValue(buildPreview({
      preview_issued_at: '2099-01-15T00:00:00Z',
      preview_expires_at: '2099-01-15T00:00:02Z',
    }))
    vi.setSystemTime(new Date('2099-01-15T00:00:00Z'))

    const wrapper = mount(SubscriptionRefundDialog, {
      props: {
        show: true,
        subscription: buildSubscription(),
      },
      global: {
        stubs: {
          BaseDialog: BaseDialogStub,
          Icon: true,
        },
      },
    })

    const previewButton = wrapper.findAll('button').find((node) => node.text().includes('userSubscriptions.refund.preview'))
    await previewButton!.trigger('click')
    await flushPromises()

    vi.setSystemTime(new Date('2099-01-15T00:00:03Z'))
    vi.advanceTimersByTime(3000)
    await flushPromises()

    const submitButton = wrapper.findAll('button').find((node) => node.text().includes('userSubscriptions.refund.submit'))
    expect((submitButton!.element as HTMLButtonElement).disabled).toBe(true)
    expect(wrapper.text()).toContain('userSubscriptions.refund.previewExpired')
  })

  it('requires manual transfer info before submitting and submits preview token payload', async () => {
    previewSubscriptionRefund.mockResolvedValue(buildPreview())
    requestSubscriptionRefund.mockResolvedValue({
      success: true,
      refund_request_id: 99,
      subscription_id: 22,
      subscription_status: 'suspended',
      refund_status: 'submitted',
      refund_residual_value: 168.5,
      gateway_refundable_total: 99,
      manual_transfer_amount: 69.5,
      currency: 'CNY',
    })

    const wrapper = mount(SubscriptionRefundDialog, {
      props: {
        show: true,
        subscription: buildSubscription(),
      },
      global: {
        stubs: {
          BaseDialog: BaseDialogStub,
          Icon: true,
        },
      },
    })

    const previewButton = wrapper.findAll('button').find((node) => node.text().includes('userSubscriptions.refund.preview'))
    await previewButton!.trigger('click')
    await flushPromises()

    let submitButton = wrapper.findAll('button').find((node) => node.text().includes('userSubscriptions.refund.submit'))
    expect((submitButton!.element as HTMLButtonElement).disabled).toBe(true)

    const inputs = wrapper.findAll('input')
    await inputs[0].setValue('wechat_qr')
    await inputs[1].setValue('张三')
    await inputs[3].setValue('https://example.com/qr.png')
    const textareas = wrapper.findAll('textarea')
    await textareas[1].setValue('请备注订单号')
    await flushPromises()

    submitButton = wrapper.findAll('button').find((node) => node.text().includes('userSubscriptions.refund.submit'))
    expect((submitButton!.element as HTMLButtonElement).disabled).toBe(false)
    await submitButton!.trigger('click')
    await flushPromises()

    expect(requestSubscriptionRefund).toHaveBeenCalledWith(22, {
      preview_id: 9001,
      preview_token: 'preview-token',
      reason: '',
      manual_transfer: {
        receiver_type: 'wechat_qr',
        receiver_name: '张三',
        receiver_account: '',
        receiver_qr_image_url: 'https://example.com/qr.png',
        remark: '请备注订单号',
      },
    })
    expect(showSuccess).toHaveBeenCalledWith('submitted:99')
    expect(wrapper.emitted('submitted')?.[0]?.[0]?.refund_request_id).toBe(99)
  })

  it('does not require receiver info for small rounding-only manual remainder', async () => {
    previewSubscriptionRefund.mockResolvedValue(buildPreview({
      refund_residual_value: 19.7046,
      gateway_refundable_total: 19.7,
      manual_transfer_amount: 0.0046,
      manual_transfer_required: false,
      allocations: [
        {
          payment_order_id: 1001,
          order_amount: 20,
          order_pay_amount: 20,
          pay_amount: 20,
          payment_type: 'alipay',
          payment_provider_key: 'alipay',
          payment_provider_instance_id: 8,
          already_refunded_amount: 0,
          refundable_order_amount: 20,
          allocated_refund_value: 19.7046,
          gateway_refund_amount: 19.7,
          currency: 'CNY',
          status: 'pending',
          refund_channel_available: true,
        },
      ],
    }))
    requestSubscriptionRefund.mockResolvedValue({
      success: true,
      refund_request_id: 100,
      subscription_id: 22,
      subscription_status: 'suspended',
      refund_status: 'submitted',
      refund_residual_value: 19.7046,
      gateway_refundable_total: 19.7,
      manual_transfer_amount: 0.0046,
      currency: 'CNY',
    })

    const wrapper = mount(SubscriptionRefundDialog, {
      props: {
        show: true,
        subscription: buildSubscription(),
      },
      global: {
        stubs: {
          BaseDialog: BaseDialogStub,
          Icon: true,
        },
      },
    })

    const previewButton = wrapper.findAll('button').find((node) => node.text().includes('userSubscriptions.refund.preview'))
    await previewButton!.trigger('click')
    await flushPromises()

    expect(wrapper.text()).toContain('userSubscriptions.refund.calculationManualWaivedHint')
    expect(wrapper.text()).not.toContain('userSubscriptions.refund.manualTransferRequired')

    const submitButton = wrapper.findAll('button').find((node) => node.text().includes('userSubscriptions.refund.submit'))
    expect((submitButton!.element as HTMLButtonElement).disabled).toBe(false)
    await submitButton!.trigger('click')
    await flushPromises()

    expect(requestSubscriptionRefund).toHaveBeenCalledWith(22, {
      preview_id: 9001,
      preview_token: 'preview-token',
      reason: '',
      manual_transfer: undefined,
    })
  })
})
