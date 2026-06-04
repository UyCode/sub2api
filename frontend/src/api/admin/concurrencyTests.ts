/**
 * Admin concurrency test API.
 */

import { apiClient } from '../client'

export type ConcurrencyTestMode =
  | 'responses'
  | 'openai_image_generations'
  | 'openai_image_edits'
  | 'gemini_image_generations'
  | 'gemini_image_edits'

export interface ConcurrencyTestConfig {
  id: number
  name: string
  description: string
  mode: ConcurrencyTestMode
  concurrency: number
  account_ids: number[]
  endpoint: string
  api_key_set: boolean
  api_key_masked: string
  method: string
  headers: Record<string, string>
  body_template: Record<string, unknown>
  timeout_seconds: number
  created_by: number
  created_at: string
  updated_at: string
  latest_run?: ConcurrencyTestRun | null
}

export interface ConcurrencyTestRun {
  id: number
  config_id: number
  name: string
  mode: ConcurrencyTestMode
  concurrency: number
  account_ids: number[]
  request_source: 'accounts' | 'custom'
  endpoint: string
  method: string
  started_at: string
  finished_at: string
  status: string
  total_requests: number
  success_count: number
  failure_count: number
  timeout_count: number
  gateway_timeouts: number
  success_rate: number
  avg_latency_ms: number
  min_latency_ms: number
  max_latency_ms: number
  p50_latency_ms: number
  p90_latency_ms: number
  p95_latency_ms: number
  p99_latency_ms: number
  summary: Record<string, unknown>
  error_message: string
  created_at: string
  logs?: ConcurrencyTestLog[]
}

export interface ConcurrencyTestLog {
  id: number
  run_id: number
  request_index: number
  account_id: number | null
  endpoint: string
  method: string
  status_code: number | null
  success: boolean
  timeout: boolean
  latency_ms: number
  error_message: string
  response_body: string
  started_at: string
  finished_at: string
  created_at: string
}

export interface ListParams {
  page?: number
  page_size?: number
  search?: string
}

export interface ListResponse {
  items: ConcurrencyTestConfig[]
  total: number
  page: number
  page_size: number
  pages: number
}

export interface UpsertConcurrencyTestParams {
  name: string
  description?: string
  mode: ConcurrencyTestMode
  concurrency: number
  account_ids?: number[]
  endpoint?: string
  api_key?: string
  method?: string
  headers?: Record<string, string>
  body_template?: Record<string, unknown>
  timeout_seconds?: number
}

export type UpdateConcurrencyTestParams = Partial<UpsertConcurrencyTestParams>

export async function list(params: ListParams = {}, options?: { signal?: AbortSignal }): Promise<ListResponse> {
  const { data } = await apiClient.get<ListResponse>('/admin/concurrency-tests', {
    params,
    signal: options?.signal,
  })
  return data
}

export async function get(id: number): Promise<ConcurrencyTestConfig> {
  const { data } = await apiClient.get<ConcurrencyTestConfig>(`/admin/concurrency-tests/${id}`)
  return data
}

export async function create(params: UpsertConcurrencyTestParams): Promise<ConcurrencyTestConfig> {
  const { data } = await apiClient.post<ConcurrencyTestConfig>('/admin/concurrency-tests', params)
  return data
}

export async function update(id: number, params: UpdateConcurrencyTestParams): Promise<ConcurrencyTestConfig> {
  const { data } = await apiClient.put<ConcurrencyTestConfig>(`/admin/concurrency-tests/${id}`, params)
  return data
}

export async function del(id: number): Promise<void> {
  await apiClient.delete(`/admin/concurrency-tests/${id}`)
}

export async function run(id: number): Promise<ConcurrencyTestRun> {
  const { data } = await apiClient.post<ConcurrencyTestRun>(`/admin/concurrency-tests/${id}/run`, undefined, {
    timeout: 620000,
  })
  return data
}

export async function listRuns(id: number, limit = 20): Promise<{ items: ConcurrencyTestRun[] }> {
  const { data } = await apiClient.get<{ items: ConcurrencyTestRun[] }>(`/admin/concurrency-tests/${id}/runs`, {
    params: { limit },
  })
  return data
}

export async function listLogs(runId: number, limit = 200): Promise<{ items: ConcurrencyTestLog[] }> {
  const { data } = await apiClient.get<{ items: ConcurrencyTestLog[] }>(`/admin/concurrency-tests/runs/${runId}/logs`, {
    params: { limit },
  })
  return data
}

export const concurrencyTestsAPI = {
  list,
  get,
  create,
  update,
  del,
  run,
  listRuns,
  listLogs,
}

export default concurrencyTestsAPI
