/**
 * Admin Subscriptions API endpoints
 * Handles user subscription management for administrators
 */

import { apiClient } from '../client'
import type {
  UserSubscription,
  UserSubscriptionStatus,
  AdminUserSubscriptionDetail,
  AdminSubscriptionRefundRequest,
  SubscriptionProgress,
  AssignSubscriptionRequest,
  BulkAssignSubscriptionRequest,
  BulkAdjustSubscriptionRequest,
  BulkAdjustSubscriptionResult,
  BulkResetSubscriptionQuotaRequest,
  BulkResetSubscriptionQuotaResult,
  ExtendSubscriptionRequest,
  PaginatedResponse,
  ResetSubscriptionQuotaRequest,
  AdminSubscriptionRefundListParams
} from '@/types'

/**
 * List all subscriptions with pagination
 * @param page - Page number (default: 1)
 * @param pageSize - Items per page (default: 20)
 * @param filters - Optional filters (status, user_id, sort_by, sort_order)
 * @returns Paginated list of subscriptions
 */
export async function list(
  page: number = 1,
  pageSize: number = 20,
  filters?: {
    status?: UserSubscriptionStatus
    user_id?: number
    sort_by?: string
    sort_order?: 'asc' | 'desc'
  },
  options?: {
    signal?: AbortSignal
  }
): Promise<PaginatedResponse<UserSubscription>> {
  const { data } = await apiClient.get<PaginatedResponse<UserSubscription>>(
    '/admin/subscriptions',
    {
      params: {
        page,
        page_size: pageSize,
        ...filters
      },
      signal: options?.signal
    }
  )
  return data
}

/**
 * Get subscription by ID
 * @param id - Subscription ID
 * @returns Subscription details
 */
export async function getById(id: number): Promise<AdminUserSubscriptionDetail> {
  const { data } = await apiClient.get<AdminUserSubscriptionDetail>(`/admin/subscriptions/${id}`)
  return data
}

/**
 * List settlement refund requests for admin review
 */
export async function listRefundRequests(
  params: AdminSubscriptionRefundListParams = {},
  options?: { signal?: AbortSignal }
): Promise<PaginatedResponse<AdminSubscriptionRefundRequest>> {
  const { data } = await apiClient.get<PaginatedResponse<AdminSubscriptionRefundRequest>>(
    '/admin/subscription-refund-requests',
    {
      params,
      signal: options?.signal,
    }
  )
  return data
}

/**
 * Get a single settlement refund request for admin review
 */
export async function getRefundRequest(id: number): Promise<AdminSubscriptionRefundRequest> {
  const { data } = await apiClient.get<AdminSubscriptionRefundRequest>(
    `/admin/subscription-refund-requests/${id}`
  )
  return data
}

/**
 * Get subscription progress
 * @param id - Subscription ID
 * @returns Subscription progress with usage stats
 */
export async function getProgress(id: number): Promise<SubscriptionProgress> {
  const { data } = await apiClient.get<SubscriptionProgress>(`/admin/subscriptions/${id}/progress`)
  return data
}

/**
 * Assign subscription plan to user
 * @param request - Assignment request
 * @returns Created subscription
 */
export async function assign(request: AssignSubscriptionRequest): Promise<UserSubscription> {
  const { data } = await apiClient.post<UserSubscription>('/admin/subscriptions/assign', request)
  return data
}

/**
 * Bulk assign subscription plan to multiple users
 * @param request - Bulk assignment request
 * @returns Created subscriptions
 */
export async function bulkAssign(
  request: BulkAssignSubscriptionRequest
): Promise<UserSubscription[]> {
  const { data } = await apiClient.post<UserSubscription[]>(
    '/admin/subscriptions/bulk-assign',
    request
  )
  return data
}

/**
 * Bulk adjust subscription validity
 * @param request - Bulk adjust request with subscription ids and days delta
 * @returns Bulk adjust result summary
 */
