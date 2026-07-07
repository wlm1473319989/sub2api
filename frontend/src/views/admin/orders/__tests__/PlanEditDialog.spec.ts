import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import type { AdminPaymentConfig } from '@/api/admin/payment'

import PlanEditDialog from '../PlanEditDialog.vue'

const { createPlan, updatePlan, showError, showSuccess } = vi.hoisted(() => ({
  createPlan: vi.fn(),
  updatePlan: vi.fn(),
  showError: vi.fn(),
  showSuccess: vi.fn(),
}))

vi.mock('@/api/admin/payment', () => ({
  adminPaymentAPI: {
    createPlan,
    updatePlan,
  },
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showError,
    showSuccess,
  }),
}))

vi.mock('@/utils/apiError', () => ({
  extractApiErrorMessage: () => 'error',
}))

vi.mock('@/components/payment/currency', () => ({
  formatPaymentAmount: (amount: number, currency?: string | null) => {
    const symbol = currency === 'CNY' ? '¥' : ''
    return `${symbol}${amount.toFixed(2)}`
  },
}))

vi.mock('vue-i18n', () => ({
  useI18n: () => ({
    t: (key: string, params?: Record<string, unknown>) => {
      if (key === 'payment.admin.subscriptionCnyPayPreview') return `preview ${params?.amount}`
      if (key === 'payment.admin.subscriptionCnyPayPreviewWithFee') return `fee ${params?.feeRate} ${params?.total}`
      return key
    },
  }),
}))

const BaseDialogStub = {
  props: ['show', 'title', 'width'],
  template: '<div v-if="show"><slot /><slot name="footer" /></div>',
}

function mountDialog(paymentConfig: Partial<AdminPaymentConfig> | null = null) {
  return mount(PlanEditDialog, {
    props: {
      show: true,
      plan: null,
      groups: [],
      paymentConfig: paymentConfig as AdminPaymentConfig | null,
    },
    global: {
      stubs: {
        BaseDialog: BaseDialogStub,
        GroupBadge: { template: '<div />' },
        Icon: { template: '<div />' },
        Teleport: true,
        Transition: false,
      },
    },
  })
}

describe('PlanEditDialog', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    createPlan.mockResolvedValue({ data: { id: 1 } })
  })

  it('creates user-level plans without requiring a group', async () => {
    const wrapper = mountDialog()

    expect(wrapper.find('.select-trigger').exists()).toBe(true)
    expect(wrapper.find('.select-trigger').text()).toContain('payment.admin.days')

    await wrapper.find('input[type="text"]').setValue('Starter Plan')
    await wrapper.find('textarea').setValue('starter description')

    const numericInputs = wrapper.findAll('input[type="number"]')
    await numericInputs[0].setValue('19.9')
    await numericInputs[2].setValue('30')
    await numericInputs[3].setValue('100')

    await wrapper.find('form').trigger('submit.prevent')
    await flushPromises()

    expect(showError).not.toHaveBeenCalled()
    expect(createPlan).toHaveBeenCalledTimes(1)
    expect(createPlan).toHaveBeenCalledWith(expect.objectContaining({
      name: 'Starter Plan',
      description: 'starter description',
      price: 19.9,
      validity_days: 30,
      daily_quota_knives: 100,
      validity_unit: 'day',
    }))
    expect(showSuccess).toHaveBeenCalled()
  })

  it('shows CNY channel charge using the configured subscription rate and fee', async () => {
    const wrapper = mountDialog({
      subscription_usd_to_cny_rate: 7.15,
      recharge_fee_rate: 2.5,
    })

    await wrapper.find('input[type="number"]').setValue('9.99')

    expect(wrapper.text()).toContain('preview')
    expect(wrapper.text()).toContain('¥71.43')
    expect(wrapper.text()).toContain('fee 2.5')
    expect(wrapper.text()).toContain('¥73.22')
  })

  it('hides the preview when the subscription rate is not configured', async () => {
    const wrapper = mountDialog({
      subscription_usd_to_cny_rate: 0,
      recharge_fee_rate: 2.5,
    })

    await wrapper.find('input[type="number"]').setValue('9.99')

    expect(wrapper.text()).not.toContain('preview')
    expect(wrapper.text()).not.toContain('¥71.43')
  })
})
