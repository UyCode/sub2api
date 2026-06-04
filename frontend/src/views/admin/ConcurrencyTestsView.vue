<template>
  <AppLayout>
    <div class="flex min-h-[calc(100vh-9rem)] flex-col gap-5 p-4 sm:p-6">
      <div class="flex flex-col gap-3 border-b border-gray-200 pb-4 dark:border-dark-700 md:flex-row md:items-end md:justify-between">
        <div>
          <h1 class="text-xl font-semibold text-gray-900 dark:text-white">{{ t('admin.concurrencyTests.title') }}</h1>
          <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">{{ t('admin.concurrencyTests.description') }}</p>
        </div>
        <div class="flex flex-col gap-2 sm:flex-row sm:items-center">
          <SearchInput
            v-model="searchQuery"
            :placeholder="t('admin.concurrencyTests.searchPlaceholder')"
            class="w-full sm:w-72"
            @search="handleSearch"
          />
          <button class="btn btn-secondary" :disabled="loading" @click="reload">
            <Icon name="refresh" size="sm" />
            {{ t('common.refresh') }}
          </button>
          <button class="btn btn-primary" @click="openCreateDialog">
            <Icon name="plus" size="sm" />
            {{ t('admin.concurrencyTests.createButton') }}
          </button>
        </div>
      </div>

      <div v-if="loading" class="grid gap-4 md:grid-cols-2 xl:grid-cols-3">
        <div v-for="n in 6" :key="n" class="h-64 animate-pulse rounded-lg border border-gray-200 bg-gray-100 dark:border-dark-700 dark:bg-dark-800" />
      </div>

      <EmptyState
        v-else-if="tests.length === 0"
        :title="t('admin.concurrencyTests.emptyTitle')"
        :description="t('admin.concurrencyTests.emptyDescription')"
        :action-text="t('admin.concurrencyTests.createButton')"
        @action="openCreateDialog"
      />

      <div v-else class="grid gap-4 md:grid-cols-2 xl:grid-cols-3">
        <article
          v-for="item in tests"
          :key="item.id"
          class="rounded-lg border border-gray-200 bg-white p-4 shadow-sm dark:border-dark-700 dark:bg-dark-800"
        >
          <div class="flex items-start justify-between gap-3">
            <div class="min-w-0">
              <h2 class="truncate text-base font-semibold text-gray-900 dark:text-white">{{ item.name }}</h2>
              <p class="mt-1 line-clamp-2 text-xs text-gray-500 dark:text-dark-400">
                {{ item.description || modeLabel(item.mode) }}
              </p>
            </div>
            <span class="shrink-0 rounded-md bg-gray-100 px-2 py-1 text-xs font-medium text-gray-700 dark:bg-dark-700 dark:text-dark-200">
              {{ modeLabel(item.mode) }}
            </span>
          </div>

          <dl class="mt-4 grid grid-cols-2 gap-3 text-sm">
            <div>
              <dt class="text-xs text-gray-500 dark:text-dark-400">{{ t('admin.concurrencyTests.fields.concurrency') }}</dt>
              <dd class="mt-0.5 font-medium text-gray-900 dark:text-white">{{ item.concurrency }}</dd>
            </div>
            <div>
              <dt class="text-xs text-gray-500 dark:text-dark-400">{{ t('admin.concurrencyTests.fields.accounts') }}</dt>
              <dd class="mt-0.5 font-medium text-gray-900 dark:text-white">
                {{ item.account_ids.length ? t('admin.concurrencyTests.accountCount', { count: item.account_ids.length }) : t('admin.concurrencyTests.customTarget') }}
              </dd>
            </div>
            <div>
              <dt class="text-xs text-gray-500 dark:text-dark-400">{{ t('admin.concurrencyTests.fields.timeout') }}</dt>
              <dd class="mt-0.5 font-medium text-gray-900 dark:text-white">{{ item.timeout_seconds }}s</dd>
            </div>
            <div>
              <dt class="text-xs text-gray-500 dark:text-dark-400">{{ t('admin.concurrencyTests.fields.endpoint') }}</dt>
              <dd class="mt-0.5 truncate font-medium text-gray-900 dark:text-white">{{ item.endpoint || t('admin.concurrencyTests.accountDefaultEndpoint') }}</dd>
            </div>
          </dl>

          <div class="mt-4 rounded-md bg-gray-50 p-3 dark:bg-dark-900">
            <template v-if="item.latest_run">
              <div class="flex items-center justify-between">
                <span class="text-xs font-medium text-gray-500 dark:text-dark-400">{{ t('admin.concurrencyTests.latestResult') }}</span>
                <span :class="successClass(item.latest_run.success_rate)" class="rounded-md px-2 py-0.5 text-xs font-semibold">
                  {{ formatPercent(item.latest_run.success_rate) }}
                </span>
              </div>
              <div class="mt-2 grid grid-cols-3 gap-2 text-xs">
                <MetricTile :label="t('admin.concurrencyTests.metrics.success')" :value="String(item.latest_run.success_count)" />
                <MetricTile :label="t('admin.concurrencyTests.metrics.failed')" :value="String(item.latest_run.failure_count)" />
                <MetricTile :label="t('admin.concurrencyTests.metrics.p95')" :value="formatMs(item.latest_run.p95_latency_ms)" />
              </div>
            </template>
            <p v-else class="text-sm text-gray-500 dark:text-dark-400">{{ t('admin.concurrencyTests.noRunsYet') }}</p>
          </div>

          <div class="mt-4 flex flex-wrap justify-end gap-2">
            <button class="btn btn-secondary btn-sm" @click="openRunsDialog(item)">{{ t('admin.concurrencyTests.history') }}</button>
            <button class="btn btn-secondary btn-sm" @click="openEditDialog(item)">{{ t('common.edit') }}</button>
            <button class="btn btn-secondary btn-sm text-red-600 hover:text-red-700" @click="openDeleteDialog(item)">{{ t('common.delete') }}</button>
            <button class="btn btn-primary btn-sm" :disabled="runningId === item.id" @click="runTest(item)">
              <Icon v-if="runningId === item.id" name="sync" size="sm" class="animate-spin" />
              <Icon v-else name="play" size="sm" />
              {{ runningId === item.id ? t('admin.concurrencyTests.running') : t('admin.concurrencyTests.run') }}
            </button>
          </div>
        </article>
      </div>

      <div v-if="pagination.total > 0" class="mt-auto pt-1">
        <Pagination
          :page="pagination.page"
          :total="pagination.total"
          :page-size="pagination.page_size"
          @update:page="onPageChange"
          @update:pageSize="onPageSizeChange"
        />
      </div>
    </div>

    <BaseDialog
      :show="showFormDialog"
      :title="editing ? t('admin.concurrencyTests.editTitle') : t('admin.concurrencyTests.createTitle')"
      width="extra-wide"
      @close="closeFormDialog"
    >
      <div class="grid gap-5 lg:grid-cols-[minmax(0,1.1fr)_minmax(320px,0.9fr)]">
        <div class="space-y-4">
          <div class="grid gap-4 md:grid-cols-2">
            <Input v-model="form.name" :label="t('admin.concurrencyTests.fields.name')" required />
            <Input v-model.number="form.concurrency" type="number" :label="t('admin.concurrencyTests.fields.concurrency')" required />
            <div>
              <label class="input-label mb-1.5 block">{{ t('admin.concurrencyTests.fields.mode') }}</label>
              <select v-model="form.mode" class="input w-full">
                <option v-for="option in modeOptions" :key="option.value" :value="option.value">{{ option.label }}</option>
              </select>
            </div>
            <Input v-model.number="form.timeout_seconds" type="number" :label="t('admin.concurrencyTests.fields.timeout')" />
          </div>

          <TextArea v-model="form.description" :label="t('admin.concurrencyTests.fields.description')" :rows="2" />

          <section class="rounded-lg border border-gray-200 p-4 dark:border-dark-700">
            <div class="flex items-center justify-between gap-3">
              <h3 class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('admin.concurrencyTests.accountSelection') }}</h3>
              <button class="btn btn-secondary btn-sm" type="button" @click="loadAccounts">{{ t('common.refresh') }}</button>
            </div>
            <div class="mt-3 max-h-52 overflow-y-auto rounded-md border border-gray-200 dark:border-dark-700">
              <label
                v-for="account in accounts"
                :key="account.id"
                class="flex cursor-pointer items-center gap-3 border-b border-gray-100 px-3 py-2 last:border-0 hover:bg-gray-50 dark:border-dark-700 dark:hover:bg-dark-700"
              >
                <input v-model="form.account_ids" type="checkbox" class="h-4 w-4 rounded border-gray-300" :value="account.id" />
                <span class="min-w-0 flex-1 truncate text-sm text-gray-800 dark:text-dark-100">
                  {{ account.name }}
                </span>
                <span class="rounded bg-gray-100 px-2 py-0.5 text-xs text-gray-600 dark:bg-dark-900 dark:text-dark-300">
                  {{ account.platform }} / {{ account.type }}
                </span>
              </label>
              <p v-if="accounts.length === 0" class="px-3 py-4 text-sm text-gray-500 dark:text-dark-400">
                {{ t('admin.concurrencyTests.noAccountsLoaded') }}
              </p>
            </div>
          </section>

          <section class="rounded-lg border border-gray-200 p-4 dark:border-dark-700">
            <h3 class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('admin.concurrencyTests.customTarget') }}</h3>
            <div class="mt-3 grid gap-4 md:grid-cols-[120px_1fr]">
              <Input v-model="form.method" :label="t('admin.concurrencyTests.fields.method')" />
              <Input v-model="form.endpoint" :label="t('admin.concurrencyTests.fields.endpoint')" :placeholder="t('admin.concurrencyTests.endpointPlaceholder')" />
            </div>
            <Input
              v-model="form.api_key"
              class="mt-4"
              type="password"
              :label="t('admin.concurrencyTests.fields.apiKey')"
              :placeholder="editing?.api_key_set ? editing.api_key_masked : ''"
              autocomplete="new-password"
            />
          </section>
        </div>

        <div class="space-y-4">
          <TextArea
            v-model="headersText"
            :label="t('admin.concurrencyTests.fields.headers')"
            :rows="8"
            :error="headersError"
          />
          <TextArea
            v-model="bodyText"
            :label="t('admin.concurrencyTests.fields.bodyTemplate')"
            :rows="16"
            :error="bodyError"
          />
          <div class="rounded-md bg-gray-50 p-3 text-xs text-gray-600 dark:bg-dark-900 dark:text-dark-300">
            {{ t('admin.concurrencyTests.templateHint') }}
          </div>
        </div>
      </div>

      <template #footer>
        <button class="btn btn-secondary" @click="closeFormDialog">{{ t('common.cancel') }}</button>
        <button class="btn btn-primary" :disabled="saving" @click="saveForm">
          {{ saving ? t('common.saving') : t('common.save') }}
        </button>
      </template>
    </BaseDialog>

    <BaseDialog :show="showResultDialog" :title="t('admin.concurrencyTests.resultTitle')" width="extra-wide" @close="showResultDialog = false">
      <RunDetail v-if="selectedRun" :run="selectedRun" />
    </BaseDialog>

    <BaseDialog :show="showRunsDialog" :title="runsDialogTitle" width="extra-wide" @close="showRunsDialog = false">
      <div class="space-y-3">
        <div v-for="run in runs" :key="run.id" class="rounded-lg border border-gray-200 p-3 dark:border-dark-700">
          <div class="flex flex-col gap-2 md:flex-row md:items-center md:justify-between">
            <div>
              <div class="font-medium text-gray-900 dark:text-white">{{ formatDate(run.created_at) }}</div>
              <div class="mt-1 text-xs text-gray-500 dark:text-dark-400">
                {{ t('admin.concurrencyTests.runSummary', { total: run.total_requests, success: run.success_count, failed: run.failure_count }) }}
              </div>
            </div>
            <div class="flex flex-wrap items-center gap-2">
              <span :class="successClass(run.success_rate)" class="rounded-md px-2 py-1 text-xs font-semibold">{{ formatPercent(run.success_rate) }}</span>
              <button class="btn btn-secondary btn-sm" @click="openRunDetail(run)">{{ t('admin.concurrencyTests.viewLogs') }}</button>
            </div>
          </div>
        </div>
        <p v-if="runs.length === 0" class="text-sm text-gray-500 dark:text-dark-400">{{ t('admin.concurrencyTests.noRunsYet') }}</p>
      </div>
    </BaseDialog>

    <ConfirmDialog
      :show="showDeleteDialog"
      :title="t('common.delete')"
      :message="deleteMessage"
      :confirm-text="t('common.delete')"
      :cancel-text="t('common.cancel')"
      :danger="true"
      @confirm="confirmDelete"
      @cancel="showDeleteDialog = false"
    />
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, defineComponent, h, nextTick, onMounted, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminAPI } from '@/api/admin'
import type { Account } from '@/types'
import type { ConcurrencyTestConfig, ConcurrencyTestLog, ConcurrencyTestMode, ConcurrencyTestRun } from '@/api/admin/concurrencyTests'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'
import AppLayout from '@/components/layout/AppLayout.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import EmptyState from '@/components/common/EmptyState.vue'
import Icon from '@/components/icons/Icon.vue'
import Input from '@/components/common/Input.vue'
import Pagination from '@/components/common/Pagination.vue'
import SearchInput from '@/components/common/SearchInput.vue'
import TextArea from '@/components/common/TextArea.vue'

