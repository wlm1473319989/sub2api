import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import SubscriptionsView from '../SubscriptionsView.vue'

const push = vi.hoisted(() => vi.fn())
const showError = vi.hoisted(() => vi.fn())
const getMySubscriptions = vi.hoisted(() => vi.fn())

vi.mock('vue-router', async () => {
  const actual = await vi.importActual<typeof import('vue-router')>('vue-router')
  return {
    ...actual,
    useRouter: () => ({
      push,
    }),
  }
})

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string, params?: Record<string, unknown>) =>
        params && typeof params.days === 'number' ? `${key}:${params.days}` : key,
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
    getMySubscriptions,
  },
}))

vi.mock('@/utils/format', () => ({
  formatDateOnly: () => '2099-01-31',
}))

vi.mock('@/utils/subscriptionQuota', () => ({
  getRemainingDurationParts: () => ({ days: 1, hours: 2, minutes: 3 }),
  isOneTimeDailyQuota: () => false,
}))

const AppLayoutStub = { template: '<div><slot /></div>' }
const IconStub = { template: '<i />' }

describe('user SubscriptionsView', () => {
  beforeEach(() => {
    push.mockReset()
    showError.mockReset()
    getMySubscriptions.mockReset()
  })

  it('prefers plan snapshot fields and renews with plan_id', async () => {
    getMySubscriptions.mockResolvedValue([
      {
        id: 9,
        user_id: 1,
        plan_id: 7,
        plan_name_snapshot: 'Starter Plan',
        status: 'active',
        starts_at: '2099-01-01T00:00:00Z',
        expires_at: '2099-01-31T00:00:00Z',
        daily_usage_usd: 9,
        weekly_usage_usd: 0,
        monthly_usage_usd: 0,
        daily_quota_knives: 100,
        daily_used_knives: 12.5,
        weekly_quota_knives: null,
        monthly_quota_knives: null,
        daily_window_start: '2099-01-01T00:00:00Z',
        weekly_window_start: null,
        monthly_window_start: null,
        created_at: '2099-01-01T00:00:00Z',
        updated_at: '2099-01-01T00:00:00Z',
      },
    ])

    const wrapper = mount(SubscriptionsView, {
      global: {
        stubs: {
          AppLayout: AppLayoutStub,
          Icon: IconStub,
        },
      },
    })

    await flushPromises()

    expect(wrapper.text()).toContain('Starter Plan')
    expect(wrapper.text()).toContain('12.50 / 100.00')
    expect(wrapper.text()).not.toContain('$9.00 / $20.00')

    const renewButton = wrapper
      .findAll('button')
      .find((node) => node.text() === 'payment.renewNow')
    expect(renewButton).toBeDefined()
    await renewButton!.trigger('click')

    expect(push).toHaveBeenCalledWith({
      path: '/purchase',
      query: { tab: 'subscription', plan: '7' },
    })
  })

  it('does not show renew button when plan_id is absent', async () => {
    getMySubscriptions.mockResolvedValue([
      {
        id: 11,
        user_id: 1,
        plan_id: null,
        plan_name_snapshot: null,
        status: 'active',
        starts_at: '2099-01-01T00:00:00Z',
        expires_at: '2099-01-31T00:00:00Z',
        daily_usage_usd: 3.2,
        weekly_usage_usd: 0,
        monthly_usage_usd: 0,
        daily_quota_knives: null,
        weekly_quota_knives: null,
        monthly_quota_knives: null,
        daily_window_start: null,
        weekly_window_start: null,
        monthly_window_start: null,
        created_at: '2099-01-01T00:00:00Z',
        updated_at: '2099-01-01T00:00:00Z',
      },
    ])

    const wrapper = mount(SubscriptionsView, {
      global: {
        stubs: {
          AppLayout: AppLayoutStub,
          Icon: IconStub,
        },
      },
    })

    await flushPromises()
    expect(wrapper.text()).toContain('payment.plan #11')
    expect(wrapper.findAll('button').some((node) => node.text() === 'payment.renewNow')).toBe(false)
    expect(push).not.toHaveBeenCalled()
  })
})
