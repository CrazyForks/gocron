import request from '@/utils/http'

// ── Types ─────────────────────────────────────────────────────────────────────

export interface McpTokenItem {
  id: number
  name: string
  last_used_at: string | null
  created_at: string
}

export interface McpTokenCreateResult {
  id: number
  name: string
  token: string
}

// ── API functions ─────────────────────────────────────────────────────────────

/**
 * GET /api/mcp-token  →  McpTokenItem[]
 */
export function fetchMcpTokenList() {
  return request.get<McpTokenItem[]>({
    url: '/api/mcp-token'
  })
}

/**
 * POST /api/mcp-token/store  →  { id, name, token }
 * The plaintext token is returned only once, here.
 */
export function createMcpToken(name: string) {
  return request.post<McpTokenCreateResult>({
    url: '/api/mcp-token/store',
    data: { name }
  })
}

/**
 * POST /api/mcp-token/remove/:id
 */
export function removeMcpToken(id: number) {
  return request.post<null>({
    url: `/api/mcp-token/remove/${id}`
  })
}
