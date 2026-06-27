<template>
  <BaseDialog :show="show" :title="t('userSubscriptions.refund.title')" width="wide" @close="handleClose">
    <div v-if="subscription" class="space-y-4">
      <div class="rounded-lg border border-gray-200 bg-gray-50 p-3 dark:border-dark-700 dark:bg-dark-800">
        <div class="flex items-center justify-between gap-3">
          <div>
            <p class="text-sm font-medium text-gray-900 dark:text-white">
              {{ subscriptionDisplayName(subscription) }}
            </p>
            <p class="text-xs text-gray-500 dark:text-dark-400">
              #{{ subscription.id }} · {{ subscription.status }}
            </p>
          </div>
          <span :class="['inline-flex rounded-full px-2 py-0.5 text-xs font-medium', statusClass(subscription.status)]">
            {{ statusLabel(subscription.status) }}
          </span>
        </div>
      </div>

      <div>
        <label class="input-label">{{ t('userSubscriptions.refund.reason') }}</label>
        <textarea v-model="reason" rows="3" class="input mt-1 w-full" :placeholder="t('userSubscriptions.refund.reasonPlaceholder')" />
      </div>

      <div class="flex items-center justify-between gap-3">
        <p class="text-xs text-gray-500 dark:text-dark-400">
          {{ t('userSubscriptions.refund.previewHint') }}
        </p>
        <button class="btn btn-secondary" :disabled="previewLoading" @click="loadPreview">
          <Icon name="refresh" size="sm" :class="previewLoading ? 'animate-spin' : ''" />
          <span class="ml-2">{{ preview ? t('userSubscriptions.refund.repreview') : t('userSubscriptions.refund.preview') }}</span>
        </button>
      </div>

      <div v-if="previewError" class="rounded-lg bg-red-50 p-3 text-sm text-red-700 dark:bg-red-900/20 dark:text-red-300">
        {{ previewError }}
      </div>

      <div v-if="preview" class="space-y-4">
        <div class="rounded-lg border border-blue-100 bg-blue-50 p-3 dark:border-blue-900/40 dark:bg-blue-900/20">
          <div class="grid gap-2 text-sm sm:grid-cols-2">
            <div class="flex justify-between gap-4">
              <span class="text-blue-700 dark:text-blue-300">{{ t('userSubscriptions.refund.currentPlan') }}</span>
              <span class="font-medium text-right text-blue-950 dark:text-blue-100">{{ preview.plan_name || subscriptionDisplayName(subscription) }}</span>
            </div>
            <div class="flex justify-between gap-4">
              <span class="text-blue-700 dark:text-blue-300">{{ t('userSubscriptions.refund.subscriptionExpiresAt') }}</span>
              <span class="font-medium text-blue-950 dark:text-blue-100">{{ formatDateTime(preview.subscription_expires_at || subscription.expires_at) }}</span>
            </div>
            <div class="flex justify-between gap-4">
              <span class="text-blue-700 dark:text-blue-300">{{ t('userSubscriptions.refund.settlementId') }}</span>
              <span class="font-medium text-blue-950 dark:text-blue-100">#{{ preview.settlement_id }}</span>
            </div>
            <div class="flex justify-between gap-4">
              <span class="text-blue-700 dark:text-blue-300">{{ t('userSubscriptions.refund.afterSettlementValue') }}</span>
              <span class="font-medium text-blue-950 dark:text-blue-100">{{ formatMoney(preview.after_settlement_value || 0, preview.currency) }}</span>
            </div>
            <div class="flex justify-between gap-4">
              <span class="text-blue-700 dark:text-blue-300">{{ t('userSubscriptions.refund.residualKnives') }}</span>
              <span class="font-medium text-blue-950 dark:text-blue-100">{{ formatKnives(preview.residual_quota_knives) }}</span>
            </div>
            <div class="flex justify-between gap-4">
              <span class="text-blue-700 dark:text-blue-300">{{ t('userSubscriptions.refund.unitCost') }}</span>
              <span class="font-medium text-blue-950 dark:text-blue-100">{{ formatMoney(preview.unit_cost || 0, preview.currency) }}</span>
            </div>
            <div class="flex justify-between gap-4">
              <span class="text-blue-700 dark:text-blue-300">{{ t('userSubscriptions.refund.residual') }}</span>
              <span class="font-medium text-blue-950 dark:text-blue-100">{{ formatMoney(preview.refund_residual_value, preview.currency) }}</span>
            </div>
            <div class="flex justify-between gap-4">
              <span class="text-blue-700 dark:text-blue-300">{{ t('userSubscriptions.refund.refundMode') }}</span>
              <span class="font-medium text-blue-950 dark:text-blue-100">{{ refundModeLabel(preview.refund_mode) }}</span>
            </div>
            <div class="flex justify-between gap-4">
              <span class="text-blue-700 dark:text-blue-300">{{ t('userSubscriptions.refund.gatewayRefundable') }}</span>
              <span class="font-medium text-blue-950 dark:text-blue-100">{{ formatMoney(preview.gateway_refundable_total, preview.currency) }}</span>
            </div>
            <div class="flex justify-between gap-4">
              <span class="text-blue-700 dark:text-blue-300">{{ t('userSubscriptions.refund.manualTransfer') }}</span>
              <span class="font-medium text-blue-950 dark:text-blue-100">{{ formatMoney(preview.manual_transfer_amount, preview.currency) }}</span>
            </div>
            <div class="flex justify-between gap-4">
              <span class="text-blue-700 dark:text-blue-300">{{ t('userSubscriptions.refund.previewCountdown') }}</span>
              <span class="font-medium text-blue-950 dark:text-blue-100">{{ previewCountdownLabel }}</span>
            </div>
          </div>
          <div class="mt-2 text-xs text-blue-700 dark:text-blue-300">
            {{ t('userSubscriptions.refund.expiresAt') }}: {{ formatDateTime(preview.preview_expires_at) }}
          </div>
          <p v-if="previewExpired" class="mt-2 text-xs text-amber-700 dark:text-amber-300">
            {{ t('userSubscriptions.refund.previewExpired') }}
          </p>
        </div>

        <div class="rounded-lg border border-gray-200 dark:border-dark-700">
          <div class="border-b border-gray-200 px-3 py-2 text-sm font-medium text-gray-700 dark:border-dark-700 dark:text-gray-200">
            {{ t('userSubscriptions.refund.allocations') }}
          </div>
          <div class="divide-y divide-gray-200 dark:divide-dark-700">
            <div v-for="item in preview.allocations" :key="item.payment_order_id" class="grid gap-3 px-3 py-3 text-sm sm:grid-cols-2">
              <div class="min-w-0">
                <p class="font-medium text-gray-900 dark:text-white">#{{ item.payment_order_id }}</p>
                <p class="text-xs text-gray-500 dark:text-dark-400">
                  {{ allocationStatusLabel(item) }}
                </p>
                <p v-if="item.payment_type || item.payment_provider_key" class="text-xs text-gray-500 dark:text-dark-400">
                  {{ allocationPaymentLabel(item) }}
                </p>
              </div>
              <div class="grid gap-1 text-xs text-gray-500 dark:text-dark-400 sm:text-right">
                <span>{{ t('userSubscriptions.refund.orderAmount') }}: {{ formatMoney(item.order_amount, item.currency) }}</span>
                <span>{{ t('userSubscriptions.refund.payAmount') }}: {{ formatMoney(item.order_pay_amount ?? item.pay_amount ?? 0, item.currency) }}</span>
                <span>{{ t('userSubscriptions.refund.alreadyRefundedAmount') }}: {{ formatMoney(item.already_refunded_amount, item.currency) }}</span>
                <span>{{ t('userSubscriptions.refund.refundableOrderAmount') }}: {{ formatMoney(item.refundable_order_amount, item.currency) }}</span>
                <span class="font-medium text-gray-900 dark:text-white">{{ t('userSubscriptions.refund.allocatedRefundValue') }}: {{ formatMoney(item.allocated_refund_value, item.currency) }}</span>
                <span>{{ t('userSubscriptions.refund.gatewayRefundAmount') }}: {{ formatMoney(item.gateway_refund_amount, item.currency) }}</span>
                <span v-if="allocationReason(item)">{{ t('userSubscriptions.refund.unavailableReason') }}: {{ allocationReason(item) }}</span>
              </div>
            </div>
          </div>
        </div>

        <div class="space-y-3 rounded-lg border border-amber-200 bg-amber-50 p-3 text-sm text-amber-900 dark:border-amber-800 dark:bg-amber-900/20 dark:text-amber-100">
          <div class="flex items-center gap-2 font-medium text-amber-800 dark:text-amber-200">
            <Icon name="exclamationTriangle" size="sm" />
            {{ t('userSubscriptions.refund.calculationTitle') }}
          </div>
          <p>{{ t('userSubscriptions.refund.calculationIntro') }}</p>
          <div class="space-y-1 font-mono text-xs sm:text-sm">
            <p>
              {{ t('userSubscriptions.refund.calculationResidualFormula') }}:
              {{ formatKnives(preview.residual_quota_knives) }} × {{ formatMoney(preview.unit_cost || 0, preview.currency) }}
              = {{ formatMoney(preview.refund_residual_value, preview.currency) }}
            </p>
            <p v-if="preview.theoretical_full_max_knives">
              {{ t('userSubscriptions.refund.calculationUnitCostHint', {
                settlement: formatMoney(preview.after_settlement_value || 0, preview.currency),
                knives: formatKnives(preview.theoretical_full_max_knives),
                unitCost: formatMoney(preview.unit_cost || 0, preview.currency),
              }) }}
            </p>
            <p>
              {{ t('userSubscriptions.refund.calculationGatewayFormula') }}:
              {{ formatMoney(preview.gateway_refundable_total, preview.currency) }}
            </p>
            <p>
              {{ t('userSubscriptions.refund.calculationManualFormula') }}:
              {{ formatMoney(preview.refund_residual_value, preview.currency) }} - {{ formatMoney(preview.gateway_refundable_total, preview.currency) }}
              = {{ formatMoney(preview.manual_transfer_amount, preview.currency) }}
            </p>
          </div>
          <p class="text-xs text-amber-700 dark:text-amber-300">
            {{
              preview.manual_transfer_required
                ? t('userSubscriptions.refund.calculationManualRequiredHint')
                : t('userSubscriptions.refund.calculationManualWaivedHint')
            }}
          </p>
        </div>

        <div v-if="preview.manual_transfer_required" class="space-y-3 rounded-lg border border-amber-200 bg-amber-50 p-3 dark:border-amber-800 dark:bg-amber-900/20">
          <div class="flex items-center gap-2 text-sm font-medium text-amber-800 dark:text-amber-200">
            <Icon name="exclamationTriangle" size="sm" />
            {{ t('userSubscriptions.refund.manualTransferRequired') }}
          </div>
          <div class="grid gap-3 sm:grid-cols-2">
            <div>
              <label class="input-label">{{ t('userSubscriptions.refund.receiverType') }}</label>
              <input v-model="manualTransfer.receiver_type" class="input mt-1 w-full" type="text" />
            </div>
            <div>
              <label class="input-label">{{ t('userSubscriptions.refund.receiverName') }}</label>
              <input v-model="manualTransfer.receiver_name" class="input mt-1 w-full" type="text" />
            </div>
            <div>
              <label class="input-label">{{ t('userSubscriptions.refund.receiverAccount') }}</label>
              <input v-model="manualTransfer.receiver_account" class="input mt-1 w-full" type="text" />
            </div>
            <div>
              <label class="input-label">{{ t('userSubscriptions.refund.receiverQrImageUrl') }}</label>
              <input
                v-model="manualTransfer.receiver_qr_image_url"
                class="input mt-1 w-full"
                type="url"
                :placeholder="t('userSubscriptions.refund.receiverQrImageHint')"
              />
            </div>
            <div class="sm:col-span-2">
              <label class="input-label">{{ t('userSubscriptions.refund.receiverRemark') }}</label>
              <textarea
                v-model="manualTransfer.remark"
                rows="2"
                class="input mt-1 w-full"
                :placeholder="t('userSubscriptions.refund.receiverRemarkPlaceholder')"
              />
            </div>
          </div>
        </div>
      </div>
    </div>

    <template #footer>
      <div class="flex items-center justify-end gap-3">
        <button type="button" class="btn btn-secondary" @click="handleClose">
          {{ t('common.cancel') }}
        </button>
        <button
          type="button"
          class="btn btn-primary"
          :disabled="submitDisabled"
          @click="submitRefund"
        >
          <Icon v-if="submitting" name="refresh" size="sm" class="animate-spin" />
          <span :class="submitting ? 'ml-2' : ''">
            {{ submitting ? t('common.processing') : t('userSubscriptions.refund.submit') }}
          </span>
        </button>
      </div>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Icon from '@/components/icons/Icon.vue'