const DEFAULT_PNG_BASE64 = 'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+/p9sAAAAASUVORK5CYII='

const { t } = useI18n()
const appStore = useAppStore()

const tests = ref<ConcurrencyTestConfig[]>([])
const accounts = ref<Account[]>([])
const runs = ref<ConcurrencyTestRun[]>([])
const selectedRun = ref<ConcurrencyTestRun | null>(null)
const loading = ref(false)
const saving = ref(false)
const runningId = ref<number | null>(null)
const searchQuery = ref('')
const pagination = reactive({ page: 1, page_size: 12, total: 0 })
const showFormDialog = ref(false)
const showResultDialog = ref(false)
const showRunsDialog = ref(false)
const showDeleteDialog = ref(false)
const editing = ref<ConcurrencyTestConfig | null>(null)
const deleting = ref<ConcurrencyTestConfig | null>(null)
const currentRunsConfig = ref<ConcurrencyTestConfig | null>(null)
const headersText = ref('{}')
const bodyText = ref('{}')
const headersError = ref('')
const bodyError = ref('')

let searchTimeout: ReturnType<typeof setTimeout> | null = null
let abortController: AbortController | null = null
const MODAL_LEAVE_MS = 220

const modeOptions = computed(() => [
  { value: 'responses' as const, label: t('admin.concurrencyTests.modes.responses') },
  { value: 'openai_image_generations' as const, label: t('admin.concurrencyTests.modes.openaiImageGenerations') },
  { value: 'openai_image_edits' as const, label: t('admin.concurrencyTests.modes.openaiImageEdits') },
  { value: 'gemini_image_generations' as const, label: t('admin.concurrencyTests.modes.geminiImageGenerations') },
  { value: 'gemini_image_edits' as const, label: t('admin.concurrencyTests.modes.geminiImageEdits') },
])

