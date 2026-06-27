<template>
  <AppLayout>
    <div class="space-y-6">
      <div class="flex flex-wrap items-center justify-between gap-3">
        <button class="btn btn-secondary" @click="router.push('/admin/subscription-refund-requests')">
          <Icon name="arrowLeft" size="sm" class="mr-2" />
          {{ t('subscriptionRefundRequests.actions.backList') }}
        </button>

        <div class="flex flex-wrap items-center gap-2">
          <button class="btn btn-secondary" :disabled="busy" @click="loadRequest">
            <Icon name="refresh" size="sm" :class="[busy ? 'animate-spin' : '', 'mr-2']" />
            {{ t('subscriptionRefundRequests.actions.refresh') }}
          </button>
          <button
            class="btn btn-secondary"
            :disabled="busy || !request || !canProcessGatewayAction"
            @click="handleProcessGateway"
          >
            <Icon name="sync" size="sm" class="mr-2" />
            {{ t('subscriptionRefundRequests.actions.processGateway') }}
          </button>
          <button
            class="btn btn-primary"
            :disabled="busy || !request || !canCompleteAction"
            @click="handleCompleteRefund"
          >
            <Icon name="checkCircle" size="sm" class="mr-2" />
            {{ t('subscriptionRefundRequests.actions.complete') }}
          </button>
          <button
            class="btn btn-danger"
            :disabled="busy || !request || !canCancelAction"
            @click="showCancelDialog = true"
          >
            <Icon name="xCircle" size="sm" class="mr-2" />
            {{ t('subscriptionRefundRequests.actions.cancel') }}
          </button>
        </div>
      </div>

      <div v-if="loading && !request" class="flex justify-center py-12">
        <div class="h-8 w-8 animate-spin rounded-full border-2 border-primary-500 border-t-transparent"></div>
      </div>

      <template v-else-if="request">
        <section class="rounded-2xl border border-gray-200 bg-white p-5 dark:border-dark-700 dark:bg-dark-800">
          <div class="flex flex-col gap-4 xl:flex-row xl:items-start xl:justify-between">
            <div class="min-w-0">
              <p class="text-sm text-gray-500 dark:text-dark-400">
                {{ t('subscriptionRefundRequests.fields.refundRequestNo') }} #{{ request.id }}
              </p>
              <h2 class="mt-1 truncate text-2xl font-semibold text-gray-900 dark:text-white">
                {{ subscriptionDisplayName(request) }}
              </h2>
              <div class="mt-2 flex flex-wrap gap-x-3 gap-y-1 text-sm text-gray-500 dark:text-dark-400">
                <span>{{ t('subscriptionRefundRequests.fields.subscriptionNo') }} #{{ request.subscription_id }}</span>
                <span>{{ request.user?.email || `#${request.user_id}` }}</span>
                <span>{{ subscriptionRefundModeLabel(t, request.refund_mode) }}</span>
              </div>
            </div>

            <span
              :class="[
                'inline-flex rounded-full px-3 py-1 text-sm font-medium',
                subscriptionRefundStatusClass(request.status)
              ]"
            >
              {{ subscriptionRefundStatusLabel(t, request.status) }}
            </span>
          </div>

          <div class="mt-5 grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
            <div class="rounded-xl bg-gray-50 p-4 dark:bg-dark-900/60">
              <p class="text-xs font-medium uppercase text-gray-500 dark:text-dark-400">
                {{ t('subscriptionRefundRequests.summary.residual') }}
              </p>
              <p class="mt-2 text-lg font-semibold text-gray-900 dark:text-white">
                {{ formatCurrency(request.refund_residual_value, request.currency || 'CNY', 4) }}
              </p>
            </div>
            <div class="rounded-xl bg-gray-50 p-4 dark:bg-dark-900/60">
              <p class="text-xs font-medium uppercase text-gray-500 dark:text-dark-400">
                {{ t('subscriptionRefundRequests.summary.gatewayRefundable') }}
              </p>
              <p class="mt-2 text-lg font-semibold text-gray-900 dark:text-white">
                {{ formatCurrency(request.gateway_refundable_total, request.currency || 'CNY', 4) }}
              </p>
            </div>
            <div class="rounded-xl bg-gray-50 p-4 dark:bg-dark-900/60">
              <p class="text-xs font-medium uppercase text-gray-500 dark:text-dark-400">
                {{ t('subscriptionRefundRequests.summary.gatewayRefunded') }}
              </p>
              <p class="mt-2 text-lg font-semibold text-gray-900 dark:text-white">
                {{ formatCurrency(request.gateway_refunded_total, request.currency || 'CNY', 4) }}
              </p>
            </div>
            <div class="rounded-xl bg-gray-50 p-4 dark:bg-dark-900/60">
              <p class="text-xs font-medium uppercase text-gray-500 dark:text-dark-400">
                {{ t('subscriptionRefundRequests.summary.manualTransfer') }}
              </p>
              <p class="mt-2 text-lg font-semibold text-gray-900 dark:text-white">
                {{ formatCurrency(request.manual_transfer_amount, request.currency || 'CNY', 4) }}
              </p>
            </div>
          </div>

          <div
            v-if="completeBlockedReason"
            class="mt-4 rounded-xl border border-amber-200 bg-amber-50 px-4 py-3 text-sm text-amber-800 dark:border-amber-800 dark:bg-amber-900/20 dark:text-amber-200"
          >
            {{ completeBlockedReason }}
          </div>
        </section>

        <div class="grid gap-6 xl:grid-cols-[minmax(0,2fr)_minmax(360px,1fr)]">
          <div class="space-y-6">
            <section class="rounded-2xl border border-gray-200 bg-white p-5 dark:border-dark-700 dark:bg-dark-800">
              <h3 class="text-lg font-semibold text-gray-900 dark:text-white">
                {{ t('subscriptionRefundRequests.sections.timeline') }}
              </h3>
              <div class="mt-4 grid gap-4 sm:grid-cols-2">
                <InfoField :label="t('subscriptionRefundRequests.fields.previewIssuedAt')" :value="formatDateTime(request.preview_issued_at) || '-'" />
                <InfoField :label="t('subscriptionRefundRequests.fields.previewExpiresAt')" :value="formatDateTime(request.preview_expires_at) || '-'" />
                <InfoField :label="t('subscriptionRefundRequests.fields.submittedAt')" :value="formatDateTime(request.submitted_at) || '-'" />
                <InfoField :label="t('subscriptionRefundRequests.fields.frozenAt')" :value="formatDateTime(request.frozen_at) || '-'" />
                <InfoField :label="t('subscriptionRefundRequests.fields.completedAt')" :value="formatDateTime(request.completed_at) || '-'" />
                <InfoField :label="t('subscriptionRefundRequests.fields.cancelledAt')" :value="formatDateTime(request.cancelled_at) || '-'" />
              </div>
            </section>

            <section class="rounded-2xl border border-gray-200 bg-white p-5 dark:border-dark-700 dark:bg-dark-800">
              <h3 class="text-lg font-semibold text-gray-900 dark:text-white">
                {{ t('subscriptionRefundRequests.sections.allocations') }}
              </h3>

              <div v-if="!request.allocations?.length" class="mt-4 rounded-xl bg-gray-50 p-4 text-sm text-gray-500 dark:bg-dark-900/60 dark:text-dark-400">
                {{ t('subscriptionRefundRequests.emptyAllocations') }}
              </div>

              <div v-else class="mt-4 overflow-x-auto">
                <table class="min-w-full divide-y divide-gray-200 dark:divide-dark-700">
                  <thead>
                    <tr class="text-left text-xs uppercase text-gray-500 dark:text-dark-400">
                      <th class="px-3 py-2">{{ t('subscriptionRefundRequests.allocationTable.orderId') }}</th>
                      <th class="px-3 py-2">{{ t('subscriptionRefundRequests.allocationTable.status') }}</th>
                      <th class="px-3 py-2">{{ t('subscriptionRefundRequests.allocationTable.orderPayAmount') }}</th>
                      <th class="px-3 py-2">{{ t('subscriptionRefundRequests.allocationTable.alreadyRefundedAmount') }}</th>
                      <th class="px-3 py-2">{{ t('subscriptionRefundRequests.allocationTable.refundableAmount') }}</th>
                      <th class="px-3 py-2">{{ t('subscriptionRefundRequests.allocationTable.allocatedAmount') }}</th>
                      <th class="px-3 py-2">{{ t('subscriptionRefundRequests.allocationTable.gatewayRefundAmount') }}</th>
                      <th class="px-3 py-2">{{ t('subscriptionRefundRequests.allocationTable.reference') }}</th>
                    </tr>
                  </thead>
                  <tbody class="divide-y divide-gray-200 dark:divide-dark-700">
                    <tr v-for="item in request.allocations" :key="item.id">
                      <td class="px-3 py-3 text-sm font-medium text-gray-900 dark:text-white">
                        #{{ item.payment_order_id }}
                      </td>
                      <td class="px-3 py-3 text-sm">
                        <span
                          :class="[
                            'inline-flex rounded-full px-2 py-0.5 text-xs font-medium',
                            subscriptionRefundAllocationStatusClass(item.status)
                          ]"
                        >
                          {{ subscriptionRefundAllocationStatusLabel(t, item.status) }}
                        </span>
                      </td>
                      <td class="px-3 py-3 text-sm text-gray-700 dark:text-gray-300">
                        {{ formatCurrency(item.order_pay_amount, item.currency || request.currency || 'CNY', 4) }}
                      </td>
                      <td class="px-3 py-3 text-sm text-gray-700 dark:text-gray-300">
                        {{ formatCurrency(item.already_refunded_amount, item.currency || request.currency || 'CNY', 4) }}
                      </td>
                      <td class="px-3 py-3 text-sm text-gray-700 dark:text-gray-300">
                        {{ formatCurrency(item.refundable_order_amount, item.currency || request.currency || 'CNY', 4) }}
                      </td>
                      <td class="px-3 py-3 text-sm font-medium text-gray-900 dark:text-white">
                        {{ formatCurrency(item.allocated_refund_value, item.currency || request.currency || 'CNY', 4) }}
                      </td>
                      <td class="px-3 py-3 text-sm text-gray-700 dark:text-gray-300">
                        {{ formatCurrency(item.gateway_refund_amount, item.currency || request.currency || 'CNY', 4) }}
                      </td>
                      <td class="px-3 py-3 text-sm text-gray-500 dark:text-dark-400">
                        <p v-if="item.gateway_refund_trade_no">{{ item.gateway_refund_trade_no }}</p>
                        <p v-else-if="item.failed_reason">{{ item.failed_reason }}</p>
                        <p v-else>{{ formatDateTime(item.processed_at) || '-' }}</p>
                      </td>
                    </tr>
                  </tbody>
                </table>
              </div>
            </section>
          </div>

          <div class="space-y-6">
            <section class="rounded-2xl border border-gray-200 bg-white p-5 dark:border-dark-700 dark:bg-dark-800">
              <h3 class="text-lg font-semibold text-gray-900 dark:text-white">
                {{ t('subscriptionRefundRequests.sections.subscription') }}
              </h3>
              <div class="mt-4 space-y-4">
                <div>
                  <p class="text-xs font-medium uppercase text-gray-500 dark:text-dark-400">
                    {{ t('subscriptionRefundRequests.fields.currentSubscriptionStatus') }}
                  </p>
                  <span
                    :class="[
                      'mt-2 inline-flex rounded-full px-2 py-0.5 text-xs font-medium',
                      subscriptionStatusClass(request.subscription?.status)
                    ]"
                  >
                    {{ subscriptionDisplayStatusLabelForRefundRequest(t, request) }}
                  </span>
                </div>
                <InfoField :label="t('subscriptionRefundRequests.fields.originalSubscriptionStatus')" :value="subscriptionStatusLabel(t, request.original_subscription_status)" />
                <InfoField :label="t('subscriptionRefundRequests.fields.reason')" :value="request.reason || '-'" />
                <InfoField :label="t('subscriptionRefundRequests.fields.adminNote')" :value="request.admin_note || '-'" />
                <InfoField :label="t('subscriptionRefundRequests.fields.currentSettlementHead')" :value="settlementSummary(request.current_settlement_head, request.currency)" />
                <InfoField :label="t('subscriptionRefundRequests.fields.expectedSettlementHead')" :value="settlementSummary(request.expected_settlement_head, request.currency)" />
              </div>
            </section>

            <section
              v-if="request.manual_transfer_required"
              class="rounded-2xl border border-amber-200 bg-amber-50 p-5 dark:border-amber-800 dark:bg-amber-900/20"
            >
              <h3 class="text-lg font-semibold text-amber-900 dark:text-amber-100">
                {{ t('subscriptionRefundRequests.sections.manualTransfer') }}
              </h3>
              <div class="mt-4 space-y-4">
                <InfoField :label="t('subscriptionRefundRequests.fields.receiverType')" :value="request.manual_receiver_type || '-'" />
                <InfoField :label="t('subscriptionRefundRequests.fields.receiverName')" :value="request.manual_receiver_name || '-'" />
                <InfoField :label="t('subscriptionRefundRequests.fields.receiverAccount')" :value="request.manual_receiver_account || '-'" />
                <InfoField :label="t('subscriptionRefundRequests.fields.receiverRemark')" :value="request.manual_receiver_remark || '-'" />

                <div v-if="request.manual_receiver_qr_image_url">
                  <p class="text-xs font-medium uppercase text-amber-800 dark:text-amber-200">
                    {{ t('subscriptionRefundRequests.fields.receiverQrImageUrl') }}
                  </p>
                  <a
                    :href="request.manual_receiver_qr_image_url"
                    target="_blank"
                    rel="noreferrer"
                    class="mt-2 inline-flex items-center gap-1 text-sm text-primary-600 hover:text-primary-700 dark:text-primary-400 dark:hover:text-primary-300"
                  >
                    <Icon name="externalLink" size="sm" />
                    <span>{{ t('subscriptionRefundRequests.actions.openImage') }}</span>
                  </a>
                  <img
                    :src="request.manual_receiver_qr_image_url"
                    alt="receiver qr"
                    class="mt-3 max-h-56 w-full rounded-xl border border-amber-200 object-contain dark:border-amber-800"
                  />
                </div>

                <div v-if="request.manual_transfer_proof_url">
                  <p class="text-xs font-medium uppercase text-amber-800 dark:text-amber-200">
                    {{ t('subscriptionRefundRequests.fields.manualTransferProofUrl') }}
                  </p>
                  <a
                    :href="request.manual_transfer_proof_url"
                    target="_blank"
                    rel="noreferrer"
                    class="mt-2 inline-flex items-center gap-1 text-sm text-primary-600 hover:text-primary-700 dark:text-primary-400 dark:hover:text-primary-300"
                  >
                    <Icon name="externalLink" size="sm" />
                    <span>{{ t('subscriptionRefundRequests.actions.openImage') }}</span>
                  </a>
                  <img
                    :src="request.manual_transfer_proof_url"
                    alt="manual proof"
                    class="mt-3 max-h-64 w-full rounded-xl border border-amber-200 object-contain dark:border-amber-800"
                  />
                </div>

                <div
                  v-if="canUploadProofAction"
                  class="rounded-xl border border-amber-200 bg-white p-4 dark:border-amber-800 dark:bg-dark-800"
                >
                  <label class="input-label">{{ t('subscriptionRefundRequests.fields.manualTransferProofUrl') }}</label>
                  <input
                    v-model="proofForm.proof_url"
                    class="input mt-1 w-full"
                    type="url"
                    :placeholder="t('subscriptionRefundRequests.hints.proofUpload')"
                  />

                  <label class="input-label mt-4">{{ t('subscriptionRefundRequests.fields.adminNote') }}</label>
                  <textarea
                    v-model="proofForm.admin_note"
                    rows="3"
                    class="input mt-1 w-full"
                    :placeholder="t('subscriptionRefundRequests.placeholders.adminNote')"
                  />

                  <button class="btn btn-primary mt-4" :disabled="busy || !proofForm.proof_url.trim()" @click="handleUploadProof">
                    <Icon name="upload" size="sm" class="mr-2" />
                    {{ t('subscriptionRefundRequests.actions.saveProof') }}
                  </button>
                </div>
              </div>
            </section>
          </div>
        </div>
      </template>
    </div>

    <BaseDialog :show="showCancelDialog" :title="t('subscriptionRefundRequests.actions.cancel')" width="narrow" @close="showCancelDialog = false">
      <div class="space-y-4">
        <p class="text-sm text-gray-600 dark:text-gray-300">
          {{ t('subscriptionRefundRequests.hints.cancelConfirm') }}
        </p>
        <div>
          <label class="input-label">{{ t('subscriptionRefundRequests.fields.adminNote') }}</label>
          <textarea
            v-model="cancelAdminNote"
            rows="3"
            class="input mt-1 w-full"
            :placeholder="t('subscriptionRefundRequests.placeholders.adminNote')"
          />
        </div>
      </div>
      <template #footer>
        <div class="flex justify-end gap-3">
          <button class="btn btn-secondary" @click="showCancelDialog = false">
            {{ t('common.cancel') }}
          </button>
          <button class="btn btn-danger" :disabled="busy" @click="handleCancelRefund">
            {{ t('subscriptionRefundRequests.actions.cancel') }}
          </button>
        </div>
      </template>
    </BaseDialog>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, defineComponent, h, onMounted, reactive, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import adminSubscriptionsAPI from '@/api/admin/subscriptions'
