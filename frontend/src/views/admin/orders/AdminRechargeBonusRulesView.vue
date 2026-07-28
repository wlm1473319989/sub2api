<template>
  <AppLayout>
    <div class="space-y-4">
      <div class="flex items-center justify-end gap-2">
        <button class="btn btn-secondary" :disabled="loading" :title="t('common.refresh')" @click="loadRules">
          <Icon name="refresh" size="md" :class="loading ? 'animate-spin' : ''" />
        </button>
        <button class="btn btn-primary" @click="openEditor(null)">
          <Icon name="plus" size="sm" />
          <span>{{ t('payment.admin.rechargeBonus.create') }}</span>
        </button>
      </div>

      <DataTable :columns="columns" :data="rules" :loading="loading">
        <template #cell-name="{ row }">
          <div>
            <div class="text-sm font-medium text-gray-900 dark:text-white">{{ row.name }}</div>
            <div v-if="row.notes" class="mt-0.5 max-w-xs truncate text-xs text-gray-500 dark:text-gray-400">{{ row.notes }}</div>
          </div>
        </template>

        <template #cell-enabled="{ row }">
          <button
            type="button"
            class="relative inline-flex h-5 w-9 flex-shrink-0 rounded-full border-2 border-transparent transition-colors focus:outline-none focus:ring-2 focus:ring-primary-500 focus:ring-offset-2"
            :class="row.enabled ? 'bg-primary-500' : 'bg-gray-300 dark:bg-dark-600'"
            :title="row.enabled ? t('common.disable') : t('common.enable')"
            @click="toggleRule(row)"
          >
            <span
              class="pointer-events-none inline-block h-4 w-4 rounded-full bg-white shadow transition-transform"
              :class="row.enabled ? 'translate-x-4' : 'translate-x-0'"
            />
          </button>
        </template>

        <template #cell-status="{ row }">
          <span class="inline-flex rounded px-2 py-1 text-xs font-medium" :class="statusClass(row)">
            {{ t(`payment.admin.rechargeBonus.status.${ruleStatus(row)}`) }}
          </span>
        </template>

        <template #cell-amount_range="{ row }">
          <span class="whitespace-nowrap text-sm text-gray-700 dark:text-gray-300">
            {{ formatMoney(row.min_amount) }} - {{ row.max_amount == null ? t('payment.admin.rechargeBonus.unlimited') : formatMoney(row.max_amount) }}
          </span>
        </template>

        <template #cell-bonus="{ row }">
          <span class="whitespace-nowrap text-sm font-medium text-green-600 dark:text-green-400">
            {{ row.bonus_type === 'fixed' ? `+${formatMoney(row.bonus_value)}` : `+${formatNumber(row.bonus_value)}%` }}
          </span>
        </template>

        <template #cell-active_time="{ row }">
          <div class="whitespace-nowrap text-xs text-gray-600 dark:text-gray-300">
            <div>{{ row.starts_at ? formatDateTime(row.starts_at) : t('payment.admin.rechargeBonus.immediately') }}</div>
            <div>{{ row.ends_at ? formatDateTime(row.ends_at) : t('payment.admin.rechargeBonus.noEndTime') }}</div>
          </div>
        </template>

        <template #cell-actions="{ row }">
          <div class="flex items-center gap-1">
            <button class="rounded p-2 text-gray-500 hover:bg-blue-50 hover:text-blue-600 dark:hover:bg-blue-900/20 dark:hover:text-blue-400" :title="t('common.edit')" @click="openEditor(row)">
              <Icon name="edit" size="sm" />
            </button>
            <button class="rounded p-2 text-gray-500 hover:bg-red-50 hover:text-red-600 dark:hover:bg-red-900/20 dark:hover:text-red-400" :title="t('common.delete')" @click="requestDelete(row)">
              <Icon name="trash" size="sm" />
            </button>
          </div>
        </template>
      </DataTable>
    </div>

    <BaseDialog
      :show="editorOpen"
      :title="editingRule ? t('payment.admin.rechargeBonus.edit') : t('payment.admin.rechargeBonus.create')"
      width="wide"
      @close="editorOpen = false"
    >
      <form id="recharge-bonus-rule-form" class="space-y-4" @submit.prevent="saveRule">
        <div class="grid gap-4 sm:grid-cols-2">
          <div>
            <label class="input-label">{{ t('payment.admin.rechargeBonus.name') }}</label>
            <input v-model.trim="form.name" class="input" maxlength="100" required />
          </div>
          <div>
            <label class="input-label">{{ t('payment.admin.rechargeBonus.priority') }}</label>
            <input v-model.number="form.priority" class="input" type="number" step="1" required />
          </div>
        </div>

        <div class="grid gap-4 sm:grid-cols-2">
          <div>
            <label class="input-label">{{ t('payment.admin.rechargeBonus.minAmount') }}</label>
            <input v-model.number="form.minAmount" class="input" type="number" min="0.01" step="0.01" required />
          </div>
          <div>
            <label class="input-label">{{ t('payment.admin.rechargeBonus.maxAmount') }}</label>
            <input v-model="form.maxAmount" class="input" type="number" min="0.01" step="0.01" :placeholder="t('payment.admin.rechargeBonus.unlimited')" />
          </div>
        </div>

        <div class="grid gap-4 sm:grid-cols-2">
          <div>
            <label class="input-label">{{ t('payment.admin.rechargeBonus.bonusType') }}</label>
            <select v-model="form.bonusType" class="input" required>
              <option value="fixed">{{ t('payment.admin.rechargeBonus.fixed') }}</option>
              <option value="percentage">{{ t('payment.admin.rechargeBonus.percentage') }}</option>
            </select>
          </div>
          <div>
            <label class="input-label">{{ t('payment.admin.rechargeBonus.bonusValue') }}</label>
            <div class="relative">
              <input
                v-model.number="form.bonusValue"
                class="input pr-10"
                type="number"
                min="0.01"
                :max="form.bonusType === 'percentage' ? 100 : undefined"
                step="0.01"
                required
              />
              <span class="pointer-events-none absolute right-3 top-1/2 -translate-y-1/2 text-sm text-gray-400">{{ form.bonusType === 'percentage' ? '%' : '$' }}</span>
            </div>
          </div>
        </div>

        <div class="grid gap-4 sm:grid-cols-2">
          <div>
            <label class="input-label">{{ t('payment.admin.rechargeBonus.startsAt') }}</label>
            <input v-model="form.startsAt" class="input" type="datetime-local" />
          </div>
          <div>
            <label class="input-label">{{ t('payment.admin.rechargeBonus.endsAt') }}</label>
            <input v-model="form.endsAt" class="input" type="datetime-local" />
          </div>
        </div>

        <div>
          <label class="input-label">{{ t('payment.admin.rechargeBonus.notes') }}</label>
          <textarea v-model.trim="form.notes" class="input" rows="2" maxlength="500" />
        </div>

        <div class="flex items-center gap-3">
          <span class="text-sm text-gray-700 dark:text-gray-300">{{ t('payment.admin.rechargeBonus.enabled') }}</span>
          <button
            type="button"
            class="relative inline-flex h-6 w-11 rounded-full border-2 border-transparent transition-colors focus:outline-none focus:ring-2 focus:ring-primary-500 focus:ring-offset-2"
            :class="form.enabled ? 'bg-primary-500' : 'bg-gray-300 dark:bg-dark-600'"
            @click="form.enabled = !form.enabled"
          >
            <span class="pointer-events-none inline-block h-5 w-5 rounded-full bg-white shadow transition-transform" :class="form.enabled ? 'translate-x-5' : 'translate-x-0'" />
          </button>
        </div>
      </form>

      <template #footer>
        <div class="flex justify-end gap-3">
          <button type="button" class="btn btn-secondary" @click="editorOpen = false">{{ t('common.cancel') }}</button>
          <button type="submit" form="recharge-bonus-rule-form" class="btn btn-primary" :disabled="saving">
            {{ saving ? t('common.saving') : t('common.save') }}
          </button>
        </div>
      </template>
    </BaseDialog>

    <ConfirmDialog
      :show="deleteOpen"
      :title="t('payment.admin.rechargeBonus.delete')"
      :message="t('payment.admin.rechargeBonus.deleteConfirm')"
      :confirm-text="t('common.delete')"
      danger
      @confirm="deleteRule"
      @cancel="deleteOpen = false"
    />
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminPaymentAPI } from '@/api/admin/payment'
import { useAppStore } from '@/stores/app'
import { extractI18nErrorMessage } from '@/utils/apiError'
import type { RechargeBonusRule, RechargeBonusRulePayload, RechargeBonusType } from '@/types/payment'
import type { Column } from '@/components/common/types'
import AppLayout from '@/components/layout/AppLayout.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import DataTable from '@/components/common/DataTable.vue'
import Icon from '@/components/icons/Icon.vue'

