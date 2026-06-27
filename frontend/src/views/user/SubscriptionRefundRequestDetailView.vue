<template>
  <AppLayout>
    <div class="space-y-6">
      <div class="flex flex-wrap items-center justify-between gap-3">
        <button class="btn btn-secondary" @click="router.push('/subscription-refund-requests')">
          <Icon name="arrowLeft" size="sm" class="mr-2" />
          {{ t('subscriptionRefundRequests.actions.backList') }}
        </button>

        <button class="btn btn-secondary" :disabled="loading" @click="loadRequest">
          <Icon name="refresh" size="sm" :class="[loading ? 'animate-spin' : '', 'mr-2']" />
          {{ t('subscriptionRefundRequests.actions.refresh') }}
        </button>
      </div>

      <div v-if="loading && !request" class="flex justify-center py-12">
        <div class="h-8 w-8 animate-spin rounded-full border-2 border-primary-500 border-t-transparent"></div>
      </div>

      <template v-else-if="request">
        <section class="rounded-2xl border border-gray-200 bg-white p-5 dark:border-dark-700 dark:bg-dark-800">
          <div class="flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between">
            <div class="min-w-0">
              <p class="text-sm text-gray-500 dark:text-dark-400">
                {{ t('subscriptionRefundRequests.fields.refundRequestNo') }} #{{ request.id }}
              </p>
              <h2 class="mt-1 truncate text-2xl font-semibold text-gray-900 dark:text-white">
                {{ subscriptionDisplayName(request) }}
              </h2>
              <p class="mt-2 text-sm text-gray-500 dark:text-dark-400">
                {{ t('subscriptionRefundRequests.fields.subscriptionNo') }} #{{ request.subscription_id }}
                <span class="mx-2">·</span>
                {{ subscriptionRefundModeLabel(t, request.refund_mode) }}
              </p>
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
        </section>

        <div class="grid gap-6 xl:grid-cols-[minmax(0,2fr)_minmax(320px,1fr)]">
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

                <InfoField
                  :label="t('subscriptionRefundRequests.fields.originalSubscriptionStatus')"
                  :value="subscriptionStatusLabel(t, request.original_subscription_status)"
                />
                <InfoField
                  :label="t('subscriptionRefundRequests.fields.reason')"
                  :value="request.reason || '-'"
                />
                <InfoField
                  :label="t('subscriptionRefundRequests.fields.currentSettlementHead')"
                  :value="settlementSummary(request.current_settlement_head, request.currency)"
                />
                <InfoField
                  :label="t('subscriptionRefundRequests.fields.expectedSettlementHead')"
                  :value="settlementSummary(request.expected_settlement_head, request.currency)"
                />
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
                  <p class="mt-2 text-xs text-amber-800 dark:text-amber-200">
                    {{ t('subscriptionRefundRequests.fields.proofUploadedAt') }}:
                    {{ formatDateTime(request.manual_transfer_proof_uploaded_at) || '-' }}
                  </p>
                </div>
              </div>
            </section>

            <section
              v-if="request.admin_note"
              class="rounded-2xl border border-gray-200 bg-white p-5 dark:border-dark-700 dark:bg-dark-800"
            >
              <h3 class="text-lg font-semibold text-gray-900 dark:text-white">
                {{ t('subscriptionRefundRequests.fields.adminNote') }}
              </h3>
              <p class="mt-3 whitespace-pre-wrap text-sm text-gray-700 dark:text-gray-300">
                {{ request.admin_note }}
              </p>
            </section>
          </div>
        </div>
      </template>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, defineComponent, h, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import subscriptionsAPI from '@/api/subscriptions'
import type { SubscriptionRefundRequest, SubscriptionSettlementOrder } from '@/types'
import { useAppStore } from '@/stores'
import { extractI18nErrorMessage } from '@/utils/apiError'
import {
  subscriptionRefundAllocationStatusClass,
  subscriptionRefundAllocationStatusLabel,
  subscriptionDisplayStatusLabelForRefundRequest,
  subscriptionRefundModeLabel,
  subscriptionRefundStatusClass,
  subscriptionRefundStatusLabel,
  subscriptionStatusClass,
  subscriptionStatusLabel,
} from '@/utils/subscriptionRefund'
import { formatCurrency, formatDateTime } from '@/utils/format'
import AppLayout from '@/components/layout/AppLayout.vue'
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
const request = ref<SubscriptionRefundRequest | null>(null)

const requestID = computed(() => Number(route.params.id))

async function loadRequest() {
  if (!Number.isFinite(requestID.value) || requestID.value <= 0) return
  loading.value = true
  try {
    request.value = await subscriptionsAPI.getSubscriptionRefundRequest(requestID.value)
  } catch (err: unknown) {
    appStore.showError(
      extractI18nErrorMessage(err, t, 'payment.errors', t('subscriptionRefundRequests.messages.loadFailed'))
    )
  } finally {
    loading.value = false
  }
}

function subscriptionDisplayName(item: SubscriptionRefundRequest): string {
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
