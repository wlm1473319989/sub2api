<template>
  <AppLayout>
    <TablePageLayout>
      <template #filters>
        <div class="flex flex-wrap items-center justify-between gap-3">
          <div class="rounded-xl border border-gray-200 bg-white px-4 py-3 dark:border-dark-700 dark:bg-dark-800">
            <p class="text-xs font-medium uppercase text-gray-500 dark:text-dark-400">
              {{ t('subscriptionRefundRequests.summary.totalRequests') }}
            </p>
            <p class="mt-1 text-2xl font-semibold text-gray-900 dark:text-white">
              {{ pagination.total }}
            </p>
          </div>

          <div class="flex items-center gap-2">
            <button class="btn btn-secondary" :disabled="loading" @click="loadRequests">
              <Icon name="refresh" size="md" :class="loading ? 'animate-spin' : ''" />
            </button>
            <button class="btn btn-primary" @click="router.push('/subscriptions')">
              <Icon name="creditCard" size="sm" class="mr-2" />
              {{ t('subscriptionRefundRequests.actions.goSubscriptions') }}
            </button>
          </div>
        </div>
      </template>

      <template #table>
        <DataTable :columns="columns" :data="requests" :loading="loading">
          <template #cell-id="{ row }">
            <button
              type="button"
              class="font-medium text-primary-600 hover:text-primary-700 dark:text-primary-400 dark:hover:text-primary-300"
              @click="openDetail(row.id)"
            >
              #{{ row.id }}
            </button>
          </template>

          <template #cell-subscription="{ row }">
            <div class="min-w-0">
              <p class="truncate font-medium text-gray-900 dark:text-white">
                {{ subscriptionDisplayName(row) }}
              </p>
              <p class="mt-1 text-xs text-gray-500 dark:text-dark-400">
                {{ t('subscriptionRefundRequests.fields.subscriptionNo') }} #{{ row.subscription_id }}
              </p>
            </div>
          </template>

          <template #cell-status="{ value }">
            <span
              :class="[
                'inline-flex rounded-full px-2 py-0.5 text-xs font-medium',
                subscriptionRefundStatusClass(value)
              ]"
            >
              {{ subscriptionRefundStatusLabel(t, value) }}
            </span>
          </template>

          <template #cell-refund_mode="{ value }">
            <span class="text-sm text-gray-700 dark:text-gray-300">
              {{ subscriptionRefundModeLabel(t, value) }}
            </span>
          </template>

          <template #cell-refund_residual_value="{ row }">
            <div class="text-sm font-medium text-gray-900 dark:text-white">
              {{ formatCurrency(row.refund_residual_value, row.currency || 'CNY', 4) }}
            </div>
          </template>

          <template #cell-gateway="{ row }">
            <div class="space-y-1 text-sm">
              <p class="font-medium text-gray-900 dark:text-white">
                {{ formatCurrency(row.gateway_refunded_total, row.currency || 'CNY', 4) }}
              </p>
              <p class="text-xs text-gray-500 dark:text-dark-400">
                / {{ formatCurrency(row.gateway_refundable_total, row.currency || 'CNY', 4) }}
              </p>
            </div>
          </template>

          <template #cell-submitted_at="{ value }">
            <span class="text-sm text-gray-700 dark:text-gray-300">
              {{ formatDateTime(value) || '-' }}
            </span>
          </template>

          <template #cell-created_at="{ value }">
            <span class="text-sm text-gray-700 dark:text-gray-300">
              {{ formatDateTime(value) || '-' }}
            </span>
          </template>

          <template #cell-actions="{ row }">
            <button
              type="button"
              class="inline-flex items-center gap-1 rounded-md px-2 py-1 text-xs font-medium text-primary-600 hover:bg-primary-50 dark:text-primary-400 dark:hover:bg-primary-900/20"
              @click="openDetail(row.id)"
            >
              <Icon name="eye" size="sm" />
              <span>{{ t('subscriptionRefundRequests.actions.view') }}</span>
            </button>
          </template>

          <template #empty>
            <div class="flex flex-col items-center py-10 text-center">
              <Icon name="document" size="xl" class="mb-4 text-gray-400 dark:text-dark-500" />
              <p class="text-lg font-medium text-gray-900 dark:text-white">
                {{ t('subscriptionRefundRequests.emptyTitle') }}
              </p>
              <p class="mt-2 text-sm text-gray-500 dark:text-dark-400">
                {{ t('subscriptionRefundRequests.emptyDescription') }}
              </p>
            </div>
          </template>
        </DataTable>
      </template>

      <template #pagination>
        <Pagination
          v-if="pagination.total > 0"
          :page="pagination.page"
          :page-size="pagination.page_size"
          :total="pagination.total"
          @update:page="handlePageChange"
          @update:pageSize="handlePageSizeChange"
        />
      </template>
    </TablePageLayout>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import subscriptionsAPI from '@/api/subscriptions'