export async function bulkExtend(
  request: BulkAdjustSubscriptionRequest
): Promise<BulkAdjustSubscriptionResult> {
  const { data } = await apiClient.post<BulkAdjustSubscriptionResult>(
    '/admin/subscriptions/bulk-extend',
    request
  )
  return data
}

/**
 * Bulk reset daily, weekly, and/or monthly usage quota for subscriptions
 * @param request - Bulk reset request with subscription ids and selected windows
 * @returns Bulk reset result summary
 */
export async function bulkResetQuota(
  request: BulkResetSubscriptionQuotaRequest
): Promise<BulkResetSubscriptionQuotaResult> {
  const { data } = await apiClient.post<BulkResetSubscriptionQuotaResult>(
    '/admin/subscriptions/bulk-reset-quota',
    request
  )
  return data
}

/**
 * Extend subscription validity
 * @param id - Subscription ID
 * @param request - Extension request with days
 * @returns Updated subscription
 */
export async function extend(
  id: number,
  request: ExtendSubscriptionRequest
): Promise<UserSubscription> {
  const { data } = await apiClient.post<UserSubscription>(
    `/admin/subscriptions/${id}/extend`,
    request
  )
  return data
}

/**
 * Revoke subscription
 * @param id - Subscription ID
 * @returns Success confirmation
 */
export async function revoke(id: number): Promise<{ message: string }> {
  const { data } = await apiClient.delete<{ message: string }>(`/admin/subscriptions/${id}`)
  return data
}

/**
 * Upload manual refund proof for a settlement refund request
 */
export async function uploadRefundProof(
  id: number,
  request: { proof_url: string; admin_note?: string }
): Promise<unknown> {
  const { data } = await apiClient.post(`/admin/subscription-refund-requests/${id}/manual-proof`, request)
  return data
}

/**
 * Run gateway refund processing for a settlement refund request
 */
export async function processRefundGateway(id: number): Promise<unknown> {
  const { data } = await apiClient.post(`/admin/subscription-refund-requests/${id}/gateway-process`)
  return data
}

/**
 * Complete a settlement refund request
 */
export async function completeRefund(id: number): Promise<unknown> {
  const { data } = await apiClient.post(`/admin/subscription-refund-requests/${id}/complete`)
  return data
}

/**
 * Cancel a settlement refund request
 */
export async function cancelRefund(id: number, request?: { admin_note?: string }): Promise<unknown> {
  const { data } = await apiClient.post(`/admin/subscription-refund-requests/${id}/cancel`, request || {})
  return data
}

/**
 * Reset daily, weekly, and/or monthly usage quota for a subscription
 * @param id - Subscription ID
 * @param options - Which windows to reset
 * @returns Updated subscription
 */
export async function resetQuota(
  id: number,
  options: ResetSubscriptionQuotaRequest
): Promise<UserSubscription> {
  const { data } = await apiClient.post<UserSubscription>(
    `/admin/subscriptions/${id}/reset-quota`,
    options
  )
  return data
}

/**
 * List subscriptions by user
 * @param userId - User ID
 * @param page - Page number
 * @param pageSize - Items per page
 * @returns Paginated list of user's subscriptions
 */
export async function listByUser(
  userId: number,
  page: number = 1,
  pageSize: number = 20
): Promise<PaginatedResponse<UserSubscription>> {
  const { data } = await apiClient.get<PaginatedResponse<UserSubscription>>(
    `/admin/users/${userId}/subscriptions`,
    {
      params: { page, page_size: pageSize }
    }
  )
  return data
}

export const subscriptionsAPI = {
  list,
  getById,
  listRefundRequests,
  getRefundRequest,
  getProgress,
  assign,
  bulkAssign,
  bulkExtend,
  bulkResetQuota,
  extend,
  revoke,
  uploadRefundProof,
  processRefundGateway,
  completeRefund,
  cancelRefund,
  resetQuota,
  listByUser
}

export default subscriptionsAPI
