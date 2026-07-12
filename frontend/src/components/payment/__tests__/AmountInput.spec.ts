import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import AmountInput from '../AmountInput.vue'

vi.mock('vue-i18n', () => ({
  useI18n: () => ({
    t: (key: string) => key,
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
})
