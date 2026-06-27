<template>
  <AppLayout>
    <TablePageLayout>
      <template #filters>
        <!-- Top Toolbar: Left (search + filters) / Right (actions) -->
        <div class="flex flex-wrap items-start justify-between gap-4">
          <!-- Left: Fuzzy user search + filters (wrap to multiple lines) -->
          <div class="flex flex-1 flex-wrap items-center gap-3">
            <!-- User Search -->
            <div
              class="relative w-full sm:w-64"
              data-filter-user-search
            >
              <Icon
                name="search"
                size="md"
                class="absolute left-3 top-1/2 -translate-y-1/2 text-gray-400"
              />
              <input
                v-model="filterUserKeyword"
                type="text"
                :placeholder="t('admin.users.searchUsers')"
                class="input pl-10 pr-8"
                @input="debounceSearchFilterUsers"
                @focus="showFilterUserDropdown = true"
              />
              <button
                v-if="selectedFilterUser"
                @click="clearFilterUser"
                type="button"
                class="absolute right-2 top-1/2 -translate-y-1/2 text-gray-400 hover:text-gray-600 dark:hover:text-gray-300"
                :title="t('common.clear')"
              >
                <Icon name="x" size="sm" :stroke-width="2" />
              </button>

              <!-- User Dropdown -->
              <div
                v-if="showFilterUserDropdown && (filterUserResults.length > 0 || filterUserKeyword)"
                class="absolute z-50 mt-1 max-h-60 w-full overflow-auto rounded-lg border border-gray-200 bg-white shadow-lg dark:border-gray-700 dark:bg-gray-800"
              >
                <div
                  v-if="filterUserLoading"
                  class="px-4 py-3 text-sm text-gray-500 dark:text-gray-400"
                >
                  {{ t('common.loading') }}
                </div>
                <div
                  v-else-if="filterUserResults.length === 0 && filterUserKeyword"
                  class="px-4 py-3 text-sm text-gray-500 dark:text-gray-400"
                >
                  {{ t('common.noOptionsFound') }}
                </div>
                <button
                  v-for="user in filterUserResults"
                  :key="user.id"
                  type="button"
                  @click="selectFilterUser(user)"
                  class="w-full px-4 py-2 text-left text-sm hover:bg-gray-100 dark:hover:bg-gray-700"
                >
                  <span class="font-medium text-gray-900 dark:text-white">{{ user.email }}</span>
                  <span class="ml-2 text-gray-500 dark:text-gray-400">#{{ user.id }}</span>
                </button>
              </div>
            </div>

            <!-- Filters -->
            <div class="w-full sm:w-40">
              <Select
                v-model="filters.status"
                :options="statusOptions"
                :placeholder="t('admin.subscriptions.allStatus')"
                @change="applyFilters"
              />
            </div>
          </div>

          <!-- Right: Actions -->
          <div class="ml-auto flex flex-wrap items-center justify-end gap-3">
            <button
              @click="loadSubscriptions"
              :disabled="loading"
              class="btn btn-secondary"
              :title="t('common.refresh')"
            >
              <Icon name="refresh" size="md" :class="loading ? 'animate-spin' : ''" />
            </button>
            <!-- Column Settings Dropdown -->
            <div class="relative" ref="columnDropdownRef">
              <button
                @click="showColumnDropdown = !showColumnDropdown"
                class="btn btn-secondary px-2 md:px-3"
                :title="t('admin.users.columnSettings')"
              >
                <svg class="h-4 w-4 md:mr-1.5" fill="none" stroke="currentColor" viewBox="0 0 24 24" stroke-width="1.5">
                  <path stroke-linecap="round" stroke-linejoin="round" d="M9 4.5v15m6-15v15m-10.875 0h15.75c.621 0 1.125-.504 1.125-1.125V5.625c0-.621-.504-1.125-1.125-1.125H4.125C3.504 4.5 3 5.004 3 5.625v12.75c0 .621.504 1.125 1.125 1.125z" />
                </svg>
                <span class="hidden md:inline">{{ t('admin.users.columnSettings') }}</span>
              </button>
              <!-- Dropdown menu -->
              <div
                v-if="showColumnDropdown"
                class="absolute right-0 z-50 mt-2 w-48 origin-top-right rounded-lg border border-gray-200 bg-white shadow-lg dark:border-gray-700 dark:bg-gray-800"
              >
                <div class="p-2">
                  <!-- User column mode selection -->
                  <div class="mb-2 border-b border-gray-200 pb-2 dark:border-gray-700">
                    <div class="px-3 py-1 text-xs font-medium text-gray-500 dark:text-gray-400">
                      {{ t('admin.subscriptions.columns.user') }}
                    </div>
                    <button
                      @click="setUserColumnMode('email')"
                      class="flex w-full items-center justify-between rounded-md px-3 py-2 text-sm text-gray-700 hover:bg-gray-100 dark:text-gray-200 dark:hover:bg-gray-700"
                    >
                      <span>{{ t('admin.users.columns.email') }}</span>
                      <Icon v-if="userColumnMode === 'email'" name="check" size="sm" class="text-primary-500" />
                    </button>
                    <button
                      @click="setUserColumnMode('username')"
                      class="flex w-full items-center justify-between rounded-md px-3 py-2 text-sm text-gray-700 hover:bg-gray-100 dark:text-gray-200 dark:hover:bg-gray-700"
                    >
                      <span>{{ t('admin.users.columns.username') }}</span>
                      <Icon v-if="userColumnMode === 'username'" name="check" size="sm" class="text-primary-500" />
                    </button>
                  </div>
                  <!-- Other columns toggle -->
                  <button
                    v-for="col in toggleableColumns"
                    :key="col.key"
                    @click="toggleColumn(col.key)"
                    class="flex w-full items-center justify-between rounded-md px-3 py-2 text-sm text-gray-700 hover:bg-gray-100 dark:text-gray-200 dark:hover:bg-gray-700"
                  >
                    <span>{{ col.label }}</span>
                    <Icon v-if="isColumnVisible(col.key)" name="check" size="sm" class="text-primary-500" />
                  </button>
                </div>
              </div>
            </div>
            <button
              @click="showGuideModal = true"
              class="btn btn-secondary"
              :title="t('admin.subscriptions.guide.showGuide')"
            >
              <Icon name="questionCircle" size="md" />
            </button>
            <button
              v-if="selectedAdjustableCount > 0"
              @click="openBulkExtendModal"
              class="btn btn-secondary"
              data-test="batch-adjust-open"
            >
              <Icon name="calendar" size="md" class="mr-2" />
              {{ t('admin.subscriptions.batchAdjustSubscription') }}
              <span class="ml-1">{{ t('common.selectedCount', { count: selectedAdjustableCount }) }}</span>
            </button>
            <button
              v-if="selectedResettableCount > 0"
              @click="openBulkResetQuotaModal"
              class="btn btn-secondary"
              data-test="batch-reset-open"
            >
              <Icon name="refresh" size="md" class="mr-2" />
              {{ t('admin.subscriptions.batchResetQuota') }}
              <span class="ml-1">{{ t('common.selectedCount', { count: selectedResettableCount }) }}</span>
            </button>
            <button @click="showAssignModal = true" class="btn btn-primary">
              <Icon name="plus" size="md" class="mr-2" />
              {{ t('admin.subscriptions.assignSubscription') }}
            </button>
          </div>
        </div>
      </template>

      <!-- Subscriptions Table -->
      <template #table>
        <DataTable
          :columns="columns"
          :data="subscriptions"
          :loading="loading"
          :server-side-sort="true"
          default-sort-key="created_at"
          default-sort-order="desc"
          @sort="handleSort"
        >
          <template #header-select>
            <div class="flex items-center justify-center">
              <input
                type="checkbox"
                class="h-4 w-4 rounded border-gray-300 text-primary-600 focus:ring-primary-500 disabled:cursor-not-allowed disabled:opacity-50"
                :checked="allAdjustableSelected"
                :disabled="adjustableSubscriptions.length === 0"
                data-test="select-all-subscriptions"
                @change="handleSelectAllChange"
              />
            </div>
          </template>

          <template #cell-select="{ row }">
            <div class="flex items-center justify-center">
              <input
                type="checkbox"
                class="h-4 w-4 rounded border-gray-300 text-primary-600 focus:ring-primary-500 disabled:cursor-not-allowed disabled:opacity-50"
                :checked="isSubscriptionSelected(row.id)"
                :disabled="!canAdjustSubscription(row)"
                data-test="subscription-select"
                @change="handleSubscriptionSelectionChange(row, $event)"
              />
            </div>
          </template>

          <template #cell-user="{ row }">
            <div class="flex items-center gap-2">
              <div
                class="flex h-8 w-8 items-center justify-center rounded-full bg-primary-100 dark:bg-primary-900/30"
              >
                <span class="text-sm font-medium text-primary-700 dark:text-primary-300">
                  {{ userColumnMode === 'email'
                    ? (row.user?.email?.charAt(0).toUpperCase() || '?')
                    : (row.user?.username?.charAt(0).toUpperCase() || '?')
                  }}
                </span>
              </div>
              <span class="font-medium text-gray-900 dark:text-white">
                {{ userColumnMode === 'email'
                  ? (row.user?.email || t('admin.redeem.userPrefix', { id: row.user_id }))
                  : (row.user?.username || '-')
                }}
              </span>
            </div>
          </template>

          <template #cell-group="{ row }">
            <span class="inline-flex items-center rounded-md border border-gray-200 bg-gray-50 px-2.5 py-1 text-xs font-medium text-gray-700 dark:border-dark-600 dark:bg-dark-700/50 dark:text-gray-200">
              {{ subscriptionDisplayName(row) }}
            </span>
          </template>

          <template #cell-usage="{ row }">
            <div class="min-w-[280px] space-y-2">
              <!-- Daily Usage -->
              <div v-if="displayDailyLimit(row) != null" class="usage-row">
                <div class="flex items-center gap-2">
                  <span class="usage-label">{{ t('admin.subscriptions.daily') }}</span>
                  <div class="h-1.5 flex-1 rounded-full bg-gray-200 dark:bg-dark-600">
                    <div
                      class="h-1.5 rounded-full transition-all"
                      :class="getProgressClass(displayDailyUsed(row), displayDailyLimit(row))"
                      :style="{
                        width: getProgressWidth(displayDailyUsed(row), displayDailyLimit(row))
                      }"
                    ></div>
                  </div>
                  <span class="usage-amount">
                    {{ formatQuotaUsage(row.daily_used_knives, displayDailyLimit(row), row.daily_usage_usd) }}
                  </span>
                </div>
                <div class="reset-info" v-if="row.daily_window_start">
                  <svg
                    class="h-3 w-3"
                    fill="none"
                    viewBox="0 0 24 24"
                    stroke="currentColor"
                    stroke-width="2"
                  >
                    <path
                      stroke-linecap="round"
                      stroke-linejoin="round"
                      d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z"
                    />
                  </svg>
                  <span>{{ formatDailyUsageWindow(row) }}</span>
                </div>
              </div>

              <!-- Weekly Usage -->
              <div v-if="displayWeeklyLimit(row) != null" class="usage-row">
                <div class="flex items-center gap-2">
                  <span class="usage-label">{{ t('admin.subscriptions.weekly') }}</span>
                  <div class="h-1.5 flex-1 rounded-full bg-gray-200 dark:bg-dark-600">
                    <div
                      class="h-1.5 rounded-full transition-all"
                      :class="getProgressClass(displayWeeklyUsed(row), displayWeeklyLimit(row))"
                      :style="{
                        width: getProgressWidth(displayWeeklyUsed(row), displayWeeklyLimit(row))
                      }"
                    ></div>
                  </div>
                  <span class="usage-amount">
                    {{ formatQuotaUsage(row.weekly_used_knives, displayWeeklyLimit(row), row.weekly_usage_usd) }}
                  </span>
                </div>
                <div class="reset-info" v-if="row.weekly_window_start">
                  <svg
                    class="h-3 w-3"
                    fill="none"
                    viewBox="0 0 24 24"
                    stroke="currentColor"
                    stroke-width="2"
                  >
                    <path
                      stroke-linecap="round"
                      stroke-linejoin="round"
                      d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z"
                    />
                  </svg>
                  <span>{{ formatResetTime(row.weekly_window_start, 'weekly') }}</span>
                </div>
              </div>

              <!-- Monthly Usage -->
              <div v-if="displayMonthlyLimit(row) != null" class="usage-row">
                <div class="flex items-center gap-2">
                  <span class="usage-label">{{ t('admin.subscriptions.monthly') }}</span>
                  <div class="h-1.5 flex-1 rounded-full bg-gray-200 dark:bg-dark-600">
                    <div
                      class="h-1.5 rounded-full transition-all"
                      :class="getProgressClass(displayMonthlyUsed(row), displayMonthlyLimit(row))"
                      :style="{
                        width: getProgressWidth(displayMonthlyUsed(row), displayMonthlyLimit(row))
                      }"
                    ></div>
                  </div>
                  <span class="usage-amount">
                    {{ formatQuotaUsage(row.monthly_used_knives, displayMonthlyLimit(row), row.monthly_usage_usd) }}
                  </span>
                </div>
                <div class="reset-info" v-if="row.monthly_window_start">
                  <svg
                    class="h-3 w-3"
                    fill="none"
                    viewBox="0 0 24 24"
                    stroke="currentColor"
                    stroke-width="2"
                  >
                    <path
                      stroke-linecap="round"
                      stroke-linejoin="round"
                      d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z"
                    />
                  </svg>
                  <span>{{ formatResetTime(row.monthly_window_start, 'monthly') }}</span>
                </div>
              </div>

              <!-- No Limits - Unlimited badge -->
              <div
                v-if="
                  displayDailyLimit(row) == null &&
                  displayWeeklyLimit(row) == null &&
                  displayMonthlyLimit(row) == null
                "
                class="flex items-center gap-2 rounded-lg bg-gradient-to-r from-emerald-50 to-teal-50 px-3 py-2 dark:from-emerald-900/20 dark:to-teal-900/20"
              >
                <span class="text-lg text-emerald-600 dark:text-emerald-400">∞</span>
                <span class="text-xs font-medium text-emerald-700 dark:text-emerald-300">
                  {{ t('admin.subscriptions.unlimited') }}
                </span>
              </div>
            </div>
          </template>

          <template #cell-expires_at="{ value }">
            <div v-if="value">
              <span
                class="text-sm"
                :class="
                  isExpiringSoon(value)
                    ? 'text-orange-600 dark:text-orange-400'
                    : 'text-gray-700 dark:text-gray-300'
                "
              >
                {{ formatDateOnly(value) }}
              </span>
              <div v-if="getDaysRemaining(value) !== null" class="text-xs text-gray-500">
                {{ getDaysRemaining(value) }} {{ t('admin.subscriptions.daysRemaining') }}
              </div>
            </div>
            <span v-else class="text-sm text-gray-500">{{
              t('admin.subscriptions.noExpiration')
            }}</span>
          </template>

          <template #cell-status="{ row, value }">
            <span
              :class="[
                'badge',
                subscriptionStatusBadgeClass(value)
              ]"
            >
              {{ subscriptionStatusLabel(row) }}
            </span>
          </template>

          <template #cell-actions="{ row }">
            <div class="flex items-center gap-1">
              <button
                v-if="canAdjustSubscription(row)"
                @click="handleExtend(row)"
                class="flex flex-col items-center gap-0.5 rounded-lg p-1.5 text-gray-500 transition-colors hover:bg-blue-50 hover:text-blue-600 dark:hover:bg-blue-900/20 dark:hover:text-blue-400"
              >
                <Icon name="calendar" size="sm" />
                <span class="text-xs">{{ t('admin.subscriptions.adjust') }}</span>
              </button>
              <button
                v-if="canResetSubscription(row)"
                @click="handleResetQuota(row)"
                :disabled="resettingQuota && resettingSubscription?.id === row.id"
                data-test="reset-quota-open"
                class="flex flex-col items-center gap-0.5 rounded-lg p-1.5 text-gray-500 transition-colors hover:bg-orange-50 hover:text-orange-600 dark:hover:bg-orange-900/20 dark:hover:text-orange-400 disabled:cursor-not-allowed disabled:opacity-50"
              >
                <Icon name="refresh" size="sm" />
                <span class="text-xs">{{ t('admin.subscriptions.resetQuota') }}</span>
              </button>
              <button
                v-if="row.status === 'active'"
                @click="handleRevoke(row)"
                class="flex flex-col items-center gap-0.5 rounded-lg p-1.5 text-gray-500 transition-colors hover:bg-red-50 hover:text-red-600 dark:hover:bg-red-900/20 dark:hover:text-red-400"
              >
                <Icon name="ban" size="sm" />
                <span class="text-xs">{{ t('admin.subscriptions.revoke') }}</span>
              </button>
            </div>
          </template>

          <template #empty>
            <EmptyState
              :title="t('admin.subscriptions.noSubscriptionsYet')"
              :description="t('admin.subscriptions.assignFirstSubscription')"
              :action-text="t('admin.subscriptions.assignSubscription')"
              @action="showAssignModal = true"
            />
          </template>
        </DataTable>
      </template>

      <!-- Pagination -->
      <template #pagination>
      <Pagination
        v-if="pagination.total > 0"
        :page="pagination.page"
        :total="pagination.total"
        :page-size="pagination.page_size"
        @update:page="handlePageChange"
        @update:pageSize="handlePageSizeChange"
      />
      </template>
    </TablePageLayout>

    <!-- Assign Subscription Modal -->
    <BaseDialog
      :show="showAssignModal"
      :title="t('admin.subscriptions.assignSubscription')"
      width="normal"
      @close="closeAssignModal"
    >
      <form
        id="assign-subscription-form"
        @submit.prevent="handleAssignSubscription"
        class="space-y-5"
      >
        <div>
          <label class="input-label">{{ t('admin.subscriptions.form.user') }}</label>
          <div class="relative" data-assign-user-search>
            <input
              v-model="userSearchKeyword"
              type="text"
              class="input pr-8"
              :placeholder="t('admin.usage.searchUserPlaceholder')"
              @input="debounceSearchUsers"
              @focus="showUserDropdown = true"
            />
            <button
              v-if="selectedUser"
              @click="clearUserSelection"
              type="button"
              class="absolute right-2 top-1/2 -translate-y-1/2 text-gray-400 hover:text-gray-600 dark:hover:text-gray-300"
            >
              <Icon name="x" size="sm" :stroke-width="2" />
            </button>
            <!-- User Dropdown -->
            <div
              v-if="showUserDropdown && (userSearchResults.length > 0 || userSearchKeyword)"
              class="absolute z-50 mt-1 max-h-60 w-full overflow-auto rounded-lg border border-gray-200 bg-white shadow-lg dark:border-gray-700 dark:bg-gray-800"
            >
              <div
                v-if="userSearchLoading"
                class="px-4 py-3 text-sm text-gray-500 dark:text-gray-400"
              >
                {{ t('common.loading') }}
              </div>
              <div
                v-else-if="userSearchResults.length === 0 && userSearchKeyword"
                class="px-4 py-3 text-sm text-gray-500 dark:text-gray-400"
              >
                {{ t('common.noOptionsFound') }}
              </div>
              <button
                v-for="user in userSearchResults"
                :key="user.id"
                type="button"
                @click="selectUser(user)"
                class="w-full px-4 py-2 text-left text-sm hover:bg-gray-100 dark:hover:bg-gray-700"
              >
                <span class="font-medium text-gray-900 dark:text-white">{{ user.email }}</span>
                <span class="ml-2 text-gray-500 dark:text-gray-400">#{{ user.id }}</span>
              </button>
            </div>
          </div>
        </div>
        <div>
          <label class="input-label">{{ t('payment.plan') }}</label>
          <Select
            v-model="assignForm.plan_id"
            :options="subscriptionPlanOptions"
            :placeholder="t('payment.selectPlan')"
          />
          <p class="input-hint">
            {{ selectedAssignPlan ? formatPlanValidity(selectedAssignPlan) : t('admin.subscriptions.planHint') }}
          </p>
        </div>
        <p v-if="selectedAssignPlan" class="input-hint">{{ t('admin.subscriptions.validityHint') }}</p>
      </form>
      <template #footer>
        <div class="flex justify-end gap-3">
          <button @click="closeAssignModal" type="button" class="btn btn-secondary">
            {{ t('common.cancel') }}
          </button>
          <button
            type="submit"
            form="assign-subscription-form"
            :disabled="submitting"
            class="btn btn-primary"
          >
            <svg
              v-if="submitting"
              class="-ml-1 mr-2 h-4 w-4 animate-spin"
              fill="none"
              viewBox="0 0 24 24"
            >
              <circle
                class="opacity-25"
                cx="12"
                cy="12"
                r="10"
                stroke="currentColor"
                stroke-width="4"
              ></circle>
              <path
                class="opacity-75"
                fill="currentColor"
                d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
              ></path>
            </svg>
            {{ submitting ? t('admin.subscriptions.assigning') : t('admin.subscriptions.assign') }}
          </button>
        </div>
      </template>
    </BaseDialog>

    <!-- Adjust Subscription Modal -->
    <BaseDialog
      :show="showExtendModal"
      :title="t('admin.subscriptions.adjustSubscription')"
      width="narrow"
      @close="closeExtendModal"
    >
      <form
        v-if="extendingSubscription"
        id="extend-subscription-form"
        @submit.prevent="handleExtendSubscription"
        class="space-y-5"
      >
        <div class="rounded-lg bg-gray-50 p-4 dark:bg-dark-700">
          <p class="text-sm text-gray-600 dark:text-gray-400">
            {{ t('admin.subscriptions.adjustingFor') }}
            <span class="font-medium text-gray-900 dark:text-white">{{
              extendingSubscription.user?.email
            }}</span>
          </p>
          <p class="mt-1 text-sm text-gray-600 dark:text-gray-400">
            {{ t('admin.subscriptions.currentExpiration') }}:
            <span class="font-medium text-gray-900 dark:text-white">
              {{
                extendingSubscription.expires_at
                  ? formatDateOnly(extendingSubscription.expires_at)
                  : t('admin.subscriptions.noExpiration')
              }}
            </span>
          </p>
          <p v-if="extendingSubscription.expires_at" class="mt-1 text-sm text-gray-600 dark:text-gray-400">
            {{ t('admin.subscriptions.remainingDays') }}:
            <span class="font-medium text-gray-900 dark:text-white">
              {{ getDaysRemaining(extendingSubscription.expires_at) ?? 0 }}
            </span>
          </p>
        </div>
        <div>
          <label class="input-label">{{ t('admin.subscriptions.form.adjustDays') }}</label>
          <div class="flex items-center gap-2">
            <input
              v-model.number="extendForm.days"
              type="number"
              required
              class="input text-center"
              :placeholder="t('admin.subscriptions.adjustDaysPlaceholder')"
            />
          </div>
          <p class="input-hint">{{ t('admin.subscriptions.adjustHint') }}</p>
        </div>
      </form>
      <template #footer>
        <div v-if="extendingSubscription" class="flex justify-end gap-3">
          <button @click="closeExtendModal" type="button" class="btn btn-secondary">
            {{ t('common.cancel') }}
          </button>
          <button
            type="submit"
            form="extend-subscription-form"
            :disabled="submitting"
            class="btn btn-primary"
          >
            {{ submitting ? t('admin.subscriptions.adjusting') : t('admin.subscriptions.adjust') }}
          </button>
        </div>
      </template>
    </BaseDialog>

    <!-- Batch Adjust Subscription Modal -->
    <BaseDialog
      :show="showBulkExtendModal"
      :title="t('admin.subscriptions.batchAdjustSubscription')"
      width="narrow"
      @close="closeBulkExtendModal"
    >
      <form
        v-if="selectedAdjustableCount > 0"
        id="bulk-extend-subscription-form"
        data-test="batch-adjust-form"
        @submit.prevent="handleBulkExtendSubscription"
        class="space-y-5"
      >
        <div class="rounded-lg bg-gray-50 p-4 dark:bg-dark-700">
          <p class="text-sm text-gray-600 dark:text-gray-400">
            {{ t('admin.subscriptions.adjustingSelectedCount', { count: selectedAdjustableCount }) }}
          </p>
          <div class="mt-3">
            <p class="text-xs font-medium uppercase tracking-wide text-gray-500 dark:text-gray-400">
              {{ t('admin.subscriptions.selectedSubscriptions') }}
            </p>
            <div class="mt-2 flex flex-wrap gap-2">
              <span
                v-for="subscription in selectedAdjustableSubscriptionsPreview"
                :key="subscription.id"
                class="inline-flex items-center rounded-full bg-white px-3 py-1 text-xs font-medium text-gray-700 shadow-sm dark:bg-dark-800 dark:text-gray-200"
              >
                {{ selectedSubscriptionLabel(subscription) }}
              </span>
            </div>
            <p
              v-if="selectedAdjustableRemainingCount > 0"
              class="mt-2 text-xs text-gray-500 dark:text-gray-400"
            >
              {{ t('admin.subscriptions.moreSelectedSubscriptions', { count: selectedAdjustableRemainingCount }) }}
            </p>
          </div>
        </div>
        <div>
          <label class="input-label">{{ t('admin.subscriptions.form.adjustDays') }}</label>
          <div class="flex items-center gap-2">
            <input
              v-model.number="bulkExtendForm.days"
              type="number"
              required
              class="input text-center"
              :placeholder="t('admin.subscriptions.adjustDaysPlaceholder')"
              data-test="batch-adjust-days-input"
            />
          </div>
          <p class="input-hint">{{ t('admin.subscriptions.adjustHint') }}</p>
        </div>
      </form>
      <template #footer>
        <div v-if="selectedAdjustableCount > 0" class="flex justify-end gap-3">
          <button @click="closeBulkExtendModal" type="button" class="btn btn-secondary">
            {{ t('common.cancel') }}
          </button>
          <button
            type="submit"
            form="bulk-extend-subscription-form"
            :disabled="submitting"
            class="btn btn-primary"
          >
            {{ submitting ? t('admin.subscriptions.adjusting') : t('admin.subscriptions.batchAdjustSubscription') }}
          </button>
        </div>
      </template>
    </BaseDialog>

    <!-- Batch Reset Quota Modal -->
    <BaseDialog
      :show="showBulkResetQuotaModal"
      :title="t('admin.subscriptions.batchResetQuota')"
      width="narrow"
      @close="closeBulkResetQuotaModal"
    >
      <form
        v-if="selectedResettableCount > 0"
        id="bulk-reset-quota-form"
        data-test="batch-reset-form"
        @submit.prevent="handleBulkResetQuota"
        class="space-y-5"
      >
        <div class="rounded-lg bg-gray-50 p-4 dark:bg-dark-700">
          <p class="text-sm text-gray-600 dark:text-gray-400">
            {{ t('admin.subscriptions.resettingSelectedCount', { count: selectedResettableCount }) }}
          </p>
          <div class="mt-3">
            <p class="text-xs font-medium uppercase tracking-wide text-gray-500 dark:text-gray-400">
              {{ t('admin.subscriptions.selectedSubscriptions') }}
            </p>
            <div class="mt-2 flex flex-wrap gap-2">
              <span
                v-for="subscription in selectedResettableSubscriptionsPreview"
                :key="subscription.id"
                class="inline-flex items-center rounded-full bg-white px-3 py-1 text-xs font-medium text-gray-700 shadow-sm dark:bg-dark-800 dark:text-gray-200"
              >
                {{ selectedSubscriptionLabel(subscription) }}
              </span>
            </div>
            <p
              v-if="selectedResettableRemainingCount > 0"
              class="mt-2 text-xs text-gray-500 dark:text-gray-400"
            >
              {{ t('admin.subscriptions.moreSelectedSubscriptions', { count: selectedResettableRemainingCount }) }}
            </p>
          </div>
        </div>
        <div>
          <label class="input-label">{{ t('admin.subscriptions.resetQuotaOptions') }}</label>
          <div class="space-y-3 rounded-lg border border-gray-200 p-4 dark:border-dark-600">
            <label class="flex items-center gap-3">
              <input
                v-model="resetQuotaForm.daily"
                type="checkbox"
                class="h-4 w-4 rounded border-gray-300 text-primary-600 focus:ring-primary-500"
                data-test="reset-daily-checkbox"
              />
              <span class="text-sm text-gray-700 dark:text-gray-200">{{ t('admin.subscriptions.resetDailyQuota') }}</span>
            </label>
            <label class="flex items-center gap-3">
              <input
                v-model="resetQuotaForm.weekly"
                type="checkbox"
                class="h-4 w-4 rounded border-gray-300 text-primary-600 focus:ring-primary-500"
                data-test="reset-weekly-checkbox"
              />
              <span class="text-sm text-gray-700 dark:text-gray-200">{{ t('admin.subscriptions.resetWeeklyQuota') }}</span>
            </label>
            <label class="flex items-center gap-3">
              <input
                v-model="resetQuotaForm.monthly"
                type="checkbox"
                class="h-4 w-4 rounded border-gray-300 text-primary-600 focus:ring-primary-500"
                data-test="reset-monthly-checkbox"
              />
              <span class="text-sm text-gray-700 dark:text-gray-200">{{ t('admin.subscriptions.resetMonthlyQuota') }}</span>
            </label>
          </div>
          <p class="input-hint">{{ t('admin.subscriptions.resetQuotaHint') }}</p>
        </div>
      </form>
      <template #footer>
        <div v-if="selectedResettableCount > 0" class="flex justify-end gap-3">
          <button @click="closeBulkResetQuotaModal" type="button" class="btn btn-secondary">
            {{ t('common.cancel') }}
          </button>
          <button
            type="submit"
            form="bulk-reset-quota-form"
            :disabled="submitting"
            class="btn btn-primary"
          >
            {{ t('admin.subscriptions.batchResetQuota') }}
          </button>
        </div>
      </template>
    </BaseDialog>

    <!-- Revoke Confirmation Dialog -->
    <ConfirmDialog
      :show="showRevokeDialog"
      :title="t('admin.subscriptions.revokeSubscription')"
      :message="t('admin.subscriptions.revokeConfirm', { user: revokingSubscription?.user?.email })"
      :confirm-text="t('admin.subscriptions.revoke')"
      :cancel-text="t('common.cancel')"
      :danger="true"
      @confirm="confirmRevoke"
      @cancel="showRevokeDialog = false"
    />

    <!-- Reset Quota Dialog -->
    <BaseDialog
      :show="showResetQuotaModal"
      :title="t('admin.subscriptions.resetQuotaTitle')"
      width="narrow"
      @close="closeResetQuotaModal"
    >
      <form
        v-if="resettingSubscription"
        id="reset-quota-form"
        data-test="reset-quota-form"
        @submit.prevent="confirmResetQuota"
        class="space-y-5"
      >
        <div class="rounded-lg bg-gray-50 p-4 text-sm text-gray-600 dark:bg-dark-700 dark:text-gray-300">
          {{ t('admin.subscriptions.resetQuotaDescription', { user: resettingSubscription.user?.email || t('admin.redeem.userPrefix', { id: resettingSubscription.user_id }) }) }}
        </div>
        <div>
          <label class="input-label">{{ t('admin.subscriptions.resetQuotaOptions') }}</label>
          <div class="space-y-3 rounded-lg border border-gray-200 p-4 dark:border-dark-600">
            <label class="flex items-center gap-3">
              <input
                v-model="resetQuotaForm.daily"
                type="checkbox"
                class="h-4 w-4 rounded border-gray-300 text-primary-600 focus:ring-primary-500"
                data-test="reset-daily-checkbox"
              />
              <span class="text-sm text-gray-700 dark:text-gray-200">{{ t('admin.subscriptions.resetDailyQuota') }}</span>
            </label>
            <label class="flex items-center gap-3">
              <input
                v-model="resetQuotaForm.weekly"
                type="checkbox"
                class="h-4 w-4 rounded border-gray-300 text-primary-600 focus:ring-primary-500"
                data-test="reset-weekly-checkbox"
              />
              <span class="text-sm text-gray-700 dark:text-gray-200">{{ t('admin.subscriptions.resetWeeklyQuota') }}</span>
            </label>
            <label class="flex items-center gap-3">
              <input
                v-model="resetQuotaForm.monthly"
                type="checkbox"
                class="h-4 w-4 rounded border-gray-300 text-primary-600 focus:ring-primary-500"
                data-test="reset-monthly-checkbox"
              />
              <span class="text-sm text-gray-700 dark:text-gray-200">{{ t('admin.subscriptions.resetMonthlyQuota') }}</span>
            </label>
          </div>
          <p class="input-hint">{{ t('admin.subscriptions.resetQuotaHint') }}</p>
        </div>
      </form>
      <template #footer>
        <div v-if="resettingSubscription" class="flex justify-end gap-3">
          <button @click="closeResetQuotaModal" type="button" class="btn btn-secondary">
            {{ t('common.cancel') }}
          </button>
          <button
            type="submit"
            form="reset-quota-form"
            :disabled="resettingQuota"
            class="btn btn-primary"
          >
            {{ t('admin.subscriptions.resetQuota') }}
          </button>
        </div>
      </template>
    </BaseDialog>
    <!-- Subscription Guide Modal -->
    <teleport to="body">
      <transition name="modal">
        <div v-if="showGuideModal" class="fixed inset-0 z-50 flex items-center justify-center p-4" @mousedown.self="showGuideModal = false">
          <div class="fixed inset-0 bg-black/50" @click="showGuideModal = false"></div>
          <div class="relative max-h-[85vh] w-full max-w-2xl overflow-y-auto rounded-xl bg-white p-6 shadow-2xl dark:bg-dark-800">
            <button type="button" class="absolute right-4 top-4 text-gray-400 hover:text-gray-600 dark:hover:text-gray-200" @click="showGuideModal = false">
              <svg class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2"><path stroke-linecap="round" stroke-linejoin="round" d="M6 18L18 6M6 6l12 12" /></svg>
            </button>

            <h2 class="mb-4 text-lg font-bold text-gray-900 dark:text-white">{{ t('admin.subscriptions.guide.title') }}</h2>
            <p class="mb-5 text-sm text-gray-500 dark:text-gray-400">{{ t('admin.subscriptions.guide.subtitle') }}</p>

            <!-- Step 1 -->
            <div class="mb-5">
              <h3 class="mb-2 flex items-center gap-2 text-sm font-semibold text-gray-900 dark:text-white">
                <span class="flex h-6 w-6 items-center justify-center rounded-full bg-primary-100 text-xs font-bold text-primary-700 dark:bg-primary-900/40 dark:text-primary-300">1</span>
                {{ t('admin.subscriptions.guide.step1.title') }}
              </h3>
              <ol class="ml-8 list-decimal space-y-1 text-sm text-gray-600 dark:text-gray-300">
                <li>{{ t('admin.subscriptions.guide.step1.line1') }}</li>
                <li>{{ t('admin.subscriptions.guide.step1.line2') }}</li>
                <li>{{ t('admin.subscriptions.guide.step1.line3') }}</li>
              </ol>
              <div class="ml-8 mt-2">
                <router-link
                  to="/admin/groups"
                  @click="showGuideModal = false"
                  class="inline-flex items-center gap-1 text-sm font-medium text-primary-600 hover:text-primary-700 dark:text-primary-400 dark:hover:text-primary-300"
                >
                  {{ t('admin.subscriptions.guide.step1.link') }}
                  <Icon name="arrowRight" size="xs" />
                </router-link>
              </div>
            </div>

            <!-- Step 2 -->
            <div class="mb-5">
              <h3 class="mb-2 flex items-center gap-2 text-sm font-semibold text-gray-900 dark:text-white">
                <span class="flex h-6 w-6 items-center justify-center rounded-full bg-primary-100 text-xs font-bold text-primary-700 dark:bg-primary-900/40 dark:text-primary-300">2</span>
                {{ t('admin.subscriptions.guide.step2.title') }}
              </h3>
              <ol class="ml-8 list-decimal space-y-1 text-sm text-gray-600 dark:text-gray-300">
                <li>{{ t('admin.subscriptions.guide.step2.line1') }}</li>
                <li>{{ t('admin.subscriptions.guide.step2.line2') }}</li>
                <li>{{ t('admin.subscriptions.guide.step2.line3') }}</li>
              </ol>
            </div>

            <!-- Step 3 -->
            <div class="mb-5">
              <h3 class="mb-2 flex items-center gap-2 text-sm font-semibold text-gray-900 dark:text-white">
                <span class="flex h-6 w-6 items-center justify-center rounded-full bg-primary-100 text-xs font-bold text-primary-700 dark:bg-primary-900/40 dark:text-primary-300">3</span>
                {{ t('admin.subscriptions.guide.step3.title') }}
              </h3>
              <div class="ml-8 overflow-hidden rounded-lg border border-gray-200 dark:border-dark-600">
                <table class="w-full text-sm">
                  <tbody>
                    <tr v-for="(row, i) in guideActionRows" :key="i" class="border-b border-gray-100 dark:border-dark-700 last:border-0">
                      <td class="whitespace-nowrap bg-gray-50 px-3 py-2 font-medium text-gray-700 dark:bg-dark-700 dark:text-gray-300">{{ row.action }}</td>
                      <td class="px-3 py-2 text-gray-600 dark:text-gray-400">{{ row.desc }}</td>
                    </tr>
                  </tbody>
                </table>
              </div>
            </div>

            <!-- Tip -->
            <div class="rounded-lg bg-blue-50 p-3 text-xs text-blue-700 dark:bg-blue-900/20 dark:text-blue-300">
              {{ t('admin.subscriptions.guide.tip') }}
            </div>

            <div class="mt-4 text-right">
              <button type="button" class="btn btn-primary btn-sm" @click="showGuideModal = false">{{ t('common.close') }}</button>
            </div>
          </div>
        </div>
      </transition>
    </teleport>
  </AppLayout>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted, onUnmounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
