<template>
  <AppLayout>
    <div class="space-y-4">
      <!-- Filters -->
      <div class="card p-4">
        <div class="flex flex-wrap items-center gap-3">
          <Select v-model="currentFilter" :options="statusFilters" class="w-36" @change="fetchOrders" />
          <div class="flex flex-1 items-center justify-end gap-2">
            <button @click="fetchOrders" :disabled="loading" class="btn btn-secondary" :title="t('common.refresh')">
              <Icon name="refresh" size="md" :class="loading ? 'animate-spin' : ''" />
            </button>
            <button class="btn btn-primary" @click="router.push('/purchase')">{{ t('payment.result.backToRecharge') }}</button>
          </div>
        </div>
      </div>

      <!-- Table -->
      <OrderTable :orders="orders" :loading="loading">
        <template #actions="{ row }">
          <div class="flex items-center gap-2">
            <button v-if="row.status === 'PENDING'" @click="handleCancel(row.id)" class="inline-flex items-center gap-1 rounded-md px-2 py-1 text-xs font-medium text-yellow-600 hover:bg-yellow-50 dark:text-yellow-400 dark:hover:bg-yellow-900/20">
              <Icon name="x" size="sm" />
              <span>{{ t('payment.orders.cancel') }}</span>
            </button>
            <button v-if="canRequestRefund(row)" @click="handleRefundEntry(row)" class="inline-flex items-center gap-1 rounded-md px-2 py-1 text-xs font-medium text-purple-600 hover:bg-purple-50 dark:text-purple-400 dark:hover:bg-purple-900/20">
              <Icon name="dollar" size="sm" />
              <span>{{ t('payment.orders.requestRefund') }}</span>
            </button>
          </div>
        </template>
      </OrderTable>

      <!-- Pagination -->
      <Pagination
        v-if="pagination.total > 0"
        :page="pagination.page"
        :total="pagination.total"
        :page-size="pagination.page_size"
        @update:page="handlePageChange"
        @update:pageSize="handlePageSizeChange"
      />
    </div>

    <!-- Cancel Confirm Dialog -->
    <BaseDialog :show="!!cancelTargetId" :title="t('payment.orders.cancel')" width="narrow" @close="cancelTargetId = null">
      <p class="text-sm text-gray-600 dark:text-gray-300">{{ t('payment.confirmCancel') }}</p>
      <template #footer>
        <div class="flex justify-end gap-3">
          <button class="btn btn-secondary" @click="cancelTargetId = null">{{ t('common.cancel') }}</button>
          <button class="btn btn-danger" :disabled="actionLoading" @click="confirmCancel">{{ actionLoading ? t('common.processing') : t('payment.orders.cancel') }}</button>
        </div>
      </template>
    </BaseDialog>

    <!-- Refund Dialog -->
    <BaseDialog :show="!!refundTarget" :title="t('payment.orders.requestRefund')" @close="closeRefundDialog">
      <div v-if="refundTarget" class="space-y-4">
        <div class="rounded-xl bg-gray-50 p-4 dark:bg-dark-800">
          <div class="flex justify-between text-sm">
            <span class="text-gray-500 dark:text-gray-400">{{ t('payment.orders.orderId') }}</span>
            <span class="font-mono text-gray-900 dark:text-white">#{{ refundTarget.id }}</span>
          </div>
          <div class="mt-2 flex justify-between text-sm">
            <span class="text-gray-500 dark:text-gray-400">{{ t('payment.orders.amount') }}</span>
            <span class="text-gray-900 dark:text-white">{{ formatOrderMoney(refundTarget.amount, refundTarget) }}</span>
          </div>
        </div>
        <div class="rounded-lg border border-blue-100 bg-blue-50 p-3 dark:border-blue-900/40 dark:bg-blue-900/20">
          <div class="text-sm font-medium text-blue-800 dark:text-blue-200">{{ t('payment.refundPreview') }}</div>
          <div v-if="refundPreviewLoading" class="mt-2 text-sm text-blue-700 dark:text-blue-300">
            {{ t('payment.refundPreviewLoading') }}
          </div>
          <div v-else-if="refundPreview" class="mt-2 space-y-1 text-sm">
            <div class="flex justify-between">
              <span class="text-blue-700 dark:text-blue-300">{{ t('payment.effectiveRefundAmount') }}</span>
              <span class="font-semibold text-blue-950 dark:text-blue-100">{{ formatOrderMoney(refundPreview.refund_amount, refundTarget) }}</span>
            </div>
            <div v-if="refundPreview.settlement_head" class="flex justify-between">
              <span class="text-blue-700 dark:text-blue-300">{{ t('payment.refundResidualValue') }}</span>
              <span class="font-medium text-blue-950 dark:text-blue-100">{{ formatOrderMoney(refundPreview.settlement_head.refund_residual_value, refundTarget) }}</span>
            </div>
            <div class="flex justify-between">
              <span class="text-blue-700 dark:text-blue-300">{{ t('payment.gatewayRefundAmount') }}</span>
              <span class="font-medium text-blue-950 dark:text-blue-100">¥{{ refundPreview.gateway_amount.toFixed(4) }}</span>
            </div>
            <p v-if="refundPreview.warning" class="pt-1 text-xs text-amber-700 dark:text-amber-300">{{ refundPreview.warning }}</p>
          </div>
          <div v-else-if="refundPreviewError" class="mt-2 text-sm text-red-700 dark:text-red-300">
            {{ refundPreviewError }}
          </div>
        </div>
        <div
          v-if="refundPreview && refundPreview.settlement_head"
          class="space-y-3 rounded-lg border border-amber-200 bg-amber-50 p-3 text-sm text-amber-900 dark:border-amber-800 dark:bg-amber-900/20 dark:text-amber-100"
        >
          <div class="font-medium text-amber-800 dark:text-amber-200">
            {{ t('payment.calculationTitle') }}
          </div>
          <p>{{ t('payment.calculationIntro') }}</p>
          <div class="space-y-1 font-mono text-xs sm:text-sm">
            <p>
              {{ t('payment.calculationResidualFormula') }}:
              {{ formatOrderMoney(refundPreview.settlement_head.current_residual_value, refundTarget) }}
            </p>
            <p>
              {{ t('payment.calculationGatewayFormula') }}:
              {{ formatGatewayMoney(refundPreview.gateway_amount) }}
            </p>
            <p>
              {{ t('payment.calculationManualFormula') }}:
              {{ formatOrderMoney(refundPreview.settlement_head.current_residual_value, refundTarget) }} - {{ formatGatewayMoney(refundPreview.gateway_amount) }}
              = {{ formatGatewayMoney(manualDifference) }}
            </p>
          </div>
          <p class="text-xs text-amber-700 dark:text-amber-300">
            {{ refundPreview?.manual_transfer_required ? t('payment.calculationManualRequiredHint') : t('payment.calculationManualWaivedHint') }}
          </p>
        </div>
        <div>
          <label class="input-label">{{ t('payment.refundReason') }}</label>
          <textarea v-model="refundReason" rows="3" class="input mt-1 w-full" :placeholder="t('payment.refundReasonPlaceholder')" />
        </div>
      </div>
      <template #footer>
        <div class="flex justify-end gap-3">
          <button class="btn btn-secondary" @click="closeRefundDialog">{{ t('common.cancel') }}</button>
          <button class="btn btn-primary" :disabled="refundSubmitDisabled" @click="confirmRefund">{{ actionLoading ? t('common.processing') : t('payment.orders.requestRefund') }}</button>
        </div>
      </template>
    </BaseDialog>
  </AppLayout>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import { useAppStore } from '@/stores'
