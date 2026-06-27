<template>
  <AppLayout>
    <div class="space-y-6">
      <!-- Loading State -->
      <div v-if="loading" class="flex justify-center py-12">
        <div
          class="h-8 w-8 animate-spin rounded-full border-2 border-primary-500 border-t-transparent"
        ></div>
      </div>

      <!-- Empty State -->
      <div v-else-if="subscriptions.length === 0" class="card p-12 text-center">
        <div
          class="mx-auto mb-4 flex h-16 w-16 items-center justify-center rounded-full bg-gray-100 dark:bg-dark-700"
        >
          <Icon name="creditCard" size="xl" class="text-gray-400" />
        </div>
        <h3 class="mb-2 text-lg font-semibold text-gray-900 dark:text-white">
          {{ t('userSubscriptions.noActiveSubscriptions') }}
        </h3>
        <p class="text-gray-500 dark:text-dark-400">
          {{ t('userSubscriptions.noActiveSubscriptionsDesc') }}
        </p>
      </div>

      <!-- Subscriptions Grid -->
      <div v-else class="grid gap-6 lg:grid-cols-2">
        <div
          v-for="subscription in subscriptions"
          :key="subscription.id"
          class="overflow-hidden rounded-2xl border border-gray-200 bg-white dark:border-dark-700 dark:bg-dark-800"
        >
          <!-- Header -->
          <div
            class="flex items-center justify-between border-b border-gray-100 p-4 dark:border-dark-700"
          >
            <div class="flex items-center gap-3">
              <div class="h-1.5 w-1.5 shrink-0 rounded-full bg-primary-500" />
              <div>
                <h3 class="font-semibold text-gray-900 dark:text-white">
                  {{ subscriptionDisplayName(subscription) }}
                </h3>
              </div>
            </div>
            <div class="flex items-center gap-2">
              <span
                :class="[
                  'rounded-full px-2 py-0.5 text-xs font-medium',
                  subscriptionStatusClass(subscription.status)
                ]"
              >
                {{ subscriptionStatusLabel(subscription) }}
              </span>
              <button
                v-if="subscription.status === 'active' && subscription.plan_id"
                class="rounded-lg bg-primary-600 px-3 py-1.5 text-xs font-semibold text-white transition-colors hover:bg-primary-700 dark:bg-primary-500 dark:hover:bg-primary-400"
                @click="router.push({ path: '/purchase', query: { tab: 'subscription', plan: String(subscription.plan_id) } })"
              >
                {{ t('payment.renewNow') }}
              </button>
              <button
                v-if="subscription.status === 'active'"
                class="rounded-lg bg-red-600 px-3 py-1.5 text-xs font-semibold text-white transition-colors hover:bg-red-700 dark:bg-red-500 dark:hover:bg-red-400"
                @click="openRefundDialog(subscription)"
              >
                <Icon name="dollar" size="sm" class="mr-1" />
                {{ t('userSubscriptions.refund.request') }}
              </button>
            </div>
          </div>

          <!-- Usage Progress -->
          <div class="space-y-4 p-4">
            <!-- Expiration Info -->
            <div v-if="subscription.expires_at" class="flex items-center justify-between text-sm">
              <span class="text-gray-500 dark:text-dark-400">{{
                t('userSubscriptions.expires')
              }}</span>
              <span :class="getExpirationClass(subscription.expires_at)">
                {{ formatExpirationDate(subscription.expires_at) }}
              </span>
            </div>
            <div v-else class="flex items-center justify-between text-sm">
              <span class="text-gray-500 dark:text-dark-400">{{
                t('userSubscriptions.expires')
              }}</span>
              <span class="text-gray-700 dark:text-gray-300">{{
                t('userSubscriptions.noExpiration')
              }}</span>
            </div>

            <!-- Daily Usage -->
            <div v-if="displayDailyLimit(subscription) != null" class="space-y-2">
              <div class="flex items-center justify-between">
                <span class="text-sm font-medium text-gray-700 dark:text-gray-300">
                  {{ t('userSubscriptions.daily') }}
                </span>
                <span class="text-sm text-gray-500 dark:text-dark-400">
                  {{ formatQuotaUsage(subscription.daily_used_knives, displayDailyLimit(subscription), subscription.daily_usage_usd) }}
                </span>
              </div>
              <div class="relative h-2 overflow-hidden rounded-full bg-gray-200 dark:bg-dark-600">
                <div
                  class="absolute inset-y-0 left-0 rounded-full transition-all duration-300"
                  :class="
                    getProgressBarClass(
                      displayDailyUsed(subscription),
                      displayDailyLimit(subscription)
                    )
                  "
                  :style="{
                    width: getProgressWidth(
                      displayDailyUsed(subscription),
                      displayDailyLimit(subscription)
                    )
                  }"
                ></div>
              </div>
              <p
                v-if="subscription.daily_window_start"
                class="text-xs text-gray-500 dark:text-dark-400"
              >
                {{ formatDailyUsageWindow(subscription) }}
              </p>
            </div>

            <!-- Weekly Usage -->
            <div v-if="displayWeeklyLimit(subscription) != null" class="space-y-2">
              <div class="flex items-center justify-between">
                <span class="text-sm font-medium text-gray-700 dark:text-gray-300">
                  {{ t('userSubscriptions.weekly') }}
                </span>
                <span class="text-sm text-gray-500 dark:text-dark-400">
                  {{ formatQuotaUsage(subscription.weekly_used_knives, displayWeeklyLimit(subscription), subscription.weekly_usage_usd) }}
                </span>
              </div>
              <div class="relative h-2 overflow-hidden rounded-full bg-gray-200 dark:bg-dark-600">
                <div
                  class="absolute inset-y-0 left-0 rounded-full transition-all duration-300"
                  :class="
                    getProgressBarClass(
                      displayWeeklyUsed(subscription),
                      displayWeeklyLimit(subscription)
                    )
                  "
                  :style="{
                    width: getProgressWidth(
                      displayWeeklyUsed(subscription),
                      displayWeeklyLimit(subscription)
                    )
                  }"
                ></div>
              </div>
              <p
                v-if="subscription.weekly_window_start"
                class="text-xs text-gray-500 dark:text-dark-400"
              >
                {{
                  t('userSubscriptions.resetIn', {
                    time: formatResetTime(subscription.weekly_window_start, 168)
                  })
                }}
              </p>
            </div>

            <!-- Monthly Usage -->
            <div v-if="displayMonthlyLimit(subscription) != null" class="space-y-2">
              <div class="flex items-center justify-between">
                <span class="text-sm font-medium text-gray-700 dark:text-gray-300">
                  {{ t('userSubscriptions.monthly') }}
                </span>
                <span class="text-sm text-gray-500 dark:text-dark-400">
                  {{ formatQuotaUsage(subscription.monthly_used_knives, displayMonthlyLimit(subscription), subscription.monthly_usage_usd) }}
                </span>
              </div>
              <div class="relative h-2 overflow-hidden rounded-full bg-gray-200 dark:bg-dark-600">
                <div
                  class="absolute inset-y-0 left-0 rounded-full transition-all duration-300"
                  :class="
                    getProgressBarClass(
                      displayMonthlyUsed(subscription),
                      displayMonthlyLimit(subscription)
                    )
                  "
                  :style="{
                    width: getProgressWidth(
                      displayMonthlyUsed(subscription),
                      displayMonthlyLimit(subscription)
                    )
                  }"
                ></div>
              </div>
              <p
                v-if="subscription.monthly_window_start"
                class="text-xs text-gray-500 dark:text-dark-400"
              >
                {{
                  t('userSubscriptions.resetIn', {
                    time: formatResetTime(subscription.monthly_window_start, 720)
                  })
                }}
              </p>
            </div>

            <!-- No limits configured - Unlimited badge -->
            <div
              v-if="
                displayDailyLimit(subscription) == null &&
                displayWeeklyLimit(subscription) == null &&
                displayMonthlyLimit(subscription) == null
              "
              class="flex items-center justify-center rounded-xl bg-gradient-to-r from-emerald-50 to-teal-50 py-6 dark:from-emerald-900/20 dark:to-teal-900/20"
            >
              <div class="flex items-center gap-3">
                <span class="text-4xl text-emerald-600 dark:text-emerald-400">∞</span>
                <div>
                  <p class="text-sm font-medium text-emerald-700 dark:text-emerald-300">
                    {{ t('userSubscriptions.unlimited') }}
                  </p>
                  <p class="text-xs text-emerald-600/70 dark:text-emerald-400/70">
                    {{ t('userSubscriptions.unlimitedDesc') }}
                  </p>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>

    <SubscriptionRefundDialog
      :show="showRefundDialog"
      :subscription="refundSubscription"
      @close="closeRefundDialog"
      @submitted="handleRefundSubmitted"
    />
  </AppLayout>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import { useAppStore } from '@/stores/app'