import { adminAPI } from '@/api/admin'
import type { UserSubscription, UserSubscriptionStatus } from '@/types'
import type { SimpleUser } from '@/api/admin/usage'
import type { Column } from '@/components/common/types'
import type { SubscriptionPlan } from '@/types/payment'
import { formatDateOnly } from '@/utils/format'
import { getPersistedPageSize } from '@/composables/usePersistedPageSize'
import AppLayout from '@/components/layout/AppLayout.vue'
import TablePageLayout from '@/components/layout/TablePageLayout.vue'
import DataTable from '@/components/common/DataTable.vue'
import Pagination from '@/components/common/Pagination.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import EmptyState from '@/components/common/EmptyState.vue'
import Select from '@/components/common/Select.vue'
import Icon from '@/components/icons/Icon.vue'
import { getRemainingDurationParts, isOneTimeDailyQuota, type RemainingDurationParts } from '@/utils/subscriptionQuota'

const { t } = useI18n()
const appStore = useAppStore()

interface PlanOption extends Record<string, unknown> {
  value: number
  label: string
  description: string
}

const MAX_ADJUST_DAYS = 36500
const BULK_PREVIEW_LIMIT = 5

// Guide modal state
const showGuideModal = ref(false)

const guideActionRows = computed(() => [
  { action: t('admin.subscriptions.guide.actions.batchAdjust'), desc: t('admin.subscriptions.guide.actions.batchAdjustDesc') },
  { action: t('admin.subscriptions.guide.actions.adjust'), desc: t('admin.subscriptions.guide.actions.adjustDesc') },
  { action: t('admin.subscriptions.guide.actions.resetQuota'), desc: t('admin.subscriptions.guide.actions.resetQuotaDesc') },
  { action: t('admin.subscriptions.guide.actions.revoke'), desc: t('admin.subscriptions.guide.actions.revokeDesc') }
])

