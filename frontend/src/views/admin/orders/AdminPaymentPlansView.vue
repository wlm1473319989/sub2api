<template>
  <AppLayout>
    <div class="space-y-4">
      <div class="flex items-center justify-end gap-2">
        <button @click="loadPlans" :disabled="plansLoading" class="btn btn-secondary" :title="t('common.refresh')">
          <Icon name="refresh" size="md" :class="plansLoading ? 'animate-spin' : ''" />
        </button>
        <button @click="openPlanEdit(null)" class="btn btn-primary">{{ t('payment.admin.createPlan') }}</button>
      </div>

      <DataTable :columns="planColumns" :data="plans" :loading="plansLoading">
        <template #cell-name="{ value, row }">
          <span class="text-sm font-medium" :class="getPlanNameClass(row)">{{ value }}</span>
        </template>

        <template #cell-price="{ value, row }">
          <div class="text-sm">
            <span class="font-medium text-gray-900 dark:text-white">${{ (value ?? 0).toFixed(2) }}</span>
            <span v-if="row.original_price" class="ml-1 text-xs text-gray-400 line-through">${{ row.original_price.toFixed(2) }}</span>
          </div>
        </template>

        <template #cell-validity_days="{ value, row }">
          <span class="text-sm">{{ value }} {{ t('payment.admin.' + validityUnitLabel(row.validity_unit)) }}</span>
        </template>

        <template #cell-quotas="{ row }">
          <div class="space-y-1 text-xs text-gray-600 dark:text-gray-300">
            <div v-if="row.daily_quota_knives != null">D: {{ row.daily_quota_knives }}</div>
            <div v-if="row.weekly_quota_knives != null">W: {{ row.weekly_quota_knives }}</div>
            <div v-if="row.monthly_quota_knives != null">M: {{ row.monthly_quota_knives }}</div>
            <div v-if="row.daily_quota_knives == null && row.weekly_quota_knives == null && row.monthly_quota_knives == null" class="text-gray-400">
              {{ t('payment.admin.unlimited') }}
            </div>
          </div>
        </template>

        <template #cell-for_sale="{ value, row }">
          <button
            type="button"
            :class="[
              'relative inline-flex h-5 w-9 flex-shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out focus:outline-none focus:ring-2 focus:ring-primary-500 focus:ring-offset-2',
              value ? 'bg-primary-500' : 'bg-gray-300 dark:bg-dark-600'
            ]"
            @click="toggleForSale(row)"
          >
            <span
              :class="[
                'pointer-events-none inline-block h-4 w-4 transform rounded-full bg-white shadow ring-0 transition duration-200 ease-in-out',
                value ? 'translate-x-4' : 'translate-x-0'
              ]"
            />
          </button>
        </template>

        <template #cell-actions="{ row }">
          <div class="flex items-center gap-2">
            <button @click="openPlanEdit(row)" class="flex flex-col items-center gap-0.5 rounded-lg p-1.5 text-gray-500 transition-colors hover:bg-blue-50 hover:text-blue-600 dark:hover:bg-blue-900/20 dark:hover:text-blue-400">
              <Icon name="edit" size="sm" />
              <span class="text-xs">{{ t('common.edit') }}</span>
            </button>
            <button @click="confirmDeletePlan(row)" class="flex flex-col items-center gap-0.5 rounded-lg p-1.5 text-gray-500 transition-colors hover:bg-red-50 hover:text-red-600 dark:hover:bg-red-900/20 dark:hover:text-red-400">
              <Icon name="trash" size="sm" />
              <span class="text-xs">{{ t('common.delete') }}</span>
            </button>
          </div>
        </template>
      </DataTable>
    </div>

    <PlanEditDialog :show="showPlanDialog" :plan="editingPlan" :groups="groups" @close="showPlanDialog = false" @saved="loadPlans" />

    <ConfirmDialog
      :show="showDeletePlanDialog"
      :title="t('payment.admin.deletePlan')"
      :message="t('payment.admin.deletePlanConfirm')"
      :confirm-text="t('common.delete')"
      danger
      @confirm="handleDeletePlan"
      @cancel="showDeletePlanDialog = false"
    />
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import adminAPI from '@/api/admin'
import { adminPaymentAPI } from '@/api/admin/payment'
import type { SubscriptionPlan } from '@/types/payment'
import type { AdminGroup } from '@/types'
import type { Column } from '@/components/common/types'
import { extractI18nErrorMessage } from '@/utils/apiError'
import { useAppStore } from '@/stores/app'
import AppLayout from '@/components/layout/AppLayout.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import DataTable from '@/components/common/DataTable.vue'
import Icon from '@/components/icons/Icon.vue'
import PlanEditDialog from './PlanEditDialog.vue'

