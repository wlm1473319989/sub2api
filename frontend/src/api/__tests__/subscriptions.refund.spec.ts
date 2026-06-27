import { beforeEach, describe, expect, it, vi } from 'vitest'

const { get } = vi.hoisted(() => ({
  get: vi.fn(),
}))

vi.mock('@/api/client', () => ({
  apiClient: {
    get,
  },
}))

import subscriptionsAPI from '@/api/subscriptions'

describe('subscription refund api', () => {
  beforeEach(() => {
    get.mockReset()
    get.mockResolvedValue({ data: {} })
  })

  it('lists current user refund requests', async () => {
    await subscriptionsAPI.getSubscriptionRefundRequests({ page: 2, page_size: 10 })

    expect(get).toHaveBeenCalledWith('/subscription-refund-requests', {
      params: { page: 2, page_size: 10 },
    })
  })

  it('gets a single refund request', async () => {
    await subscriptionsAPI.getSubscriptionRefundRequest(42)

    expect(get).toHaveBeenCalledWith('/subscription-refund-requests/42')
  })
})
