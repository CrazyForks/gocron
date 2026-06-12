import request from '@/utils/http'
import { useUserStore } from '@/store/modules/user'

// 把当前界面语言（zh/en）作为 Accept-Language 传给后端，
// 让 AI 的输出语言与界面切换保持一致（后端 GetLocale 据此选 prompt 语言）。
function langHeader() {
  return { 'Accept-Language': useUserStore().language }
}

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

// LLM 推理（尤其本地大模型）较慢，这两个接口单独放宽超时，避免前端 15s 默认超时提前断开。
const AI_TIMEOUT = 120000

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
    data: { text, timezone: timezone || '' },
    timeout: AI_TIMEOUT,
    headers: langHeader()
  })
}

// ── Failure log diagnosis ─────────────────────────────────────────────────────

export interface DiagnoseResult {
  root_cause: string
  suggestions: string[]
}

/** POST /api/task/log/diagnose/:id */
export function diagnoseLog(id: number) {
  return request.post<DiagnoseResult>({
    url: `/api/task/log/diagnose/${id}`,
    timeout: AI_TIMEOUT,
    headers: langHeader()
  })
}
