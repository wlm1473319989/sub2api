<template>
  <AppLayout>
    <div class="space-y-4">
      <div v-if="loading" class="flex justify-center py-12">
        <div class="h-8 w-8 animate-spin rounded-full border-2 border-primary-500 border-t-transparent"></div>
      </div>

      <div v-else-if="ledger.length === 0" class="card p-12 text-center">
        <div class="mx-auto mb-4 flex h-16 w-16 items-center justify-center rounded-full bg-gray-100 dark:bg-dark-700">
          <Icon name="clipboard" size="xl" class="text-gray-400" />
        </div>
        <h3 class="mb-2 text-lg font-semibold text-gray-900 dark:text-white">
          {{ t('subscriptionLedger.emptyTitle') }}
        </h3>
        <p class="text-sm text-gray-500 dark:text-dark-400">
          {{ t('subscriptionLedger.emptyDesc') }}
        </p>
      </div>

      <template v-else>
        <div class="grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
          <div class="rounded-lg border border-gray-200 bg-white p-4 dark:border-dark-700 dark:bg-dark-800">
            <p class="text-xs font-medium uppercase text-gray-500 dark:text-dark-400">{{ t('subscriptionLedger.summary.totalOrders') }}</p>
            <p class="mt-2 text-2xl font-semibold text-gray-900 dark:text-white">{{ ledger.length }}</p>
          </div>
          <div class="rounded-lg border border-gray-200 bg-white p-4 dark:border-dark-700 dark:bg-dark-800">
            <p class="text-xs font-medium uppercase text-gray-500 dark:text-dark-400">{{ t('subscriptionLedger.summary.totalChains') }}</p>
            <p class="mt-2 text-2xl font-semibold text-gray-900 dark:text-white">{{ settlementChains.length }}</p>
          </div>
          <div class="rounded-lg border border-gray-200 bg-white p-4 dark:border-dark-700 dark:bg-dark-800">
            <p class="text-xs font-medium uppercase text-gray-500 dark:text-dark-400">{{ t('subscriptionLedger.summary.currentStatus') }}</p>
            <p class="mt-2 text-2xl font-semibold text-gray-900 dark:text-white">{{ afterStatusLabel(currentNode?.after_subscription_status) }}</p>
          </div>
          <div class="rounded-lg border border-gray-200 bg-white p-4 dark:border-dark-700 dark:bg-dark-800">
            <p class="text-xs font-medium uppercase text-gray-500 dark:text-dark-400">{{ t('subscriptionLedger.summary.totalWriteoff') }}</p>
            <p class="mt-2 text-2xl font-semibold text-gray-900 dark:text-white">{{ formatCurrency(totalWriteoff) }}</p>
          </div>
        </div>

        <div class="space-y-4">
          <section
            v-for="(chain, chainIndex) in settlementChains"
            :key="chain.root.id"
            class="overflow-hidden rounded-lg border border-gray-200 bg-white dark:border-dark-700 dark:bg-dark-800"
          >
            <div class="flex flex-col gap-3 border-b border-gray-200 bg-gray-50 px-4 py-3 dark:border-dark-700 dark:bg-dark-900/60 sm:flex-row sm:items-center sm:justify-between">
              <div>
                <h3 class="text-sm font-semibold text-gray-900 dark:text-white">
                  {{ t('subscriptionLedger.chain.title', { index: chainIndex + 1 }) }}
                </h3>
                <p class="mt-1 text-xs text-gray-500 dark:text-dark-400">
                  {{ t('subscriptionLedger.chain.head') }} #{{ chain.root.id }} · {{ formatDateTime(chain.root.effective_at) }}
                </p>
              </div>
              <div class="flex flex-wrap gap-2">
                <span :class="['inline-flex rounded-full px-2 py-0.5 text-xs font-medium', chainStateClass(chain)]">
                  {{ chainStateLabel(chain) }}
                </span>
                <span class="inline-flex rounded-full bg-gray-100 px-2 py-0.5 text-xs font-medium text-gray-600 dark:bg-dark-700 dark:text-gray-300">
                  {{ t('subscriptionLedger.chain.nodes', { count: chain.nodes.length }) }}
                </span>
              </div>
            </div>

            <div class="px-4 py-4">
              <div
                v-for="(node, nodeIndex) in chain.nodes"
                :key="node.order.id"
                class="relative pb-5 pl-8 last:pb-0"
                :style="{ marginLeft: `${node.depth * 1.25}rem` }"
              >
                <div
                  v-if="nodeIndex < chain.nodes.length - 1"
                  class="absolute left-2 top-6 h-[calc(100%-1rem)] w-px bg-gray-200 dark:bg-dark-700"
                ></div>
                <div :class="['absolute left-0 top-1 h-5 w-5 rounded-full border-2 bg-white dark:bg-dark-800', nodeDotClass(node.order, nodeIndex)]"></div>

                <div class="grid gap-4 lg:grid-cols-[minmax(0,1fr)_minmax(280px,auto)]">
                  <div class="min-w-0">
                    <div class="flex flex-wrap items-center gap-2">
                      <span :class="['inline-flex rounded-full px-2 py-0.5 text-xs font-semibold', actionClass(node.order.action_type)]">
                        {{ actionLabel(node.order.action_type) }}
                      </span>
                      <span :class="['inline-flex rounded-full px-2 py-0.5 text-xs font-medium', settlementStatusClass(node.order.status)]">
                        {{ nodeRoleLabel(chain, nodeIndex) }}
                      </span>
                      <span class="text-xs text-gray-500 dark:text-dark-400">{{ sourceLabel(node.order.action_source) }}</span>
                    </div>

                    <p class="mt-2 truncate text-sm font-medium text-gray-900 dark:text-white">{{ planLabel(node.order) }}</p>
                    <div class="mt-1 flex flex-wrap gap-x-3 gap-y-1 text-xs text-gray-500 dark:text-dark-400">
                      <span>{{ formatDateTime(node.order.effective_at) }}</span>
                      <span>{{ subscriptionRef(node.order) }}</span>
                      <span v-if="node.order.prev_settlement_id">{{ t('subscriptionLedger.chain.previous') }} #{{ node.order.prev_settlement_id }}</span>
                    </div>
                    <p v-if="node.order.action_note" class="mt-2 text-xs text-gray-500 dark:text-dark-400">
                      {{ node.order.action_note }}
                    </p>
                  </div>

                  <div class="grid grid-cols-2 gap-3 text-sm sm:grid-cols-4 lg:grid-cols-2">
                    <div>
                      <p class="text-xs text-gray-500 dark:text-dark-400">{{ t('subscriptionLedger.values.carryIn') }}</p>
                      <p class="font-medium text-gray-900 dark:text-white">{{ formatCurrency(node.order.carry_in_residual_value) }}</p>
                    </div>
                    <div>
                      <p class="text-xs text-gray-500 dark:text-dark-400">{{ t('subscriptionLedger.values.delta') }}</p>
                      <p class="font-medium text-gray-900 dark:text-white">{{ formatSignedCurrency(node.order.action_delta_value) }}</p>
                    </div>
                    <div>
                      <p class="text-xs text-gray-500 dark:text-dark-400">{{ t('subscriptionLedger.values.after') }}</p>
                      <p class="font-medium text-gray-900 dark:text-white">{{ formatCurrency(node.order.after_settlement_value) }}</p>
                    </div>
                    <div v-if="node.order.refund_residual_value != null">
                      <p class="text-xs text-gray-500 dark:text-dark-400">{{ t('subscriptionLedger.values.refund') }}</p>
                      <p class="font-medium text-gray-900 dark:text-white">{{ formatCurrency(node.order.refund_residual_value) }}</p>
                    </div>
                    <div v-if="node.order.writeoff_value > 0">
                      <p class="text-xs text-gray-500 dark:text-dark-400">{{ t('subscriptionLedger.values.writeoff') }}</p>
                      <p class="font-medium text-gray-900 dark:text-white">{{ formatCurrency(node.order.writeoff_value) }}</p>
                    </div>
                    <div>
                      <p class="text-xs text-gray-500 dark:text-dark-400">{{ t('subscriptionLedger.columns.status') }}</p>
                      <p class="font-medium text-gray-900 dark:text-white">{{ afterStatusLabel(node.order.after_subscription_status) }}</p>
                    </div>
                  </div>
                </div>
              </div>
            </div>
          </section>
        </div>
      </template>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