import subscriptionsAPI from '@/api/subscriptions'
import { useAppStore } from '@/stores/app'
import { extractI18nErrorMessage } from '@/utils/apiError'
import type {
  SubscriptionRefundPreviewResponse,
  SubscriptionRefundSubmitResult,
  UserSubscription
} from '@/types'

const props = defineProps<{
  show: boolean
  subscription: UserSubscription | null
}>()

const emit = defineEmits<{
  (e: 'close'): void
  (e: 'submitted', result: SubscriptionRefundSubmitResult): void
}>()

const { t } = useI18n()
const appStore = useAppStore()

const reason = ref('')
const preview = ref<SubscriptionRefundPreviewResponse | null>(null)
const previewLoading = ref(false)
const previewError = ref('')
const submitting = ref(false)
const now = ref(Date.now())
const manualTransfer = reactive({
  receiver_type: '',
  receiver_name: '',
  receiver_account: '',
  receiver_qr_image_url: '',
  remark: '',
})

let timer: ReturnType<typeof setInterval> | null = null

const previewExpired = computed(() => {
  if (!preview.value) return false
  return now.value >= new Date(preview.value.preview_expires_at).getTime()
})

const previewCountdownLabel = computed(() => {
  if (!preview.value) return '-'
  const remainingMs = new Date(preview.value.preview_expires_at).getTime() - now.value
  if (remainingMs <= 0) return t('userSubscriptions.refund.previewExpired')
  const totalSeconds = Math.ceil(remainingMs / 1000)
  const minutes = Math.floor(totalSeconds / 60)
  const seconds = totalSeconds % 60
  return `${minutes}:${String(seconds).padStart(2, '0')}`
})

