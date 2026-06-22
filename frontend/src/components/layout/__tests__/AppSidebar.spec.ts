import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

import { beforeEach, describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { nextTick } from 'vue'

import AppSidebar from '../AppSidebar.vue'
import { authAPI } from '@/api'
import { i18n, loadLocaleMessages } from '@/i18n'
import { useAdminSettingsStore, useAppStore, useAuthStore } from '@/stores'

const componentPath = resolve(dirname(fileURLToPath(import.meta.url)), '../AppSidebar.vue')
const componentSource = readFileSync(componentPath, 'utf8')
const stylePath = resolve(dirname(fileURLToPath(import.meta.url)), '../../../style.css')
const styleSource = readFileSync(stylePath, 'utf8')

vi.mock('vue-router', () => ({
  useRoute: () => ({ path: '/admin/dashboard' }),
  useRouter: () => ({ push: vi.fn() }),
  RouterLink: {
    props: ['to'],
    template: '<a :href="typeof to === \'string\' ? to : to?.path"><slot /></a>',
  },
}))

beforeEach(async () => {
  localStorage.clear()
  document.documentElement.classList.remove('dark')
  vi.stubGlobal('matchMedia', vi.fn().mockImplementation(() => ({
    matches: false,
    media: '(prefers-color-scheme: dark)',
    onchange: null,
    addListener: vi.fn(),
    removeListener: vi.fn(),
    addEventListener: vi.fn(),
    removeEventListener: vi.fn(),
    dispatchEvent: vi.fn(),
  })))
  setActivePinia(createPinia())
  await loadLocaleMessages('zh')
  i18n.global.locale.value = 'zh'

  const appStore = useAppStore()
  appStore.publicSettingsLoaded = true
  appStore.siteName = 'Sub2API'
  appStore.siteLogo = ''
  appStore.siteVersion = 'test'
  appStore.cachedPublicSettings = {
    backend_mode_enabled: false,
    custom_menu_items: [],
  } as typeof appStore.cachedPublicSettings

  const authStore = useAuthStore()
  vi.spyOn(authAPI, 'login').mockResolvedValue({
    access_token: 'token',
    refresh_token: 'refresh-token',
    expires_in: 3600,
    token_type: 'Bearer',
    user: {
      id: 1,
      username: 'admin',
      email: 'admin@example.com',
      role: 'admin',
      status: 'active',
      balance: 0,
      concurrency: 1,
      allowed_groups: null,
      balance_notify_enabled: false,
      balance_notify_threshold: null,
      balance_notify_extra_emails: [],
      created_at: '',
      updated_at: '',
      run_mode: 'simple',
    },
  })
  vi.spyOn(authAPI, 'getCurrentUser').mockResolvedValue({
    data: {
      id: 1,
      username: 'admin',
      email: 'admin@example.com',
      role: 'admin',
      status: 'active',
      balance: 0,
      concurrency: 1,
      allowed_groups: null,
      balance_notify_enabled: false,
      balance_notify_threshold: null,
      balance_notify_extra_emails: [],
      created_at: '',
      updated_at: '',
      run_mode: 'simple',
    },
  })
  await authStore.login({ email: 'admin@example.com', password: 'Admin123!' })

  const adminSettingsStore = useAdminSettingsStore()
  adminSettingsStore.customMenuItems = []
})

describe('AppSidebar custom SVG styles', () => {
  it('does not override uploaded SVG fill or stroke colors', () => {
    expect(componentSource).toContain('.sidebar-svg-icon {')
    expect(componentSource).toContain('color: currentColor;')
    expect(componentSource).toContain('display: block;')
    expect(componentSource).not.toContain('stroke: currentColor;')
    expect(componentSource).not.toContain('fill: none;')
  })
})

describe('AppSidebar header styles', () => {
  it('does not clip the version badge dropdown', () => {
    const sidebarHeaderBlockMatch = styleSource.match(/\.sidebar-header\s*\{[\s\S]*?\n {2}\}/)
    const sidebarBrandBlockMatch = componentSource.match(/\.sidebar-brand\s*\{[\s\S]*?\n\}/)

    expect(sidebarHeaderBlockMatch).not.toBeNull()
    expect(sidebarBrandBlockMatch).not.toBeNull()
    expect(sidebarHeaderBlockMatch?.[0]).not.toContain('@apply overflow-hidden;')
    expect(sidebarBrandBlockMatch?.[0]).not.toContain('overflow: hidden;')
  })
})

describe('AppSidebar admin payment plans entry', () => {
  it('shows the plans entry in simple mode when payment is enabled', async () => {
    const adminSettingsStore = useAdminSettingsStore()
    adminSettingsStore.setPaymentEnabledLocal(true)

    const wrapper = mount(AppSidebar, {
      global: {
        plugins: [i18n],
        stubs: {
          'router-link': { props: ['to'], template: '<a :href="typeof to === \'string\' ? to : to?.path"><slot /></a>' },
          VersionBadge: { template: '<span />' },
        },
      },
    })

    await nextTick()

    expect(wrapper.text()).toContain('nav.paymentPlans')
    expect(wrapper.html()).toContain('/admin/orders/plans')
  })

  it('hides the plans entry in simple mode when payment is disabled', async () => {
    const adminSettingsStore = useAdminSettingsStore()
    adminSettingsStore.setPaymentEnabledLocal(false)

    const wrapper = mount(AppSidebar, {
      global: {
        plugins: [i18n],
        stubs: {
          'router-link': { props: ['to'], template: '<a :href="typeof to === \'string\' ? to : to?.path"><slot /></a>' },
          VersionBadge: { template: '<span />' },
        },
      },
    })

    await nextTick()

    expect(wrapper.text()).not.toContain('订阅套餐')
    expect(wrapper.html()).not.toContain('/admin/orders/plans')
  })
})