import { paymentAPI } from '@/api/payment'
import { extractI18nErrorMessage } from '@/utils/apiError'
import type { PaymentOrder, RefundPreview } from '@/types/payment'
import AppLayout from '@/components/layout/AppLayout.vue'
import Pagination from '@/components/common/Pagination.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Select from '@/components/common/Select.vue'
import Icon from '@/components/icons/Icon.vue'
import OrderTable from '@/components/payment/OrderTable.vue'

const { t } = useI18n()
const router = useRouter()
const appStore = useAppStore()

const loading = ref(false)
const actionLoading = ref(false)
const orders = ref<PaymentOrder[]>([])
const refundEligibleProviders = ref<Set<string>>(new Set())
const currentFilter = ref('')
const cancelTargetId = ref<number | null>(null)
const refundTarget = ref<PaymentOrder | null>(null)
const refundReason = ref('')
const refundPreview = ref<RefundPreview | null>(null)
const refundPreviewLoading = ref(false)
const refundPreviewError = ref('')
const pagination = reactive({ page: 1, page_size: 20, total: 0 })

const statusFilters = computed(() => [
  { value: '', label: t('common.all') },
  { value: 'PENDING', label: t('payment.status.pending') },
  { value: 'COMPLETED', label: t('payment.status.completed') },
  { value: 'FAILED', label: t('payment.status.failed') },
  { value: 'REFUNDED', label: t('payment.status.refunded') },
])