const submitDisabled = computed(() => {
  if (!props.subscription || !preview.value || previewExpired.value || previewLoading.value || submitting.value) {
    return true
  }
  if (!preview.value.manual_transfer_required) {
    return false
  }
  const receiverType = manualTransfer.receiver_type.trim()
  const receiverName = manualTransfer.receiver_name.trim()
  const receiverAccount = manualTransfer.receiver_account.trim()
  const receiverQr = manualTransfer.receiver_qr_image_url.trim()
  return !receiverType || !receiverName || (!receiverAccount && !receiverQr)
})

watch(
  () => props.show,
  (visible) => {
    if (visible) {
      resetState()
    } else {
      stopTimer()
    }
  }
)

onBeforeUnmount(() => stopTimer())

function resetState() {
  reason.value = ''
  preview.value = null
  previewError.value = ''
  previewLoading.value = false
  submitting.value = false
  now.value = Date.now()
  manualTransfer.receiver_type = ''
  manualTransfer.receiver_name = ''
  manualTransfer.receiver_account = ''
  manualTransfer.receiver_qr_image_url = ''
  manualTransfer.remark = ''
  stopTimer()
}

function startTimer() {
  stopTimer()
  timer = setInterval(() => {
    now.value = Date.now()
  }, 1000)
}

