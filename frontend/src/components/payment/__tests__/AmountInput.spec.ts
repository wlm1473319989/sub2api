import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import AmountInput from '../AmountInput.vue'

vi.mock('vue-i18n', () => ({
  useI18n: () => ({
    t: (key: string, params?: { amount?: string }) => params?.amount ? `${key}:${params.amount}` : key,
  }),
}))

describe('AmountInput', () => {
  it('hides the custom amount field when custom amounts are disabled', () => {
    const wrapper = mount(AmountInput, {
      props: {
        modelValue: null,
        allowCustom: false,
      },
    })

    expect(wrapper.find('input').exists()).toBe(false)
    expect(wrapper.text()).not.toContain('payment.customAmount')
  })

  it('shows the custom amount field by default', () => {
    const wrapper = mount(AmountInput, {
      props: {
        modelValue: null,
      },
    })

    expect(wrapper.find('input').exists()).toBe(true)
  })

  it('shows bonus labels only on eligible quick amounts', () => {
    const wrapper = mount(AmountInput, {
      props: {
        modelValue: null,
        amounts: [50, 100, 200],
        bonuses: [
          { amount: 100, bonus: 10 },
          { amount: 200, bonus: 25.5 },
        ],
      },
    })

    const labels = wrapper.findAll('[data-testid="quick-amount-bonus"]')
    expect(labels).toHaveLength(2)
    expect(labels[0].text()).toBe('payment.quickAmountBonus:10')
    expect(labels[1].text()).toBe('payment.quickAmountBonus:25.5')
  })
})
