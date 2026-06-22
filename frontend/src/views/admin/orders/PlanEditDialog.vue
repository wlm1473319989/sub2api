<template>
  <BaseDialog :show="show" :title="plan ? t('payment.admin.editPlan') : t('payment.admin.createPlan')" width="wide" @close="emit('close')">
    <form id="plan-form" @submit.prevent="handleSavePlan" class="space-y-4">
      <div class="grid grid-cols-2 gap-4">
        <div>
          <label class="input-label">{{ t('payment.admin.planName') }} <span class="text-red-500">*</span></label>
          <input v-model="planForm.name" type="text" class="input" required />
        </div>
        <div>
          <label class="input-label">{{ t('payment.planCard.quota') }}</label>
          <div class="flex h-[42px] items-center rounded-xl border border-gray-200 bg-gray-50 px-3 text-sm text-gray-600 dark:border-dark-600 dark:bg-dark-800 dark:text-gray-300">
            {{ t('payment.admin.userLevelPlan') }}
          </div>
        </div>
      </div>

      <div>
        <label class="input-label">{{ t('payment.admin.planDescription') }} <span class="text-red-500">*</span></label>
        <textarea v-model="planForm.description" rows="2" class="input" required></textarea>
      </div>

      <div class="grid grid-cols-2 gap-4">
        <div>
          <label class="input-label">{{ t('payment.admin.price') }} <span class="text-red-500">*</span></label>
          <input v-model.number="planForm.price" type="number" step="0.01" min="0.01" class="input" required />
        </div>
        <div>
          <label class="input-label">{{ t('payment.admin.originalPrice') }}</label>
          <input v-model.number="planForm.original_price" type="number" step="0.01" min="0" class="input" />
        </div>
      </div>

      <div class="grid grid-cols-2 gap-4">
        <div>
          <label class="input-label">{{ t('payment.admin.validityDays') }} <span class="text-red-500">*</span></label>
          <input v-model.number="planForm.validity_days" type="number" min="1" class="input" required />
        </div>
        <div>
          <label class="input-label">{{ t('payment.admin.validityUnit') }} <span class="text-red-500">*</span></label>
          <Select v-model="planForm.validity_unit" :options="validityUnitOptions" />
        </div>
      </div>

      <div class="grid grid-cols-3 gap-4">
        <div>
          <label class="input-label">{{ t('payment.planCard.dailyLimit') }}</label>
          <input v-model.number="planForm.daily_quota_knives" type="number" min="0" step="0.01" class="input" :placeholder="t('payment.admin.unlimited')" />
        </div>
        <div>
          <label class="input-label">{{ t('payment.planCard.weeklyLimit') }}</label>
          <input v-model.number="planForm.weekly_quota_knives" type="number" min="0" step="0.01" class="input" :placeholder="t('payment.admin.unlimited')" />
        </div>
        <div>
          <label class="input-label">{{ t('payment.planCard.monthlyLimit') }}</label>
          <input v-model.number="planForm.monthly_quota_knives" type="number" min="0" step="0.01" class="input" :placeholder="t('payment.admin.unlimited')" />
        </div>
      </div>

      <div class="grid grid-cols-2 gap-4">
        <div>
          <label class="input-label">{{ t('payment.admin.sortOrder') }}</label>
          <input v-model.number="planForm.sort_order" type="number" min="0" class="input" />
        </div>
      </div>

      <div>
        <label class="input-label">{{ t('payment.admin.features') }}</label>
        <textarea v-model="planFeaturesText" rows="3" class="input" :placeholder="t('payment.admin.featuresPlaceholder')"></textarea>
        <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">{{ t('payment.admin.featuresHint') }}</p>
      </div>

      <div class="flex items-center gap-3">
        <label class="text-sm text-gray-700 dark:text-gray-300">{{ t('payment.admin.forSale') }}</label>
        <button
          type="button"
          :class="[
            'relative inline-flex h-6 w-11 flex-shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out focus:outline-none focus:ring-2 focus:ring-primary-500 focus:ring-offset-2',
            planForm.for_sale ? 'bg-primary-500' : 'bg-gray-300 dark:bg-dark-600'
          ]"
          @click="planForm.for_sale = !planForm.for_sale"
        >
          <span
            :class="[
              'pointer-events-none inline-block h-5 w-5 transform rounded-full bg-white shadow ring-0 transition duration-200 ease-in-out',
              planForm.for_sale ? 'translate-x-5' : 'translate-x-0'
            ]"
          />
        </button>
      </div>
    </form>

    <template #footer>
      <div class="flex justify-end gap-3">
        <button type="button" @click="emit('close')" class="btn btn-secondary">{{ t('common.cancel') }}</button>
        <button type="submit" form="plan-form" :disabled="saving" class="btn btn-primary">{{ saving ? t('common.saving') : t('common.save') }}</button>
      </div>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminPaymentAPI, type PlanPayload } from '@/api/admin/payment'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'