import subscriptionsAPI from '@/api/subscriptions'
import type { SubscriptionSettlementActionSource, SubscriptionSettlementActionType, SubscriptionSettlementOrder, SubscriptionSettlementStatus, UserSubscription } from '@/types'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import { formatCurrency, formatDateTime } from '@/utils/format'

interface SettlementChainNode {
  order: SubscriptionSettlementOrder
  depth: number
}

interface SettlementChain {
  root: SubscriptionSettlementOrder
  nodes: SettlementChainNode[]
  isCurrent: boolean
}

const { t } = useI18n()
const appStore = useAppStore()

const ledger = ref<SubscriptionSettlementOrder[]>([])
const loading = ref(true)

const sortedLedger = computed(() => [...ledger.value].sort(compareSettlementOrders))
const currentNode = computed(() => sortedLedger.value.find((item) => item.status === 'effective') ?? sortedLedger.value[sortedLedger.value.length - 1] ?? null)
const totalWriteoff = computed(() => ledger.value.reduce((sum, item) => sum + Math.max(item.writeoff_value || 0, 0), 0))

const settlementChains = computed<SettlementChain[]>(() => {
  const orders = sortedLedger.value
  const byID = new Map<number, SubscriptionSettlementOrder>()
  for (const order of orders) {
    byID.set(order.id, order)
  }

  const childrenByPrevious = new Map<number, SubscriptionSettlementOrder[]>()
  for (const order of orders) {
    const previousID = order.prev_settlement_id
    if (previousID == null || !byID.has(previousID)) continue
    const children = childrenByPrevious.get(previousID) ?? []
    children.push(order)
    childrenByPrevious.set(previousID, children)
  }
  for (const children of childrenByPrevious.values()) {
    children.sort(compareSettlementOrders)
  }

  const roots = orders.filter((order) => order.prev_settlement_id == null || !byID.has(order.prev_settlement_id))
  const visited = new Set<number>()
  const chains: SettlementChain[] = []

  function appendNode(order: SubscriptionSettlementOrder, nodes: SettlementChainNode[], depth: number) {
    if (visited.has(order.id)) return
    visited.add(order.id)
    nodes.push({ order, depth })
    for (const child of childrenByPrevious.get(order.id) ?? []) {
      appendNode(child, nodes, depth + 1)
    }
  }

  for (const root of roots) {
    const nodes: SettlementChainNode[] = []
    appendNode(root, nodes, 0)
    chains.push({
      root,
      nodes,
      isCurrent: nodes.some((node) => node.order.status === 'effective'),
    })
  }

  for (const order of orders) {
    if (visited.has(order.id)) continue
    const nodes: SettlementChainNode[] = []
    appendNode(order, nodes, 0)
    chains.push({
      root: order,
      nodes,
      isCurrent: nodes.some((node) => node.order.status === 'effective'),
    })
  }

  return chains
})

