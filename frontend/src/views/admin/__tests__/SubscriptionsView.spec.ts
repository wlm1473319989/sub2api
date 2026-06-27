import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { defineComponent, h } from 'vue'

import SubscriptionsView from '../SubscriptionsView.vue'

const {
  listSubscriptions,
  assignSubscription,
  bulkExtendSubscription,
  bulkResetQuotaSubscription,
  resetQuotaSubscription,
  searchUsers,
  getGroups,
  getPlans,
  showError,
  showSuccess,
} = vi.hoisted(() => ({
  listSubscriptions: vi.fn(),
  assignSubscription: vi.fn(),
  bulkExtendSubscription: vi.fn(),
  bulkResetQuotaSubscription: vi.fn(),
  resetQuotaSubscription: vi.fn(),
  searchUsers: vi.fn(),
  getGroups: vi.fn(),
  getPlans: vi.fn(),
  showError: vi.fn(),
  showSuccess: vi.fn(),
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  const translations: Record<string, string> = {
    'admin.subscriptions.status.active': 'Active',
    'admin.subscriptions.status.expired': 'Expired',
    'admin.subscriptions.status.suspended': 'Suspended',
    'admin.subscriptions.status.superseded': 'Superseded',
    'admin.subscriptions.status.refunded': 'Refunded',
    'admin.subscriptions.status.revoked': 'Revoked',
    'userSubscriptions.status.suspended_refund': '退款处理中（已冻结）',
  }
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => translations[key] ?? key,
    }),
  }
})

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showError,
    showSuccess,
  }),
}))

vi.mock('@/composables/usePersistedPageSize', () => ({
  getPersistedPageSize: () => 20,
}))

vi.mock('@/utils/format', () => ({
  formatDateOnly: () => '2099-01-31',
}))

vi.mock('@/utils/subscriptionQuota', () => ({
  getRemainingDurationParts: () => ({ days: 1, hours: 2, minutes: 3 }),
  isOneTimeDailyQuota: () => false,
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    subscriptions: {
      list: listSubscriptions,
      assign: assignSubscription,
      bulkExtend: bulkExtendSubscription,
      bulkResetQuota: bulkResetQuotaSubscription,
      extend: vi.fn(),
      revoke: vi.fn(),
      resetQuota: resetQuotaSubscription,
    },
    usage: {
      searchUsers,
    },
    groups: {
      getAll: getGroups,
    },
    payment: {
      getPlans,
    },
  },
}))

const AppLayoutStub = { template: '<div><slot /></div>' }
const TablePageLayoutStub = {
  template: '<div><slot name="filters" /><slot name="table" /><slot name="pagination" /></div>',
}
const PaginationStub = { template: '<div />' }
const EmptyStateStub = { template: '<div />' }
const GroupBadgeStub = defineComponent({
  props: ['name'],
  setup(props) {
    return () => h('div', { class: 'group-badge-stub' }, String(props.name ?? ''))
  },
})
const GroupOptionItemStub = defineComponent({
  props: ['name'],
  setup(props) {
    return () => h('div', { class: 'group-option-item-stub' }, String(props.name ?? ''))
  },
})
const IconStub = { template: '<i />' }
const RouterLinkStub = defineComponent({
  props: {
    to: {
      type: [String, Object],
      default: '',
    },
  },
  setup(_, { slots }) {
    return () => h('a', { class: 'router-link-stub' }, slots.default?.())
  },
})

const SelectStub = defineComponent({
  props: {
    modelValue: {
      type: [String, Number, null],
      default: '',
    },
    options: {
      type: Array,
      default: () => [],
    },
    placeholder: {
      type: String,
      default: '',
    },
  },
  emits: ['update:modelValue', 'change'],
  setup(props, { emit, slots, attrs }) {
    const onChange = (event: Event) => {
      const target = event.target as HTMLSelectElement
      const value = target.value
      emit('update:modelValue', value === '' ? null : (Number.isNaN(Number(value)) ? value : Number(value)))
      emit('change', value)
    }

    return () =>
      h('select', {
        ...attrs,
        class: 'select-stub',
        value: props.modelValue ?? '',
        'data-placeholder': props.placeholder,
        onChange,
      }, [
        ...(props.options as Array<Record<string, unknown>>).map((option) =>
          h('option', { key: String(option.value), value: option.value as string | number }, String(option.label)),
        ),
      ])
  },
})

const BaseDialogStub = defineComponent({
  props: {
    show: {
      type: Boolean,
      default: false,
    },
    title: {
      type: String,
      default: '',
    },
  },
  emits: ['close'],
  setup(props, { slots }) {
    return () =>
      props.show
        ? h('div', { class: 'base-dialog-stub' }, [
            h('div', { class: 'dialog-title' }, props.title),
            slots.default?.(),
            slots.footer?.(),
          ])
        : null
  },
})

