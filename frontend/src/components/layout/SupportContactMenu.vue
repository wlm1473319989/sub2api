<template>
  <div
    v-if="hasContacts"
    class="hidden min-w-0 max-w-[34rem] items-center gap-2 rounded-xl bg-primary-50 px-3 py-1.5 dark:bg-primary-900/20 md:flex"
    :aria-label="t('nav.supportContacts')"
  >
    <Icon name="chatBubble" size="sm" class="flex-shrink-0 text-primary-600 dark:text-primary-400" />

    <button
      v-if="normalizedQQGroupNumber"
      type="button"
      class="group flex min-w-0 max-w-[11rem] items-center gap-1.5 text-left text-sm font-semibold text-primary-700 transition-colors hover:text-primary-900 dark:text-primary-300 dark:hover:text-primary-100"
      :aria-label="`${t('nav.qqGroup')}: ${normalizedQQGroupNumber}`"
      :title="`${t('nav.qqGroup')}: ${normalizedQQGroupNumber} (${t('common.copy')})`"
      @click="copyNumber(normalizedQQGroupNumber)"
    >
      <Icon name="users" size="sm" class="flex-shrink-0 text-primary-600 dark:text-primary-400" />
      <span class="hidden whitespace-nowrap text-xs font-medium lg:inline">
        {{ t('nav.qqGroup') }}
      </span>
      <span class="min-w-0 truncate tabular-nums">
        {{ normalizedQQGroupNumber }}
      </span>
    </button>

    <span
      v-if="normalizedQQGroupNumber && normalizedContactInfo"
      class="h-4 w-px flex-shrink-0 bg-primary-200 dark:bg-primary-800"
      aria-hidden="true"
    />

    <button
      v-if="normalizedContactInfo"
      type="button"
      class="group flex min-w-0 max-w-[11rem] items-center gap-1.5 text-left text-sm font-semibold text-primary-700 transition-colors hover:text-primary-900 dark:text-primary-300 dark:hover:text-primary-100"
      :aria-label="`${t('nav.customerService')}: ${normalizedContactInfo}`"
      :title="`${t('nav.customerService')}: ${normalizedContactInfo} (${t('common.copy')})`"
      @click="copyNumber(normalizedContactInfo)"
    >
      <Icon name="user" size="sm" class="flex-shrink-0 text-primary-600 dark:text-primary-400" />
      <span class="hidden whitespace-nowrap text-xs font-medium lg:inline">
        {{ t('nav.customerService') }}
      </span>
      <span class="min-w-0 truncate tabular-nums">
        {{ normalizedContactInfo }}
      </span>
    </button>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { useClipboard } from '@/composables/useClipboard'
import Icon from '@/components/icons/Icon.vue'

const props = defineProps<{
  qqGroupNumber?: string
  contactInfo?: string
}>()

const { t } = useI18n()
const { copyToClipboard } = useClipboard()

const normalizedQQGroupNumber = computed(() => props.qqGroupNumber?.trim() || '')
const normalizedContactInfo = computed(() => props.contactInfo?.trim() || '')
const hasContacts = computed(
  () => Boolean(normalizedQQGroupNumber.value || normalizedContactInfo.value)
)

function copyNumber(value: string): void {
  void copyToClipboard(value)
}
</script>