const form = reactive({
  name: '',
  description: '',
  mode: 'responses' as ConcurrencyTestMode,
  concurrency: 100,
  account_ids: [] as number[],
  endpoint: '',
  api_key: '',
  method: 'POST',
  timeout_seconds: 60,
})

const deleteMessage = computed(() => t('admin.concurrencyTests.deleteConfirm', { name: deleting.value?.name || '' }))
const runsDialogTitle = computed(() => currentRunsConfig.value ? t('admin.concurrencyTests.historyFor', { name: currentRunsConfig.value.name }) : t('admin.concurrencyTests.history'))

watch(() => form.mode, (mode) => {
  if (!editing.value) {
    bodyText.value = JSON.stringify(defaultBodyForMode(mode), null, 2)
  }
})

async function reload() {
  abortController?.abort()
  const ctrl = new AbortController()
  abortController = ctrl
  loading.value = true
  try {
    const res = await adminAPI.concurrencyTests.list({
      page: pagination.page,
      page_size: pagination.page_size,
      search: searchQuery.value.trim() || undefined,
    }, { signal: ctrl.signal })
    if (ctrl.signal.aborted) return
    tests.value = res.items || []
    pagination.total = res.total
  } catch (err: unknown) {
    const e = err as { code?: string; name?: string }
    if (e?.code === 'ERR_CANCELED' || e?.name === 'AbortError') return
    appStore.showError(extractApiErrorMessage(err, t('admin.concurrencyTests.loadError')))
  } finally {
    if (abortController === ctrl) {
      loading.value = false
      abortController = null
    }
  }
}