async function loadLedger() {
  try {
    loading.value = true
    ledger.value = await subscriptionsAPI.getSubscriptionLedger()
  } catch (error) {
    console.error('Failed to load subscription ledger:', error)
    appStore.showError(t('subscriptionLedger.failedToLoad'))
  } finally {
    loading.value = false
  }
}

function compareSettlementOrders(a: SubscriptionSettlementOrder, b: SubscriptionSettlementOrder): number {
  const timeDiff = timestamp(a.effective_at) - timestamp(b.effective_at)
  if (timeDiff !== 0) return timeDiff
  return a.id - b.id
}

function timestamp(value: string): number {
  const parsed = new Date(value).getTime()
  return Number.isFinite(parsed) ? parsed : 0
}

function translated(key: string, fallback: string): string {
  const label = t(key)
  return label === key ? fallback : label
}

function actionLabel(action: SubscriptionSettlementActionType | string): string {
  return translated(`subscriptionLedger.actions.${action}`, action)
}

function sourceLabel(source: SubscriptionSettlementActionSource | string): string {
  return translated(`subscriptionLedger.sources.${source}`, source)
}

function afterStatusLabel(status?: UserSubscription['status'] | string): string {
  if (!status) return t('common.unknown')
  return translated(`userSubscriptions.status.${status}`, status)
}

