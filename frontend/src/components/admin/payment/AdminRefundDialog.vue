<template>
  <BaseDialog
    :show="show"
    :title="t('payment.admin.refundOrder')"
    width="normal"
    @close="emit('cancel')"
  >
    <form id="refund-form" @submit.prevent="handleSubmit" class="space-y-4">
      <div
        v-if="isSubscriptionOrder"
        class="rounded-lg border border-amber-200 bg-amber-50 p-3 text-sm text-amber-800 dark:border-amber-800 dark:bg-amber-900/20 dark:text-amber-200"
      >
        {{ t('payment.admin.subscriptionRefundRedirect') }}
      </div>

      <!-- Refund Request Info -->
      <div
        v-if="order?.refund_requested_at || order?.refund_request_reason"
        class="rounded-lg border border-violet-200 bg-violet-50 p-3 dark:border-violet-800 dark:bg-violet-900/20"
      >
        <div class="flex items-center gap-2 text-sm font-medium text-violet-700 dark:text-violet-300">
          <svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
          </svg>
          {{ t('payment.admin.refundRequestInfo') }}
        </div>
        <div v-if="order?.refund_requested_at" class="mt-2 flex justify-between text-sm">
          <span class="text-violet-600 dark:text-violet-400">{{ t('payment.admin.refundRequestedAt') }}</span>
          <span class="text-violet-800 dark:text-violet-200">{{ formatDateTime(order.refund_requested_at) }}</span>
        </div>
        <div v-if="order?.refund_request_reason" class="mt-1 text-sm">
          <span class="text-violet-600 dark:text-violet-400">{{ t('payment.admin.refundRequestReason') }}:</span>
          <span class="ml-1 text-violet-800 dark:text-violet-200">{{ order.refund_request_reason }}</span>
        </div>
      </div>

      <!-- Order Info -->
      <div class="rounded-lg bg-gray-50 p-3 dark:bg-dark-700">
        <div class="flex justify-between text-sm">
          <span class="text-gray-500 dark:text-gray-400">{{ t('payment.orders.orderId') }}</span>
          <span class="font-mono text-gray-900 dark:text-white">#{{ order?.id }}</span>
        </div>
        <div class="mt-1 flex justify-between text-sm">
          <span class="text-gray-500 dark:text-gray-400">{{ t('payment.orders.creditedAmount') }}</span>
          <span class="font-medium text-gray-900 dark:text-white">{{ order?.order_type === 'balance' ? '$' : '¥' }}{{ order?.amount?.toFixed(4) }}</span>
        </div>
        <div class="mt-1 flex justify-between text-sm">
          <span class="text-gray-500 dark:text-gray-400">{{ t('payment.orders.payAmount') }}</span>
          <span class="font-medium text-gray-900 dark:text-white">¥{{ order?.pay_amount?.toFixed(4) }}</span>
        </div>
        <div v-if="actuallyRefunded > 0" class="mt-1 flex justify-between text-sm">
          <span class="text-gray-500 dark:text-gray-400">{{ t('payment.admin.alreadyRefunded') }}</span>
          <span class="font-medium text-red-600 dark:text-red-400">{{ order?.order_type === 'balance' ? '$' : '¥' }}{{ actuallyRefunded.toFixed(4) }}</span>
        </div>
      </div>

      <!-- Refund Preview -->
      <div class="rounded-lg border border-blue-100 bg-blue-50 p-3 dark:border-blue-900/40 dark:bg-blue-900/20">
        <div class="text-sm font-medium text-blue-800 dark:text-blue-200">{{ t('payment.admin.refundPreview') }}</div>
        <div v-if="previewLoading" class="mt-2 text-sm text-blue-700 dark:text-blue-300">
          {{ t('payment.admin.refundPreviewLoading') }}
        </div>
        <div v-else-if="refundPreview" class="mt-2 space-y-1 text-sm">
          <div class="flex justify-between">
            <span class="text-blue-700 dark:text-blue-300">{{ t('payment.admin.effectiveRefundAmount') }}</span>
            <span class="font-semibold text-blue-950 dark:text-blue-100">{{ formatOrderMoney(refundPreview.refund_amount) }}</span>
          </div>
          <div v-if="refundPreview.settlement_head" class="flex justify-between">
            <span class="text-blue-700 dark:text-blue-300">{{ t('payment.admin.currentResidualValue') }}</span>
            <span class="font-medium text-blue-950 dark:text-blue-100">{{ formatOrderMoney(refundPreview.settlement_head.current_residual_value) }}</span>
          </div>
          <div v-if="refundPreview.settlement_head" class="flex justify-between">
            <span class="text-blue-700 dark:text-blue-300">{{ t('payment.admin.refundResidualValue') }}</span>
            <span class="font-medium text-blue-950 dark:text-blue-100">{{ formatOrderMoney(refundPreview.settlement_head.refund_residual_value) }}</span>
          </div>
          <div class="flex justify-between">
            <span class="text-blue-700 dark:text-blue-300">{{ t('payment.admin.gatewayRefundAmount') }}</span>
            <span class="font-medium text-blue-950 dark:text-blue-100">¥{{ refundPreview.gateway_amount.toFixed(4) }}</span>
          </div>
        </div>
        <div v-else-if="previewError" class="mt-2 text-sm text-red-700 dark:text-red-300">
          {{ previewError }}
        </div>
      </div>
      <div
        v-if="refundPreview?.settlement_head"
        class="space-y-3 rounded-lg border border-amber-200 bg-amber-50 p-3 text-sm text-amber-900 dark:border-amber-800 dark:bg-amber-900/20 dark:text-amber-100"
      >
        <div class="font-medium text-amber-800 dark:text-amber-200">
          {{ t('payment.admin.calculationTitle') }}
        </div>
        <p>{{ t('payment.admin.calculationIntro') }}</p>
        <div class="space-y-1 font-mono text-xs sm:text-sm">
          <p>
            {{ t('payment.admin.calculationResidualFormula') }}:
            {{ formatOrderMoney(refundPreview.settlement_head.current_residual_value) }}
          </p>
          <p>
            {{ t('payment.admin.calculationGatewayFormula') }}:
            {{ formatGatewayMoney(refundPreview.gateway_amount) }}
          </p>
          <p>
            {{ t('payment.admin.calculationManualFormula') }}:
            {{ formatOrderMoney(refundPreview.settlement_head.current_residual_value) }} - {{ formatGatewayMoney(refundPreview.gateway_amount) }}
            = {{ formatGatewayMoney(manualDifference) }}
          </p>
        </div>
        <p class="text-xs text-amber-700 dark:text-amber-300">
          {{ refundPreview?.manual_transfer_required ? t('payment.admin.calculationManualRequiredHint') : t('payment.admin.calculationManualWaivedHint') }}
        </p>
      </div>

      <!-- Deduct Balance -->
      <div>
        <div class="flex items-center gap-2">
          <input
            id="deduct-balance"
            v-model="form.deduct_balance"
            type="checkbox"
            class="h-4 w-4 rounded border-gray-300 text-primary-600 focus:ring-primary-500"
          />
          <label for="deduct-balance" class="text-sm text-gray-700 dark:text-gray-300">
            {{ deductLabel }}
          </label>
          <span class="text-xs text-gray-500 dark:text-gray-400">{{ deductHint }}</span>
        </div>

        <!-- User Balance Info (when deduct_balance is checked) -->
        <div v-if="form.deduct_balance && userBalance != null" class="mt-3 grid grid-cols-2 gap-3">
          <div class="rounded-lg bg-gray-50 p-3 text-sm dark:bg-dark-700">
            <div class="text-gray-500 dark:text-gray-400">{{ t('payment.admin.userBalance') }}</div>
            <div class="mt-1 font-semibold text-gray-900 dark:text-white">${{ userBalance.toFixed(2) }}</div>
          </div>
          <div class="rounded-lg bg-gray-50 p-3 text-sm dark:bg-dark-700">
            <div class="text-gray-500 dark:text-gray-400">{{ t('payment.admin.orderAmount') }}</div>
            <div class="mt-1 font-semibold text-gray-900 dark:text-white">{{ order?.order_type === 'balance' ? '$' : '¥' }}{{ order?.amount?.toFixed(4) }}</div>
          </div>
        </div>

        <!-- Insufficient balance warning -->
        <div
          v-if="form.deduct_balance && balanceInsufficient"
          class="mt-2 rounded-lg bg-amber-50 p-3 text-sm text-amber-700 dark:bg-amber-900/20 dark:text-amber-300"
        >
          {{ t('payment.admin.insufficientBalance') }}
        </div>

        <!-- No deduction info -->
        <div
          v-if="!form.deduct_balance"
          class="mt-2 rounded-lg bg-blue-50 p-3 text-sm text-blue-700 dark:bg-blue-900/20 dark:text-blue-300"
        >
          {{ t('payment.admin.noDeduction') }}
        </div>
      </div>

      <!-- Refund Amount -->
      <div>
        <label class="input-label">{{ t('payment.admin.refundAmount') }}</label>
        <div class="relative">
          <span class="absolute left-3 top-1/2 -translate-y-1/2 text-gray-500">{{ order?.order_type === 'balance' ? '$' : '¥' }}</span>
          <input
            v-model.number="form.amount"
            type="number"
            step="0.01"
            min="0.01"
            :max="maxRefundable"
            class="input pl-7"
            required
          />
        </div>
        <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
          {{ t('payment.admin.maxRefundable') }}: {{ order?.order_type === 'balance' ? '$' : '¥' }}{{ maxRefundable.toFixed(4) }}
        </p>
      </div>

      <!-- Reason -->
      <div>
        <label class="input-label">{{ t('payment.admin.refundReason') }}</label>
        <textarea
          v-model="form.reason"
          rows="3"
          class="input"
          :placeholder="t('payment.admin.refundReasonPlaceholder')"
          required
        ></textarea>
      </div>

      <!-- Warning -->
      <div
        v-if="effectiveWarning"
        class="rounded-lg bg-yellow-50 p-3 text-sm text-yellow-700 dark:bg-yellow-900/20 dark:text-yellow-300"
      >
        {{ effectiveWarning }}
      </div>

      <!-- Force Refund -->
      <div v-if="effectiveRequireForce" class="flex items-center gap-2">
        <input
          id="force-refund"
          v-model="form.force"
          type="checkbox"
          class="h-4 w-4 rounded border-gray-300 text-red-600 focus:ring-red-500"
        />
        <label for="force-refund" class="text-sm font-medium text-red-600 dark:text-red-400">
          {{ t('payment.admin.forceRefund') }}
        </label>
      </div>
    </form>

    <template #footer>
      <div class="flex justify-end gap-3">
        <button type="button" @click="emit('cancel')" class="btn btn-secondary">
          {{ t('common.cancel') }}
        </button>
        <button
          type="submit"
          form="refund-form"
          :disabled="isSubscriptionOrder || submitting || previewLoading || !!previewError || form.amount <= 0 || (effectiveRequireForce && !form.force)"
          class="rounded-md bg-red-600 px-4 py-2 text-sm font-medium text-white hover:bg-red-700 focus:outline-none focus:ring-2 focus:ring-red-500 focus:ring-offset-2 disabled:opacity-50 dark:focus:ring-offset-dark-800"
        >
          {{ submitting ? t('common.processing') : t('payment.admin.confirmRefund') }}
        </button>
      </div>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { reactive, computed, watch, ref, onBeforeUnmount } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminPaymentAPI } from '@/api/admin/payment'