// User column display mode: 'email' or 'username'
const userColumnMode = ref<'email' | 'username'>('email')
const USER_COLUMN_MODE_KEY = 'subscription-user-column-mode'

const loadUserColumnMode = () => {
  try {
    const saved = localStorage.getItem(USER_COLUMN_MODE_KEY)
    if (saved === 'email' || saved === 'username') {
      userColumnMode.value = saved
    }
  } catch (e) {
    console.error('Failed to load user column mode:', e)
  }
}

const saveUserColumnMode = () => {
  try {
    localStorage.setItem(USER_COLUMN_MODE_KEY, userColumnMode.value)
  } catch (e) {
    console.error('Failed to save user column mode:', e)
  }
}

const setUserColumnMode = (mode: 'email' | 'username') => {
  userColumnMode.value = mode
  saveUserColumnMode()
}

const canAdjustSubscription = (subscription: UserSubscription): boolean =>
  subscription.status === 'active' || subscription.status === 'expired'

const canResetSubscription = (subscription: UserSubscription): boolean =>
  subscription.status === 'active'

const selectedSubscriptionLabel = (subscription: UserSubscription): string =>
  subscription.user?.email || t('admin.redeem.userPrefix', { id: subscription.user_id })

// All available columns
const allColumns = computed<Column[]>(() => [
  { key: 'select', label: '', sortable: false, class: 'w-12' },
  {
    key: 'user',
    label: userColumnMode.value === 'email'
      ? t('admin.subscriptions.columns.user')
      : t('admin.users.columns.username'),
    sortable: false
  },
  { key: 'group', label: t('payment.plan'), sortable: false },
  { key: 'usage', label: t('admin.subscriptions.columns.usage'), sortable: false },
  { key: 'expires_at', label: t('admin.subscriptions.columns.expires'), sortable: true },
  { key: 'status', label: t('admin.subscriptions.columns.status'), sortable: true },
  { key: 'actions', label: t('admin.subscriptions.columns.actions'), sortable: false }
])