const { t } = useI18n()
const appStore = useAppStore()

const groups = ref<AdminGroup[]>([])
const plans = ref<SubscriptionPlan[]>([])
const plansLoading = ref(false)
const showPlanDialog = ref(false)
const showDeletePlanDialog = ref(false)
const editingPlan = ref<SubscriptionPlan | null>(null)
const deletingPlanId = ref<number | null>(null)

const planColumns = computed((): Column[] => [
  { key: 'id', label: 'ID' },
  { key: 'name', label: t('payment.admin.planName') },
  { key: 'price', label: t('payment.admin.price') },
  { key: 'quotas', label: t('payment.planCard.quota') },
  { key: 'validity_days', label: t('payment.admin.validityDays') },
  { key: 'for_sale', label: t('payment.admin.forSale') },
  { key: 'sort_order', label: t('payment.admin.sortOrder') },
  { key: 'actions', label: t('common.actions') },
])

function validityUnitLabel(unit?: string | null): 'days' | 'weeks' | 'months' {
  switch (unit) {
    case 'week':
      return 'weeks'
    case 'month':
      return 'months'
    default:
      return 'days'
  }
}

function normalizePlanFeatures(plan: SubscriptionPlan): SubscriptionPlan {
  return {
    ...plan,
    features: Array.isArray(plan.features)
      ? plan.features
      : String((plan as SubscriptionPlan & { features?: string }).features || '')
          .split('\n')
          .map((feature) => feature.trim())
          .filter(Boolean),
  }
}

async function loadGroups() {
  try {
    groups.value = await adminAPI.groups.getAll()
  } catch {
    // ignore group preview failures
  }
}

function getPlanNameClass(plan: SubscriptionPlan): string {
  void plan
  return 'text-gray-900 dark:text-white'
}

async function loadPlans() {
  plansLoading.value = true
  try {
    const res = await adminPaymentAPI.getPlans()
    plans.value = (res.data || []).map(normalizePlanFeatures)
  } catch (err: unknown) {
    appStore.showError(extractI18nErrorMessage(err, t, 'payment.errors', t('common.error')))
  } finally {
    plansLoading.value = false
  }
}

function openPlanEdit(plan: SubscriptionPlan | null) {
  editingPlan.value = plan
  showPlanDialog.value = true
}

async function toggleForSale(plan: SubscriptionPlan) {
  try {
    await adminPaymentAPI.updatePlan(plan.id, { for_sale: !plan.for_sale })
    plan.for_sale = !plan.for_sale
  } catch (err: unknown) {
    appStore.showError(extractI18nErrorMessage(err, t, 'payment.errors', t('common.error')))
  }
}

function confirmDeletePlan(plan: SubscriptionPlan) {
  deletingPlanId.value = plan.id
  showDeletePlanDialog.value = true
}

async function handleDeletePlan() {
  if (!deletingPlanId.value) return
  try {
    await adminPaymentAPI.deletePlan(deletingPlanId.value)
    appStore.showSuccess(t('common.deleted'))
    showDeletePlanDialog.value = false
    await loadPlans()
  } catch (err: unknown) {
    appStore.showError(extractI18nErrorMessage(err, t, 'payment.errors', t('common.error')))
  }
}

onMounted(() => {
  loadGroups()
  loadPlans()
})
</script>