type RuleStatus = 'disabled' | 'upcoming' | 'ended' | 'active'

const { t } = useI18n()
const appStore = useAppStore()
const rules = ref<RechargeBonusRule[]>([])
const loading = ref(false)
const saving = ref(false)
const editorOpen = ref(false)
const deleteOpen = ref(false)
const editingRule = ref<RechargeBonusRule | null>(null)
const deletingRule = ref<RechargeBonusRule | null>(null)

const form = reactive({
  name: '',
  enabled: true,
  priority: 0,
  minAmount: 1,
  maxAmount: '' as number | '',
  bonusType: 'fixed' as RechargeBonusType,
  bonusValue: 1,
  startsAt: '',
  endsAt: '',
  notes: '',
})

const columns = computed((): Column[] => [
  { key: 'name', label: t('payment.admin.rechargeBonus.name') },
  { key: 'enabled', label: t('payment.admin.rechargeBonus.enabled') },
  { key: 'status', label: t('payment.admin.rechargeBonus.statusLabel') },
  { key: 'priority', label: t('payment.admin.rechargeBonus.priority') },
  { key: 'amount_range', label: t('payment.admin.rechargeBonus.amountRange') },
  { key: 'bonus', label: t('payment.admin.rechargeBonus.bonus') },
  { key: 'active_time', label: t('payment.admin.rechargeBonus.activeTime') },
  { key: 'actions', label: t('common.actions') },
])

