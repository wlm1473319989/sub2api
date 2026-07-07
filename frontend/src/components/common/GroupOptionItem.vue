<template>
  <div class="flex min-w-0 flex-1 items-start justify-between gap-3">
    <div
      class="flex min-w-0 flex-1 flex-col items-start"
      :title="description || undefined"
    >
      <GroupBadge
        :name="name"
        :platform="platform"
        :subscription-rate-multiplier="subscriptionRateMultiplier"
        :show-rate="false"
        class="groupOptionItemBadge"
      />
      <span
        v-if="description"
        class="mt-1.5 w-full text-left text-xs leading-relaxed text-gray-500 dark:text-gray-400 line-clamp-2"
      >
        {{ description }}
      </span>
    </div>

    <div class="flex shrink-0 items-center gap-2 pt-0.5">
      <div class="flex shrink-0 flex-col items-end gap-1">
        <span v-if="rateText" :class="['inline-flex items-center whitespace-nowrap rounded-full px-3 py-1 text-xs font-semibold', ratePillClass]">
          <template v-if="hasCustomRate && !hasSplitRate">
            <span class="mr-1 line-through opacity-50">{{ rateMultiplier }}x</span>
            <span class="font-bold">{{ displayBalanceRate }}x</span>
          </template>
          <template v-else>
            {{ rateText }}
          </template>
        </span>
        <span
          v-if="hasPeakRate"
          class="inline-flex items-center whitespace-nowrap rounded-full bg-amber-50 px-3 py-1 text-xs font-semibold text-amber-700 dark:bg-amber-900/20 dark:text-amber-300"
          :title="peakRateTitle"
        >
          {{ peakRateText }}
        </span>
      </div>
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
import { useAppStore } from '@/stores/app'
import { formatPeakRateWindow, serverTimezoneLabel } from '@/utils/peak-rate'

interface Props {
  name: string
  platform: GroupPlatform
  rateMultiplier?: number
  subscriptionRateMultiplier?: number | null
  userRateMultiplier?: number | null
  peakRateEnabled?: boolean
  peakStart?: string
  peakEnd?: string
  peakRateMultiplier?: number
  description?: string | null
  selected?: boolean
  showCheckmark?: boolean
}

const props = withDefaults(defineProps<Props>(), {
  selected: false,
  showCheckmark: true,
  subscriptionRateMultiplier: null,
  userRateMultiplier: null,
  peakRateEnabled: false,
})

const { t } = useI18n()
const appStore = useAppStore()

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

const hasPeakRate = computed(() => {
  return Boolean(props.peakRateEnabled && props.peakStart && props.peakEnd)
})

const peakRateText = computed(() => {
  return formatPeakRateWindow(
    {
      peak_rate_enabled: props.peakRateEnabled,
      peak_start: props.peakStart,
      peak_end: props.peakEnd,
      peak_rate_multiplier: props.peakRateMultiplier,
    },
    serverTimezoneLabel(appStore.cachedPublicSettings?.server_utc_offset)
  )
})

const peakRateTitle = computed(() => {
  return t('common.peakRateTooltip', { window: peakRateText.value })
})

const ratePillClass = computed(() => {
  switch (props.platform) {
    case 'anthropic':
      return 'bg-amber-50 text-amber-700 dark:bg-amber-900/20 dark:text-amber-400'
    case 'openai':
      return 'bg-green-50 text-green-700 dark:bg-green-900/20 dark:text-green-400'
    case 'gemini':
      return 'bg-sky-50 text-sky-700 dark:bg-sky-900/20 dark:text-sky-400'
    case 'grok':
      return 'bg-zinc-100 text-zinc-700 dark:bg-zinc-800 dark:text-zinc-200'
    default:
      return 'bg-violet-50 text-violet-700 dark:bg-violet-900/20 dark:text-violet-400'
  }
})
</script>

<style scoped>
.groupOptionItemBadge :deep(span.truncate) {
  font-weight: 600;
}
</style>