import type { SubscriptionPlan } from '@/types/payment'
import type { AdminGroup } from '@/types'
import BaseDialog from '@/components/common/BaseDialog.vue'

const props = defineProps<{
  show: boolean
  plan: SubscriptionPlan | null
  groups: AdminGroup[]
}>()

const emit = defineEmits<{
  close: []
  saved: []
}>()

const { t } = useI18n()
const appStore = useAppStore()

const saving = ref(false)
const planFeaturesText = ref('')
const planForm = reactive({
  name: '',
  description: '',
  price: 0,
  original_price: 0,
  validity_days: 30,
  validity_unit: 'day',
  daily_quota_knives: null as number | null,
  weekly_quota_knives: null as number | null,
  monthly_quota_knives: null as number | null,
  sort_order: 0,
  for_sale: true,
})

const validityUnitOptions = computed(() => [
  { value: 'day', label: t('payment.admin.days') },
  { value: 'week', label: t('payment.admin.weeks') },
  { value: 'month', label: t('payment.admin.months') },
])

watch(
  () => props.show,
  (visible) => {
    if (!visible) return
    if (props.plan) {
      Object.assign(planForm, {
        name: props.plan.name,
        description: props.plan.description,
        price: props.plan.price,
        original_price: props.plan.original_price || 0,
        validity_days: props.plan.validity_days,
        validity_unit: props.plan.validity_unit || 'day',
        daily_quota_knives: props.plan.daily_quota_knives ?? null,
        weekly_quota_knives: props.plan.weekly_quota_knives ?? null,
        monthly_quota_knives: props.plan.monthly_quota_knives ?? null,
        sort_order: props.plan.sort_order || 0,
        for_sale: props.plan.for_sale,
      })
      planFeaturesText.value = (props.plan.features || []).join('\n')
      return
    }

    Object.assign(planForm, {
      name: '',
      description: '',
      price: 0,
      original_price: 0,
      validity_days: 30,
      validity_unit: 'day',
      daily_quota_knives: null,
      weekly_quota_knives: null,
      monthly_quota_knives: null,
      sort_order: 0,
      for_sale: true,
    })
    planFeaturesText.value = ''
  },
)

function normalizedQuota(value: number | null): number | null {
  if (value == null || value <= 0) return null
  return value
}

function buildPlanPayload(): PlanPayload {
  const features = planFeaturesText.value
    .split('\n')
    .map((line) => line.trim())
    .filter(Boolean)
    .join('\n')

  return {
    name: planForm.name,
    description: planForm.description,
    price: planForm.price,
    original_price: planForm.original_price > 0 ? planForm.original_price : null,
    validity_days: planForm.validity_days,
    validity_unit: planForm.validity_unit,
    daily_quota_knives: normalizedQuota(planForm.daily_quota_knives),
    weekly_quota_knives: normalizedQuota(planForm.weekly_quota_knives),
    monthly_quota_knives: normalizedQuota(planForm.monthly_quota_knives),
    features,
    for_sale: planForm.for_sale,
    sort_order: planForm.sort_order,
  }
}

async function handleSavePlan() {
  if (
    !normalizedQuota(planForm.daily_quota_knives) &&
    !normalizedQuota(planForm.weekly_quota_knives) &&
    !normalizedQuota(planForm.monthly_quota_knives)
  ) {
    appStore.showError(t('payment.planCard.quota'))
    return
  }
  if (!planForm.price || planForm.price <= 0) {
    appStore.showError(t('payment.admin.priceRequired'))
    return
  }
  if (!planForm.validity_days || planForm.validity_days < 1) {
    appStore.showError(t('payment.admin.validityDaysRequired'))
    return
  }

  saving.value = true
  try {
    const data = buildPlanPayload()
    if (props.plan) {
      await adminPaymentAPI.updatePlan(props.plan.id, data)
    } else {
      await adminPaymentAPI.createPlan(data)
    }
    appStore.showSuccess(t('common.saved'))
    emit('close')
    emit('saved')
  } catch (err: unknown) {
    appStore.showError(extractApiErrorMessage(err, t('common.error')))
  } finally {
    saving.value = false
  }
}
</script>