function formatNumber(value: number): string {
  return new Intl.NumberFormat(undefined, { maximumFractionDigits: 4 }).format(value)
}

function formatMoney(value: number): string {
  return `$${new Intl.NumberFormat(undefined, { minimumFractionDigits: 2, maximumFractionDigits: 2 }).format(value)}`
}

function formatDateTime(value: string): string {
  return new Intl.DateTimeFormat(undefined, { dateStyle: 'short', timeStyle: 'short' }).format(new Date(value))
}

function ruleStatus(rule: RechargeBonusRule): RuleStatus {
  if (!rule.enabled) return 'disabled'
  const now = Date.now()
  if (rule.starts_at && new Date(rule.starts_at).getTime() > now) return 'upcoming'
  if (rule.ends_at && new Date(rule.ends_at).getTime() < now) return 'ended'
  return 'active'
}

function statusClass(rule: RechargeBonusRule): string {
  const status = ruleStatus(rule)
  if (status === 'active') return 'bg-green-100 text-green-700 dark:bg-green-900/30 dark:text-green-300'
  if (status === 'upcoming') return 'bg-blue-100 text-blue-700 dark:bg-blue-900/30 dark:text-blue-300'
  if (status === 'ended') return 'bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-300'
  return 'bg-gray-100 text-gray-600 dark:bg-dark-700 dark:text-gray-300'
}

function toLocalDateTime(value?: string | null): string {
  if (!value) return ''
  const date = new Date(value)
  const offset = date.getTimezoneOffset() * 60_000
  return new Date(date.getTime() - offset).toISOString().slice(0, 16)
}