import type { AdminSubscriptionRefundRequest, SubscriptionSettlementOrder } from '@/types'
import { useAppStore } from '@/stores'
import { extractI18nErrorMessage } from '@/utils/apiError'
import {
  allocationsReadyForCompletion,
  canCancelRefundRequest,
  canCompleteRefundRequest,
  canProcessRefundGateway,
  canUploadManualProof,
  subscriptionDisplayStatusLabelForRefundRequest,
  subscriptionRefundAllocationStatusClass,
  subscriptionRefundAllocationStatusLabel,
  subscriptionRefundModeLabel,
  subscriptionRefundStatusClass,
  subscriptionRefundStatusLabel,
  subscriptionStatusClass,
  subscriptionStatusLabel,
} from '@/utils/subscriptionRefund'
import { formatCurrency, formatDateTime } from '@/utils/format'
import AppLayout from '@/components/layout/AppLayout.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Icon from '@/components/icons/Icon.vue'

const InfoField = defineComponent({
  name: 'InfoField',
  props: {
    label: { type: String, required: true },
    value: { type: String, required: true },
  },
  setup(props) {
    return () =>
      h('div', { class: 'space-y-1' }, [
        h('p', { class: 'text-xs font-medium uppercase text-gray-500 dark:text-dark-400' }, props.label),
        h('p', { class: 'text-sm text-gray-900 dark:text-white break-all' }, props.value),
      ])
  },
})

