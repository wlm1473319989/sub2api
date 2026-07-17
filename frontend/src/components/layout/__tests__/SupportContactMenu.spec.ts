import { beforeEach, describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'

const copyToClipboard = vi.fn().mockResolvedValue(true)

const messages: Record<string, string> = {
  'common.copy': '复制',
  'nav.supportContacts': '群聊/客服',
  'nav.qqGroup': 'QQ群',
  'nav.customerService': '客服号码',
}

vi.mock('vue-i18n', () => ({
  useI18n: () => ({
    t: (key: string) => messages[key] ?? key,
  }),
}))

vi.mock('@/composables/useClipboard', () => ({
  useClipboard: () => ({ copyToClipboard }),
}))

import SupportContactMenu from '../SupportContactMenu.vue'

describe('SupportContactMenu', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('两个号码都未配置时隐藏入口', () => {
    const wrapper = mount(SupportContactMenu)

    expect(wrapper.html()).toBe('<!--v-if-->')
  })

  it('直接展示群号和客服号码，并支持点击复制', async () => {
    const wrapper = mount(SupportContactMenu, {
      props: {
        qqGroupNumber: ' 123456789 ',
        contactInfo: ' 987654321 ',
      },
    })

    expect(wrapper.text()).toContain('123456789')
    expect(wrapper.text()).toContain('987654321')
    expect(wrapper.find('[aria-haspopup="dialog"]').exists()).toBe(false)

    const numberButtons = wrapper.findAll('button')
    await numberButtons[0].trigger('click')
    await numberButtons[1].trigger('click')

    expect(copyToClipboard).toHaveBeenNthCalledWith(1, '123456789')
    expect(copyToClipboard).toHaveBeenNthCalledWith(2, '987654321')
  })

  it('只配置其中一个号码时仅展示对应项目', async () => {
    const wrapper = mount(SupportContactMenu, {
      props: { contactInfo: 'support-qq' },
    })

    expect(wrapper.text()).toContain('客服号码')
    expect(wrapper.text()).toContain('support-qq')
    expect(wrapper.text()).not.toContain('QQ群')
  })
})