// Columns that can be toggled (exclude user and actions which are always visible)
const toggleableColumns = computed(() =>
  allColumns.value.filter(col => col.key !== 'select' && col.key !== 'user' && col.key !== 'actions')
)

// Hidden columns set
const hiddenColumns = reactive<Set<string>>(new Set())

// Default hidden columns
const DEFAULT_HIDDEN_COLUMNS: string[] = []

// localStorage key
const HIDDEN_COLUMNS_KEY = 'subscription-hidden-columns'

// Load saved column settings
const loadSavedColumns = () => {
  try {
    const saved = localStorage.getItem(HIDDEN_COLUMNS_KEY)
    if (saved) {
      const parsed = JSON.parse(saved) as string[]
      parsed.forEach(key => hiddenColumns.add(key))
    } else {
      DEFAULT_HIDDEN_COLUMNS.forEach(key => hiddenColumns.add(key))
    }
  } catch (e) {
    console.error('Failed to load saved columns:', e)
    DEFAULT_HIDDEN_COLUMNS.forEach(key => hiddenColumns.add(key))
  }
}

// Save column settings to localStorage
const saveColumnsToStorage = () => {
  try {
    localStorage.setItem(HIDDEN_COLUMNS_KEY, JSON.stringify([...hiddenColumns]))
  } catch (e) {
    console.error('Failed to save columns:', e)
  }
}

