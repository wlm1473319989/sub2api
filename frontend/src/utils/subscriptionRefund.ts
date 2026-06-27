import type {
  SubscriptionRefundAllocation,
  SubscriptionRefundAllocationStatus,
  SubscriptionRefundMode,
  SubscriptionRefundRequest,
  SubscriptionRefundStatus,
  UserSubscription,
} from '@/types'

type TranslateFn = (key: string, params?: Record<string, unknown>) => string

function translated(t: TranslateFn, key: string, fallback: string): string {
  const label = t(key)
  return label === key ? fallback : label
}

export function subscriptionRefundStatusLabel(t: TranslateFn, status: SubscriptionRefundStatus | string): string {
  return translated(t, `subscriptionRefundRequests.status.${status}`, status)
}

export function subscriptionRefundStatusClass(status: SubscriptionRefundStatus | string): string {
  switch (status) {
    case 'cancelled':
      return 'bg-gray-100 text-gray-600 dark:bg-dark-700 dark:text-gray-300'
    case 'submitted':
      return 'bg-blue-100 text-blue-700 dark:bg-blue-900/40 dark:text-blue-300'
    case 'gateway_processing':
      return 'bg-sky-100 text-sky-700 dark:bg-sky-900/40 dark:text-sky-300'
    case 'manual_pending':
      return 'bg-amber-100 text-amber-700 dark:bg-amber-900/40 dark:text-amber-300'
    case 'completed':
      return 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/40 dark:text-emerald-300'
    case 'failed':
      return 'bg-red-100 text-red-700 dark:bg-red-900/40 dark:text-red-300'
    default:
      return 'bg-gray-100 text-gray-600 dark:bg-dark-700 dark:text-gray-300'
  }
}

export function subscriptionRefundModeLabel(t: TranslateFn, mode: SubscriptionRefundMode | string): string {
  return translated(t, `subscriptionRefundRequests.modes.${mode}`, mode)
}

export function subscriptionRefundAllocationStatusLabel(
  t: TranslateFn,
  status: SubscriptionRefundAllocationStatus | string
): string {
  return translated(t, `subscriptionRefundRequests.allocationStatus.${status}`, status)
}

export function subscriptionRefundAllocationStatusClass(
  status: SubscriptionRefundAllocationStatus | string
): string {
  switch (status) {
    case 'succeeded':
      return 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/40 dark:text-emerald-300'
    case 'processing':
      return 'bg-blue-100 text-blue-700 dark:bg-blue-900/40 dark:text-blue-300'
    case 'failed':
      return 'bg-red-100 text-red-700 dark:bg-red-900/40 dark:text-red-300'
    case 'skipped':
      return 'bg-gray-100 text-gray-600 dark:bg-dark-700 dark:text-gray-300'
    case 'pending':
    default:
      return 'bg-amber-100 text-amber-700 dark:bg-amber-900/40 dark:text-amber-300'
  }
}

export function subscriptionStatusLabel(t: TranslateFn, status?: UserSubscription['status'] | string | null): string {
  if (!status) return t('common.unknown')
  return translated(t, `userSubscriptions.status.${status}`, status)
}

export function subscriptionStatusClass(status?: UserSubscription['status'] | string | null): string {
  switch (status) {
    case 'active':
      return 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/40 dark:text-emerald-300'
    case 'suspended':
      return 'bg-amber-100 text-amber-700 dark:bg-amber-900/40 dark:text-amber-300'
    case 'expired':
      return 'bg-gray-100 text-gray-600 dark:bg-dark-700 dark:text-gray-300'
    case 'superseded':
      return 'bg-indigo-100 text-indigo-700 dark:bg-indigo-900/40 dark:text-indigo-300'
    case 'refunded':
      return 'bg-purple-100 text-purple-700 dark:bg-purple-900/40 dark:text-purple-300'
    case 'revoked':
    default:
      return 'bg-red-100 text-red-700 dark:bg-red-900/40 dark:text-red-300'
  }
}

export function isRefundFrozenSubscription(request: Pick<SubscriptionRefundRequest, 'status' | 'subscription'>): boolean {
  return request.subscription?.status === 'suspended' &&
    ['submitted', 'gateway_processing', 'manual_pending', 'failed'].includes(request.status)
}

export function subscriptionDisplayStatusLabelForRefundRequest(
  t: TranslateFn,
  request: Pick<SubscriptionRefundRequest, 'status' | 'subscription'>
): string {
  if (isRefundFrozenSubscription(request)) {
    return translated(t, 'userSubscriptions.status.suspended_refund', '退款处理中（已冻结）')
  }
  return subscriptionStatusLabel(t, request.subscription?.status)
}

export function hasRefundPayoutEvidence(request: SubscriptionRefundRequest): boolean {
  if (request.manual_transfer_proof_url?.trim()) return true
  return (request.allocations ?? []).some((item) => item.status === 'succeeded')
}

export function canProcessRefundGateway(request: SubscriptionRefundRequest): boolean {
  if (!['submitted', 'gateway_processing', 'manual_pending', 'failed'].includes(request.status)) return false
  if (request.gateway_refundable_total > 0) return true
  return (request.allocations ?? []).some((item) => item.gateway_refund_amount > 0)
}

export function canUploadManualProof(request: SubscriptionRefundRequest): boolean {
  return request.manual_transfer_required && ['submitted', 'gateway_processing', 'manual_pending', 'failed'].includes(request.status)
}

export function canCancelRefundRequest(request: SubscriptionRefundRequest): boolean {
  if (!['submitted', 'gateway_processing', 'manual_pending', 'failed'].includes(request.status)) return false
  return !hasRefundPayoutEvidence(request)
}

export function allocationsReadyForCompletion(allocations: SubscriptionRefundAllocation[] | undefined): boolean {
  if (!allocations || allocations.length === 0) return true
  return allocations.every((item) => !['pending', 'processing', 'failed'].includes(item.status))
}

export function canCompleteRefundRequest(request: SubscriptionRefundRequest): boolean {
  if (!['submitted', 'gateway_processing', 'manual_pending'].includes(request.status)) return false
  if (request.manual_transfer_required && !request.manual_transfer_proof_url?.trim()) return false
  return allocationsReadyForCompletion(request.allocations)
}
