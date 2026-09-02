import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import UserOrdersView from '../UserOrdersView.vue'

const getMyOrders = vi.hoisted(() => vi.fn())
const getRefundEligibleProviders = vi.hoisted(() => vi.fn())

vi.mock('vue-router', async () => {
  const actual = await vi.importActual<typeof import('vue-router')>('vue-router')
  return {
    ...actual,
    useRouter: () => ({ push: vi.fn() }),
  }
})

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({ t: (key: string) => key }),
  }
})

vi.mock('@/stores', () => ({
  useAppStore: () => ({
    showError: vi.fn(),
    showSuccess: vi.fn(),
  }),
}))

vi.mock('@/api/payment', () => ({
  paymentAPI: {
    getMyOrders,
    getRefundEligibleProviders,
    cancelOrder: vi.fn(),
    previewRefund: vi.fn(),
    requestRefund: vi.fn(),
  },
}))

const OrderTableStub = {
  props: ['orders'],
  template: `
    <div>
      <div v-for="row in orders" :key="row.id" :data-order-type="row.order_type">
        <slot name="actions" :row="row" />
      </div>
    </div>
  `,
}

describe('user UserOrdersView', () => {
  beforeEach(() => {
    getMyOrders.mockReset()
    getRefundEligibleProviders.mockReset()
  })

  it('hides refund actions for subscription orders', async () => {
    getMyOrders.mockResolvedValue({
      data: {
        items: [
          { id: 1, status: 'COMPLETED', order_type: 'subscription', provider_instance_id: '1' },
          { id: 2, status: 'COMPLETED', order_type: 'balance', provider_instance_id: '1' },
        ],
        total: 2,
      },
    })
    getRefundEligibleProviders.mockResolvedValue({
      data: { provider_instance_ids: ['1'] },
    })

    const wrapper = mount(UserOrdersView, {
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          OrderTable: OrderTableStub,
          Pagination: true,
          BaseDialog: true,
          Select: true,
          Icon: true,
        },
      },
    })

    await flushPromises()

    expect(wrapper.get('[data-order-type="subscription"]').text()).not.toContain('payment.orders.requestRefund')
    expect(wrapper.get('[data-order-type="balance"]').text()).toContain('payment.orders.requestRefund')
  })
})
