<template>
  <span
    :class="[
      'inline-flex items-center gap-1.5 rounded-md px-2 py-0.5 text-xs font-medium transition-colors',
      badgeClass
    ]"
  >
    <PlatformIcon v-if="platform" :platform="platform" size="sm" />
    <span class="truncate">{{ name }}</span>
    <span v-if="showLabel" :class="labelClass">
      <template v-if="hasCustomRate && !hasSplitRate">
        <span class="mr-0.5 line-through opacity-50">{{ rateMultiplier }}x</span>
        <span class="font-bold">{{ displayBalanceRate }}x</span>
      </template>
      <template v-else>
        {{ labelText }}
      </template>
    </span>
    <span v-if="hasPeakRate" :class="peakRateClass" :title="peakRateTitle">
      {{ peakRateText }}
    </span>
  </span>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { GroupPlatform } from '@/types'
import { useAppStore } from '@/stores/app'
import { formatPeakRateWindow, serverTimezoneLabel } from '@/utils/peak-rate'
import PlatformIcon from './PlatformIcon.vue'

interface Props {
  name: string
  platform?: GroupPlatform
  rateMultiplier?: number
  subscriptionRateMultiplier?: number | null
  userRateMultiplier?: number | null
  peakRateEnabled?: boolean
  peakStart?: string
  peakEnd?: string
  peakRateMultiplier?: number
  showRate?: boolean
  alwaysShowRate?: boolean
  daysRemaining?: number | null
}

const props = withDefaults(defineProps<Props>(), {
  showRate: true,
  daysRemaining: null,
  subscriptionRateMultiplier: null,
  userRateMultiplier: null,
  peakRateEnabled: false,
  alwaysShowRate: false,
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

const hasPeakRate = computed(() => {
  return Boolean(props.showRate && props.peakRateEnabled && props.peakStart && props.peakEnd)
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

const showLabel = computed(() => {
  if (props.daysRemaining !== null && props.daysRemaining !== undefined) return true
  if (!props.showRate) return false
  return (
    props.alwaysShowRate ||
    displayBalanceRate.value !== undefined ||
    displaySubscriptionRate.value !== undefined ||
    hasCustomRate.value
  )
})

const labelText = computed(() => {
  if (props.daysRemaining !== null && props.daysRemaining !== undefined) {
    if (props.daysRemaining <= 0) {
      return t('admin.users.expired')
    }
    return t('admin.users.daysRemaining', { days: props.daysRemaining })
  }
  if (hasSplitRate.value) {
    return t('admin.groups.rateMultiplierSplitSummary', {
      balance: `${displayBalanceRate.value}x`,
      subscription: `${displaySubscriptionRate.value}x`
    })
  }
  const rate = displayBalanceRate.value ?? displaySubscriptionRate.value
  return rate !== undefined ? `${rate}x` : ''
})

const labelClass = computed(() => {
  const base = 'rounded px-1.5 py-0.5 text-[10px] font-semibold'

  if (props.daysRemaining === null || props.daysRemaining === undefined) {
    return `${base} bg-black/10 dark:bg-white/10`
  }
  if (props.daysRemaining <= 0 || props.daysRemaining <= 3) {
    return `${base} bg-red-200/80 text-red-800 dark:bg-red-800/50 dark:text-red-300`
  }
  if (props.daysRemaining <= 7) {
    return `${base} bg-amber-200/80 text-amber-800 dark:bg-amber-800/50 dark:text-amber-300`
  }
  if (props.platform === 'openai') {
    return `${base} bg-emerald-200/60 text-emerald-800 dark:bg-emerald-800/40 dark:text-emerald-300`
  }
  if (props.platform === 'gemini') {
    return `${base} bg-blue-200/60 text-blue-800 dark:bg-blue-800/40 dark:text-blue-300`
  }
  if (props.platform === 'antigravity') {
    return `${base} bg-purple-200/60 text-purple-800 dark:bg-purple-800/40 dark:text-purple-300`
  }
  if (props.platform === 'grok') {
    return `${base} bg-zinc-300/70 text-zinc-800 dark:bg-zinc-700/60 dark:text-zinc-200`
  }
  return `${base} bg-violet-200/60 text-violet-800 dark:bg-violet-800/40 dark:text-violet-300`
})

const peakRateClass = computed(() => {
  return 'px-1.5 py-0.5 rounded text-[10px] font-semibold bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-300'
})

const badgeClass = computed(() => {
  if (props.platform === 'anthropic') {
    return 'bg-amber-50 text-amber-700 dark:bg-amber-900/20 dark:text-amber-400'
  }
  if (props.platform === 'openai') {
    return 'bg-green-50 text-green-700 dark:bg-green-900/20 dark:text-green-400'
  }
  if (props.platform === 'gemini') {
    return 'bg-sky-50 text-sky-700 dark:bg-sky-900/20 dark:text-sky-400'
  }
  if (props.platform === 'antigravity') {
    return 'bg-fuchsia-50 text-fuchsia-700 dark:bg-fuchsia-900/20 dark:text-fuchsia-400'
  }
  if (props.platform === 'grok') {
    return 'bg-zinc-100 text-zinc-700 dark:bg-zinc-800 dark:text-zinc-200'
  }
  return 'bg-violet-100 text-violet-700 dark:bg-violet-900/30 dark:text-violet-400'
})
</script>