function chainStateLabel(chain: SettlementChain): string {
  return chain.isCurrent ? t('subscriptionLedger.chain.current') : t('subscriptionLedger.chain.closed')
}

function chainStateClass(chain: SettlementChain): string {
  return chain.isCurrent
    ? 'bg-green-100 text-green-700 dark:bg-green-900/40 dark:text-green-300'
    : 'bg-gray-100 text-gray-600 dark:bg-dark-700 dark:text-gray-400'
}

function nodeRoleLabel(chain: SettlementChain, nodeIndex: number): string {
  const node = chain.nodes[nodeIndex]
  if (nodeIndex === 0) return t('subscriptionLedger.chain.head')
  if (node.order.status === 'effective') return t('subscriptionLedger.chain.currentNode')
  return translated(`subscriptionLedger.status.${node.order.status}`, node.order.status)
}

function nodeDotClass(order: SubscriptionSettlementOrder, nodeIndex: number): string {
  if (nodeIndex === 0) return 'border-emerald-500'
  if (order.status === 'effective') return 'border-primary-500'
  return 'border-gray-300 dark:border-dark-600'
}

function actionClass(action: SubscriptionSettlementActionType | string): string {
  switch (action) {
    case 'purchase':
      return 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/40 dark:text-emerald-300'
    case 'renew':
      return 'bg-sky-100 text-sky-700 dark:bg-sky-900/40 dark:text-sky-300'
    case 'upgrade':
      return 'bg-indigo-100 text-indigo-700 dark:bg-indigo-900/40 dark:text-indigo-300'
    case 'refund':
      return 'bg-purple-100 text-purple-700 dark:bg-purple-900/40 dark:text-purple-300'
    case 'revoke':
    default:
      return 'bg-red-100 text-red-700 dark:bg-red-900/40 dark:text-red-300'
  }
}

function settlementStatusClass(status: SubscriptionSettlementStatus | string): string {
  return status === 'effective'
    ? 'bg-green-100 text-green-700 dark:bg-green-900/40 dark:text-green-300'
    : 'bg-gray-100 text-gray-600 dark:bg-dark-700 dark:text-gray-400'
}

function planLabel(order: SubscriptionSettlementOrder): string {
  if (order.after_plan_name_snapshot?.trim()) return order.after_plan_name_snapshot
  if (order.after_plan_id) return `${t('payment.plan')} #${order.after_plan_id}`
  return t('subscriptionLedger.planUnknown')
}

function subscriptionRef(order: SubscriptionSettlementOrder): string {
  if (!order.after_user_subscription_id) return t('subscriptionLedger.subscriptionUnknown')
  return `${t('subscriptionLedger.subscriptionNo')} #${order.after_user_subscription_id}`
}

function formatSignedCurrency(value: number): string {
  if (value > 0) return `+${formatCurrency(value)}`
  if (value < 0) return `-${formatCurrency(Math.abs(value))}`
  return formatCurrency(0)
}

onMounted(() => {
  loadLedger()
})
</script>
