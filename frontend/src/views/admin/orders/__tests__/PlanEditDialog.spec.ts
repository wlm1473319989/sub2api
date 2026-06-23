import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'

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

vi.mock('vue-i18n', () => ({
  useI18n: () => ({
    t: (key: string) => key,
  }),
}))

const BaseDialogStub = {
  props: ['show', 'title', 'width'],
  template: '<div v-if="show"><slot /><slot name="footer" /></div>',
}

describe('PlanEditDialog', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    setActivePinia(createPinia())
    createPlan.mockResolvedValue({ data: { id: 1 } })
  })

  it('creates user-level plans without requiring a group', async () => {
    const wrapper = mount(PlanEditDialog, {
      props: {
        show: true,
        plan: null,
        groups: [
          {
            id: 7,
            name: 'openai-default',
            platform: 'openai',
            rate_multiplier: 1,
          },
        ],
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
})