const { t } = useI18n()
const route = useRoute()
const router = useRouter()
const appStore = useAppStore()

const loading = ref(false)
const processingGateway = ref(false)
const uploadingProof = ref(false)
const completing = ref(false)
const cancelling = ref(false)
const request = ref<AdminSubscriptionRefundRequest | null>(null)
const showCancelDialog = ref(false)
const cancelAdminNote = ref('')
const proofForm = reactive({
  proof_url: '',
  admin_note: '',
})

const requestID = computed(() => Number(route.params.id))
const busy = computed(() => loading.value || processingGateway.value || uploadingProof.value || completing.value || cancelling.value)
const canProcessGatewayAction = computed(() => (request.value ? canProcessRefundGateway(request.value) : false))
const canUploadProofAction = computed(() => (request.value ? canUploadManualProof(request.value) : false))
const canCompleteAction = computed(() => (request.value ? canCompleteRefundRequest(request.value) : false))
const canCancelAction = computed(() => (request.value ? canCancelRefundRequest(request.value) : false))

const completeBlockedReason = computed(() => {
  if (!request.value) return ''
  if (canCompleteAction.value) return ''
  if (!['submitted', 'gateway_processing', 'manual_pending'].includes(request.value.status)) return ''
  if (request.value.manual_transfer_required && !request.value.manual_transfer_proof_url?.trim()) {
    return t('subscriptionRefundRequests.hints.completeRequiresProof')
  }
  if (!allocationsReadyForCompletion(request.value.allocations)) {
    return t('subscriptionRefundRequests.hints.completeWaitGateway')
  }
  return ''
})