import type { SubscriptionRefundRequest } from '@/types'
import { useAppStore } from '@/stores'
import { extractI18nErrorMessage } from '@/utils/apiError'
import {
  subscriptionRefundModeLabel,
  subscriptionRefundStatusClass,
  subscriptionRefundStatusLabel,
} from '@/utils/subscriptionRefund'
import { formatCurrency, formatDateTime } from '@/utils/format'
import type { Column } from '@/components/common/types'
import AppLayout from '@/components/layout/AppLayout.vue'
import TablePageLayout from '@/components/layout/TablePageLayout.vue'
import DataTable from '@/components/common/DataTable.vue'
import Pagination from '@/components/common/Pagination.vue'
import Icon from '@/components/icons/Icon.vue'

const { t } = useI18n()
const router = useRouter()
const appStore = useAppStore()

const loading = ref(false)
const requests = ref<SubscriptionRefundRequest[]>([])
const pagination = reactive({
  page: 1,
  page_size: 20,
  total: 0,
})

const columns = computed<Column[]>(() => [
  { key: 'id', label: t('subscriptionRefundRequests.table.id') },
  { key: 'subscription', label: t('subscriptionRefundRequests.table.subscription') },
  { key: 'status', label: t('subscriptionRefundRequests.table.status') },
  { key: 'refund_mode', label: t('subscriptionRefundRequests.table.mode') },
  { key: 'refund_residual_value', label: t('subscriptionRefundRequests.table.residual') },
  { key: 'gateway', label: t('subscriptionRefundRequests.table.gatewayRefund') },
  { key: 'submitted_at', label: t('subscriptionRefundRequests.table.submittedAt') },
  { key: 'created_at', label: t('subscriptionRefundRequests.table.createdAt') },
  { key: 'actions', label: t('subscriptionRefundRequests.table.actions') },
])

async function loadRequests() {
  loading.value = true
  try {
    const response = await subscriptionsAPI.getSubscriptionRefundRequests({
      page: pagination.page,
      page_size: pagination.page_size,
    })
    requests.value = response.items || []
    pagination.total = response.total || 0
  } catch (err: unknown) {
    appStore.showError(
      extractI18nErrorMessage(err, t, 'payment.errors', t('subscriptionRefundRequests.messages.loadFailed'))
    )
  } finally {
    loading.value = false
  }
}

function openDetail(id: number) {
  router.push(`/subscription-refund-requests/${id}`)
}

function handlePageChange(page: number) {
  pagination.page = page
  loadRequests()
}

function handlePageSizeChange(pageSize: number) {
  pagination.page = 1
  pagination.page_size = pageSize
  loadRequests()
}

function subscriptionDisplayName(request: SubscriptionRefundRequest): string {
  const name = request.subscription?.plan_name_snapshot?.trim()
  if (name) return name
  return `${t('payment.plan')} #${request.subscription_id}`
}

onMounted(() => {
  loadRequests()
})
</script>