async function fetchOrders() {
  loading.value = true
  try {
    const res = await paymentAPI.getMyOrders({
      page: pagination.page,
      page_size: pagination.page_size,
      status: currentFilter.value || undefined,
    })
    orders.value = res.data.items || []
    pagination.total = res.data.total || 0
  } catch (err: unknown) {
    appStore.showError(extractI18nErrorMessage(err, t, 'payment.errors', t('common.error')))
  } finally {
    loading.value = false
  }
}

function handlePageChange(page: number) { pagination.page = page; fetchOrders() }
function handlePageSizeChange(size: number) { pagination.page_size = size; pagination.page = 1; fetchOrders() }

function handleCancel(orderId: number) { cancelTargetId.value = orderId }

async function confirmCancel() {
  if (!cancelTargetId.value) return
  actionLoading.value = true
  try {
    await paymentAPI.cancelOrder(cancelTargetId.value)
    appStore.showSuccess(t('common.success'))
    cancelTargetId.value = null
    await fetchOrders()
  } catch (err: unknown) {
    appStore.showError(extractI18nErrorMessage(err, t, 'payment.errors', t('common.error')))
  } finally {
    actionLoading.value = false
  }
}

const refundSubmitDisabled = computed(() => (
  actionLoading.value ||
  refundPreviewLoading.value ||
  !!refundPreviewError.value ||
  !!refundPreview.value?.require_force ||
  !refundReason.value.trim()
))

const manualDifference = computed(() => {
  if (!refundPreview.value?.settlement_head) return 0
  return Math.max(0, refundPreview.value.settlement_head.current_residual_value - refundPreview.value.gateway_amount)
})

function formatOrderMoney(amount: number, order: PaymentOrder): string {
  const symbol = order.order_type === 'balance' ? '$' : '¥'
  return `${symbol}${amount.toFixed(4)}`
}

function formatGatewayMoney(amount: number): string {
  return `¥${amount.toFixed(4)}`
}

async function openRefundDialog(order: PaymentOrder) {
  refundTarget.value = order
  refundReason.value = ''
  refundPreview.value = null
  refundPreviewError.value = ''
  refundPreviewLoading.value = true
  try {
    const res = await paymentAPI.previewRefund(order.id)
    refundPreview.value = res.data
  } catch (err: unknown) {
    refundPreviewError.value = extractI18nErrorMessage(err, t, 'payment.errors', t('payment.refundPreviewFailed'))
  } finally {
    refundPreviewLoading.value = false
  }
}

async function handleRefundEntry(order: PaymentOrder) {
  if (order.order_type === 'subscription') {
    appStore.showInfo(t('payment.subscriptionRefundRedirect'))
    await router.push('/subscriptions')
    return
  }
  await openRefundDialog(order)
}

function closeRefundDialog() {
  refundTarget.value = null
  refundReason.value = ''
  refundPreview.value = null
  refundPreviewError.value = ''
}

async function confirmRefund() {
  if (!refundTarget.value || !refundReason.value.trim()) return
  if (refundSubmitDisabled.value) return
  actionLoading.value = true
  try {
    await paymentAPI.requestRefund(refundTarget.value.id, { reason: refundReason.value.trim() })
    appStore.showSuccess(t('common.success'))
    closeRefundDialog()
    await fetchOrders()
  } catch (err: unknown) {
    appStore.showError(extractI18nErrorMessage(err, t, 'payment.errors', t('common.error')))
  } finally {
    actionLoading.value = false
  }
}

function canRequestRefund(order: PaymentOrder): boolean {
  if (order.status !== 'COMPLETED') return false
  if (order.order_type === 'subscription') return true
  if (!order.provider_instance_id) return false
  return refundEligibleProviders.value.has(order.provider_instance_id)
}

async function loadRefundEligibility() {
  try {
    const res = await paymentAPI.getRefundEligibleProviders()
    refundEligibleProviders.value = new Set(res.data.provider_instance_ids || [])
  } catch { /* ignore — default to hiding refund button */ }
}

onMounted(() => { fetchOrders(); loadRefundEligibility() })
</script>