import subscriptionsAPI from '@/api/subscriptions'
import type { SubscriptionRefundSubmitResult, UserSubscription } from '@/types'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import SubscriptionRefundDialog from '@/components/user/SubscriptionRefundDialog.vue'
import { formatDateOnly } from '@/utils/format'
import { getRemainingDurationParts, isOneTimeDailyQuota, type RemainingDurationParts } from '@/utils/subscriptionQuota'

const { t } = useI18n()
const router = useRouter()
const appStore = useAppStore()

const subscriptions = ref<UserSubscription[]>([])
const loading = ref(true)
const showRefundDialog = ref(false)
const refundSubscription = ref<UserSubscription | null>(null)

function subscriptionDisplayName(subscription: UserSubscription): string {
  if (subscription.plan_name_snapshot?.trim()) {
    return subscription.plan_name_snapshot
  }
  return `${t('payment.plan')} #${subscription.id}`
}

function subscriptionStatusLabel(subscription: UserSubscription): string {
  if (subscription.status === 'suspended' && subscription.refund_freeze_active) {
    const refundLabel = t('userSubscriptions.status.suspended_refund')
    if (refundLabel !== 'userSubscriptions.status.suspended_refund') {
      return refundLabel
    }
  }
  const key = `userSubscriptions.status.${subscription.status}`
  const label = t(key)
  return label === key ? subscription.status : label
}