function stopTimer() {
  if (timer) {
    clearInterval(timer)
    timer = null
  }
}

function subscriptionDisplayName(subscription: UserSubscription): string {
  return subscription.plan_name_snapshot?.trim() || `${t('payment.plan')} #${subscription.id}`
}

function statusLabel(status: string): string {
  const key = `userSubscriptions.status.${status}`
  const label = t(key)
  return label === key ? status : label
}

function statusClass(status: string): string {
  switch (status) {
    case 'active':
      return 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/40 dark:text-emerald-300'
    case 'suspended':
      return 'bg-amber-100 text-amber-700 dark:bg-amber-900/40 dark:text-amber-300'
    case 'expired':
      return 'bg-gray-100 text-gray-600 dark:bg-dark-700 dark:text-gray-400'
    case 'refunded':
      return 'bg-purple-100 text-purple-700 dark:bg-purple-900/40 dark:text-purple-300'
    default:
      return 'bg-red-100 text-red-700 dark:bg-red-900/40 dark:text-red-300'
  }
}

function formatMoney(amount: number, currency: string): string {
  return formatMoneyWithPrecision(amount, currency, 4)
}

function formatKnives(value?: number): string {
  if (typeof value !== 'number' || Number.isNaN(value)) return '-'
  return value.toFixed(4)
}