const ConfirmDialogStub = { template: '<div />' }

const DataTableStub = defineComponent({
  props: {
    data: {
      type: Array,
      default: () => [],
    },
  },
  setup(props, { slots }) {
    return () =>
      h(
        'div',
        { class: 'data-table-stub' },
        [
          h('div', { class: 'table-header-stub' }, [
            slots['header-select']?.({
              column: { key: 'select' },
              sortKey: '',
              sortOrder: 'asc',
            }),
          ]),
          ...(props.data as Array<Record<string, unknown>>).map((row, index) =>
            h('div', { key: index, class: 'table-row-stub' }, [
              slots['cell-select']?.({ row, value: undefined }),
              slots['cell-group']?.({ row }),
              slots['cell-usage']?.({ row }),
              slots['cell-status']?.({ row, value: row.status }),
              slots['cell-actions']?.({ row, value: undefined }),
            ]),
          ),
        ],
      )
  },
})

describe('admin SubscriptionsView', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    listSubscriptions.mockReset()
    assignSubscription.mockReset()
    bulkExtendSubscription.mockReset()
    bulkResetQuotaSubscription.mockReset()
    resetQuotaSubscription.mockReset()
    searchUsers.mockReset()
    getGroups.mockReset()
    getPlans.mockReset()
    showError.mockReset()
    showSuccess.mockReset()
    localStorage.clear()

    listSubscriptions.mockResolvedValue({
      items: [
        {
          id: 5,
          user_id: 1,
          plan_id: 7,
          plan_name_snapshot: 'Starter Plan',
          status: 'active',
          starts_at: '2099-01-01T00:00:00Z',
          expires_at: '2099-01-31T00:00:00Z',
          daily_usage_usd: 6,
          weekly_usage_usd: 0,
          monthly_usage_usd: 0,
          daily_quota_knives: 50,
          daily_used_knives: 4.25,
          weekly_quota_knives: null,
          monthly_quota_knives: null,
          daily_window_start: '2099-01-01T00:00:00Z',
          weekly_window_start: null,
          monthly_window_start: null,
          created_at: '2099-01-01T00:00:00Z',
          updated_at: '2099-01-01T00:00:00Z',
          user: {
            id: 1,
            email: 'demo@example.com',
            username: 'demo',
          },
        },
      ],
      total: 1,
      pages: 1,
      page: 1,
      page_size: 20,
    })

    getGroups.mockResolvedValue([
      {
        id: 2,
        name: 'Legacy Group',
      },
    ])

    getPlans.mockResolvedValue({
      data: [
        {
          id: 7,
          name: 'Starter Plan',
          description: 'Plan description',
          validity_days: 30,
          validity_unit: 'day',
          for_sale: true,
          group_platform: 'openai',
          rate_multiplier: 1,
        },
      ],
    })

    searchUsers.mockResolvedValue([
      {
        id: 13,
        email: 'chosen@example.com',
        deleted: false,
      },
    ])

    resetQuotaSubscription.mockResolvedValue({})
  })

  afterEach(() => {
    vi.runOnlyPendingTimers()
    vi.useRealTimers()
  })

  it('renders plan snapshot usage and assigns by plan_id', async () => {
    const wrapper = mount(SubscriptionsView, {
      global: {
        stubs: {
          AppLayout: AppLayoutStub,
          TablePageLayout: TablePageLayoutStub,
          DataTable: DataTableStub,
          Pagination: PaginationStub,
          BaseDialog: BaseDialogStub,
          ConfirmDialog: ConfirmDialogStub,
          EmptyState: EmptyStateStub,
          Select: SelectStub,
          GroupBadge: GroupBadgeStub,
          GroupOptionItem: GroupOptionItemStub,
          Icon: IconStub,
          RouterLink: RouterLinkStub,
          Teleport: true,
          Transition: false,
        },
      },
    })

    await flushPromises()

    expect(wrapper.text()).toContain('Starter Plan')
    expect(wrapper.text()).toContain('4.25 / 50.00')
    expect(wrapper.text()).not.toContain('$6.00 / $20.00')

    await wrapper.get('button.btn.btn-primary').trigger('click')
    await flushPromises()

    const inputs = wrapper.findAll('input[type="text"]')
    await inputs[1].trigger('focus')
    await inputs[1].setValue('chosen')
    await vi.advanceTimersByTimeAsync(350)
    await flushPromises()

    const userButtons = wrapper
      .findAll('button')
      .filter((node) => node.text().includes('chosen@example.com'))
    expect(userButtons).toHaveLength(1)
    await userButtons[0].trigger('click')

    const selects = wrapper.findAll('select.select-stub')
    expect(selects.length).toBeGreaterThan(0)
    await selects[selects.length - 1].setValue('7')
    await flushPromises()

    await wrapper.get('form#assign-subscription-form').trigger('submit.prevent')
    await flushPromises()

    expect(assignSubscription).toHaveBeenCalledWith({
      user_id: 13,
      plan_id: 7,
      validity_days: 30,
    })
    expect(showSuccess).toHaveBeenCalledWith('admin.subscriptions.subscriptionAssigned')
  })

  it('requires plan selection before assigning', async () => {
    const wrapper = mount(SubscriptionsView, {
      global: {
        stubs: {
          AppLayout: AppLayoutStub,
          TablePageLayout: TablePageLayoutStub,
          DataTable: DataTableStub,
          Pagination: PaginationStub,
          BaseDialog: BaseDialogStub,
          ConfirmDialog: ConfirmDialogStub,
          EmptyState: EmptyStateStub,
          Select: SelectStub,
          GroupBadge: GroupBadgeStub,
          GroupOptionItem: GroupOptionItemStub,
          Icon: IconStub,
          RouterLink: RouterLinkStub,
          Teleport: true,
          Transition: false,
        },
      },
    })

    await flushPromises()

    await wrapper.get('button.btn.btn-primary').trigger('click')
    await flushPromises()

    const inputs = wrapper.findAll('input[type="text"]')
    await inputs[1].trigger('focus')
    await inputs[1].setValue('chosen')
    await vi.advanceTimersByTimeAsync(350)
    await flushPromises()

    const userButtons = wrapper
      .findAll('button')
      .filter((node) => node.text().includes('chosen@example.com'))
    await userButtons[0].trigger('click')

    await wrapper.get('form#assign-subscription-form').trigger('submit.prevent')
    await flushPromises()

    expect(assignSubscription).not.toHaveBeenCalled()
    expect(showError).toHaveBeenCalledWith('admin.subscriptions.pleaseSelectPlan')
  })

  it('renders all subscription status filter options and sends the selected enum value', async () => {
    const wrapper = mount(SubscriptionsView, {
      global: {
        stubs: {
          AppLayout: AppLayoutStub,
          TablePageLayout: TablePageLayoutStub,
          DataTable: DataTableStub,
          Pagination: PaginationStub,
          BaseDialog: BaseDialogStub,
          ConfirmDialog: ConfirmDialogStub,
          EmptyState: EmptyStateStub,
          Select: SelectStub,
          GroupBadge: GroupBadgeStub,
          GroupOptionItem: GroupOptionItemStub,
          Icon: IconStub,
          RouterLink: RouterLinkStub,
          Teleport: true,
          Transition: false,
        },
      },
    })

    await flushPromises()

    const statusSelect = wrapper.find('select.select-stub')
    const optionLabels = statusSelect.findAll('option').map((option) => option.text())
    expect(optionLabels).toEqual([
      'admin.subscriptions.allStatus',
      'Active',
      'Expired',
      'Suspended',
      'Superseded',
      'Refunded',
      'Revoked',
    ])

    await statusSelect.setValue('revoked')
    await flushPromises()

    expect(listSubscriptions).toHaveBeenLastCalledWith(
      1,
      20,
      {
        status: 'revoked',
        user_id: undefined,
        sort_by: 'created_at',
        sort_order: 'desc',
      },
      expect.objectContaining({
        signal: expect.any(AbortSignal),
      }),
    )
  })

  it('renders refund frozen suspended subscription with dedicated label', async () => {
    listSubscriptions.mockResolvedValue({
      items: [
        {
          id: 8,
          user_id: 2,
          plan_id: 7,
          plan_name_snapshot: 'Refund Hold',
          status: 'suspended',
          refund_freeze_active: true,
          active_refund_request_id: 77,
          active_refund_status: 'gateway_processing',
          starts_at: '2099-01-01T00:00:00Z',
          expires_at: '2099-01-31T00:00:00Z',
          daily_usage_usd: 0,
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
          user: {
            id: 2,
            email: 'refund@example.com',
            username: 'refund-user',
          },
        },
      ],
      total: 1,
      pages: 1,
      page: 1,
      page_size: 20,
    })

    const wrapper = mount(SubscriptionsView, {
      global: {
        stubs: {
          AppLayout: AppLayoutStub,
          TablePageLayout: TablePageLayoutStub,
          DataTable: DataTableStub,
          Pagination: PaginationStub,
          BaseDialog: BaseDialogStub,
          ConfirmDialog: ConfirmDialogStub,
          EmptyState: EmptyStateStub,
          Select: SelectStub,
          GroupBadge: GroupBadgeStub,
          GroupOptionItem: GroupOptionItemStub,
          Icon: IconStub,
          RouterLink: RouterLinkStub,
          Teleport: true,
          Transition: false,
        },
      },
    })

    await flushPromises()

    expect(wrapper.text()).toContain('退款处理中（已冻结）')
  })

  it('submits batch adjust for selected subscriptions', async () => {
    bulkExtendSubscription.mockResolvedValue({
      success_count: 1,
      failed_count: 0,
      subscriptions: [],
      errors: [],
      statuses: { '5': 'adjusted' },
    })

    const wrapper = mount(SubscriptionsView, {
      global: {
        stubs: {
          AppLayout: AppLayoutStub,
          TablePageLayout: TablePageLayoutStub,
          DataTable: DataTableStub,
          Pagination: PaginationStub,
          BaseDialog: BaseDialogStub,
          ConfirmDialog: ConfirmDialogStub,
          EmptyState: EmptyStateStub,
          Select: SelectStub,
          GroupBadge: GroupBadgeStub,
          GroupOptionItem: GroupOptionItemStub,
          Icon: IconStub,
          RouterLink: RouterLinkStub,
          Teleport: true,
          Transition: false,
        },
      },
    })

    await flushPromises()

    await wrapper.get('[data-test="subscription-select"]').setValue(true)
    await flushPromises()

    await wrapper.get('[data-test="batch-adjust-open"]').trigger('click')
    await flushPromises()

    await wrapper.get('[data-test="batch-adjust-days-input"]').setValue('15')
    await wrapper.get('[data-test="batch-adjust-form"]').trigger('submit')
    await flushPromises()

    expect(bulkExtendSubscription).toHaveBeenCalledWith({
      subscription_ids: [5],
      days: 15,
    })
    expect(showSuccess).toHaveBeenCalledWith('admin.subscriptions.batchAdjustSummary')
  })

  it('submits selective quota reset for a single subscription', async () => {
    const wrapper = mount(SubscriptionsView, {
      global: {
        stubs: {
          AppLayout: AppLayoutStub,
          TablePageLayout: TablePageLayoutStub,
          DataTable: DataTableStub,
          Pagination: PaginationStub,
          BaseDialog: BaseDialogStub,
          ConfirmDialog: ConfirmDialogStub,
          EmptyState: EmptyStateStub,
          Select: SelectStub,
          GroupBadge: GroupBadgeStub,
          GroupOptionItem: GroupOptionItemStub,
          Icon: IconStub,
          RouterLink: RouterLinkStub,
          Teleport: true,
          Transition: false,
        },
      },
    })

    await flushPromises()

    await wrapper.get('[data-test="reset-quota-open"]').trigger('click')
    await flushPromises()

    await wrapper.get('[data-test="reset-daily-checkbox"]').setValue(false)
    await wrapper.get('[data-test="reset-monthly-checkbox"]').setValue(false)
    await wrapper.get('[data-test="reset-quota-form"]').trigger('submit')
    await flushPromises()

    expect(resetQuotaSubscription).toHaveBeenCalledWith(5, {
      daily: false,
      weekly: true,
      monthly: false,
    })
    expect(showSuccess).toHaveBeenCalledWith('admin.subscriptions.quotaResetSuccess')
  })

  it('submits batch quota reset for selected active subscriptions', async () => {
    bulkResetQuotaSubscription.mockResolvedValue({
      success_count: 1,
      failed_count: 0,
      subscriptions: [],
      errors: [],
      statuses: { '5': 'reset' },
    })

    const wrapper = mount(SubscriptionsView, {
      global: {
        stubs: {
          AppLayout: AppLayoutStub,
          TablePageLayout: TablePageLayoutStub,
          DataTable: DataTableStub,
          Pagination: PaginationStub,
          BaseDialog: BaseDialogStub,
          ConfirmDialog: ConfirmDialogStub,
          EmptyState: EmptyStateStub,
          Select: SelectStub,
          GroupBadge: GroupBadgeStub,
          GroupOptionItem: GroupOptionItemStub,
          Icon: IconStub,
          RouterLink: RouterLinkStub,
          Teleport: true,
          Transition: false,
        },
      },
    })

    await flushPromises()

    await wrapper.get('[data-test="subscription-select"]').setValue(true)
    await flushPromises()

    await wrapper.get('[data-test="batch-reset-open"]').trigger('click')
    await flushPromises()

    await wrapper.get('[data-test="reset-daily-checkbox"]').setValue(false)
    await wrapper.get('[data-test="reset-weekly-checkbox"]').setValue(false)
    await wrapper.get('[data-test="batch-reset-form"]').trigger('submit')
    await flushPromises()

    expect(bulkResetQuotaSubscription).toHaveBeenCalledWith({
      subscription_ids: [5],
      daily: false,
      weekly: false,
      monthly: true,
    })
    expect(showSuccess).toHaveBeenCalledWith('admin.subscriptions.batchResetQuotaSummary')
  })
})
