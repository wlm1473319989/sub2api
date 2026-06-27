import { describe, expect, it } from 'vitest'

import {
  canCancelRefundRequest,
  canCompleteRefundRequest,
  canProcessRefundGateway,
  canUploadManualProof,
  isRefundFrozenSubscription,
  subscriptionDisplayStatusLabelForRefundRequest,
} from './subscriptionRefund'

describe('subscription refund helpers', () => {
  it('detects refund-frozen suspended subscriptions', () => {
    expect(isRefundFrozenSubscription({
      status: 'submitted',
      subscription: { status: 'suspended' },
    } as any)).toBe(true)

    expect(isRefundFrozenSubscription({
      status: 'completed',
      subscription: { status: 'suspended' },
    } as any)).toBe(false)
  })

  it('uses refund-frozen label for suspended subscriptions with active refund flow', () => {
    const t = (key: string) => {
      if (key === 'userSubscriptions.status.suspended_refund') return '退款处理中（已冻结）'
      if (key === 'userSubscriptions.status.suspended') return '已冻结'
      return key
    }

    expect(subscriptionDisplayStatusLabelForRefundRequest(t as any, {
      status: 'gateway_processing',
      subscription: { status: 'suspended' },
    } as any)).toBe('退款处理中（已冻结）')

    expect(subscriptionDisplayStatusLabelForRefundRequest(t as any, {
      status: 'completed',
      subscription: { status: 'suspended' },
    } as any)).toBe('已冻结')
  })

  it('only allows gateway processing when gateway refund is actually required', () => {
    expect(canProcessRefundGateway({
      status: 'submitted',
      gateway_refundable_total: 0,
      allocations: [],
    } as any)).toBe(false)

    expect(canProcessRefundGateway({
      status: 'manual_pending',
      gateway_refundable_total: 0,
      allocations: [{ gateway_refund_amount: 12.5 }],
    } as any)).toBe(true)

    expect(canProcessRefundGateway({
      status: 'completed',
      gateway_refundable_total: 99,
      allocations: [],
    } as any)).toBe(false)
  })

  it('allows cancelling failed requests only when there is no payout evidence', () => {
    expect(canCancelRefundRequest({
      status: 'failed',
      manual_transfer_proof_url: '',
      allocations: [{ status: 'failed' }],
    } as any)).toBe(true)

    expect(canCancelRefundRequest({
      status: 'failed',
      manual_transfer_proof_url: 'https://example.com/proof.png',
      allocations: [],
    } as any)).toBe(false)
  })

  it('does not require proof or upload flow for below-threshold manual remainder', () => {
    const request = {
      status: 'gateway_processing',
      manual_transfer_amount: 0.0062,
      manual_transfer_required: false,
      manual_transfer_proof_url: '',
      allocations: [{ status: 'succeeded' }],
    } as any

    expect(canUploadManualProof(request)).toBe(false)
    expect(canCompleteRefundRequest(request)).toBe(true)
  })

  it('still requires proof when manual transfer is actually required', () => {
    const request = {
      status: 'manual_pending',
      manual_transfer_amount: 20,
      manual_transfer_required: true,
      manual_transfer_proof_url: '',
      allocations: [{ status: 'succeeded' }],
    } as any

    expect(canUploadManualProof(request)).toBe(true)
    expect(canCompleteRefundRequest(request)).toBe(false)
  })
})