function subscriptionStatusClass(status: UserSubscription['status'] | string): string {
  switch (status) {
    case 'active':
      return 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/40 dark:text-emerald-300'
    case 'suspended':
      return 'bg-amber-100 text-amber-700 dark:bg-amber-900/40 dark:text-amber-300'
    case 'expired':
      return 'bg-gray-100 text-gray-600 dark:bg-dark-700 dark:text-gray-400'
    case 'superseded':
      return 'bg-amber-100 text-amber-700 dark:bg-amber-900/40 dark:text-amber-300'
    case 'refunded':
      return 'bg-purple-100 text-purple-700 dark:bg-purple-900/40 dark:text-purple-300'
    case 'revoked':
    default:
      return 'bg-red-100 text-red-700 dark:bg-red-900/40 dark:text-red-300'
  }
}

async function loadSubscriptions() {
  try {
    loading.value = true
    subscriptions.value = await subscriptionsAPI.getMySubscriptions()
  } catch (error) {
    console.error('Failed to load subscriptions:', error)
    appStore.showError(t('userSubscriptions.failedToLoad'))
  } finally {
    loading.value = false
  }
}

function openRefundDialog(subscription: UserSubscription) {
  refundSubscription.value = subscription
  showRefundDialog.value = true
}

function closeRefundDialog() {
  showRefundDialog.value = false
  refundSubscription.value = null
}

async function handleRefundSubmitted(result: SubscriptionRefundSubmitResult) {
  closeRefundDialog()
  await router.push(`/subscription-refund-requests/${result.refund_request_id}`)
}

function getProgressWidth(used: number | undefined, limit: number | null | undefined): string {
  if (!limit || limit === 0) return '0%'
  const percentage = Math.min(((used || 0) / limit) * 100, 100)
  return `${percentage}%`
}

function getProgressBarClass(used: number | undefined, limit: number | null | undefined): string {
  if (!limit || limit === 0) return 'bg-gray-400'
  const percentage = ((used || 0) / limit) * 100
  if (percentage >= 90) return 'bg-red-500'
  if (percentage >= 70) return 'bg-orange-500'
  return 'bg-green-500'
}

