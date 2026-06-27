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

import { paymentAPI } from '@/api/payment'

describe('payment api', () => {
  beforeEach(() => {
    get.mockReset()
    post.mockReset()
    get.mockResolvedValue({ data: {} })
    post.mockResolvedValue({ data: {} })
  })

  it('keeps legacy public out_trade_no verification for upgrade compatibility', async () => {
    await paymentAPI.verifyOrderPublic('legacy-order-no')

    expect(post).toHaveBeenCalledWith('/payment/public/orders/verify', {
      out_trade_no: 'legacy-order-no',
    })
  })

  it('keeps signed public resume-token resolve endpoint', async () => {
    await paymentAPI.resolveOrderPublicByResumeToken('resume-token-123')

    expect(post).toHaveBeenCalledWith('/payment/public/orders/resolve', {
      resume_token: 'resume-token-123',
    })
  })

  it('posts to subscription preview endpoint', async () => {
    await paymentAPI.previewSubscription(7, 'alipay')

    expect(post).toHaveBeenCalledWith('/payment/subscription/preview', {
      plan_id: 7,
      payment_type: 'alipay',
    })
  })

  it('posts refund preview request body to legacy order endpoint', async () => {
    await paymentAPI.previewRefund(12, { reason: 'changed mind' })

    expect(post).toHaveBeenCalledWith('/payment/orders/12/refund-preview', {
      reason: 'changed mind',
    })
  })

  it('posts settlement preview token fields when requesting legacy order refund', async () => {
    await paymentAPI.requestRefund(12, {
      reason: 'changed mind',
      preview_id: 9001,
      preview_token: 'preview-token',
      manual_transfer: {
        receiver_type: 'wechat_qr',
        receiver_name: 'Zhang San',
        receiver_account: '',
        receiver_qr_image_url: 'uploads/refund/qr/9001.png',
        remark: 'please include order number',
      },
    })

    expect(post).toHaveBeenCalledWith('/payment/orders/12/refund-request', {
      reason: 'changed mind',
      preview_id: 9001,
      preview_token: 'preview-token',
      manual_transfer: {
        receiver_type: 'wechat_qr',
        receiver_name: 'Zhang San',
        receiver_account: '',
        receiver_qr_image_url: 'uploads/refund/qr/9001.png',
        remark: 'please include order number',
      },
    })
  })
})
