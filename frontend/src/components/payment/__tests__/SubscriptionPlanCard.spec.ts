import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'

import SubscriptionPlanCard from '../SubscriptionPlanCard.vue'

const translations: Record<string, string> = {
  'payment.days': 'days',
  'payment.perMonth': 'month',
  'payment.perYear': 'year',
  'payment.subscribeNow': 'Subscribe now',
  'payment.renewNow': 'Renew now',
  'payment.upgradeNow': 'Upgrade now',
  'payment.notSupportedYet': 'Not supported',
  'payment.admin.weeks': 'weeks',
  'payment.planCard.quota': 'Quota',
  'payment.planCard.unlimited': 'Unlimited',
  'payment.planCard.dailyLimit': 'Daily',
  'payment.planCard.weeklyLimit': 'Weekly',
  'payment.planCard.monthlyLimit': 'Monthly',
}

vi.mock('vue-i18n', () => ({
  useI18n: () => ({
    t: (key: string) => translations[key] ?? key,
  }),
}))

const basePlan = {
  id: 1,
  name: 'Pro',
  description: 'pro tier',
  price: 10,
  features: [],
  validity_days: 30,
  validity_unit: 'day',
  for_sale: true,
  sort_order: 1,
}

describe('SubscriptionPlanCard', () => {
  it('shows configured quota limits', () => {
    const text = mount(SubscriptionPlanCard, {
      props: {
        plan: {
          ...basePlan,
          daily_quota_knives: 10,
          weekly_quota_knives: 20,
          monthly_quota_knives: 30,
        },
      },
    }).text()

    expect(text).toContain('Daily')
    expect(text).toContain('Weekly')
    expect(text).toContain('Monthly')
    expect(text).toContain('10')
    expect(text).toContain('20')
    expect(text).toContain('30')
    expect(text).not.toContain('Unlimited')
  })

  it('shows unlimited when no quota limits are configured', () => {
    const text = mount(SubscriptionPlanCard, {
      props: {
        plan: basePlan,
      },
    }).text()

    expect(text).toContain('Quota')
    expect(text).toContain('Unlimited')
  })

  it('shows renew action label', () => {
    const text = mount(SubscriptionPlanCard, {
      props: {
        plan: basePlan,
        action: 'renew',
      },
    }).text()

    expect(text).toContain('Renew now')
  })

  it('shows upgrade action label', () => {
    const text = mount(SubscriptionPlanCard, {
      props: {
        plan: basePlan,
        action: 'upgrade',
      },
    }).text()

    expect(text).toContain('Upgrade now')
  })

  it('disables unavailable action', () => {
    const wrapper = mount(SubscriptionPlanCard, {
      props: {
        plan: basePlan,
        action: 'unavailable',
      },
    })

    expect(wrapper.text()).toContain('Not supported')
    expect(wrapper.get('button').attributes('disabled')).toBeDefined()
  })
})