// Toggle column visibility
const toggleColumn = (key: string) => {
  if (hiddenColumns.has(key)) {
    hiddenColumns.delete(key)
  } else {
    hiddenColumns.add(key)
  }
  saveColumnsToStorage()
}

// Check if column is visible
const isColumnVisible = (key: string) => !hiddenColumns.has(key)

// Filtered columns for display
const columns = computed<Column[]>(() =>
  allColumns.value.filter(col =>
    col.key === 'select' || col.key === 'user' || col.key === 'actions' || !hiddenColumns.has(col.key)
  )
)

// Column dropdown state
const showColumnDropdown = ref(false)
const columnDropdownRef = ref<HTMLElement | null>(null)

// Filter options
const subscriptionStatusValues: UserSubscriptionStatus[] = [
  'active',
  'expired',
  'suspended',
  'superseded',
  'refunded',
  'revoked',
]

const statusOptions = computed(() => [
  { value: '', label: t('admin.subscriptions.allStatus') },
  ...subscriptionStatusValues.map((status) => ({
    value: status,
    label: t(`admin.subscriptions.status.${status}`),
  })),
])

const subscriptions = ref<UserSubscription[]>([])
const subscriptionPlans = ref<SubscriptionPlan[]>([])
const loading = ref(false)
let abortController: AbortController | null = null