async function loadAccounts() {
  try {
    const res = await adminAPI.accounts.list(1, 1000, { lite: 'true', sort_by: 'id', sort_order: 'desc' })
    accounts.value = res.items || []
  } catch (err: unknown) {
    appStore.showError(extractApiErrorMessage(err, t('admin.concurrencyTests.accountsLoadError')))
  }
}

function handleSearch() {
  if (searchTimeout) clearTimeout(searchTimeout)
  searchTimeout = setTimeout(() => {
    pagination.page = 1
    reload()
  }, 300)
}

function onPageChange(page: number) {
  pagination.page = page
  reload()
}

function onPageSizeChange(size: number) {
  pagination.page_size = size
  pagination.page = 1
  reload()
}

function openCreateDialog() {
  editing.value = null
  Object.assign(form, {
    name: '',
    description: '',
    mode: 'responses',
    concurrency: 100,
    account_ids: [],
    endpoint: '',
    api_key: '',
    method: 'POST',
    timeout_seconds: 60,
  })
  headersText.value = '{}'
  bodyText.value = JSON.stringify(defaultBodyForMode('responses'), null, 2)
  clearJsonErrors()
  showFormDialog.value = true
  if (accounts.value.length === 0) void loadAccounts()
}

function openEditDialog(item: ConcurrencyTestConfig) {
  editing.value = item
  Object.assign(form, {
    name: item.name,
    description: item.description || '',
    mode: item.mode,
    concurrency: item.concurrency,
    account_ids: [...item.account_ids],
    endpoint: item.endpoint || '',
    api_key: '',
    method: item.method || 'POST',
    timeout_seconds: item.timeout_seconds || 60,
  })
  headersText.value = JSON.stringify(item.headers || {}, null, 2)
  bodyText.value = JSON.stringify(item.body_template || {}, null, 2)
  clearJsonErrors()
  showFormDialog.value = true
  if (accounts.value.length === 0) void loadAccounts()
}

