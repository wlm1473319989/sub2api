/**
 * User Subscription API
 * API for regular users to view their own subscriptions and progress
 */

import { apiClient } from './client'
import type {
  UserSubscription,
  SubscriptionSummary,
  UserSubscriptionProgressInfo,
  SubscriptionSettlementOrder,
  SubscriptionRefundRequest,
  SubscriptionRefundPreviewResponse,
  SubscriptionRefundSubmitRequest,
  SubscriptionRefundSubmitResult,
  PaginatedResponse,
  SubscriptionRefundListParams
} from '@/types'

/**
 * Get list of current user's subscriptions
 */
export async function getMySubscriptions(): Promise<UserSubscription[]> {
  const response = await apiClient.get<UserSubscription[]>('/subscriptions')
  return response.data
}

/**
 * Get current user's active subscriptions
 */
export async function getActiveSubscriptions(): Promise<UserSubscription[]> {
  const response = await apiClient.get<UserSubscription[]>('/subscriptions/active')
  return response.data
}

/**
 * Get progress for all user's active subscriptions
 */
export async function getSubscriptionsProgress(): Promise<UserSubscriptionProgressInfo[]> {
  const response = await apiClient.get<UserSubscriptionProgressInfo[]>('/subscriptions/progress')
  return response.data
}

/**
 * Get subscription summary for dashboard display
 */
export async function getSubscriptionSummary(): Promise<SubscriptionSummary> {
  const response = await apiClient.get<SubscriptionSummary>('/subscriptions/summary')
  return response.data
}

/**
 * Get current user's subscription settlement ledger
 */
export async function getSubscriptionLedger(): Promise<SubscriptionSettlementOrder[]> {
  const response = await apiClient.get<SubscriptionSettlementOrder[]>('/subscriptions/ledger')
  return response.data
}

/**
 * Get current user's settlement refund requests
 */
export async function getSubscriptionRefundRequests(
  params: SubscriptionRefundListParams = {}
): Promise<PaginatedResponse<SubscriptionRefundRequest>> {
  const response = await apiClient.get<PaginatedResponse<SubscriptionRefundRequest>>(
    '/subscription-refund-requests',
    { params }
  )
  return response.data
}

/**
 * Get a single settlement refund request
 */
export async function getSubscriptionRefundRequest(
  id: number
): Promise<SubscriptionRefundRequest> {
  const response = await apiClient.get<SubscriptionRefundRequest>(`/subscription-refund-requests/${id}`)
  return response.data
}

/**
 * Preview a settlement-based refund for a subscription
 */
export async function previewSubscriptionRefund(
  subscriptionId: number,
  reason = ''
): Promise<SubscriptionRefundPreviewResponse> {
  const response = await apiClient.post<SubscriptionRefundPreviewResponse>(
    `/subscriptions/${subscriptionId}/refund-preview`,
    reason ? { reason } : {}
  )
  return response.data
}

/**
 * Submit a settlement-based refund request
 */
export async function requestSubscriptionRefund(
  subscriptionId: number,
  request: SubscriptionRefundSubmitRequest
): Promise<SubscriptionRefundSubmitResult> {
  const response = await apiClient.post<SubscriptionRefundSubmitResult>(
    `/subscriptions/${subscriptionId}/refund-request`,
    request
  )
  return response.data
}

export default {
  getMySubscriptions,
  getActiveSubscriptions,
  getSubscriptionsProgress,
  getSubscriptionSummary,
  getSubscriptionLedger,
  getSubscriptionRefundRequests,
  getSubscriptionRefundRequest,
  previewSubscriptionRefund,
  requestSubscriptionRefund
}