import BaseDialog from '@/components/common/BaseDialog.vue'
import type { PaymentOrder, RefundPreview } from '@/types/payment'
import { formatOrderDateTime } from '@/components/payment/orderUtils'
import { extractI18nErrorMessage } from '@/utils/apiError'

const { t } = useI18n()

const props = defineProps<{
  show: boolean
  order: PaymentOrder | null
  submitting?: boolean
  userBalance?: number | null
  requireForce?: boolean
  warning?: string
}>()

const emit = defineEmits<{
  (e: 'confirm', data: { amount: number; reason: string; deduct_balance: boolean; force: boolean }): void
  (e: 'cancel'): void
}>()

const form = reactive({
  amount: 0,
  reason: '',
  deduct_balance: true,
  force: false,
})
const refundPreview = ref<RefundPreview | null>(null)
const previewLoading = ref(false)
const previewError = ref('')
let previewTimer: ReturnType<typeof setTimeout> | null = null
const isSubscriptionOrder = computed(() => props.order?.order_type === 'subscription')

// In REFUND_REQUESTED status, refund_amount is the REQUESTED amount, not actually refunded.
// Only PARTIALLY_REFUNDED / REFUNDED have real refund amounts.
const actuallyRefunded = computed(() => {
  if (!props.order) return 0
  const s = props.order.status
  if (s === 'PARTIALLY_REFUNDED' || s === 'REFUNDED') return props.order.refund_amount || 0
  return 0
})