function refundModeLabel(mode: string): string {
  const key = `subscriptionRefundRequests.modes.${mode}`
  const label = t(key)
  return label === key ? mode : label
}

function allocationStatusLabel(item: SubscriptionRefundPreviewResponse['allocations'][number]): string {
  if (item.refund_channel_available != null) {
    return item.refund_channel_available
      ? t('userSubscriptions.refund.channelAvailable')
      : t('userSubscriptions.refund.channelUnavailable')
  }
  if (item.status === 'skipped') {
    return t('userSubscriptions.refund.channelUnavailable')
  }
  return t('userSubscriptions.refund.channelAvailable')
}

function allocationReason(item: SubscriptionRefundPreviewResponse['allocations'][number]): string {
  return item.failed_reason || item.skipped_reason || ''
}

function allocationPaymentLabel(item: SubscriptionRefundPreviewResponse['allocations'][number]): string {
  const parts = [item.payment_type, item.payment_provider_key].filter(Boolean)
  return parts.join(' / ')
}

function formatDateTime(value?: string | null): string {
  if (!value) return '-'
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? '-' : date.toLocaleString()
}

function formatMoneyWithPrecision(amount: number, currency: string, fractionDigits = 4): string {
  const code = currency?.trim() || 'CNY'
  const symbol = code.toUpperCase() === 'CNY' ? '¥' : code.toUpperCase() === 'USD' ? '$' : `${code} `
  return `${symbol}${amount.toFixed(fractionDigits)}`
}

async function loadPreview() {
  if (!props.subscription) return
  previewLoading.value = true
  previewError.value = ''
  try {
    preview.value = await subscriptionsAPI.previewSubscriptionRefund(props.subscription.id, reason.value.trim())
    now.value = Date.now()
    startTimer()
  } catch (err: unknown) {
    preview.value = null
    previewError.value = extractI18nErrorMessage(err, t, 'payment.errors', t('userSubscriptions.refund.previewFailed'))
  } finally {
    previewLoading.value = false
  }
}

async function submitRefund() {
  if (!props.subscription || !preview.value) return
  if (previewExpired.value) {
    appStore.showError(t('userSubscriptions.refund.previewExpired'))
    return
  }

  submitting.value = true
  try {
    const manualTransferPayload = preview.value.manual_transfer_required
      ? {
          receiver_type: manualTransfer.receiver_type.trim(),
          receiver_name: manualTransfer.receiver_name.trim(),
          receiver_account: manualTransfer.receiver_account.trim(),
          receiver_qr_image_url: manualTransfer.receiver_qr_image_url.trim(),
          remark: manualTransfer.remark.trim(),
        }
      : undefined

    const result = await subscriptionsAPI.requestSubscriptionRefund(props.subscription.id, {
      preview_id: preview.value.preview_id,
      preview_token: preview.value.preview_token,
      reason: reason.value.trim(),
      manual_transfer: manualTransferPayload,
    })
    appStore.showSuccess(t('userSubscriptions.refund.submittedWithId', { id: result.refund_request_id }))
    emit('submitted', result)
    handleClose()
  } catch (err: unknown) {
    appStore.showError(extractI18nErrorMessage(err, t, 'payment.errors', t('userSubscriptions.refund.submitFailed')))
  } finally {
    submitting.value = false
  }
}

function handleClose() {
  stopTimer()
  emit('close')
}
</script>
