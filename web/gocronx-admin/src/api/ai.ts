import request from '@/utils/http'

// ── LLM config ──────────────────────────────────────────────────────────────

export interface LLMConfig {
  enable: boolean
  base_url: string
  model: string
  api_key_set: boolean
}

export interface LLMConfigUpdate {
  enable: boolean
  base_url: string
  api_key: string // 留空表示不修改
  model: string
}

/** GET /api/system/llm  →  LLMConfig (never returns the key) */
export function fetchLLMConfig() {
  return request.get<LLMConfig>({ url: '/api/system/llm' })
}

/** POST /api/system/llm/update */
export function updateLLMConfig(data: LLMConfigUpdate) {
  return request.post<null>({ url: '/api/system/llm/update', data })
}

// ── NL → cron ─────────────────────────────────────────────────────────────────

export interface NlToCronResult {
  spec: string
  preview: {
    valid: boolean
    error?: string
    next_runs?: { text: string }[]
  }
}

/** POST /api/task/nl-to-cron */
export function nlToCron(text: string, timezone?: string) {
  return request.post<NlToCronResult>({
    url: '/api/task/nl-to-cron',
    data: { text, timezone: timezone || '' }
  })
}

// ── Failure log diagnosis ─────────────────────────────────────────────────────

export interface DiagnoseResult {
  diagnosis: string
}

/** POST /api/task/log/diagnose/:id */
export function diagnoseLog(id: number) {
  return request.post<DiagnoseResult>({ url: `/api/task/log/diagnose/${id}` })
}
