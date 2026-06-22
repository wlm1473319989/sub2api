import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'

import SubscriptionPlanCard from '../SubscriptionPlanCard.vue'

const translations: Record<string, string> = {
  'payment.days': 'days',
  'payment.perMonth': 'month',
  'payment.perYear': 'year',
  'payment.subscribeNow': 'Subscribe now',
  'payment.renewNow': 'Renew now',
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

  it('prefers plan_id for renewal matching', () => {
    const text = mount(SubscriptionPlanCard, {
      props: {
        plan: basePlan,
        activeSubscriptions: [
          {
            id: 99,
            user_id: 1,
            plan_id: 1,
            status: 'active',
            starts_at: '2026-01-01T00:00:00Z',
            daily_usage_usd: 0,
            weekly_usage_usd: 0,
            monthly_usage_usd: 0,
            daily_window_start: null,
            weekly_window_start: null,
            monthly_window_start: null,
            created_at: '2026-01-01T00:00:00Z',
            updated_at: '2026-01-01T00:00:00Z',
            expires_at: '2026-02-01T00:00:00Z',
          },
        ],
      },
    }).text()

    expect(text).toContain('Renew now')
  })
})