// Toolbar user filter (fuzzy search -> select user_id)
const filterUserKeyword = ref('')
const filterUserResults = ref<SimpleUser[]>([])
const filterUserLoading = ref(false)
const showFilterUserDropdown = ref(false)
const selectedFilterUser = ref<SimpleUser | null>(null)
let filterUserSearchTimeout: ReturnType<typeof setTimeout> | null = null

// User search state
const userSearchKeyword = ref('')
const userSearchResults = ref<SimpleUser[]>([])
const userSearchLoading = ref(false)
const showUserDropdown = ref(false)
const selectedUser = ref<SimpleUser | null>(null)
let userSearchTimeout: ReturnType<typeof setTimeout> | null = null

const filters = reactive({
  status: 'active' as UserSubscriptionStatus | '',
  user_id: null as number | null
})

// Sorting state
const sortState = reactive({
  sort_by: 'created_at',
  sort_order: 'desc' as 'asc' | 'desc'
})

const pagination = reactive({
  page: 1,
  page_size: getPersistedPageSize(),
  total: 0,
  pages: 0
})

const showAssignModal = ref(false)
const showExtendModal = ref(false)
const showBulkExtendModal = ref(false)
const showBulkResetQuotaModal = ref(false)
const showRevokeDialog = ref(false)
const showResetQuotaModal = ref(false)
const submitting = ref(false)
const resettingSubscription = ref<UserSubscription | null>(null)
const resettingQuota = ref(false)
const extendingSubscription = ref<UserSubscription | null>(null)
const revokingSubscription = ref<UserSubscription | null>(null)
const selectedSubscriptionIds = ref<number[]>([])

const assignForm = reactive({
  user_id: null as number | null,
  plan_id: null as number | null,
  validity_days: 30
})

const extendForm = reactive({
  days: 30
})

const bulkExtendForm = reactive({
  days: 30
})

const resetQuotaForm = reactive({
  daily: true,
  weekly: true,
  monthly: true
})

const subscriptionPlanOptions = computed<PlanOption[]>(() =>
  subscriptionPlans.value.map((plan) => ({
    value: plan.id,
    label: plan.name,
    description: plan.description,
  }))
)

const selectedAssignPlan = computed(() =>
  subscriptionPlans.value.find((plan) => plan.id === assignForm.plan_id) ?? null
)

const adjustableSubscriptions = computed(() =>
  subscriptions.value.filter(canAdjustSubscription)
)

const selectedAdjustableSubscriptions = computed(() => {
  const selectedIDs = new Set(selectedSubscriptionIds.value)
  return subscriptions.value.filter(subscription => selectedIDs.has(subscription.id) && canAdjustSubscription(subscription))
})

const selectedResettableSubscriptions = computed(() => {
  const selectedIDs = new Set(selectedSubscriptionIds.value)
  return subscriptions.value.filter(subscription => selectedIDs.has(subscription.id) && canResetSubscription(subscription))
})

const selectedAdjustableCount = computed(() => selectedAdjustableSubscriptions.value.length)

const selectedResettableCount = computed(() => selectedResettableSubscriptions.value.length)

const selectedAdjustableSubscriptionsPreview = computed(() =>
  selectedAdjustableSubscriptions.value.slice(0, BULK_PREVIEW_LIMIT)
)

const selectedResettableSubscriptionsPreview = computed(() =>
  selectedResettableSubscriptions.value.slice(0, BULK_PREVIEW_LIMIT)
)

const selectedAdjustableRemainingCount = computed(() =>
  Math.max(0, selectedAdjustableCount.value - selectedAdjustableSubscriptionsPreview.value.length)
)

const selectedResettableRemainingCount = computed(() =>
  Math.max(0, selectedResettableCount.value - selectedResettableSubscriptionsPreview.value.length)
)

const allAdjustableSelected = computed(() =>
  adjustableSubscriptions.value.length > 0 &&
  adjustableSubscriptions.value.every(subscription => selectedSubscriptionIds.value.includes(subscription.id))
)

const validateAdjustDays = (days: number): boolean => {
  if (!Number.isFinite(days) || days < -MAX_ADJUST_DAYS || days > MAX_ADJUST_DAYS) {
    appStore.showError(t('admin.subscriptions.adjustOutOfRange'))
    return false
  }
  return true
}

const resetResetQuotaForm = () => {
  resetQuotaForm.daily = true
  resetQuotaForm.weekly = true
  resetQuotaForm.monthly = true
}

const hasSelectedResetQuotaWindow = (): boolean =>
  resetQuotaForm.daily || resetQuotaForm.weekly || resetQuotaForm.monthly

const getResetQuotaPayload = () => ({
  daily: resetQuotaForm.daily,
  weekly: resetQuotaForm.weekly,
  monthly: resetQuotaForm.monthly
})

const validateResetQuotaSelection = (): boolean => {
  if (!hasSelectedResetQuotaWindow()) {
    appStore.showError(t('admin.subscriptions.pleaseSelectResetQuotaWindow'))
    return false
  }
  return true
}

const syncBulkActionModals = () => {
  if (selectedAdjustableCount.value === 0) {
    showBulkExtendModal.value = false
  }
  if (selectedResettableCount.value === 0) {
    showBulkResetQuotaModal.value = false
  }
}

const syncSelectedSubscriptionIDs = (items: UserSubscription[]) => {
  const adjustableIDs = new Set(items.filter(canAdjustSubscription).map(subscription => subscription.id))
  selectedSubscriptionIds.value = selectedSubscriptionIds.value.filter(id => adjustableIDs.has(id))
  syncBulkActionModals()
}

const isSubscriptionSelected = (subscriptionID: number): boolean =>
  selectedSubscriptionIds.value.includes(subscriptionID)

const toggleSubscriptionSelection = (subscription: UserSubscription, checked: boolean) => {
  if (!canAdjustSubscription(subscription)) return

  if (checked) {
    if (!selectedSubscriptionIds.value.includes(subscription.id)) {
      selectedSubscriptionIds.value = [...selectedSubscriptionIds.value, subscription.id]
    }
    syncBulkActionModals()
    return
  }

  selectedSubscriptionIds.value = selectedSubscriptionIds.value.filter(id => id !== subscription.id)
  syncBulkActionModals()
}

const clearSelectedSubscriptions = () => {
  selectedSubscriptionIds.value = []
  syncBulkActionModals()
}

const toggleSelectAllAdjustable = (checked: boolean) => {
  selectedSubscriptionIds.value = checked
    ? adjustableSubscriptions.value.map(subscription => subscription.id)
    : []
  syncBulkActionModals()
}

const handleSelectAllChange = (event: Event) => {
  toggleSelectAllAdjustable((event.target as HTMLInputElement).checked)
}

const handleSubscriptionSelectionChange = (subscription: UserSubscription, event: Event) => {
  toggleSubscriptionSelection(subscription, (event.target as HTMLInputElement).checked)
}

const applyFilters = () => {
  pagination.page = 1
  loadSubscriptions()
}

const loadSubscriptions = async () => {
  if (abortController) {
    abortController.abort()
  }
  const requestController = new AbortController()
  abortController = requestController
  const { signal } = requestController

  loading.value = true
  try {
    const response = await adminAPI.subscriptions.list(
      pagination.page,
      pagination.page_size,
      {
        status: filters.status || undefined,
        user_id: filters.user_id || undefined,
        sort_by: sortState.sort_by,
        sort_order: sortState.sort_order
      },
      {
        signal
      }
    )
    if (signal.aborted || abortController !== requestController) return
    subscriptions.value = response.items
    syncSelectedSubscriptionIDs(response.items)
    pagination.total = response.total
    pagination.pages = response.pages
  } catch (error: any) {
    if (signal.aborted || error?.name === 'AbortError' || error?.code === 'ERR_CANCELED') {
      return
    }
    appStore.showError(t('admin.subscriptions.failedToLoad'))
    console.error('Error loading subscriptions:', error)
  } finally {
    if (abortController === requestController) {
      loading.value = false
      abortController = null
    }
  }
}

const loadSubscriptionPlans = async () => {
  try {
    const response = await adminAPI.payment.getPlans()
    subscriptionPlans.value = (response.data || []).filter((plan) => plan.for_sale !== false)
  } catch (error) {
    subscriptionPlans.value = []
    console.error('Error loading subscription plans:', error)
  }
}

// Toolbar user filter search with debounce
const debounceSearchFilterUsers = () => {
  if (filterUserSearchTimeout) {
    clearTimeout(filterUserSearchTimeout)
  }
  filterUserSearchTimeout = setTimeout(searchFilterUsers, 300)
}