function openEditor(rule: RechargeBonusRule | null) {
  editingRule.value = rule
  Object.assign(form, rule ? {
    name: rule.name,
    enabled: rule.enabled,
    priority: rule.priority,
    minAmount: rule.min_amount,
    maxAmount: rule.max_amount ?? '',
    bonusType: rule.bonus_type,
    bonusValue: rule.bonus_value,
    startsAt: toLocalDateTime(rule.starts_at),
    endsAt: toLocalDateTime(rule.ends_at),
    notes: rule.notes ?? '',
  } : {
    name: '', enabled: true, priority: 0, minAmount: 1, maxAmount: '',
    bonusType: 'fixed', bonusValue: 1, startsAt: '', endsAt: '', notes: '',
  })
  editorOpen.value = true
}

function buildPayload(): RechargeBonusRulePayload | null {
  const maxAmount = form.maxAmount === '' ? null : Number(form.maxAmount)
  if (maxAmount !== null && maxAmount < form.minAmount) {
    appStore.showError(t('payment.admin.rechargeBonus.invalidAmountRange'))
    return null
  }
  if (form.bonusType === 'percentage' && form.bonusValue > 100) {
    appStore.showError(t('payment.admin.rechargeBonus.invalidPercentage'))
    return null
  }
  if (form.startsAt && form.endsAt && new Date(form.endsAt) <= new Date(form.startsAt)) {
    appStore.showError(t('payment.admin.rechargeBonus.invalidTimeRange'))
    return null
  }
  return {
    name: form.name,
    enabled: form.enabled,
    priority: form.priority,
    min_amount: form.minAmount,
    max_amount: maxAmount,
    bonus_type: form.bonusType,
    bonus_value: form.bonusValue,
    starts_at: form.startsAt ? new Date(form.startsAt).toISOString() : null,
    ends_at: form.endsAt ? new Date(form.endsAt).toISOString() : null,
    notes: form.notes || null,
  }
}

function rulePayload(rule: RechargeBonusRule, enabled = rule.enabled): RechargeBonusRulePayload {
  return {
    name: rule.name,
    enabled,
    priority: rule.priority,
    min_amount: rule.min_amount,
    max_amount: rule.max_amount ?? null,
    bonus_type: rule.bonus_type,
    bonus_value: rule.bonus_value,
    starts_at: rule.starts_at ?? null,
    ends_at: rule.ends_at ?? null,
    notes: rule.notes ?? null,
  }
}

async function loadRules() {
  loading.value = true
  try {
    const response = await adminPaymentAPI.listRechargeBonusRules()
    rules.value = response.data || []
  } catch (error: unknown) {
    appStore.showError(extractI18nErrorMessage(error, t, 'payment.errors', t('common.error')))
  } finally {
    loading.value = false
  }
}

async function saveRule() {
  const payload = buildPayload()
  if (!payload || saving.value) return
  saving.value = true
  try {
    if (editingRule.value) {
      await adminPaymentAPI.updateRechargeBonusRule(editingRule.value.id, payload)
    } else {
      await adminPaymentAPI.createRechargeBonusRule(payload)
    }
    editorOpen.value = false
    appStore.showSuccess(t('common.saved'))
    await loadRules()
  } catch (error: unknown) {
    appStore.showError(extractI18nErrorMessage(error, t, 'payment.errors', t('common.error')))
  } finally {
    saving.value = false
  }
}

async function toggleRule(rule: RechargeBonusRule) {
  try {
    await adminPaymentAPI.updateRechargeBonusRule(rule.id, rulePayload(rule, !rule.enabled))
    await loadRules()
  } catch (error: unknown) {
    appStore.showError(extractI18nErrorMessage(error, t, 'payment.errors', t('common.error')))
  }
}

function requestDelete(rule: RechargeBonusRule) {
  deletingRule.value = rule
  deleteOpen.value = true
}

async function deleteRule() {
  if (!deletingRule.value) return
  try {
    await adminPaymentAPI.deleteRechargeBonusRule(deletingRule.value.id)
    deleteOpen.value = false
    appStore.showSuccess(t('common.deleted'))
    await loadRules()
  } catch (error: unknown) {
    appStore.showError(extractI18nErrorMessage(error, t, 'payment.errors', t('payment.admin.rechargeBonus.deleteInUse')))
  }
}

onMounted(loadRules)
</script>
