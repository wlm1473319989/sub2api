import { beforeEach, describe, expect, it, vi } from 'vitest'

const { get, post } = vi.hoisted(() => ({
  get: vi.fn(),
  post: vi.fn(),
}))

vi.mock('@/api/client', () => ({
  apiClient: {
    get,
    post,
  },
}))

import adminSubscriptionsAPI from '@/api/admin/subscriptions'

describe('admin subscription refund api', () => {
  beforeEach(() => {
    get.mockReset()
    post.mockReset()
    get.mockResolvedValue({ data: {} })
    post.mockResolvedValue({ data: {} })
  })

  it('lists refund requests with filters', async () => {
    await adminSubscriptionsAPI.listRefundRequests({
      page: 3,
      page_size: 20,
      status: 'submitted',
      user_id: 7,
      subscription_id: 9,
    })

    expect(get).toHaveBeenCalledWith('/admin/subscription-refund-requests', {
      params: {
        page: 3,
        page_size: 20,
        status: 'submitted',
        user_id: 7,
        subscription_id: 9,
      },
      signal: undefined,
    })
  })

  it('gets a single refund request', async () => {
    await adminSubscriptionsAPI.getRefundRequest(12)

    expect(get).toHaveBeenCalledWith('/admin/subscription-refund-requests/12')
  })

  it('uses new refund request mutation routes', async () => {
    await adminSubscriptionsAPI.uploadRefundProof(12, { proof_url: 'proof.png', admin_note: 'done' })
    await adminSubscriptionsAPI.processRefundGateway(12)
    await adminSubscriptionsAPI.completeRefund(12)
    await adminSubscriptionsAPI.cancelRefund(12, { admin_note: 'cancelled' })

    expect(post).toHaveBeenNthCalledWith(1, '/admin/subscription-refund-requests/12/manual-proof', {
      proof_url: 'proof.png',
      admin_note: 'done',
    })
    expect(post).toHaveBeenNthCalledWith(2, '/admin/subscription-refund-requests/12/gateway-process')
    expect(post).toHaveBeenNthCalledWith(3, '/admin/subscription-refund-requests/12/complete')
    expect(post).toHaveBeenNthCalledWith(4, '/admin/subscription-refund-requests/12/cancel', {
      admin_note: 'cancelled',
    })
  })
})