async function loadRequest() {
  if (!Number.isFinite(requestID.value) || requestID.value <= 0) return
  loading.value = true
  try {
    request.value = await adminSubscriptionsAPI.getRefundRequest(requestID.value)
    proofForm.proof_url = request.value.manual_transfer_proof_url || ''
    proofForm.admin_note = request.value.admin_note || ''
  } catch (err: unknown) {
    appStore.showError(
      extractI18nErrorMessage(err, t, 'payment.errors', t('subscriptionRefundRequests.messages.loadFailed'))
    )
  } finally {
    loading.value = false
  }
}

async function handleProcessGateway() {
  if (!request.value) return
  processingGateway.value = true
  try {
    await adminSubscriptionsAPI.processRefundGateway(request.value.id)
    appStore.showSuccess(t('subscriptionRefundRequests.messages.processGatewaySuccess'))
    await loadRequest()
  } catch (err: unknown) {
    appStore.showError(
      extractI18nErrorMessage(err, t, 'payment.errors', t('subscriptionRefundRequests.messages.processGatewayFailed'))
    )
  } finally {
    processingGateway.value = false
  }
}

async function handleUploadProof() {
  if (!request.value || !proofForm.proof_url.trim()) return
  uploadingProof.value = true
  try {
    await adminSubscriptionsAPI.uploadRefundProof(request.value.id, {
      proof_url: proofForm.proof_url.trim(),
      admin_note: proofForm.admin_note.trim() || undefined,
    })
    appStore.showSuccess(t('subscriptionRefundRequests.messages.uploadProofSuccess'))
    await loadRequest()
  } catch (err: unknown) {
    appStore.showError(
      extractI18nErrorMessage(err, t, 'payment.errors', t('subscriptionRefundRequests.messages.uploadProofFailed'))
    )
  } finally {
    uploadingProof.value = false
  }
}

