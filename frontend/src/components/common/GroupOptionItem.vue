<template>
  <div class="flex min-w-0 flex-1 items-start justify-between gap-3">
    <!-- Left: name + description -->
    <div
      class="flex min-w-0 flex-1 flex-col items-start"
      :title="description || undefined"
    >
      <!-- Row 1: platform badge (name bold) -->
      <GroupBadge
        :name="name"
        :platform="platform"
        :subscription-rate-multiplier="subscriptionRateMultiplier"
        :access-expires-at="accessExpiresAt"
        :now-ms="nowMs"
        :show-rate="false"
        class="groupOptionItemBadge"
      />
      <!-- Row 2: description with top spacing -->
      <span
        v-if="description"
        class="mt-1.5 w-full text-left text-xs leading-relaxed text-gray-500 dark:text-gray-400 line-clamp-2"
      >
        {{ description }}
      </span>
    </div>

    <!-- Right: rate pill + checkmark (vertically centered to first row) -->
    <div class="flex shrink-0 items-center gap-2 pt-0.5">
      <span
        v-if="accessCountdownText"
        :class="['inline-flex items-center whitespace-nowrap rounded-full px-2.5 py-1 text-xs font-semibold', accessPillClass]"
        :title="accessExpiresAtTitle"
      >
        {{ accessCountdownText }}
      </span>
      <!-- Rate pill (platform color) -->
      <span v-if="rateText" :class="['inline-flex items-center whitespace-nowrap rounded-full px-3 py-1 text-xs font-semibold', ratePillClass]">
        <template v-if="hasCustomRate && !hasSplitRate">
          <span class="mr-1 line-through opacity-50">{{ rateMultiplier }}x</span>
          <span class="font-bold">{{ displayBalanceRate }}x</span>
        </template>
        <template v-else>
          {{ rateText }}
        </template>
      </span>
      <!-- Checkmark -->
      <svg
        v-if="showCheckmark && selected"
        class="h-4 w-4 shrink-0 text-primary-600 dark:text-primary-400"
        fill="none"
        stroke="currentColor"
        viewBox="0 0 24 24"
        stroke-width="2"
      >
        <path stroke-linecap="round" stroke-linejoin="round" d="M5 13l4 4L19 7" />
      </svg>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import GroupBadge from './GroupBadge.vue'
import type { GroupPlatform } from '@/types'

interface Props {
  name: string
  platform: GroupPlatform
  rateMultiplier?: number
  subscriptionRateMultiplier?: number | null
  userRateMultiplier?: number | null
  description?: string | null
  selected?: boolean
  showCheckmark?: boolean
  accessExpiresAt?: string | null
  nowMs?: number
}

const props = withDefaults(defineProps<Props>(), {
  selected: false,
  showCheckmark: true,
  userRateMultiplier: null,
  accessExpiresAt: null,
  nowMs: undefined
})

const { t } = useI18n()

// Whether user has a custom rate different from default
const hasCustomRate = computed(() => {
  return (
    props.userRateMultiplier !== null &&
    props.userRateMultiplier !== undefined &&
    props.rateMultiplier !== undefined &&
    props.userRateMultiplier !== props.rateMultiplier
  )
})

const displayBalanceRate = computed(() => {
  if (props.userRateMultiplier !== null && props.userRateMultiplier !== undefined) {
    return props.userRateMultiplier
  }
  return props.rateMultiplier
})

const displaySubscriptionRate = computed(() => {
  if (props.subscriptionRateMultiplier !== null && props.subscriptionRateMultiplier !== undefined) {
    return props.subscriptionRateMultiplier
  }
  if (props.rateMultiplier !== undefined) {
    return props.rateMultiplier
  }
  if (props.userRateMultiplier !== null && props.userRateMultiplier !== undefined) {
    return props.userRateMultiplier
  }
  return undefined
})

const hasSplitRate = computed(() => {
  return (
    displayBalanceRate.value !== undefined &&
    displaySubscriptionRate.value !== undefined &&
    displaySubscriptionRate.value !== displayBalanceRate.value
  )
})

const rateText = computed(() => {
  if (hasSplitRate.value) {
    return t('admin.groups.rateMultiplierSplitSummary', {
      balance: `${displayBalanceRate.value}x`,
      subscription: `${displaySubscriptionRate.value}x`
    })
  }
  const rate = displayBalanceRate.value ?? displaySubscriptionRate.value
  return rate !== undefined ? `${rate}x` : ''
})

const currentNowMs = computed(() => props.nowMs ?? Date.now())

const accessExpiresAtMs = computed(() => {
  if (!props.accessExpiresAt) return null
  const parsed = Date.parse(props.accessExpiresAt)
  return Number.isFinite(parsed) ? parsed : null
})

const accessRemainingMs = computed(() => {
  if (accessExpiresAtMs.value === null) return 0
  return accessExpiresAtMs.value - currentNowMs.value
})

const accessCountdownText = computed(() => {
  if (accessExpiresAtMs.value === null) return ''
  return t('keys.timedGroupAccessRemaining', { time: formatAccessRemaining(accessRemainingMs.value) })
})

const accessExpiresAtTitle = computed(() => {
  if (!props.accessExpiresAt || accessExpiresAtMs.value === null) return ''
  return t('keys.timedGroupAccessExpiresAt', {
    time: new Date(accessExpiresAtMs.value).toLocaleString()
  })
})

const accessPillClass = computed(() => {
  if (accessRemainingMs.value <= 3 * 24 * 60 * 60 * 1000) {
    return 'bg-red-50 text-red-700 dark:bg-red-900/20 dark:text-red-300'
  }
  if (accessRemainingMs.value <= 7 * 24 * 60 * 60 * 1000) {
    return 'bg-amber-50 text-amber-700 dark:bg-amber-900/20 dark:text-amber-300'
  }
  return 'bg-emerald-50 text-emerald-700 dark:bg-emerald-900/20 dark:text-emerald-300'
})

function formatAccessRemaining(ms: number): string {
  if (ms <= 0) return t('keys.timedGroupAccessExpired')
  const totalMinutes = Math.max(1, Math.ceil(ms / 60000))
  if (totalMinutes < 60) {
    return t('keys.timeRemainingMinutes', { minutes: totalMinutes })
  }
  const totalHours = Math.ceil(totalMinutes / 60)
  if (totalHours < 24) {
    return t('keys.timeRemainingHours', { hours: totalHours })
  }
  const days = Math.floor(totalHours / 24)
  const hours = totalHours % 24
  if (hours > 0) {
    return t('keys.timeRemainingDaysHours', { days, hours })
  }
  return t('keys.timeRemainingDays', { days })
}

// Rate pill color matches platform badge color
const ratePillClass = computed(() => {
  switch (props.platform) {
    case 'anthropic':
      return 'bg-amber-50 text-amber-700 dark:bg-amber-900/20 dark:text-amber-400'
    case 'openai':
      return 'bg-green-50 text-green-700 dark:bg-green-900/20 dark:text-green-400'
    case 'gemini':
      return 'bg-sky-50 text-sky-700 dark:bg-sky-900/20 dark:text-sky-400'
    default: // antigravity and others
      return 'bg-violet-50 text-violet-700 dark:bg-violet-900/20 dark:text-violet-400'
  }
})
</script>

<style scoped>
/* Bold the group name inside GroupBadge when used in dropdown option */
.groupOptionItemBadge :deep(span.truncate) {
  font-weight: 600;
}
</style>