function closeFormDialog() {
  showFormDialog.value = false
  editing.value = null
}

async function saveForm() {
  clearJsonErrors()
  const headers = parseJsonObject(headersText.value, 'headers')
  const body = parseJsonObject(bodyText.value, 'body')
  if (!headers || !body) return
  saving.value = true
  try {
    const payload = {
      name: form.name,
      description: form.description,
      mode: form.mode,
      concurrency: Number(form.concurrency),
      account_ids: form.account_ids,
      endpoint: form.endpoint,
      api_key: form.api_key.trim(),
      method: form.method || 'POST',
      headers: stringifyHeaders(headers),
      body_template: body,
      timeout_seconds: Number(form.timeout_seconds) || 60,
    }
    if (editing.value) {
      const { api_key, ...rest } = payload
      const updatePayload = { ...rest } as typeof rest & { api_key?: string }
      if (api_key) updatePayload.api_key = api_key
      await adminAPI.concurrencyTests.update(editing.value.id, updatePayload)
      appStore.showSuccess(t('admin.concurrencyTests.updateSuccess'))
    } else {
      await adminAPI.concurrencyTests.create(payload)
      appStore.showSuccess(t('admin.concurrencyTests.createSuccess'))
    }
    closeFormDialog()
    await reload()
  } catch (err: unknown) {
    appStore.showError(extractApiErrorMessage(err, t('admin.concurrencyTests.saveError')))
  } finally {
    saving.value = false
  }
}