async function handleCompleteRefund() {
  if (!request.value) return
  completing.value = true
  try {
    await adminSubscriptionsAPI.completeRefund(request.value.id)
    appStore.showSuccess(t('subscriptionRefundRequests.messages.completeSuccess'))
    await loadRequest()
  } catch (err: unknown) {
    appStore.showError(
      extractI18nErrorMessage(err, t, 'payment.errors', t('subscriptionRefundRequests.messages.completeFailed'))
    )
  } finally {
    completing.value = false
  }
}

async function handleCancelRefund() {
  if (!request.value) return
  cancelling.value = true
  try {
    await adminSubscriptionsAPI.cancelRefund(request.value.id, {
      admin_note: cancelAdminNote.value.trim() || undefined,
    })
    appStore.showSuccess(t('subscriptionRefundRequests.messages.cancelSuccess'))
    showCancelDialog.value = false
    cancelAdminNote.value = ''
    await loadRequest()
  } catch (err: unknown) {
    appStore.showError(
      extractI18nErrorMessage(err, t, 'payment.errors', t('subscriptionRefundRequests.messages.cancelFailed'))
    )
  } finally {
    cancelling.value = false
  }
}

function subscriptionDisplayName(item: AdminSubscriptionRefundRequest): string {
  return item.subscription?.plan_name_snapshot?.trim() || `${t('payment.plan')} #${item.subscription_id}`
}

function settlementSummary(settlement?: SubscriptionSettlementOrder | null, currency?: string): string {
  if (!settlement) return '-'
  return `#${settlement.id} · ${formatCurrency(settlement.after_settlement_value, currency || 'CNY', 4)}`
}

onMounted(() => {
  loadRequest()
})
</script>
