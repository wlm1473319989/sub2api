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
  </span>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { GroupPlatform } from '@/types'
import PlatformIcon from './PlatformIcon.vue'

interface Props {
  name: string
  platform?: GroupPlatform
  rateMultiplier?: number
  subscriptionRateMultiplier?: number | null
  userRateMultiplier?: number | null
  showRate?: boolean
  daysRemaining?: number | null
}

const props = withDefaults(defineProps<Props>(), {
  showRate: true,
  daysRemaining: null,
  userRateMultiplier: null
})

const { t } = useI18n()

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

const showLabel = computed(() => {
  if (props.daysRemaining !== null && props.daysRemaining !== undefined) return true
  if (!props.showRate) return false
  return (
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
  return `${base} bg-emerald-200/60 text-emerald-800 dark:bg-emerald-800/40 dark:text-emerald-300`
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
  return 'bg-violet-100 text-violet-700 dark:bg-violet-900/30 dark:text-violet-400'
})
</script>
