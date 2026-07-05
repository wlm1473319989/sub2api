/**
 * Admin API Keys API endpoints
 * Handles API key management for administrators
 */

import { apiClient } from '../client'
import type { ApiKey } from '@/types'

export interface UpdateApiKeyGroupResult {
  api_key: ApiKey
  auto_granted_group_access: boolean
  granted_group_id?: number
  granted_group_name?: string
}

/**
 * Update an API key's admin-managed fields
 * @param id - API Key ID
 * @param groupId - Group ID (0 to unbind, positive to bind, null/undefined to skip)
 * @param allowPaidFailover - Whether paid backup failover is allowed for this key
 * @returns Updated API key with auto-grant info
 */
export async function updateApiKeyGroup(
  id: number,
  groupId: number | null,
  allowPaidFailover?: boolean
): Promise<UpdateApiKeyGroupResult> {
  const payload: { group_id: number; allow_paid_failover?: boolean } = {
    group_id: groupId === null ? 0 : groupId
  }
  if (allowPaidFailover !== undefined) {
    payload.allow_paid_failover = allowPaidFailover
  }
  const { data } = await apiClient.put<UpdateApiKeyGroupResult>(`/admin/api-keys/${id}`, payload)
  return data
}

export async function updateApiKeyAllowPaidFailover(
  id: number,
  allowPaidFailover: boolean
): Promise<UpdateApiKeyGroupResult> {
  const { data } = await apiClient.put<UpdateApiKeyGroupResult>(`/admin/api-keys/${id}`, {
    allow_paid_failover: allowPaidFailover
  })
  return data
}

export const apiKeysAPI = {
  updateApiKeyGroup,
  updateApiKeyAllowPaidFailover
}

export default apiKeysAPI