const searchFilterUsers = async () => {
  const keyword = filterUserKeyword.value.trim()

  // Clear active user filter if user modified the search keyword
  if (selectedFilterUser.value && keyword !== selectedFilterUser.value.email) {
    selectedFilterUser.value = null
    filters.user_id = null
    applyFilters()
  }

  if (!keyword) {
    filterUserResults.value = []
    return
  }

  filterUserLoading.value = true
  try {
    filterUserResults.value = await adminAPI.usage.searchUsers(keyword)
  } catch (error) {
    console.error('Failed to search users:', error)
    filterUserResults.value = []
  } finally {
    filterUserLoading.value = false
  }
}

const selectFilterUser = (user: SimpleUser) => {
  selectedFilterUser.value = user
  filterUserKeyword.value = user.email
  showFilterUserDropdown.value = false
  filters.user_id = user.id
  applyFilters()
}

const clearFilterUser = () => {
  selectedFilterUser.value = null
  filterUserKeyword.value = ''
  filterUserResults.value = []
  showFilterUserDropdown.value = false
  filters.user_id = null
  applyFilters()
}

// User search with debounce
const debounceSearchUsers = () => {
  if (userSearchTimeout) {
    clearTimeout(userSearchTimeout)
  }
  userSearchTimeout = setTimeout(searchUsers, 300)
}

const searchUsers = async () => {
  const keyword = userSearchKeyword.value.trim()

  // Clear selection if user modified the search keyword
  if (selectedUser.value && keyword !== selectedUser.value.email) {
    selectedUser.value = null
    assignForm.user_id = null
  }

  if (!keyword) {
    userSearchResults.value = []
    return
  }

  userSearchLoading.value = true
  try {
    userSearchResults.value = await adminAPI.usage.searchUsers(keyword)
  } catch (error) {
    console.error('Failed to search users:', error)
    userSearchResults.value = []
  } finally {
    userSearchLoading.value = false
  }
}

const selectUser = (user: SimpleUser) => {
  selectedUser.value = user
  userSearchKeyword.value = user.email
  showUserDropdown.value = false
  assignForm.user_id = user.id
}

const clearUserSelection = () => {
  selectedUser.value = null
  userSearchKeyword.value = ''
  userSearchResults.value = []
  assignForm.user_id = null
}

const handlePageChange = (page: number) => {
  pagination.page = page
  loadSubscriptions()
}

const handlePageSizeChange = (pageSize: number) => {
  pagination.page_size = pageSize
  pagination.page = 1
  loadSubscriptions()
}

const handleSort = (key: string, order: 'asc' | 'desc') => {
  sortState.sort_by = key
  sortState.sort_order = order
  pagination.page = 1
  loadSubscriptions()
}

const closeAssignModal = () => {
  showAssignModal.value = false
  assignForm.user_id = null
  assignForm.plan_id = null
  assignForm.validity_days = 30
  // Clear user search state
  selectedUser.value = null
  userSearchKeyword.value = ''
  userSearchResults.value = []
  showUserDropdown.value = false
}

const handleAssignSubscription = async () => {
  if (!assignForm.user_id) {
    appStore.showError(t('admin.subscriptions.pleaseSelectUser'))
    return
  }
  if (!assignForm.plan_id) {
    appStore.showError(t('admin.subscriptions.pleaseSelectPlan'))
    return
  }
  if (!selectedAssignPlan.value) {
    appStore.showError(t('admin.subscriptions.pleaseSelectPlan'))
    return
  }

  submitting.value = true
  try {
    await adminAPI.subscriptions.assign({
      user_id: assignForm.user_id,
      plan_id: assignForm.plan_id,
      validity_days: selectedAssignPlan.value.validity_days
    })
    appStore.showSuccess(t('admin.subscriptions.subscriptionAssigned'))
    closeAssignModal()
    loadSubscriptions()
  } catch (error: any) {
    appStore.showError(error.response?.data?.detail || t('admin.subscriptions.failedToAssign'))
    console.error('Error assigning subscription:', error)
  } finally {
    submitting.value = false
  }
}

const handleExtend = (subscription: UserSubscription) => {
  extendingSubscription.value = subscription
  extendForm.days = 30
  showExtendModal.value = true
}

const closeExtendModal = () => {
  showExtendModal.value = false
  extendingSubscription.value = null
}

const openBulkExtendModal = () => {
  if (selectedAdjustableCount.value === 0) {
    appStore.showError(t('admin.subscriptions.pleaseSelectSubscriptions'))
    return
  }
  bulkExtendForm.days = 30
  showBulkExtendModal.value = true
}

const closeBulkExtendModal = () => {
  showBulkExtendModal.value = false
}

const openBulkResetQuotaModal = () => {
  if (selectedResettableCount.value === 0) {
    appStore.showError(t('admin.subscriptions.pleaseSelectSubscriptionsToReset'))
    return
  }
  resetResetQuotaForm()
  showBulkResetQuotaModal.value = true
}

const closeBulkResetQuotaModal = () => {
  showBulkResetQuotaModal.value = false
  resetResetQuotaForm()
}

const closeResetQuotaModal = () => {
  showResetQuotaModal.value = false
  resettingSubscription.value = null
  resetResetQuotaForm()
}

const handleExtendSubscription = async () => {
  if (!extendingSubscription.value) return
  if (!validateAdjustDays(extendForm.days)) return

  // 前端验证：调整后的过期时间必须在未来
  if (extendingSubscription.value.expires_at) {
    const expiresAt = new Date(extendingSubscription.value.expires_at)
    const newExpiresAt = new Date(expiresAt.getTime() + extendForm.days * 24 * 60 * 60 * 1000)
    if (newExpiresAt <= new Date()) {
      appStore.showError(t('admin.subscriptions.adjustWouldExpire'))
      return
    }
  }

  submitting.value = true
  try {
    await adminAPI.subscriptions.extend(extendingSubscription.value.id, {
      days: extendForm.days
    })
    appStore.showSuccess(t('admin.subscriptions.subscriptionAdjusted'))
    closeExtendModal()
    loadSubscriptions()
  } catch (error: any) {
    appStore.showError(error.response?.data?.detail || t('admin.subscriptions.failedToAdjust'))
    console.error('Error adjusting subscription:', error)
  } finally {
    submitting.value = false
  }
}

const handleBulkExtendSubscription = async () => {
  if (selectedAdjustableCount.value === 0) {
    appStore.showError(t('admin.subscriptions.pleaseSelectSubscriptions'))
    return
  }
  if (!validateAdjustDays(bulkExtendForm.days)) return

  submitting.value = true
  try {
    const result = await adminAPI.subscriptions.bulkExtend({
      subscription_ids: selectedAdjustableSubscriptions.value.map(subscription => subscription.id),
      days: bulkExtendForm.days
    })
    const summary = t('admin.subscriptions.batchAdjustSummary', {
      success: result.success_count,
      failed: result.failed_count
    })

    if (result.success_count > 0) {
      appStore.showSuccess(summary)
      if (result.failed_count > 0 && result.errors.length > 0) {
        appStore.showError(result.errors[0])
      }
      closeBulkExtendModal()
      clearSelectedSubscriptions()
      await loadSubscriptions()
      return
    }

    appStore.showError(result.errors[0] || summary)
  } catch (error: any) {
    appStore.showError(error.response?.data?.detail || t('admin.subscriptions.failedToBatchAdjust'))
    console.error('Error batch adjusting subscriptions:', error)
  } finally {
    submitting.value = false
  }
}

const handleBulkResetQuota = async () => {
  if (selectedResettableCount.value === 0) {
    appStore.showError(t('admin.subscriptions.pleaseSelectSubscriptionsToReset'))
    return
  }
  if (!validateResetQuotaSelection()) return

  submitting.value = true
  try {
    const result = await adminAPI.subscriptions.bulkResetQuota({
      subscription_ids: selectedResettableSubscriptions.value.map(subscription => subscription.id),
      ...getResetQuotaPayload()
    })
    const summary = t('admin.subscriptions.batchResetQuotaSummary', {
      success: result.success_count,
      failed: result.failed_count
    })

    if (result.success_count > 0) {
      appStore.showSuccess(summary)
      if (result.failed_count > 0 && result.errors.length > 0) {
        appStore.showError(result.errors[0])
      }
      closeBulkResetQuotaModal()
      clearSelectedSubscriptions()
      await loadSubscriptions()
      return
    }

    appStore.showError(result.errors[0] || summary)
  } catch (error: any) {
    appStore.showError(error.response?.data?.detail || t('admin.subscriptions.failedToBatchResetQuota'))
    console.error('Error batch resetting subscriptions:', error)
  } finally {
    submitting.value = false
  }
}

const handleRevoke = (subscription: UserSubscription) => {
  revokingSubscription.value = subscription
  showRevokeDialog.value = true
}