const maxRefundable = computed(() => {
  if (!props.order) return 0
  return props.order.amount - actuallyRefunded.value
})

const balanceInsufficient = computed(() => {
  if (props.userBalance == null || !props.order) return false
  return props.userBalance < props.order.amount
})

const effectiveRequireForce = computed(() => props.requireForce || !!refundPreview.value?.require_force)
const effectiveWarning = computed(() => refundPreview.value?.warning || props.warning || '')
const deductLabel = computed(() => props.order?.order_type === 'subscription' ? t('payment.admin.deductSubscription') : t('payment.admin.deductBalance'))
const deductHint = computed(() => props.order?.order_type === 'subscription' ? t('payment.admin.deductSubscriptionHint') : t('payment.admin.deductBalanceHint'))
const manualDifference = computed(() => {
  if (!refundPreview.value?.settlement_head) return 0
  return Math.max(0, refundPreview.value.settlement_head.current_residual_value - refundPreview.value.gateway_amount)
})

watch(() => props.show, (val) => {
  if (val && props.order) {
    // For REFUND_REQUESTED, pre-fill with the requested amount
    if (props.order.status === 'REFUND_REQUESTED' && props.order.refund_amount) {
      form.amount = props.order.refund_amount
    } else {
      form.amount = maxRefundable.value
    }
    form.reason = props.order.refund_request_reason || ''
    form.deduct_balance = true
    form.force = false
    refundPreview.value = null
    previewError.value = ''
    if (!isSubscriptionOrder.value) {
      schedulePreview()
    }
  } else {
    clearPreviewTimer()
    refundPreview.value = null
    previewError.value = ''
  }
})

