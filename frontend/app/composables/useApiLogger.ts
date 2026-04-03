// ─── API 操作紀錄 composable ───────────────────────────────────────────────
// 使用 useState 讓日誌跨頁面共享（SPA session 內持久）

export interface ApiLogEntry {
  id: number
  timestamp: string
  action: string        // 中文操作名稱，例：「觸發籌碼爬取」
  trigger: string       // UI 觸發來源，例：「手動按鈕」
  method: string        // HTTP method
  endpoint: string      // API path
  requestBody?: unknown // 請求 payload
  statusCode: number | null
  responseRaw: unknown  // 完整原始 JSON 回應
  responseAnalysis: string // 欄位與邏輯說明
  durationMs: number    // 耗時 ms
  success: boolean
  error?: string        // 失敗時的錯誤訊息
}

let _nextId = 0

export function useApiLogger() {
  const entries = useState<ApiLogEntry[]>('apilog:entries', () => [])

  /**
   * 包裝一個 API 呼叫並自動記錄到 entries。
   * 成功或失敗都會記錄，失敗後會重新拋出原始錯誤。
   */
  async function logCall<T>(opts: {
    action: string
    trigger?: string
    method: string
    endpoint: string
    requestBody?: unknown
    analysis: string
    call: () => Promise<T>
  }): Promise<T> {
    const id = ++_nextId
    const now = new Date()
    const timestamp =
      now.toLocaleTimeString('zh-TW', { hour: '2-digit', minute: '2-digit', second: '2-digit' }) +
      '.' +
      String(now.getMilliseconds()).padStart(3, '0')
    const t0 = performance.now()

    const push = (partial: Omit<ApiLogEntry, 'id' | 'timestamp' | 'action' | 'trigger' | 'method' | 'endpoint' | 'requestBody' | 'responseAnalysis'>) => {
      entries.value.unshift({
        id,
        timestamp,
        action: opts.action,
        trigger: opts.trigger ?? opts.action,
        method: opts.method,
        endpoint: opts.endpoint,
        requestBody: opts.requestBody,
        responseAnalysis: opts.analysis,
        ...partial,
      })
      if (entries.value.length > 300) entries.value.length = 300
    }

    try {
      const result = await opts.call()
      push({
        statusCode: 200,
        responseRaw: result,
        durationMs: Math.round(performance.now() - t0),
        success: true,
      })
      return result
    } catch (err: unknown) {
      const e = err as {
        response?: { status?: number; _data?: unknown }
        data?: unknown
        message?: string
      }
      const statusCode = e?.response?.status ?? null
      const responseRaw = e?.response?._data ?? e?.data ?? null
      const errorMsg =
        (responseRaw as { error?: string } | null)?.error ??
        e?.message ??
        '未知錯誤'
      push({
        statusCode,
        responseRaw,
        durationMs: Math.round(performance.now() - t0),
        success: false,
        error: errorMsg,
      })
      throw err
    }
  }

  function clearLogs() {
    entries.value = []
  }

  /**
   * 直接推入一條預建的日誌（適合 SSE / WebSocket 等非標準 fetch 操作）。
   */
  function pushEntry(entry: Omit<ApiLogEntry, 'id'>) {
    const id = ++_nextId
    entries.value.unshift({ id, ...entry })
    if (entries.value.length > 300) entries.value.length = 300
  }

  return { entries, logCall, clearLogs, pushEntry }
}