const confirmRevoke = async () => {
  if (!revokingSubscription.value) return

  try {
    await adminAPI.subscriptions.revoke(revokingSubscription.value.id)
    appStore.showSuccess(t('admin.subscriptions.subscriptionRevoked'))
    showRevokeDialog.value = false
    revokingSubscription.value = null
    loadSubscriptions()
  } catch (error: any) {
    appStore.showError(error.response?.data?.detail || t('admin.subscriptions.failedToRevoke'))
    console.error('Error revoking subscription:', error)
  }
}

const handleResetQuota = (subscription: UserSubscription) => {
  resettingSubscription.value = subscription
  resetResetQuotaForm()
  showResetQuotaModal.value = true
}

const confirmResetQuota = async () => {
  if (!resettingSubscription.value) return
  if (resettingQuota.value) return
  if (!validateResetQuotaSelection()) return
  resettingQuota.value = true
  try {
    await adminAPI.subscriptions.resetQuota(resettingSubscription.value.id, getResetQuotaPayload())
    appStore.showSuccess(t('admin.subscriptions.quotaResetSuccess'))
    closeResetQuotaModal()
    await loadSubscriptions()
  } catch (error: any) {
    appStore.showError(error.response?.data?.detail || t('admin.subscriptions.failedToResetQuota'))
    console.error('Error resetting quota:', error)
  } finally {
    resettingQuota.value = false
  }
}

// Helper functions
const subscriptionStatusLabel = (subscription: UserSubscription): string => {
  if (!subscription?.status) return '-'
  if (subscription.status === 'suspended' && subscription.refund_freeze_active) {
    const refundLabel = t('userSubscriptions.status.suspended_refund')
    if (refundLabel !== 'userSubscriptions.status.suspended_refund') {
      return refundLabel
    }
  }
  return t(`admin.subscriptions.status.${subscription.status}`)
}

const subscriptionStatusBadgeClass = (status: UserSubscriptionStatus | string): string => {
  switch (status) {
    case 'active':
      return 'badge-success'
    case 'expired':
    case 'suspended':
      return 'badge-warning'
    case 'superseded':
      return 'badge-gray'
    case 'refunded':
      return 'badge-purple'
    case 'revoked':
      return 'badge-danger'
    default:
      return 'badge-gray'
  }
}

const getDaysRemaining = (expiresAt: string): number | null => {
  const now = new Date()
  const expires = new Date(expiresAt)
  const diff = expires.getTime() - now.getTime()
  if (diff < 0) return null
  return Math.ceil(diff / (1000 * 60 * 60 * 24))
}

const isExpiringSoon = (expiresAt: string): boolean => {
  const days = getDaysRemaining(expiresAt)
  return days !== null && days <= 7
}

const getProgressWidth = (used: number | null | undefined, limit: number | null): string => {
  if (!limit || limit === 0) return '0%'
  const usedValue = used ?? 0
  const percentage = Math.min((usedValue / limit) * 100, 100)
  return `${percentage}%`
}

const subscriptionDisplayName = (subscription: UserSubscription): string =>
  subscription.plan_name_snapshot?.trim() || `${t('payment.plan')} #${subscription.id}`

const displayDailyLimit = (subscription: UserSubscription): number | null =>
  subscription.daily_quota_knives ?? null

const displayWeeklyLimit = (subscription: UserSubscription): number | null =>
  subscription.weekly_quota_knives ?? null

const displayMonthlyLimit = (subscription: UserSubscription): number | null =>
  subscription.monthly_quota_knives ?? null

const displayDailyUsed = (subscription: UserSubscription): number =>
  subscription.daily_quota_knives != null ? (subscription.daily_used_knives ?? 0) : (subscription.daily_usage_usd ?? 0)

const displayWeeklyUsed = (subscription: UserSubscription): number =>
  subscription.weekly_quota_knives != null ? (subscription.weekly_used_knives ?? 0) : (subscription.weekly_usage_usd ?? 0)

const displayMonthlyUsed = (subscription: UserSubscription): number =>
  subscription.monthly_quota_knives != null ? (subscription.monthly_used_knives ?? 0) : (subscription.monthly_usage_usd ?? 0)

const formatQuotaUsage = (knivesUsed: number | null | undefined, limit: number | null, usdUsed: number | null | undefined): string => {
  if (limit == null) return t('admin.subscriptions.unlimited')
  if (typeof knivesUsed === 'number' && Number.isFinite(knivesUsed) && limit >= 0) {
    return `${knivesUsed.toFixed(2)} / ${limit.toFixed(2)}`
  }
  return `$${(usdUsed ?? 0).toFixed(2)} / ${limit.toFixed(2)}`
}

const formatPlanValidity = (plan: SubscriptionPlan): string => {
  const unit = plan.validity_unit || 'day'
  if (unit === 'month') return `${plan.validity_days} ${t('payment.perMonth')}`
  if (unit === 'year') return `${plan.validity_days} ${t('payment.perYear')}`
  if (unit === 'week') return `${plan.validity_days} ${t('payment.admin.weeks')}`
  return `${plan.validity_days}${t('payment.days')}`
}

const getProgressClass = (used: number | null | undefined, limit: number | null): string => {
  if (!limit || limit === 0) return 'bg-gray-400'
  const usedValue = used ?? 0
  const percentage = (usedValue / limit) * 100
  if (percentage >= 90) return 'bg-red-500'
  if (percentage >= 70) return 'bg-orange-500'
  return 'bg-green-500'
}

const formatResetDuration = (parts: RemainingDurationParts): string => {
  if (parts.days > 0) {
    return t('admin.subscriptions.resetInDaysHours', { days: parts.days, hours: parts.hours })
  }

  if (parts.hours > 0) {
    return t('admin.subscriptions.resetInHoursMinutes', { hours: parts.hours, minutes: parts.minutes })
  }

  return t('admin.subscriptions.resetInMinutes', { minutes: parts.minutes })
}

const formatQuotaEndDuration = (parts: RemainingDurationParts): string => {
  if (parts.days > 0) {
    return t('admin.subscriptions.quotaEndsInDaysHours', { days: parts.days, hours: parts.hours })
  }

  if (parts.hours > 0) {
    return t('admin.subscriptions.quotaEndsInHoursMinutes', { hours: parts.hours, minutes: parts.minutes })
  }

  return t('admin.subscriptions.quotaEndsInMinutes', { minutes: parts.minutes })
}

const formatDailyUsageWindow = (subscription: UserSubscription): string => {
  if (isOneTimeDailyQuota(subscription) && subscription.expires_at) {
    const parts = getRemainingDurationParts(subscription.expires_at)
    return parts ? formatQuotaEndDuration(parts) : t('admin.subscriptions.windowNotActive')
  }

  return formatResetTime(subscription.daily_window_start, 'daily')
}

// Format reset time based on window start and period type
const formatResetTime = (windowStart: string | null, period: 'daily' | 'weekly' | 'monthly'): string => {
  if (!windowStart) return t('admin.subscriptions.windowNotActive')

  const start = new Date(windowStart)
  const now = new Date()

  // Calculate reset time based on period
  let resetTime: Date
  switch (period) {
    case 'daily':
      resetTime = new Date(start.getTime() + 24 * 60 * 60 * 1000)
      break
    case 'weekly':
      resetTime = new Date(start.getTime() + 7 * 24 * 60 * 60 * 1000)
      break
    case 'monthly':
      resetTime = new Date(start.getTime() + 30 * 24 * 60 * 60 * 1000)
      break
  }

  const parts = getRemainingDurationParts(resetTime, now)

  return parts ? formatResetDuration(parts) : t('admin.subscriptions.windowNotActive')
}

// Handle click outside to close dropdowns
const handleClickOutside = (event: MouseEvent) => {
  const target = event.target as HTMLElement
  if (!target.closest('[data-assign-user-search]')) showUserDropdown.value = false
  if (!target.closest('[data-filter-user-search]')) showFilterUserDropdown.value = false
  if (columnDropdownRef.value && !columnDropdownRef.value.contains(target)) {
    showColumnDropdown.value = false
  }
}

onMounted(() => {
  loadUserColumnMode()
  loadSavedColumns()
  loadSubscriptions()
  loadSubscriptionPlans()
  document.addEventListener('click', handleClickOutside)
})

onUnmounted(() => {
  document.removeEventListener('click', handleClickOutside)
  if (filterUserSearchTimeout) {
    clearTimeout(filterUserSearchTimeout)
  }
  if (userSearchTimeout) {
    clearTimeout(userSearchTimeout)
  }
})
</script>

<style scoped>
.usage-row {
  @apply space-y-1;
}

.usage-label {
  @apply w-10 flex-shrink-0 text-xs font-medium text-gray-500 dark:text-gray-400;
}

.usage-amount {
  @apply whitespace-nowrap text-xs tabular-nums text-gray-600 dark:text-gray-300;
}

.reset-info {
  @apply flex items-center gap-1 pl-12 text-[10px] text-blue-600 dark:text-blue-400;
}
</style>