function displayDailyLimit(subscription: UserSubscription): number | null {
  return subscription.daily_quota_knives ?? null
}

function displayWeeklyLimit(subscription: UserSubscription): number | null {
  return subscription.weekly_quota_knives ?? null
}

function displayMonthlyLimit(subscription: UserSubscription): number | null {
  return subscription.monthly_quota_knives ?? null
}

function displayDailyUsed(subscription: UserSubscription): number {
  return subscription.daily_quota_knives != null
    ? (subscription.daily_used_knives || 0)
    : (subscription.daily_usage_usd || 0)
}

function displayWeeklyUsed(subscription: UserSubscription): number {
  return subscription.weekly_quota_knives != null
    ? (subscription.weekly_used_knives || 0)
    : (subscription.weekly_usage_usd || 0)
}

function displayMonthlyUsed(subscription: UserSubscription): number {
  return subscription.monthly_quota_knives != null
    ? (subscription.monthly_used_knives || 0)
    : (subscription.monthly_usage_usd || 0)
}

function formatQuotaUsage(knivesUsed: number | undefined, limit: number | null, usdUsed: number | undefined): string {
  if (limit == null) return t('payment.planCard.unlimited')
  if (typeof knivesUsed === 'number' && Number.isFinite(knivesUsed) && limit >= 0) {
    return `${knivesUsed.toFixed(2)} / ${limit.toFixed(2)}`
  }
  return `$${(usdUsed || 0).toFixed(2)} / ${limit.toFixed(2)}`
}

function formatExpirationDate(expiresAt: string): string {
  const now = new Date()
  const expires = new Date(expiresAt)
  const diff = expires.getTime() - now.getTime()
  const days = Math.ceil(diff / (1000 * 60 * 60 * 24))

  if (days < 0) {
    return t('userSubscriptions.status.expired')
  }

  const dateStr = formatDateOnly(expires)

  if (days === 0) {
    return `${dateStr} (${t('common.today')})`
  }
  if (days === 1) {
    return `${dateStr} (${t('common.tomorrow')})`
  }

  return t('userSubscriptions.daysRemaining', { days }) + ` (${dateStr})`
}

function getExpirationClass(expiresAt: string): string {
  const now = new Date()
  const expires = new Date(expiresAt)
  const diff = expires.getTime() - now.getTime()
  const days = Math.ceil(diff / (1000 * 60 * 60 * 24))

  if (days <= 0) return 'text-red-600 dark:text-red-400 font-medium'
  if (days <= 3) return 'text-red-600 dark:text-red-400'
  if (days <= 7) return 'text-orange-600 dark:text-orange-400'
  return 'text-gray-700 dark:text-gray-300'
}

function formatDurationParts(parts: RemainingDurationParts): string {
  if (parts.days > 0) {
    return `${parts.days}d ${parts.hours}h`
  }

  if (parts.hours > 0) {
    return `${parts.hours}h ${parts.minutes}m`
  }

  return `${parts.minutes}m`
}

function formatDailyUsageWindow(subscription: UserSubscription): string {
  if (isOneTimeDailyQuota(subscription) && subscription.expires_at) {
    const parts = getRemainingDurationParts(subscription.expires_at)
    if (!parts) return t('userSubscriptions.windowNotActive')
    return t('userSubscriptions.quotaEndsIn', { time: formatDurationParts(parts) })
  }

  return t('userSubscriptions.resetIn', {
    time: formatResetTime(subscription.daily_window_start, 24)
  })
}

function formatResetTime(windowStart: string | null, windowHours: number): string {
  if (!windowStart) return t('userSubscriptions.windowNotActive')

  const start = new Date(windowStart)
  const end = new Date(start.getTime() + windowHours * 60 * 60 * 1000)
  const parts = getRemainingDurationParts(end)

  return parts ? formatDurationParts(parts) : t('userSubscriptions.windowNotActive')
}

onMounted(() => {
  loadSubscriptions()
})
</script>