async function runTest(item: ConcurrencyTestConfig) {
  if (runningId.value != null) return
  runningId.value = item.id
  try {
    const run = await adminAPI.concurrencyTests.run(item.id)
    selectedRun.value = run
    showResultDialog.value = true
    appStore.showSuccess(t('admin.concurrencyTests.runComplete'))
    await reload()
  } catch (err: unknown) {
    appStore.showError(extractApiErrorMessage(err, t('admin.concurrencyTests.runFailed')))
  } finally {
    runningId.value = null
  }
}

async function openRunsDialog(item: ConcurrencyTestConfig) {
  currentRunsConfig.value = item
  showRunsDialog.value = true
  try {
    const res = await adminAPI.concurrencyTests.listRuns(item.id, 30)
    runs.value = res.items || []
  } catch (err: unknown) {
    appStore.showError(extractApiErrorMessage(err, t('admin.concurrencyTests.loadRunsError')))
  }
}

async function openRunDetail(run: ConcurrencyTestRun) {
  showRunsDialog.value = false
  const dialogClosed = waitForDialogLeave()
  try {
    const res = await adminAPI.concurrencyTests.listLogs(run.id, 500)
    await dialogClosed
    selectedRun.value = { ...run, logs: res.items || [] }
    showResultDialog.value = true
  } catch (err: unknown) {
    appStore.showError(extractApiErrorMessage(err, t('admin.concurrencyTests.loadLogsError')))
  }
}

async function waitForDialogLeave() {
  await nextTick()
  await new Promise(resolve => window.setTimeout(resolve, MODAL_LEAVE_MS))
}

function openDeleteDialog(item: ConcurrencyTestConfig) {
  deleting.value = item
  showDeleteDialog.value = true
}

async function confirmDelete() {
  if (!deleting.value) return
  try {
    await adminAPI.concurrencyTests.del(deleting.value.id)
    appStore.showSuccess(t('admin.concurrencyTests.deleteSuccess'))
    showDeleteDialog.value = false
    deleting.value = null
    await reload()
  } catch (err: unknown) {
    appStore.showError(extractApiErrorMessage(err, t('admin.concurrencyTests.deleteError')))
  }
}

function parseJsonObject(raw: string, field: 'headers' | 'body'): Record<string, unknown> | null {
  try {
    const parsed = raw.trim() ? JSON.parse(raw) : {}
    if (!parsed || Array.isArray(parsed) || typeof parsed !== 'object') {
      throw new Error(t('admin.concurrencyTests.jsonObjectOnly'))
    }
    return parsed as Record<string, unknown>
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : t('admin.concurrencyTests.invalidJson')
    if (field === 'headers') headersError.value = msg
    if (field === 'body') bodyError.value = msg
    return null
  }
}

function clearJsonErrors() {
  headersError.value = ''
  bodyError.value = ''
}

function stringifyHeaders(headers: Record<string, unknown>): Record<string, string> {
  const out: Record<string, string> = {}
  for (const [key, value] of Object.entries(headers)) {
    const normalizedKey = key.trim()
    if (!normalizedKey || value == null) continue
    out[normalizedKey] = typeof value === 'string' ? value : String(value)
  }
  return out
}

function defaultBodyForMode(mode: ConcurrencyTestMode): Record<string, unknown> {
  switch (mode) {
    case 'openai_image_generations':
      return { model: 'gpt-image-1', prompt: 'A small blue cube on a white background.', size: '1024x1024', n: 1 }
    case 'openai_image_edits':
      return { model: 'gpt-image-1', prompt: 'Make the object brighter.', image_base64: '', image_filename: 'input.png' }
    case 'gemini_image_generations':
      return { contents: [{ parts: [{ text: 'Generate a small blue cube on a white background.' }] }] }
    case 'gemini_image_edits':
      return { contents: [{ parts: [{ text: 'Edit the supplied image according to the prompt.' }, { inline_data: { mime_type: 'image/png', data: DEFAULT_PNG_BASE64 } }] }] }
    default:
      return { model: 'gpt-4.1-mini', input: 'Return the word ok.', max_output_tokens: 16 }
  }
}