watch(() => [form.amount, form.deduct_balance, form.force], () => {
  if (props.show && props.order && !isSubscriptionOrder.value) schedulePreview()
})

function formatDateTime(dateStr: string): string {
  return formatOrderDateTime(dateStr)
}

function formatOrderMoney(amount: number): string {
  const symbol = props.order?.order_type === 'balance' ? '$' : '¥'
  return `${symbol}${amount.toFixed(4)}`
}

function formatGatewayMoney(amount: number): string {
  return `¥${amount.toFixed(4)}`
}

function clearPreviewTimer() {
  if (previewTimer) {
    clearTimeout(previewTimer)
    previewTimer = null
  }
}

function schedulePreview() {
  clearPreviewTimer()
  previewTimer = setTimeout(() => {
    void loadRefundPreview()
  }, 200)
}

async function loadRefundPreview() {
  if (!props.order || isSubscriptionOrder.value) return
  previewLoading.value = true
  previewError.value = ''
  try {
    const { data } = await adminPaymentAPI.previewRefund(props.order.id, {
      amount: form.amount,
      reason: form.reason,
      deduct_balance: form.deduct_balance,
      force: form.force,
    })
    refundPreview.value = data
  } catch (err: unknown) {
    refundPreview.value = null
    previewError.value = extractI18nErrorMessage(err, t, 'payment.errors', t('payment.admin.refundPreviewFailed'))
  } finally {
    previewLoading.value = false
  }
}

function handleSubmit() {
  if (isSubscriptionOrder.value) return
  if (form.amount <= 0 || form.amount > maxRefundable.value) return
  if (effectiveRequireForce.value && !form.force) return
  if (previewLoading.value || previewError.value) return
  emit('confirm', { ...form })
}

onBeforeUnmount(() => clearPreviewTimer())
</script>
