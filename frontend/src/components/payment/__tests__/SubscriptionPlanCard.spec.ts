import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import { createI18n } from 'vue-i18n'
import SubscriptionPlanCard from '../SubscriptionPlanCard.vue'

const i18n = createI18n({
  legacy: false,
  locale: 'en',
  fallbackWarn: false,
  missingWarn: false,
  messages: {
    en: {
      payment: {
        days: 'days',
        perMonth: 'month',
        perYear: 'year',
        subscribeNow: 'Subscribe now',
        renewNow: 'Renew now',
        admin: {
          weeks: 'weeks',
        },
        planCard: {
          quota: 'Quota',
          rate: 'Rate',
          unlimited: 'Unlimited',
          dailyLimit: 'Daily',
          weeklyLimit: 'Weekly',
          monthlyLimit: 'Monthly',
          models: 'Models',
        },
      },
    },
  },
})

const basePlan = {
  id: 1,
  group_id: 10,
  group_platform: 'antigravity',
  name: 'Pro',
  price: 10,
  features: [],
  rate_multiplier: 1,
  validity_days: 30,
  validity_unit: 'day',
  supported_model_scopes: ['claude', 'gemini_text', 'gemini_image'],
  for_sale: true,
  sort_order: 1,
}

describe('SubscriptionPlanCard', () => {
  it('shows model scopes for Antigravity plans', () => {
    const text = mount(SubscriptionPlanCard, {
      props: {
        plan: {
          ...basePlan,
          daily_quota_knives: 10,
        },
      },
      global: { plugins: [i18n] },
    }).text()

    expect(text).toContain('Claude')
    expect(text).toContain('Gemini')
    expect(text).toContain('Imagen')
    expect(text).toContain('10')
  })

  it('does not show Antigravity model scopes for OpenAI plans', () => {
    const text = mount(SubscriptionPlanCard, {
      props: {
        plan: {
          ...basePlan,
          group_platform: 'openai',
        },
      },
      global: { plugins: [i18n] },
    }).text()

    expect(text).not.toContain('Claude')
    expect(text).not.toContain('Gemini')
    expect(text).not.toContain('Imagen')
  })

  it('prefers plan_id for renewal matching', () => {
    const text = mount(SubscriptionPlanCard, {
      props: {
        plan: {
          ...basePlan,
          group_id: null,
        },
        activeSubscriptions: [
          {
            id: 99,
            user_id: 1,
            group_id: 777,
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
      global: { plugins: [i18n] },
    }).text()

    expect(text).toContain('payment.renewNow')
  })
})