function modeLabel(mode: ConcurrencyTestMode) {
  const found = modeOptions.value.find(option => option.value === mode)
  return found?.label || mode
}

function formatPercent(value: number) {
  return `${Number(value || 0).toFixed(1)}%`
}

function formatMs(value: number) {
  return `${value || 0}ms`
}

function formatDate(value: string) {
  if (!value) return '-'
  return new Date(value).toLocaleString()
}

function successClass(value: number) {
  if (value >= 95) return 'bg-green-100 text-green-700 dark:bg-green-900/30 dark:text-green-300'
  if (value >= 80) return 'bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-300'
  return 'bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-300'
}

const MetricTile = defineComponent({
  props: {
    label: { type: String, required: true },
    value: { type: String, required: true },
  },
  setup(props) {
    return () => h('div', { class: 'rounded-md bg-white p-2 dark:bg-dark-800' }, [
      h('div', { class: 'text-[11px] text-gray-500 dark:text-dark-400' }, props.label),
      h('div', { class: 'mt-1 font-semibold text-gray-900 dark:text-white' }, props.value),
    ])
  },
})

const RunDetail = defineComponent({
  props: {
    run: { type: Object as () => ConcurrencyTestRun, required: true },
  },
  setup(props) {
    return () => h('div', { class: 'space-y-4' }, [
      h('div', { class: 'grid gap-3 md:grid-cols-4' }, [
        h(MetricTile, { label: t('admin.concurrencyTests.metrics.successRate'), value: formatPercent(props.run.success_rate) }),
        h(MetricTile, { label: t('admin.concurrencyTests.metrics.success'), value: String(props.run.success_count) }),
        h(MetricTile, { label: t('admin.concurrencyTests.metrics.failed'), value: String(props.run.failure_count) }),
        h(MetricTile, { label: t('admin.concurrencyTests.metrics.timeout'), value: String(props.run.timeout_count) }),
        h(MetricTile, { label: 'P50', value: formatMs(props.run.p50_latency_ms) }),
        h(MetricTile, { label: 'P90', value: formatMs(props.run.p90_latency_ms) }),
        h(MetricTile, { label: 'P95', value: formatMs(props.run.p95_latency_ms) }),
        h(MetricTile, { label: 'P99', value: formatMs(props.run.p99_latency_ms) }),
      ]),
      h('div', { class: 'overflow-hidden rounded-lg border border-gray-200 dark:border-dark-700' }, [
        h('div', { class: 'max-h-[420px] overflow-auto' }, [
          h('table', { class: 'min-w-full divide-y divide-gray-200 text-sm dark:divide-dark-700' }, [
            h('thead', { class: 'sticky top-0 bg-gray-50 dark:bg-dark-800' }, [
              h('tr', [
                'index', 'account', 'status', 'latency', 'timeout', 'error'
              ].map(key => h('th', { class: 'px-3 py-2 text-left text-xs font-medium uppercase text-gray-500 dark:text-dark-400' }, t(`admin.concurrencyTests.logColumns.${key}`))))
            ]),
            h('tbody', { class: 'divide-y divide-gray-100 bg-white dark:divide-dark-700 dark:bg-dark-900' },
              (props.run.logs || []).map((log: ConcurrencyTestLog) => h('tr', [
                h('td', { class: 'px-3 py-2' }, String(log.request_index)),
                h('td', { class: 'px-3 py-2' }, log.account_id ? String(log.account_id) : '-'),
                h('td', { class: 'px-3 py-2' }, log.status_code == null ? '-' : String(log.status_code)),
                h('td', { class: 'px-3 py-2' }, formatMs(log.latency_ms)),
                h('td', { class: 'px-3 py-2' }, log.timeout ? 'Y' : ''),
                h('td', { class: 'max-w-md truncate px-3 py-2 text-red-600 dark:text-red-300' }, log.error_message || ''),
              ]))
            ),
          ])
        ])
      ])
    ])
  },
})

onMounted(() => {
  void reload()
  void loadAccounts()
})
</script>
