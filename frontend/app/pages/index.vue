<script setup lang="ts">
useHead({
  link: [
    { rel: 'preconnect', href: 'https://fonts.googleapis.com' },
    { rel: 'preconnect', href: 'https://fonts.gstatic.com', crossorigin: '' },
    {
      rel: 'stylesheet',
      href: 'https://fonts.googleapis.com/css2?family=DM+Sans:ital,opsz,wght@0,9..40,300;0,9..40,400;0,9..40,500;0,9..40,600;0,9..40,700;1,9..40,400&family=Fira+Code:wght@400;500;600&display=swap',
    },
  ],
})

interface Stock {
  id: number
  symbol: string
  name: string
  price: number
  change: number
  change_pct: number
  volume: number
  updated_at: string
}

interface SyncState {
  stage: string
  message: string
  progress: number
  url: string
  total: number
  synced: number
  error: string
}

const { data: stocks, status, refresh } = await useFetch<Stock[]>('/api/stocks')

// ── 籌碼金字塔 ────────────────────────────
interface ChipsStatus {
  status: 'never' | 'running' | 'completed' | 'failed'
  is_fresh: boolean
  next_run: string
  started_at?: string
  completed_at?: string
  total?: number
  success?: number
  fail?: number
  message?: string
}

const { data: chipsStatus, refresh: refreshChips } = await useFetch<ChipsStatus>('/api/chips/status')

const chipsTriggering = ref(false)
const chipsError = ref('')
let chipsPollTimer: ReturnType<typeof setInterval> | null = null

function startChipsPolling() {
  if (chipsPollTimer) return
  chipsPollTimer = setInterval(() => {
    refreshChips()
  }, 3000)
}

function stopChipsPolling() {
  if (!chipsPollTimer) return
  clearInterval(chipsPollTimer)
  chipsPollTimer = null
}

async function triggerChips() {
  if (chipsTriggering.value) return
  chipsTriggering.value = true
  chipsError.value = ''
  try {
    await $fetch('/api/chips/trigger', { method: 'POST' })
    await refreshChips()
    startChipsPolling()
  } catch (err: unknown) {
    const e = err as { response?: { status?: number; _data?: { error?: string } }; message?: string; data?: { error?: string } }
    if (e?.response?.status === 409) {
      chipsError.value = '已有爬取任務執行中'
      startChipsPolling()
    } else if (e?.response?.status === 400) {
      chipsError.value = e?.response?._data?.error || e?.data?.error || '目前沒有股票可爬，請先同步股票清單'
    } else {
      chipsError.value = e?.response?._data?.error || e?.data?.error || '籌碼作業啟動失敗，請查看 backend 日誌'
    }
  } finally {
    chipsTriggering.value = false
  }
}

function formatChipsDate(value?: string, options?: Intl.DateTimeFormatOptions) {
  if (!value) return '未知'
  const d = new Date(value)
  if (Number.isNaN(d.getTime()) || d.getFullYear() < 2000) return '未知'
  return d.toLocaleDateString('zh-TW', options ?? {
    year: 'numeric',
    month: 'long',
    day: 'numeric',
  })
}

const chipsLastSync = computed(() => {
  if (!chipsStatus.value || chipsStatus.value.status === 'never') return '從未爬取'
  return formatChipsDate(chipsStatus.value.completed_at || chipsStatus.value.started_at)
})

const chipsNextRun = computed(() => {
  if (!chipsStatus.value?.next_run) return '—'
  const d = new Date(chipsStatus.value.next_run)
  return d.toLocaleDateString('zh-TW', { month: 'long', day: 'numeric', weekday: 'short' })
})

const chipsProcessed = computed(() =>
  (chipsStatus.value?.success ?? 0) + (chipsStatus.value?.fail ?? 0)
)

const chipsProgressLabel = computed(() => {
  const total = chipsStatus.value?.total ?? 0
  if (!total) return '—'
  return `${chipsProcessed.value} / ${total}`
})

const chipsProgressPct = computed(() => {
  const total = chipsStatus.value?.total ?? 0
  if (!total) return 0
  return Math.min(100, Math.round((chipsProcessed.value / total) * 100))
})

const chipsBadgeClass = computed(() => {
  if (chipsStatus.value?.is_fresh) return 'chips-fresh-badge--ok'
  if (chipsStatus.value?.status === 'completed') return 'chips-fresh-badge--done'
  if (chipsStatus.value?.status === 'failed') return 'chips-fresh-badge--fail'
  return 'chips-fresh-badge--stale'
})

const chipsBadgeText = computed(() => {
  if (chipsStatus.value?.is_fresh) return '本週資料已是最新'
  if (chipsStatus.value?.status === 'running') return '爬取中…'
  if (chipsStatus.value?.status === 'failed') return '本次爬取失敗'
  if (chipsStatus.value?.status === 'completed') return '已完成，但資料未達本週最新'
  if (chipsStatus.value?.status === 'never') return '尚未爬取'
  return '資料已過期'
})

const chipsSummaryTitle = computed(() => {
  if (chipsStatus.value?.status === 'running') return '正在回填籌碼資料'
  if (chipsStatus.value?.status === 'completed') return '最近一次籌碼任務已完成'
  if (chipsStatus.value?.status === 'failed') return '最近一次籌碼任務未完成'
  if (chipsStatus.value?.status === 'never') return '尚未建立籌碼任務'
  return '籌碼資料待更新'
})

const chipsSummaryText = computed(() => {
  const total = chipsStatus.value?.total ?? 0
  const success = chipsStatus.value?.success ?? 0
  const fail = chipsStatus.value?.fail ?? 0

  if (chipsStatus.value?.status === 'running') {
    return chipsStatus.value?.message || `已處理 ${chipsProcessed.value} / ${total}`
  }
  if (chipsStatus.value?.status === 'completed') {
    // 有失敗時，額外顯示 message 中的失敗明細
    const base = `本次共處理 ${total} 檔，成功 ${success} 檔，失敗 ${fail} 檔。`
    if (fail > 0 && chipsStatus.value?.message?.includes('失敗範例')) {
      return base + '\n' + chipsStatus.value.message.split('\n\n').slice(1).join('\n\n')
    }
    return base
  }
  if (chipsStatus.value?.status === 'failed') {
    return chipsStatus.value?.message || '爬取程序中斷，請查看錯誤摘要後重新觸發。'
  }
  if (chipsStatus.value?.status === 'never') {
    return '可手動觸發一次增量更新；若資料已齊全，系統會自動跳過不需重抓的股票。'
  }
  return '等待下一次手動或排程更新。'
})

const chipsFailureDetail = computed(() => {
  if (chipsStatus.value?.status !== 'failed') return ''
  const msg = chipsStatus.value?.message?.trim() || ''
  if (!msg) return '未提供細節，建議查看 backend 容器日誌。'
  if (msg.includes('scraper restarted')) return '爬蟲服務在任務完成前重啟，這次作業已中止。'
  if (msg.includes('backend restarted')) return '後端服務在任務完成前重啟，這次作業已中止。'
  if (msg.includes('job failed:')) return msg.replace('job failed:', '').trim()
  // 直接顯示 message（包含失敗明細）
  return msg
})

const chipsCompletedAt = computed(() => {
  if (!chipsStatus.value?.completed_at) return '—'
  return formatChipsDate(chipsStatus.value.completed_at, {
    year: 'numeric',
    month: 'long',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
  })
})

const chipsSummaryClass = computed(() => {
  if (chipsStatus.value?.status === 'completed') return 'chips-summary--done'
  if (chipsStatus.value?.status === 'failed') return 'chips-summary--fail'
  return ''
})

const chipsCanRetry = computed(() =>
  chipsStatus.value?.status === 'failed' || chipsStatus.value?.status === 'completed'
)

const chipsShouldShowSecondaryActions = computed(() =>
  chipsStatus.value?.status === 'failed' || chipsStatus.value?.status === 'completed' || chipsStatus.value?.status === 'never'
)

watch(
  () => chipsStatus.value?.status,
  (status) => {
    if (status === 'running') startChipsPolling()
    else stopChipsPolling()
  },
  { immediate: true },
)

onBeforeUnmount(() => {
  stopChipsPolling()
  stopPriceSyncPolling()
})

// ── 全股票歷史日K批次爬取 ──────────────────────────────
interface PriceSyncStatus {
  status: 'never' | 'running' | 'completed' | 'failed'
  started_at?: string
  completed_at?: string
  total?: number
  success?: number
  fail?: number
  message?: string
}

const { data: priceSyncStatus, refresh: refreshPriceSync } = await useFetch<PriceSyncStatus>('/api/scraper/prices/all/status')

const priceSyncTriggering = ref(false)
const priceSyncError = ref('')
let priceSyncPollTimer: ReturnType<typeof setInterval> | null = null

function startPriceSyncPolling() {
  if (priceSyncPollTimer) return
  priceSyncPollTimer = setInterval(() => { refreshPriceSync() }, 3000)
}

function stopPriceSyncPolling() {
  if (!priceSyncPollTimer) return
  clearInterval(priceSyncPollTimer)
  priceSyncPollTimer = null
}

async function triggerPriceSync() {
  if (priceSyncTriggering.value) return
  priceSyncTriggering.value = true
  priceSyncError.value = ''
  try {
    await $fetch('/api/scraper/prices/all/trigger', { method: 'POST' })
    await refreshPriceSync()
    startPriceSyncPolling()
  } catch (err: unknown) {
    const e = err as { response?: { status?: number; _data?: { error?: string } }; data?: { error?: string } }
    if (e?.response?.status === 409) {
      priceSyncError.value = '已有爬取作業執行中'
      startPriceSyncPolling()
    } else if (e?.response?.status === 400) {
      priceSyncError.value = e?.response?._data?.error || e?.data?.error || '請先同步股票清單'
    } else {
      priceSyncError.value = e?.response?._data?.error || e?.data?.error || '啟動失敗，請查看後端日誌'
    }
  } finally {
    priceSyncTriggering.value = false
  }
}

const priceSyncProcessed = computed(() =>
  (priceSyncStatus.value?.success ?? 0) + (priceSyncStatus.value?.fail ?? 0)
)

const priceSyncProgressPct = computed(() => {
  const total = priceSyncStatus.value?.total ?? 0
  if (!total) return 0
  return Math.min(100, Math.round((priceSyncProcessed.value / total) * 100))
})

const priceSyncBadgeText = computed(() => {
  const s = priceSyncStatus.value?.status
  if (s === 'running') return '爬取中…'
  if (s === 'completed') return '已完成'
  if (s === 'failed') return '作業失敗'
  return '尚未執行'
})

const priceSyncSummaryText = computed(() => {
  const s = priceSyncStatus.value?.status
  const total = priceSyncStatus.value?.total ?? 0
  const success = priceSyncStatus.value?.success ?? 0
  const fail = priceSyncStatus.value?.fail ?? 0
  if (s === 'running') return priceSyncStatus.value?.message || `已處理 ${priceSyncProcessed.value} / ${total}`
  if (s === 'completed') return `共處理 ${total} 檔，成功 ${success}，失敗 ${fail}。`
  if (s === 'failed') return priceSyncStatus.value?.message || '作業中斷，請重新觸發。'
  return '可手動觸發一次全量歷史日K回填，爬完所有股票的全部月份資料。'
})

const priceSyncCompletedAt = computed(() => {
  if (!priceSyncStatus.value?.completed_at) return '—'
  return new Date(priceSyncStatus.value.completed_at).toLocaleString('zh-TW', {
    year: 'numeric', month: 'long', day: 'numeric', hour: '2-digit', minute: '2-digit',
  })
})

watch(
  () => priceSyncStatus.value?.status,
  (status) => {
    if (status === 'running') startPriceSyncPolling()
    else stopPriceSyncPolling()
  },
  { immediate: true },
)

// ── 單股測試 Modal ─────────────────────────────────────────────
interface TestStep {
  id: number
  text: string
  status: 'running' | 'done' | 'error' | 'info'
}

const testModalOpen    = ref(false)
const testModalType    = ref<'chips' | 'price' | null>(null)
const testModalSymbol  = ref('')
const testModalRunning = ref(false)
const testModalSteps   = ref<TestStep[]>([])
const testModalDone    = ref(false)
const logEl            = ref<HTMLElement | null>(null)
let _stepId = 0

function _addStep(text: string, status: TestStep['status'] = 'info'): number {
  const id = ++_stepId
  testModalSteps.value.push({ id, text, status })
  nextTick(() => { if (logEl.value) logEl.value.scrollTop = logEl.value.scrollHeight })
  return id
}

function _updateStep(id: number, text: string, status: TestStep['status']) {
  const s = testModalSteps.value.find(s => s.id === id)
  if (s) { s.text = text; s.status = status }
  nextTick(() => { if (logEl.value) logEl.value.scrollTop = logEl.value.scrollHeight })
}

function openTestModal(type: 'chips' | 'price') {
  testModalType.value    = type
  testModalSymbol.value  = ''
  testModalSteps.value   = []
  testModalRunning.value = false
  testModalDone.value    = false
  _stepId = 0
  testModalOpen.value    = true
}

function closeTestModal() { testModalOpen.value = false }

function _sleep(ms: number) { return new Promise<void>(r => setTimeout(r, ms)) }

async function runTest() {
  const sym = testModalSymbol.value.trim().toUpperCase()
  if (!sym || testModalRunning.value) return
  testModalRunning.value = true
  testModalDone.value    = false
  testModalSteps.value   = []
  _stepId = 0
  try {
    if (testModalType.value === 'chips') await _runChipsTest(sym)
    else                                  await _runPriceTest(sym)
  } finally {
    testModalRunning.value = false
    testModalDone.value    = true
  }
}

async function _runChipsTest(sym: string) {
  const s1 = _addStep('[ 1 / 5 ]  驗證代號格式', 'running')
  await _sleep(100)
  if (!/^[1-9][0-9]{3}$/.test(sym)) {
    _updateStep(s1, `[ 1 / 5 ]  ✕  「${sym}」格式不符：須為 4 碼普通股代號（首碼非 0）`, 'error')
    return
  }
  _updateStep(s1, `[ 1 / 5 ]  ✓  代號格式正確：${sym} 符合 ^[1-9][0-9]{3}$ 規則`, 'done')

  const s2 = _addStep('[ 2 / 5 ]  觸發爬取  POST /api/chips/trigger-single', 'running')
  try {
    await $fetch('/api/chips/trigger-single', { method: 'POST', body: { symbol: sym } })
    _updateStep(s2, `[ 2 / 5 ]  ✓  後端已接受，籌碼爬取任務已排入佇列`, 'done')
  } catch (err: unknown) {
    const e = err as { response?: { _data?: { error?: string } }; data?: { error?: string } }
    _updateStep(s2, `[ 2 / 5 ]  ✕  觸發失敗：${e?.response?._data?.error ?? e?.data?.error ?? '未知錯誤'}`, 'error')
    return
  }

  _addStep(`[ 3 / 5 ]  Playwright 爬蟲啟動：瀏覽 norway.twsthr.info 解析持股分布圖`, 'info')
  _addStep(`[ 4 / 5 ]  資料將寫入 PostgreSQL  ·  stock_holders 資料表`, 'info')
  const s5 = _addStep('[ 5 / 5 ]  監控爬取狀態（每 2s 輪詢 /api/chips/status）…', 'running')
  const t0 = Date.now()
  const maxWait = 120_000
  while (Date.now() - t0 < maxWait) {
    await _sleep(2000)
    try {
      const st = await $fetch<ChipsStatus>('/api/chips/status')
      const sec = Math.round((Date.now() - t0) / 1000)
      if (st.status === 'running') {
        _updateStep(s5, `[ 5 / 5 ]  爬取進行中（已等 ${sec}s）${st.message ? '  ·  ' + st.message : ''}`, 'running')
      } else if (st.status === 'completed') {
        _updateStep(s5, `[ 5 / 5 ]  ✓  爬取完成，耗時 ${sec}s  ·  成功 ${st.success ?? 0} 檔 / 失敗 ${st.fail ?? 0} 檔`, 'done')
        _addStep(`📋  資料已寫入 stock_holders 資料表`, 'info')
        _addStep(`📍  查閱：前往 /stocks/${sym}  →  滾動至「籌碼金字塔」區塊查看持股分布`, 'info')
        await refreshChips()
        startChipsPolling()
        return
      } else if (st.status === 'failed') {
        _updateStep(s5, `[ 5 / 5 ]  ✕  爬取失敗：${st.message ?? '未知原因'}`, 'error')
        _addStep(`💡  排查：docker logs scraper  —  查看 Playwright 錯誤訊息`, 'info')
        return
      }
    } catch { /* polling, keep trying */ }
  }
  _updateStep(s5, `[ 5 / 5 ]  ⚠  等待逾時（120s），任務可能仍在背景執行中`, 'error')
  _addStep(`💡  至主畫面「籌碼金字塔」卡確認狀態，或執行 docker logs scraper`, 'info')
}

async function _runPriceTest(sym: string) {
  const s1 = _addStep('[ 1 / 5 ]  驗證代號格式', 'running')
  await _sleep(100)
  if (!/^[1-9][0-9]{3}$/.test(sym)) {
    _updateStep(s1, `[ 1 / 5 ]  ✕  「${sym}」格式不符：須為 4 碼普通股代號（首碼非 0）`, 'error')
    return
  }
  _updateStep(s1, `[ 1 / 5 ]  ✓  代號格式正確：${sym} 符合 ^[1-9][0-9]{3}$ 規則`, 'done')

  _addStep(`[ 2 / 5 ]  確認股票存在於資料庫，辨識 TWSE（上市）或 TPEX（上櫃）市場`, 'info')
  _addStep(`[ 3 / 5 ]  依市場選擇 API 端點，逐月向 TWSE / TPEX OpenAPI 發出 GET 請求`, 'info')
  _addStep(`[ 4 / 5 ]  每月請求間隔 120ms，錯誤後 300ms，最多爬取 360 個月（~ 30 年歷史）`, 'info')
  const s5 = _addStep('[ 5 / 5 ]  POST /api/scraper/prices/all/test  →  同步執行中…', 'running')
  const t0 = Date.now()
  const timer = setInterval(() => {
    const sec = Math.round((Date.now() - t0) / 1000)
    _updateStep(s5, `[ 5 / 5 ]  後端逐月拉取中，已等待 ${sec}s（同步執行，請耐心等候）`, 'running')
  }, 3000)
  try {
    const res = await $fetch<{ ok: boolean; symbol: string; market: string; records: number }>(
      '/api/scraper/prices/all/test',
      { method: 'POST', body: { symbol: sym } },
    )
    clearInterval(timer)
    const sec = Math.round((Date.now() - t0) / 1000)
    _updateStep(s5, `[ 5 / 5 ]  ✓  拉取完成，耗時 ${sec}s  ·  市場：${res.market}`, 'done')
    _addStep(`📋  寫入 PostgreSQL  ·  daily_prices 資料表：共 ${res.records} 筆 OHLCV 記錄（日期 / 開高低收 / 成交量）`, 'done')
    _addStep(`📍  查閱 K 線：前往 /stocks/${sym}  →  K 線圖區塊  →  可查看完整歷史蠟燭圖與成交量`, 'info')
    _addStep(`📎  SQL 驗證：SELECT count(*), min(date), max(date) FROM daily_prices WHERE symbol='${sym}'`, 'info')
  } catch (err: unknown) {
    clearInterval(timer)
    const e = err as { response?: { _data?: { error?: string } }; data?: { error?: string } }
    _updateStep(s5, `[ 5 / 5 ]  ✕  請求失敗：${e?.response?._data?.error ?? e?.data?.error ?? '未知錯誤'}`, 'error')
    _addStep(`💡  排查：1) 確認 stocks 資料表有 ${sym}  2) docker logs backend 查看詳細錯誤`, 'info')
  }
}

const syncing = ref(false)
const syncState = ref<SyncState | null>(null)
const syncLabel = ref('')

function startSSE(url: string, label: string) {
  if (syncing.value) return
  syncing.value = true
  syncState.value = null
  syncLabel.value = label

  const es = new EventSource(url)

  es.onmessage = async (e) => {
    const data: SyncState = JSON.parse(e.data)
    syncState.value = data

    if (data.stage === 'done') {
      es.close()
      syncing.value = false
      await refresh()
    } else if (data.stage === 'error') {
      es.close()
      syncing.value = false
    }
  }

  es.onerror = () => {
    es.close()
    syncing.value = false
    if (!syncState.value || syncState.value.stage !== 'done') {
      syncState.value = {
        stage: 'error', message: '', progress: 0, url: '', total: 0, synced: 0,
        error: '連線失敗，請確認後端服務是否正常',
      }
    }
  }
}

function syncStocks() { startSSE('/api/scraper/stocks', 'stocks') }
function syncPrices() { startSSE('/api/scraper/prices', 'prices') }

const stockList = computed<Stock[]>(() =>
  Array.isArray(stocks.value) ? stocks.value : []
)

// K 線圖快速跳轉
const jumpSymbol = ref('')
const router = useRouter()

const jumpSuggestions = computed(() => {
  const q = jumpSymbol.value.trim().toLowerCase()
  if (!q) return []
  return stockList.value
    .filter(s => s.symbol.toLowerCase().startsWith(q) || s.name.toLowerCase().includes(q))
    .slice(0, 5)
})

function jumpToChart() {
  const q = jumpSymbol.value.trim().toUpperCase()
  if (!q) return
  // 優先精確比對；無結果時直接導航讓 stock 頁處理
  const exact = stockList.value.find(s => s.symbol === q)
  if (exact || jumpSuggestions.value.length === 0) {
    router.push(`/stocks/${q}`)
  } else {
    router.push(`/stocks/${jumpSuggestions.value[0]!.symbol}`)
  }
}

// 統計
const totalStocks = computed(() => stockList.value.length)
const lastSyncDisplay = computed(() => {
  if (!stockList.value.length) return '尚未同步'
  const latest = stockList.value
    .map(s => new Date(s.updated_at))
    .filter(d => !isNaN(d.getTime()))
    .reduce((a, b) => (a > b ? a : b), new Date(0))
  if (latest.getFullYear() < 2000) return '尚未同步'
  return latest.toLocaleDateString('zh-TW', { year: 'numeric', month: 'long', day: 'numeric' })
})

const today = new Date().toLocaleDateString('zh-TW', {
  year: 'numeric',
  month: 'long',
  day: 'numeric',
  weekday: 'long',
})

const { isDark, appStyle, isBento, isClassic, toggleTheme, setTheme, setStyle } = useAppPrefs()
const settingsOpen = ref(false)
</script>

<template>
  <div class="page" :class="{ light: !isDark, classic: isClassic }">

    <!-- ══ Header ══ -->
    <!-- ══ Bento / Terminal Header ══ -->
    <header v-if="!isClassic" class="header">
      <div class="header__inner">
        <div class="brand">
          <div class="brand-logo" aria-hidden="true">
            <svg width="26" height="26" viewBox="0 0 26 26" fill="none">
              <rect x="0"  y="0"  width="11" height="11" rx="2" fill="var(--blue)" />
              <rect x="15" y="0"  width="11" height="11" rx="2" fill="var(--blue)" opacity="0.45" />
              <rect x="0"  y="15" width="11" height="11" rx="2" fill="var(--gold)" opacity="0.55" />
              <rect x="15" y="15" width="11" height="11" rx="2" fill="var(--blue)" opacity="0.8" />
            </svg>
          </div>
          <div class="brand-info">
            <span class="brand-main">台股監控</span>
            <span class="brand-sub">Taiwan Stock Monitor</span>
          </div>
        </div>
        <nav class="header-nav">
          <span class="api-status" :class="status === 'error' ? 'api-status--err' : 'api-status--ok'">
            <span class="api-pip" />
            {{ status === 'error' ? 'API 離線' : status === 'pending' ? '連線中' : 'API 正常' }}
          </span>
          <span class="header-date">{{ today }}</span>

          <!-- Settings -->
          <div class="settings-wrap">
            <button class="btn-icon" aria-label="外觀設定" @click="settingsOpen = !settingsOpen">
              <svg width="15" height="15" viewBox="0 0 16 16" fill="none">
                <circle cx="8" cy="8" r="2.3" stroke="currentColor" stroke-width="1.4"/>
                <path d="M8 1v1.5M8 13.5V15M1 8h1.5M13.5 8H15M3.05 3.05l1.06 1.06M11.89 11.89l1.06 1.06M3.05 12.95l1.06-1.06M11.89 4.11l1.06-1.06" stroke="currentColor" stroke-width="1.4" stroke-linecap="round"/>
              </svg>
            </button>
            <div v-if="settingsOpen" class="settings-overlay" @click="settingsOpen = false" />
            <div v-if="settingsOpen" class="settings-panel">
              <p class="sp-title">外觀設定</p>
              <div class="sp-group">
                <p class="sp-label">主題</p>
                <div class="sp-btns">
                  <button class="sp-btn" :class="{ active: !isDark }" @click="setTheme(false)">
                    <svg width="12" height="12" viewBox="0 0 16 16" fill="none"><circle cx="8" cy="8" r="2.8" fill="currentColor"/><path d="M8 1.5V3M8 13v1.5M1.5 8H3M13 8h1.5M3.4 3.4l1.06 1.06M11.54 11.54l1.06 1.06M3.4 12.6l1.06-1.06M11.54 4.46l1.06-1.06" stroke="currentColor" stroke-width="1.4" stroke-linecap="round"/></svg>
                    亮色
                  </button>
                  <button class="sp-btn" :class="{ active: isDark }" @click="setTheme(true)">
                    <svg width="12" height="12" viewBox="0 0 16 16" fill="none"><path d="M13.2 9.3A5.8 5.8 0 0 1 6.7 2.8a.4.4 0 0 0-.46-.5A6.3 6.3 0 1 0 13.7 9.76a.4.4 0 0 0-.5-.46Z" fill="currentColor"/></svg>
                    暗色
                  </button>
                </div>
              </div>
              <div class="sp-group">
                <p class="sp-label">版面風格</p>
                <div class="sp-btns">
                  <button class="sp-btn" :class="{ active: isClassic }" @click="setStyle('classic')">Classic</button>
                  <button class="sp-btn" :class="{ active: isBento }" @click="setStyle('bento')">Bento</button>
                </div>
              </div>
            </div>
          </div>

          <!-- Theme toggle -->
          <button class="btn-icon" :aria-label="isDark ? '切換亮色模式' : '切換暗色模式'" @click="toggleTheme">
            <svg v-if="isDark" width="16" height="16" viewBox="0 0 16 16" fill="none">
              <circle cx="8" cy="8" r="2.8" fill="currentColor"/>
              <path d="M8 1.5V3M8 13v1.5M1.5 8H3M13 8h1.5M3.4 3.4l1.06 1.06M11.54 11.54l1.06 1.06M3.4 12.6l1.06-1.06M11.54 4.46l1.06-1.06" stroke="currentColor" stroke-width="1.4" stroke-linecap="round"/>
            </svg>
            <svg v-else width="16" height="16" viewBox="0 0 16 16" fill="none">
              <path d="M13.2 9.3A5.8 5.8 0 0 1 6.7 2.8a.4.4 0 0 0-.46-.5A6.3 6.3 0 1 0 13.7 9.76a.4.4 0 0 0-.5-.46Z" fill="currentColor"/>
            </svg>
          </button>
        </nav>
      </div>
    </header>

    <!-- ══ Classic Header ══ -->
    <header v-if="isClassic" class="classic-header">
      <div class="classic-header__inner">
        <div class="classic-brand">
          <span class="classic-badge">TSM</span>
          <div class="classic-brand-text">
            <span class="classic-brand-sub">Taiwan Stock Monitor</span>
            <span class="classic-brand-name">台股監控系統</span>
          </div>
        </div>
        <div class="classic-header-right">
          <span class="classic-api-status" :class="status === 'error' ? 'classic-api--err' : 'classic-api--ok'">
            <span class="classic-pip" />
            {{ status === 'error' ? 'API 離線' : status === 'pending' ? '連線中' : 'API 正常' }}
          </span>
          <span class="classic-date">「 {{ today }} 」</span>
          <div class="settings-wrap">
            <button class="classic-settings-btn" aria-label="外觀設定" @click="settingsOpen = !settingsOpen">⚙</button>
            <div v-if="settingsOpen" class="settings-overlay" @click="settingsOpen = false" />
            <div v-if="settingsOpen" class="settings-panel">
              <p class="sp-title">外觀設定</p>
              <div class="sp-group">
                <p class="sp-label">主題</p>
                <div class="sp-btns">
                  <button class="sp-btn" :class="{ active: !isDark }" @click="setTheme(false)">亮色</button>
                  <button class="sp-btn" :class="{ active: isDark }" @click="setTheme(true)">暗色</button>
                </div>
              </div>
              <div class="sp-group">
                <p class="sp-label">版面風格</p>
                <div class="sp-btns">
                  <button class="sp-btn" :class="{ active: isClassic }" @click="setStyle('classic')">Classic</button>
                  <button class="sp-btn" :class="{ active: isBento }" @click="setStyle('bento')">Bento</button>
                </div>
              </div>
            </div>
          </div>
          <button class="classic-toggle-btn" :aria-label="isDark ? '切換亮色' : '切換暗色'" @click="toggleTheme">
            <span v-if="isDark">☀</span><span v-else>☾</span>
          </button>
        </div>
      </div>
    </header>

    <!-- ══ Sync Bar ══ -->
    <Transition name="slide-down">
      <div v-if="syncState" class="sync-bar" :class="`sync-bar--${syncState.stage}`">
        <div class="sync-bar__inner">
          <span class="sync-bar__icon">
            <span v-if="syncState.stage === 'error'" class="sb-x">✕</span>
            <span v-else-if="syncState.stage === 'done'" class="sb-ok">✓</span>
            <span v-else class="sb-spin">◌</span>
          </span>
          <span class="sync-bar__msg">{{ syncState.stage === 'error' ? syncState.error : syncState.message }}</span>
          <div v-if="syncState.stage !== 'error'" class="sync-bar__track">
            <div class="sync-bar__fill" :style="{ width: syncState.progress + '%' }" />
          </div>
          <span v-if="syncState.stage !== 'error'" class="sync-bar__pct">{{ syncState.progress }}%</span>
          <a v-if="syncState.url" :href="syncState.url" target="_blank" rel="noopener" class="sync-bar__url">{{ syncState.url }}</a>
        </div>
      </div>
    </Transition>

    <!-- ══ Classic Portal ══ -->
    <section v-if="isClassic" class="portal">
      <div class="cards-grid">

        <!-- Card 1: Overview (2 cols) -->
        <article class="card card--overview">
          <p class="card-eyebrow">Database Overview</p>
          <div class="overview-body">
            <div class="overview-left">
              <span class="big-num">{{ totalStocks > 0 ? totalStocks.toLocaleString() : '—' }}</span>
              <span class="big-label">上市上櫃股票</span>
            </div>
            <div class="overview-right">
              <div class="meta-row">
                <span class="meta-key">最後同步</span>
                <span class="meta-val">{{ lastSyncDisplay }}</span>
              </div>
              <div class="meta-row">
                <span class="meta-key">資料來源</span>
                <span class="meta-val">TWSE · TPEX</span>
              </div>
            </div>
          </div>
          <NuxtLink to="/stocks" class="ghost-btn">
            瀏覽完整列表 <span class="ghost-btn__arr">→</span>
          </NuxtLink>
        </article>

        <!-- Card 2: Status (2 cols) -->
        <article class="card card--status">
          <p class="card-eyebrow">System Status</p>
          <ul class="status-list">
            <li class="status-item">
              <span class="pip" :class="status !== 'error' ? 'pip--ok' : 'pip--err'" />
              <span class="status-name">API 連線</span>
              <span class="status-val">{{ status === 'error' ? '失敗' : status === 'pending' ? '連線中…' : '正常運作' }}</span>
            </li>
            <li class="status-item">
              <span class="pip" :class="totalStocks > 0 ? 'pip--ok' : 'pip--warn'" />
              <span class="status-name">資料庫</span>
              <span class="status-val">{{ totalStocks > 0 ? `${totalStocks.toLocaleString()} 筆` : '空白' }}</span>
            </li>
            <li class="status-item">
              <span class="pip" :class="syncing ? 'pip--busy' : 'pip--idle'" />
              <span class="status-name">同步作業</span>
              <span class="status-val">{{ syncing ? '進行中…' : '閒置' }}</span>
            </li>
          </ul>
        </article>

        <!-- Card 3: Sync Stocks (1 col) -->
        <article class="card card--action">
          <p class="card-eyebrow">Data Sync</p>
          <h2 class="card-title">同步股票清單</h2>
          <p class="card-desc">從 TWSE 及 TPEX 抓取最新上市、上櫃股票名冊，更新本地資料庫。</p>
          <button
            class="action-btn"
            :class="{ 'action-btn--busy': syncing && syncLabel === 'stocks' }"
            :disabled="syncing"
            @click="syncStocks"
          >{{ syncing && syncLabel === 'stocks' ? '同步中…' : '立即同步' }}</button>
        </article>

        <!-- Card 4: Sync Prices (1 col) -->
        <article class="card card--action">
          <p class="card-eyebrow">Data Sync</p>
          <h2 class="card-title">同步日 K 資料</h2>
          <p class="card-desc">批次更新全部股票之歷史日 K 價量資料，作為技術分析基礎。</p>
          <button
            class="action-btn"
            :class="{ 'action-btn--busy': syncing && syncLabel === 'prices' }"
            :disabled="syncing"
            @click="syncPrices"
          >{{ syncing && syncLabel === 'prices' ? '同步中…' : '立即同步' }}</button>
        </article>

        <!-- Card 5: K-Chart Lookup (2 cols) -->
        <article class="card card--lookup">
          <p class="card-eyebrow">Chart Analysis</p>
          <h2 class="card-title">K 線圖查詢</h2>
          <p class="card-desc">輸入股票代號查看個股蠟燭圖與成交量走勢。</p>
          <div class="lookup-wrap">
            <div class="lookup-field" :class="{ 'lookup-field--active': jumpSymbol }">
              <input
                v-model="jumpSymbol"
                class="lookup-input"
                type="text"
                placeholder="輸入代號，如 2330"
                @keyup.enter="jumpToChart"
              />
              <button class="lookup-go" @click="jumpToChart">前往</button>
            </div>
            <ul v-if="jumpSuggestions.length > 0 && jumpSymbol" class="suggestions">
              <li
                v-for="s in jumpSuggestions"
                :key="s.symbol"
                class="suggestion"
                @click="router.push(`/stocks/${s.symbol}`)"
              >
                <span class="sug-sym">{{ s.symbol }}</span>
                <span class="sug-name">{{ s.name }}</span>
              </li>
            </ul>
          </div>
        </article>

        <!-- Card 6: Chips Pyramid (2 cols) -->
        <article class="card card--chips">
          <p class="card-eyebrow">Chips Pyramid · 籌碼金字塔</p>
          <div class="chips-body">
            <div class="chips-fresh-badge" :class="chipsBadgeClass">
              <span class="pip pip--lg" :class="chipsStatus?.is_fresh ? 'pip--ok' : (chipsStatus?.status === 'running' ? 'pip--busy' : chipsStatus?.status === 'failed' ? 'pip--fail' : 'pip--warn')" />
              <span>{{ chipsBadgeText }}</span>
            </div>
            <div class="chips-summary" :class="chipsSummaryClass">
              <p class="chips-summary__title">{{ chipsSummaryTitle }}</p>
              <p class="chips-summary__text">{{ chipsSummaryText }}</p>
            </div>
            <div class="chips-meta">
              <div class="meta-row">
                <span class="meta-key">上次爬取</span>
                <span class="meta-val">{{ chipsLastSync }}</span>
              </div>
              <div v-if="chipsStatus && chipsStatus.status !== 'never'" class="meta-row">
                <span class="meta-key">成功 / 總計</span>
                <span class="meta-val">{{ chipsStatus.success ?? 0 }} / {{ chipsStatus.total ?? 0 }}</span>
              </div>
              <div v-if="chipsStatus && chipsStatus.status !== 'never'" class="meta-row">
                <span class="meta-key">目前進度</span>
                <span class="meta-val">{{ chipsProgressLabel }}（{{ chipsProgressPct }}%）</span>
              </div>
              <div class="meta-row">
                <span class="meta-key">下次排程</span>
                <span class="meta-val">{{ chipsNextRun }}（週日自動）</span>
              </div>
              <div v-if="chipsStatus && chipsStatus.status !== 'running' && chipsStatus.status !== 'never'" class="meta-row">
                <span class="meta-key">完成時間</span>
                <span class="meta-val">{{ chipsCompletedAt }}</span>
              </div>
            </div>
            <div v-if="chipsStatus?.status === 'running'" class="chips-progress">
              <div class="chips-progress__track">
                <div class="chips-progress__fill" :style="{ width: `${chipsProgressPct}%` }" />
              </div>
              <p class="chips-progress__text">{{ chipsStatus?.message || '爬取進行中…' }}</p>
            </div>
            <div v-else-if="chipsStatus?.status === 'completed'" class="chips-result chips-result--done">
              <span class="chips-result__label">完成摘要</span>
              <span class="chips-result__value">成功 {{ chipsStatus?.success ?? 0 }} 檔，失敗 {{ chipsStatus?.fail ?? 0 }} 檔</span>
            </div>
            <!-- 失敗明細區：Completed 且有失敗時顯示 -->
            <div v-if="chipsStatus?.status === 'completed' && (chipsStatus?.fail ?? 0) > 0" class="chips-result chips-result--warn">
              <span class="chips-result__label">失敗明細</span>
              <span class="chips-result__value" style="white-space: pre-wrap; font-size: 0.8em; line-height: 1.6">{{
                chipsStatus?.message?.includes('失敗範例')
                  ? chipsStatus.message.split('\n\n').slice(1).join('\n\n').trim()
                  : `常見原因：ETF/權證/特別股不在來源網站追蹤範圍內，屬正常現象。`
              }}</span>
            </div>
            <div v-else-if="chipsStatus?.status === 'failed'" class="chips-result chips-result--fail">
              <span class="chips-result__label">失敗原因</span>
              <span class="chips-result__value">{{ chipsFailureDetail }}</span>
            </div>
            <div v-if="chipsShouldShowSecondaryActions" class="chips-actions">
              <button
                v-if="chipsCanRetry"
                class="chips-secondary chips-secondary--retry"
                :disabled="chipsTriggering || chipsStatus?.status === 'running'"
                @click="triggerChips"
              >
                再試一次
              </button>
              <NuxtLink to="/stocks" class="chips-secondary chips-secondary--link">
                查看股票列表
              </NuxtLink>
            </div>
          </div>
          <div class="test-sep">
            <button class="test-open-btn" @click="openTestModal('chips')">
              <svg width="9" height="10" viewBox="0 0 9 10" fill="none" aria-hidden="true"><path d="M1 1l7 4-7 4V1Z" fill="currentColor"/></svg>
              單股測試
            </button>
          </div>
          <button
            class="action-btn"
            :class="{ 'action-btn--busy': chipsTriggering || chipsStatus?.status === 'running' }"
            :disabled="chipsTriggering || chipsStatus?.status === 'running'"
            @click="triggerChips"
          >
            {{ chipsTriggering || chipsStatus?.status === 'running' ? '爬取中…' : '手動觸發爬取' }}
          </button>
          <p v-if="chipsError" class="chips-err">⚠ {{ chipsError }}</p>
        </article>

        <!-- Card 7: Price Sync All (2 cols) -->
        <article class="card card--chips">
          <p class="card-eyebrow">Full History Sync · 全量日K回填</p>
          <div class="chips-body">
            <div class="chips-fresh-badge" :class="{
              'chips-fresh-badge--ok':    priceSyncStatus?.status === 'completed',
              'chips-fresh-badge--stale': priceSyncStatus?.status === 'running',
              'chips-fresh-badge--fail':  priceSyncStatus?.status === 'failed',
            }">
              <span class="pip pip--lg" :class="{
                'pip--ok':   priceSyncStatus?.status === 'completed',
                'pip--busy': priceSyncStatus?.status === 'running',
                'pip--fail': priceSyncStatus?.status === 'failed',
                'pip--warn': !priceSyncStatus || priceSyncStatus?.status === 'never',
              }" />
              <span>{{ priceSyncBadgeText }}</span>
            </div>
            <div class="chips-summary">
              <p class="chips-summary__title">
                <template v-if="priceSyncStatus?.status === 'running'">正在回填所有股票歷史日K</template>
                <template v-else-if="priceSyncStatus?.status === 'completed'">上次全量回填已完成</template>
                <template v-else-if="priceSyncStatus?.status === 'failed'">上次全量回填失敗</template>
                <template v-else>尚未執行全量日K回填</template>
              </p>
              <p class="chips-summary__text">{{ priceSyncSummaryText }}</p>
            </div>
            <div class="chips-meta">
              <template v-if="priceSyncStatus && priceSyncStatus.status !== 'never'">
                <div class="meta-row">
                  <span class="meta-key">成功 / 總計</span>
                  <span class="meta-val">{{ priceSyncStatus.success ?? 0 }} / {{ priceSyncStatus.total ?? 0 }}</span>
                </div>
                <div class="meta-row">
                  <span class="meta-key">目前進度</span>
                  <span class="meta-val">{{ priceSyncProcessed }} / {{ priceSyncStatus.total ?? 0 }}（{{ priceSyncProgressPct }}%）</span>
                </div>
              </template>
              <div v-if="priceSyncStatus && priceSyncStatus.status !== 'running' && priceSyncStatus.status !== 'never'" class="meta-row">
                <span class="meta-key">完成時間</span>
                <span class="meta-val">{{ priceSyncCompletedAt }}</span>
              </div>
            </div>
            <div v-if="priceSyncStatus?.status === 'running'" class="chips-progress">
              <div class="chips-progress__track">
                <div class="chips-progress__fill" :style="{ width: `${priceSyncProgressPct}%` }" />
              </div>
              <p class="chips-progress__text">{{ priceSyncStatus?.message || '爬取進行中…' }}</p>
            </div>
          </div>
          <div class="test-sep">
            <button class="test-open-btn" @click="openTestModal('price')">
              <svg width="9" height="10" viewBox="0 0 9 10" fill="none" aria-hidden="true"><path d="M1 1l7 4-7 4V1Z" fill="currentColor"/></svg>
              單股測試
            </button>
          </div>
          <button
            class="action-btn"
            :class="{ 'action-btn--busy': priceSyncTriggering || priceSyncStatus?.status === 'running' }"
            :disabled="priceSyncTriggering || priceSyncStatus?.status === 'running'"
            @click="triggerPriceSync"
          >{{ priceSyncTriggering || priceSyncStatus?.status === 'running' ? '回填中…' : '全量歷史回填' }}</button>
          <p v-if="priceSyncError" class="chips-err">⚠ {{ priceSyncError }}</p>
        </article>

      </div>
    </section>

    <!-- ══ Main ══ -->
    <main v-else class="main">
      <div class="bento">

        <!-- ── Overview (2 cols) ── -->
        <article class="card card-overview">
          <p class="eyebrow">Database Overview</p>
          <div class="overview-body">
            <div class="num-block">
              <span class="big-num">{{ totalStocks > 0 ? totalStocks.toLocaleString() : '—' }}</span>
            </div>
            <span class="num-label">上市上櫃股票</span>
          </div>
          <div class="meta-stack">
            <div class="meta-row">
              <span class="mkey">最後同步</span>
              <span class="mval">{{ lastSyncDisplay }}</span>
            </div>
            <div class="meta-row">
              <span class="mkey">資料來源</span>
              <span class="mval">TWSE · TPEX</span>
            </div>
          </div>
          <NuxtLink to="/stocks" class="link-btn">
            瀏覽完整列表
            <svg width="14" height="14" viewBox="0 0 14 14" fill="none" aria-hidden="true">
              <path d="M2 7h10M8.5 3.5l4 3.5-4 3.5" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round"/>
            </svg>
          </NuxtLink>
        </article>

        <!-- ── System Status (1 col) ── -->
        <article class="card card-status">
          <p class="eyebrow">System Status</p>
          <ul class="status-list">
            <li class="status-item">
              <span class="pip" :class="status !== 'error' ? 'pip-ok' : 'pip-err'" />
              <span class="si-name">API 連線</span>
              <span class="si-val">{{ status === 'error' ? '失敗' : status === 'pending' ? '連線中…' : '正常' }}</span>
            </li>
            <li class="status-item">
              <span class="pip" :class="totalStocks > 0 ? 'pip-ok' : 'pip-warn'" />
              <span class="si-name">資料庫</span>
              <span class="si-val">{{ totalStocks > 0 ? `${totalStocks.toLocaleString()} 筆` : '空白' }}</span>
            </li>
            <li class="status-item">
              <span class="pip" :class="syncing ? 'pip-busy' : 'pip-idle'" />
              <span class="si-name">同步作業</span>
              <span class="si-val">{{ syncing ? '進行中' : '閒置' }}</span>
            </li>
          </ul>
        </article>

        <!-- ── K-Chart Lookup (1 col, featured) ── -->
        <article class="card card-jump">
          <p class="eyebrow eyebrow-blue">Chart Analysis</p>
          <h2 class="card-title">K 線圖查詢</h2>
          <p class="card-desc">輸入股票代號，直接前往 K 線蠟燭圖與成交量分析頁。</p>
          <div class="jump-wrap">
            <div class="jump-field" :class="{ active: jumpSymbol }">
              <input
                v-model="jumpSymbol"
                class="jump-input"
                type="text"
                placeholder="代號，如 2330"
                autocomplete="off"
                @keyup.enter="jumpToChart"
              />
              <button class="jump-go" aria-label="前往查詢" @click="jumpToChart">
                <svg width="16" height="16" viewBox="0 0 16 16" fill="none" aria-hidden="true">
                  <path d="M2 8h12M10 4l4 4-4 4" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"/>
                </svg>
              </button>
            </div>
            <ul v-if="jumpSuggestions.length > 0 && jumpSymbol" class="suggestions">
              <li
                v-for="s in jumpSuggestions"
                :key="s.symbol"
                class="suggestion"
                @click="router.push(`/stocks/${s.symbol}`)"
              >
                <span class="sug-sym">{{ s.symbol }}</span>
                <span class="sug-name">{{ s.name }}</span>
              </li>
            </ul>
          </div>
        </article>

        <!-- ── Sync Stocks (1 col) ── -->
        <article class="card card-sync">
          <p class="eyebrow">Data Sync</p>
          <h2 class="card-title">同步股票清單</h2>
          <p class="card-desc">從 TWSE 及 TPEX 抓取最新上市、上櫃股票名冊，更新本地資料庫。</p>
          <button
            class="action-btn"
            :class="{ busy: syncing && syncLabel === 'stocks' }"
            :disabled="syncing"
            @click="syncStocks"
          >{{ syncing && syncLabel === 'stocks' ? '同步中…' : '立即同步' }}</button>
        </article>

        <!-- ── Sync Prices (1 col) ── -->
        <article class="card card-sync">
          <p class="eyebrow">Data Sync</p>
          <h2 class="card-title">同步日 K 資料</h2>
          <p class="card-desc">批次更新全部股票之歷史日 K 價量資料，作為技術分析基礎。</p>
          <button
            class="action-btn"
            :class="{ busy: syncing && syncLabel === 'prices' }"
            :disabled="syncing"
            @click="syncPrices"
          >{{ syncing && syncLabel === 'prices' ? '同步中…' : '立即同步' }}</button>
        </article>

        <!-- ── Chips Pyramid (2 cols) ── -->
        <article class="card card-chips">
          <p class="eyebrow">Chips Pyramid · 籌碼金字塔</p>

          <div class="chips-badge" :class="chipsBadgeClass">
            <span class="pip pip-lg" :class="{
              'pip-ok':   chipsStatus?.is_fresh,
              'pip-busy': chipsStatus?.status === 'running',
              'pip-err':  chipsStatus?.status === 'failed',
              'pip-warn': !chipsStatus?.is_fresh && chipsStatus?.status !== 'running' && chipsStatus?.status !== 'failed'
            }" />
            {{ chipsBadgeText }}
          </div>

          <div class="chips-summary" :class="chipsSummaryClass">
            <p class="cs-title">{{ chipsSummaryTitle }}</p>
            <p class="cs-text">{{ chipsSummaryText }}</p>
          </div>

          <div class="chips-meta">
            <div class="meta-row">
              <span class="mkey">上次爬取</span>
              <span class="mval">{{ chipsLastSync }}</span>
            </div>
            <template v-if="chipsStatus && chipsStatus.status !== 'never'">
              <div class="meta-row">
                <span class="mkey">成功 / 總計</span>
                <span class="mval">{{ chipsStatus.success ?? 0 }} / {{ chipsStatus.total ?? 0 }}</span>
              </div>
              <div class="meta-row">
                <span class="mkey">目前進度</span>
                <span class="mval">{{ chipsProgressLabel }}（{{ chipsProgressPct }}%）</span>
              </div>
            </template>
            <div class="meta-row">
              <span class="mkey">下次排程</span>
              <span class="mval">{{ chipsNextRun }}（週日自動）</span>
            </div>
            <div v-if="chipsStatus && chipsStatus.status !== 'running' && chipsStatus.status !== 'never'" class="meta-row">
              <span class="mkey">完成時間</span>
              <span class="mval">{{ chipsCompletedAt }}</span>
            </div>
          </div>

          <div v-if="chipsStatus?.status === 'running'" class="chips-progress">
            <div class="cp-track">
              <div class="cp-fill" :style="{ width: `${chipsProgressPct}%` }" />
            </div>
            <p class="cp-text">{{ chipsStatus?.message || '爬取進行中…' }}</p>
          </div>

          <div v-else-if="chipsStatus?.status === 'completed'" class="chips-result chips-result-ok">
            <span class="cr-label">完成摘要</span>
            <span class="cr-value">成功 {{ chipsStatus?.success ?? 0 }} 檔，失敗 {{ chipsStatus?.fail ?? 0 }} 檔</span>
          </div>
          <!-- 失敗明細：Completed 且有失敗時顯示 -->
          <div v-if="chipsStatus?.status === 'completed' && (chipsStatus?.fail ?? 0) > 0" class="chips-result chips-result-warn">
            <span class="cr-label">失敗明細</span>
            <span class="cr-value" style="white-space: pre-wrap; font-size: 0.8em; line-height: 1.6">{{
              chipsStatus?.message?.includes('失敗範例')
                ? chipsStatus.message.split('\n\n').slice(1).join('\n\n').trim()
                : `常見原因：ETF/權證/特別股不在來源網站追蹤範圍內，屬正常現象。`
            }}</span>
          </div>
          <div v-else-if="chipsStatus?.status === 'failed'" class="chips-result chips-result-fail">
            <span class="cr-label">失敗原因</span>
            <span class="cr-value">{{ chipsFailureDetail }}</span>
          </div>

          <div class="test-sep">
            <button class="test-open-btn" @click="openTestModal('chips')">
              <svg width="9" height="10" viewBox="0 0 9 10" fill="none" aria-hidden="true"><path d="M1 1l7 4-7 4V1Z" fill="currentColor"/></svg>
              單股測試
            </button>
          </div>

          <div class="chips-actions">
            <button
              class="action-btn"
              :class="{ busy: chipsTriggering || chipsStatus?.status === 'running' }"
              :disabled="chipsTriggering || chipsStatus?.status === 'running'"
              @click="triggerChips"
            >{{ chipsTriggering || chipsStatus?.status === 'running' ? '爬取中…' : '手動觸發爬取' }}</button>

            <div v-if="chipsShouldShowSecondaryActions" class="sec-row">
              <button
                v-if="chipsCanRetry"
                class="sec-btn sec-retry"
                :disabled="chipsTriggering || chipsStatus?.status === 'running'"
                @click="triggerChips"
              >再試一次</button>
              <NuxtLink to="/stocks" class="sec-btn sec-link">查看股票列表</NuxtLink>
            </div>
          </div>

          <p v-if="chipsError" class="chips-err">{{ chipsError }}</p>
        </article>

        <!-- ── Price Sync All (2 cols) ── -->
        <article class="card card-chips">
          <p class="eyebrow">Full History Sync · 全量日K回填</p>

          <div class="chips-badge" :class="{
            'chips-fresh-badge--ok':   priceSyncStatus?.status === 'completed',
            'chips-fresh-badge--stale': priceSyncStatus?.status === 'running',
            'chips-fresh-badge--fail': priceSyncStatus?.status === 'failed',
          }">
            <span class="pip pip-lg" :class="{
              'pip-ok':   priceSyncStatus?.status === 'completed',
              'pip-busy': priceSyncStatus?.status === 'running',
              'pip-err':  priceSyncStatus?.status === 'failed',
              'pip-warn': !priceSyncStatus || priceSyncStatus?.status === 'never',
            }" />
            {{ priceSyncBadgeText }}
          </div>

          <div class="chips-summary">
            <p class="cs-title">
              <template v-if="priceSyncStatus?.status === 'running'">正在回填所有股票歷史日K</template>
              <template v-else-if="priceSyncStatus?.status === 'completed'">上次全量回填已完成</template>
              <template v-else-if="priceSyncStatus?.status === 'failed'">上次全量回填失敗</template>
              <template v-else>尚未執行全量日K回填</template>
            </p>
            <p class="cs-text">{{ priceSyncSummaryText }}</p>
          </div>

          <div class="chips-meta">
            <template v-if="priceSyncStatus && priceSyncStatus.status !== 'never'">
              <div class="meta-row">
                <span class="mkey">成功 / 總計</span>
                <span class="mval">{{ priceSyncStatus.success ?? 0 }} / {{ priceSyncStatus.total ?? 0 }}</span>
              </div>
              <div class="meta-row">
                <span class="mkey">目前進度</span>
                <span class="mval">{{ priceSyncProcessed }} / {{ priceSyncStatus.total ?? 0 }}（{{ priceSyncProgressPct }}%）</span>
              </div>
            </template>
            <div v-if="priceSyncStatus && priceSyncStatus.status !== 'running' && priceSyncStatus.status !== 'never'" class="meta-row">
              <span class="mkey">完成時間</span>
              <span class="mval">{{ priceSyncCompletedAt }}</span>
            </div>
          </div>

          <div v-if="priceSyncStatus?.status === 'running'" class="chips-progress">
            <div class="cp-track">
              <div class="cp-fill" :style="{ width: `${priceSyncProgressPct}%` }" />
            </div>
            <p class="cp-text">{{ priceSyncStatus?.message || '爬取進行中…' }}</p>
          </div>

          <div class="test-sep">
            <button class="test-open-btn" @click="openTestModal('price')">
              <svg width="9" height="10" viewBox="0 0 9 10" fill="none" aria-hidden="true"><path d="M1 1l7 4-7 4V1Z" fill="currentColor"/></svg>
              單股測試
            </button>
          </div>

          <div class="chips-actions">
            <button
              class="action-btn"
              :class="{ busy: priceSyncTriggering || priceSyncStatus?.status === 'running' }"
              :disabled="priceSyncTriggering || priceSyncStatus?.status === 'running'"
              @click="triggerPriceSync"
            >{{ priceSyncTriggering || priceSyncStatus?.status === 'running' ? '回填中…' : '全量歷史回填' }}</button>
            <div v-if="priceSyncStatus?.status === 'completed' || priceSyncStatus?.status === 'failed'" class="sec-row">
              <button
                class="sec-btn sec-retry"
                :disabled="priceSyncTriggering || priceSyncStatus?.status === 'running'"
                @click="triggerPriceSync"
              >再次執行</button>
            </div>
          </div>

          <p v-if="priceSyncError" class="chips-err">{{ priceSyncError }}</p>
        </article>

      </div>
    </main>

    <!-- ══ 單股測試 Modal ══ -->
    <div v-if="testModalOpen" class="tm-overlay" @click.self="closeTestModal">
      <div class="tm-panel" role="dialog" aria-modal="true">
        <div class="tm-header">
          <span class="tm-badge">TEST</span>
          <span class="tm-title">{{ testModalType === 'chips' ? '籌碼金字塔 · 單股測試' : '全量日K · 單股測試' }}</span>
          <button class="tm-close" aria-label="關閉" @click="closeTestModal">
            <svg width="12" height="12" viewBox="0 0 14 14" fill="none"><path d="M1 1l12 12M13 1L1 13" stroke="currentColor" stroke-width="1.8" stroke-linecap="round"/></svg>
          </button>
        </div>
        <div class="tm-input-row">
          <span class="tm-prompt">$</span>
          <input
            v-model="testModalSymbol"
            class="tm-input"
            type="text"
            placeholder="輸入股票代號，如 2330"
            maxlength="6"
            autocomplete="off"
            spellcheck="false"
            :disabled="testModalRunning"
            @keyup.enter="runTest"
          />
          <button class="tm-run-btn" :disabled="testModalRunning || !testModalSymbol" @click="runTest">
            <span v-if="testModalRunning" class="tm-spin">◌</span>
            <template v-else>▶ 開始</template>
          </button>
        </div>
        <div ref="logEl" class="tm-log">
          <div v-if="!testModalSteps.length" class="tm-empty">
            輸入四碼股票代號後點擊「▶ 開始」，可查看完整逐步執行過程。
          </div>
          <div
            v-for="step in testModalSteps"
            :key="step.id"
            class="tm-line"
            :class="`tm-line--${step.status}`"
          >
            <span class="tm-icon">
              <span v-if="step.status === 'running'" class="tm-spin">◌</span>
              <span v-else-if="step.status === 'done'">✓</span>
              <span v-else-if="step.status === 'error'">✕</span>
              <span v-else class="tm-dot">·</span>
            </span>
            <span class="tm-text">{{ step.text }}</span>
          </div>
        </div>
        <div class="tm-footer">
          <button class="tm-btn-cancel" @click="closeTestModal">{{ testModalDone ? '關閉' : '取消' }}</button>
          <NuxtLink
            v-if="testModalDone && testModalSymbol"
            :to="`/stocks/${testModalSymbol.trim().toUpperCase()}`"
            class="tm-btn-go"
            @click="closeTestModal"
          >
            前往查看 {{ testModalSymbol.trim().toUpperCase() }} →
          </NuxtLink>
        </div>
      </div>
    </div>

  </div>
</template>

<style scoped>
/* ═══════════════════════════════════════
   Design Tokens
═══════════════════════════════════════ */
.page {
  /* Dark (default — OLED-inspired) */
  --bg:    oklch(9.5%  0.018 256);
  --s1:    oklch(13%   0.020 257);
  --s2:    oklch(16.5% 0.022 258);
  --s3:    oklch(21%   0.024 258);
  --line:  oklch(22%   0.023 258);
  --line2: oklch(33%   0.023 258);

  --blue:  oklch(63%   0.20  264);
  --blue2: oklch(42%   0.17  264);
  --gold:  oklch(76%   0.13  82);

  --t1:    oklch(96%   0.006 218);
  --t2:    oklch(72%   0.013 240);
  --t3:    oklch(50%   0.012 240);

  --up:    oklch(62%   0.18  22);
  --dn:    oklch(64%   0.18  148);
  --warn:  oklch(73%   0.13  72);

  --radius: 18px;
  --font:   'DM Sans', system-ui, 'PingFang TC', 'Microsoft JhengHei', sans-serif;
  --mono:   'Fira Code', 'JetBrains Mono', ui-monospace, monospace;

  min-height: 100vh;
  background: var(--bg);
  color: var(--t1);
  font-family: var(--font);
  font-size: 15px;
  line-height: 1.55;
  -webkit-font-smoothing: antialiased;
}

.page *, .page *::before, .page *::after { box-sizing: border-box; margin: 0; padding: 0; }

/* Light Mode */
.page.light {
  --bg:    oklch(96.5% 0.009 220);
  --s1:    oklch(100%  0     0);
  --s2:    oklch(97%   0.010 220);
  --s3:    oklch(92%   0.014 220);
  --line:  oklch(88%   0.012 220);
  --line2: oklch(72%   0.015 240);
  --blue:  oklch(47%   0.21  264);
  --blue2: oklch(36%   0.17  264);
  --gold:  oklch(52%   0.16  72);
  --t1:    oklch(10%   0.018 256);
  --t2:    oklch(35%   0.016 240);
  --t3:    oklch(57%   0.012 240);
  --up:    oklch(44%   0.22  22);
  --dn:    oklch(38%   0.20  148);
  --warn:  oklch(50%   0.16  72);
}

/* Classic Mode — Original Dark tokens */
.page.classic {
  --bg:    oklch(14.5% 0.016 258);
  --s1:    oklch(19%   0.018 258);
  --s2:    oklch(23%   0.018 258);
  --s3:    oklch(27%   0.020 258);
  --line:  oklch(28%   0.020 258);
  --line2: oklch(36%   0.020 258);
  --blue:  oklch(56%   0.20  264);
  --blue2: oklch(40%   0.17  264);
  --gold:  oklch(76%   0.095 80);
  --t1:    oklch(97%   0.006 82);
  --t2:    oklch(78%   0.012 258);
  --t3:    oklch(58%   0.014 258);
  --up:    oklch(59%   0.18  22);
  --dn:    oklch(62%   0.17  148);
  --warn:  oklch(72%   0.13  72);
}

.page.classic.light {
  --bg:    oklch(96.5% 0.007 82);
  --s1:    oklch(93%   0.008 82);
  --s2:    oklch(99%   0.004 82);
  --s3:    oklch(90%   0.007 82);
  --line:  oklch(84%   0.012 258);
  --line2: oklch(68%   0.015 258);
  --blue:  oklch(44%   0.21  264);
  --blue2: oklch(34%   0.17  264);
  --gold:  oklch(48%   0.13  60);
  --t1:    oklch(13%   0.020 258);
  --t2:    oklch(34%   0.016 258);
  --t3:    oklch(54%   0.014 258);
  --up:    oklch(44%   0.21  22);
  --dn:    oklch(38%   0.19  148);
  --warn:  oklch(52%   0.14  72);
}

/* ═══════════════════════════════════════
   Header
═══════════════════════════════════════ */
.header {
  position: sticky;
  top: 0;
  z-index: 50;
  background: color-mix(in oklch, var(--s1) 85%, transparent);
  backdrop-filter: blur(16px);
  -webkit-backdrop-filter: blur(16px);
  border-bottom: 1px solid var(--line);
}

.header__inner {
  max-width: 1240px;
  margin: 0 auto;
  padding: 0 32px;
  height: 56px;
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.brand {
  display: flex;
  align-items: center;
  gap: 12px;
}

.brand-logo { flex-shrink: 0; line-height: 0; }

.brand-info {
  display: flex;
  flex-direction: column;
  gap: 1px;
}

.brand-main {
  font-size: 16px;
  font-weight: 700;
  letter-spacing: -0.01em;
  color: var(--t1);
  line-height: 1;
}

.brand-sub {
  font-size: 10px;
  letter-spacing: 0.14em;
  text-transform: uppercase;
  color: var(--t3);
  line-height: 1;
}

.header-nav {
  display: flex;
  align-items: center;
  gap: 20px;
}

.api-status {
  display: flex;
  align-items: center;
  gap: 7px;
  font-size: 12.5px;
  color: var(--t2);
}

.api-pip {
  width: 7px;
  height: 7px;
  border-radius: 50%;
  flex-shrink: 0;
  background: var(--t3);
}

.api-status--ok .api-pip  { background: var(--dn); box-shadow: 0 0 6px color-mix(in oklch, var(--dn) 70%, transparent); }
.api-status--err .api-pip { background: var(--up); box-shadow: 0 0 6px color-mix(in oklch, var(--up) 70%, transparent); }

.header-date {
  font-size: 12px;
  color: var(--t3);
  font-variant-numeric: tabular-nums;
}

.btn-icon {
  width: 34px;
  height: 34px;
  display: flex;
  align-items: center;
  justify-content: center;
  background: var(--s2);
  border: 1px solid var(--line);
  border-radius: 8px;
  color: var(--t2);
  cursor: pointer;
  transition: background 0.2s, border-color 0.2s, color 0.2s;
  flex-shrink: 0;
}
.btn-icon:hover { background: var(--s3); border-color: var(--line2); color: var(--t1); }

/* ═══════════════════════════════════════
   Sync Bar
═══════════════════════════════════════ */
.sync-bar {
  background: var(--s1);
  border-bottom: 1px solid var(--line);
  border-left: 3px solid var(--gold);
}
.sync-bar--error { border-left-color: var(--up); }
.sync-bar--done  { border-left-color: var(--dn); }

.sync-bar__inner {
  max-width: 1240px;
  margin: 0 auto;
  padding: 10px 32px;
  display: flex;
  align-items: center;
  gap: 12px;
  font-size: 12.5px;
  color: var(--t2);
}

.sync-bar__icon { font-size: 11px; font-weight: 700; flex-shrink: 0; }
.sb-ok  { color: var(--dn); }
.sb-x   { color: var(--up); }
.sb-spin { display: inline-block; animation: spin 1.4s linear infinite; }

.sync-bar__msg { flex-shrink: 0; }

.sync-bar__track {
  flex: 1;
  max-width: 200px;
  height: 3px;
  background: var(--line);
  border-radius: 2px;
  overflow: hidden;
}

.sync-bar__fill {
  height: 100%;
  background: linear-gradient(90deg, var(--blue), var(--gold));
  border-radius: 2px;
  transition: width 0.35s cubic-bezier(0.25, 1, 0.5, 1);
}
.sync-bar--done .sync-bar__fill { background: var(--dn); }

.sync-bar__pct {
  font-size: 11.5px;
  font-family: var(--mono);
  font-variant-numeric: tabular-nums;
  color: var(--t3);
  min-width: 34px;
}

.sync-bar__url {
  font-size: 11px;
  color: var(--t3);
  text-decoration: none;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  max-width: 240px;
}
.sync-bar__url:hover { color: var(--blue); }

@keyframes spin { to { transform: rotate(360deg); } }

.slide-down-enter-active,
.slide-down-leave-active {
  transition: opacity 0.2s ease, transform 0.2s ease;
}
.slide-down-enter-from,
.slide-down-leave-to { opacity: 0; transform: translateY(-6px); }

/* ═══════════════════════════════════════
   Main / Bento Grid
═══════════════════════════════════════ */
.main {
  max-width: 1240px;
  margin: 0 auto;
  padding: 24px 32px 48px;
}

.bento {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 14px;
}

/* ═══════════════════════════════════════
   Base Card
═══════════════════════════════════════ */
.card {
  background: var(--s1);
  border: 1px solid var(--line);
  border-radius: var(--radius);
  padding: 24px 26px;
  display: flex;
  flex-direction: column;
  gap: 0;
  position: relative;
  overflow: hidden;
  transition: border-color 0.25s, box-shadow 0.25s;
}

.card:hover {
  border-color: var(--line2);
  box-shadow: 0 4px 24px color-mix(in oklch, var(--bg) 30%, transparent);
}

/* Card column spans */
.card-overview { grid-column: span 2; min-height: 230px; }
.card-status   { grid-column: span 1; }
.card-jump     { grid-column: span 1; position: relative; }
.card-sync     { grid-column: span 1; min-height: 200px; }
.card-chips    { grid-column: span 2; }

/* ─ Overview ─ */

.overview-body {
  flex: 1;
  display: flex;
  flex-direction: column;
  gap: 6px;
  padding: 16px 0 20px;
}

.num-block { position: relative; display: inline-flex; }

.big-num {
  font-family: var(--mono);
  font-size: clamp(52px, 5.8vw, 76px);
  font-weight: 700;
  letter-spacing: -0.04em;
  line-height: 0.9;
  color: var(--t1);
  font-variant-numeric: tabular-nums;
  position: relative;
  z-index: 1;
}



.num-label {
  font-size: 11.5px;
  letter-spacing: 0.12em;
  text-transform: uppercase;
  color: var(--t3);
  font-weight: 500;
}

/* ─ Jump card featured ─ */
.card-jump {
  border-color: color-mix(in oklch, var(--blue) 28%, var(--line));
}

.card-jump:hover {
  border-color: color-mix(in oklch, var(--blue) 50%, var(--line));
  box-shadow: 0 4px 20px color-mix(in oklch, var(--blue) 10%, transparent);
}

/* ─ Eyebrow ─ */
.eyebrow {
  font-size: 10.5px;
  font-weight: 600;
  letter-spacing: 0.16em;
  text-transform: uppercase;
  color: var(--t3);
  margin-bottom: 12px;
}

.eyebrow-blue { color: var(--blue); }

/* ─ Card Title / Desc ─ */
.card-title {
  font-size: 19px;
  font-weight: 700;
  letter-spacing: -0.02em;
  color: var(--t1);
  margin-bottom: 9px;
  line-height: 1.2;
}

.card-desc {
  font-size: 13.5px;
  color: var(--t2);
  line-height: 1.7;
  flex: 1;
  margin-bottom: 18px;
}

/* ─ Meta rows ─ */
.meta-stack {
  display: flex;
  flex-direction: column;
  gap: 10px;
  margin-bottom: 20px;
}

.chips-meta {
  display: flex;
  flex-direction: column;
  gap: 8px;
  margin-bottom: 14px;
}

.meta-row {
  display: flex;
  align-items: baseline;
  gap: 10px;
}

.mkey {
  font-size: 11px;
  letter-spacing: 0.12em;
  text-transform: uppercase;
  color: var(--t3);
  flex-shrink: 0;
  min-width: 64px;
}

.mval {
  font-size: 13.5px;
  color: var(--t2);
  font-variant-numeric: tabular-nums;
}

/* ─ Link button ─ */
.link-btn {
  align-self: flex-start;
  display: inline-flex;
  align-items: center;
  gap: 7px;
  font-family: var(--font);
  font-size: 13px;
  font-weight: 600;
  letter-spacing: 0.03em;
  color: var(--gold);
  text-decoration: none;
  transition: color 0.2s, gap 0.2s;
  margin-top: auto;
}

.link-btn:hover { color: var(--t1); gap: 10px; }

/* ─ Status card ─ */
.status-list {
  list-style: none;
  flex: 1;
  display: flex;
  flex-direction: column;
}

.status-item {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 12px 0;
  border-bottom: 1px solid var(--line);
}
.status-item:last-child { border-bottom: none; }

.pip {
  width: 7px;
  height: 7px;
  border-radius: 50%;
  flex-shrink: 0;
  transition: box-shadow 0.3s;
}

.pip-ok   { background: var(--dn);   box-shadow: 0 0 7px color-mix(in oklch, var(--dn)   65%, transparent); }
.pip-err  { background: var(--up);   box-shadow: 0 0 7px color-mix(in oklch, var(--up)   65%, transparent); }
.pip-warn { background: var(--warn); box-shadow: 0 0 7px color-mix(in oklch, var(--warn) 60%, transparent); }
.pip-busy { background: var(--gold); animation: pulse 1.5s ease-in-out infinite; }
.pip-idle { background: var(--line2); }
.pip-lg   { width: 9px; height: 9px; }

@keyframes pulse { 0%, 100% { opacity: 1; } 50% { opacity: 0.25; } }

.si-name { flex: 1; font-size: 14px; color: var(--t2); }

.si-val {
  font-size: 12.5px;
  color: var(--t3);
  font-family: var(--mono);
  font-variant-numeric: tabular-nums;
}

/* ─ Jump field ─ */
.jump-wrap { position: relative; margin-top: auto; }

.jump-field {
  display: flex;
  border: 1px solid var(--line2);
  border-radius: 10px;
  overflow: hidden;
  transition: border-color 0.2s, box-shadow 0.2s;
}

.jump-field:focus-within,
.jump-field.active {
  border-color: var(--blue);
  box-shadow: 0 0 0 3px color-mix(in oklch, var(--blue) 18%, transparent);
}

.jump-input {
  flex: 1;
  padding: 11px 14px;
  background: transparent;
  border: none;
  outline: none;
  font-family: var(--mono);
  font-size: 14.5px;
  letter-spacing: 0.04em;
  color: var(--t1);
  font-variant-numeric: tabular-nums;
}
.jump-input::placeholder { color: var(--t3); font-family: var(--font); letter-spacing: 0; }

.jump-go {
  padding: 0 16px;
  background: var(--blue);
  color: oklch(98% 0.01 220);
  border: none;
  cursor: pointer;
  display: flex;
  align-items: center;
  transition: background 0.2s;
  flex-shrink: 0;
}
.jump-go:hover { background: color-mix(in oklch, var(--blue) 80%, var(--t1)); }

.suggestions {
  position: absolute;
  top: calc(100% + 6px);
  left: 0;
  right: 0;
  background: var(--s2);
  border: 1px solid var(--line2);
  border-radius: 10px;
  list-style: none;
  z-index: 20;
  overflow: hidden;
  box-shadow: 0 8px 24px color-mix(in oklch, var(--bg) 50%, transparent);
}

.suggestion {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 10px 14px;
  cursor: pointer;
  border-bottom: 1px solid var(--line);
  transition: background 0.15s;
}
.suggestion:last-child { border-bottom: none; }
.suggestion:hover { background: var(--s3); }

.sug-sym {
  font-family: var(--mono);
  font-weight: 600;
  font-size: 13.5px;
  min-width: 48px;
  color: var(--blue);
  font-variant-numeric: tabular-nums;
}
.sug-name { font-size: 13px; color: var(--t2); }

/* ─ Action button ─ */
.action-btn {
  align-self: flex-start;
  font-family: var(--font);
  font-size: 13px;
  font-weight: 600;
  letter-spacing: 0.04em;
  padding: 10px 22px;
  background: transparent;
  color: var(--t1);
  border: 1px solid var(--line2);
  border-radius: 9px;
  cursor: pointer;
  transition: background 0.2s, border-color 0.2s, color 0.2s, box-shadow 0.2s;
  margin-top: auto;
}

.action-btn:hover:not(:disabled):not(.busy) {
  background: var(--gold);
  border-color: var(--gold);
  color: oklch(10% 0.02 80);
  box-shadow: 0 2px 12px color-mix(in oklch, var(--gold) 35%, transparent);
}

.action-btn:disabled,
.action-btn.busy { opacity: 0.35; cursor: not-allowed; }

/* ─ Chips card ─ */
.chips-badge {
  display: flex;
  align-items: center;
  gap: 9px;
  padding: 9px 14px;
  border: 1px solid var(--line);
  border-radius: 9px;
  font-size: 13px;
  font-weight: 600;
  margin-bottom: 12px;
}

/* map to old classes used in :class binding */
.chips-fresh-badge--ok    { border-color: var(--dn);    color: var(--dn); }
.chips-fresh-badge--done  { border-color: color-mix(in oklch, var(--dn) 65%, var(--gold)); color: var(--dn); }
.chips-fresh-badge--fail  { border-color: var(--up);    color: var(--up); }
.chips-fresh-badge--stale { border-color: var(--line2); color: var(--t2); }

.chips-summary {
  display: flex;
  flex-direction: column;
  gap: 5px;
  padding: 12px 14px;
  background: color-mix(in oklch, var(--s2) 80%, transparent);
  border-left: 2.5px solid var(--gold);
  border-radius: 0 6px 6px 0;
  margin-bottom: 14px;
}

.chips-summary--done {
  background: color-mix(in oklch, var(--dn) 8%, var(--s2));
  border-left-color: var(--dn);
}
.chips-summary--fail {
  background: color-mix(in oklch, var(--up) 8%, var(--s2));
  border-left-color: var(--up);
}

.cs-title {
  font-size: 14px;
  font-weight: 600;
  letter-spacing: -0.01em;
  color: var(--t1);
}

.cs-text {
  font-size: 12.5px;
  color: var(--t2);
  line-height: 1.6;
}

/* Chips progress */
.chips-progress {
  display: flex;
  flex-direction: column;
  gap: 7px;
  margin-bottom: 12px;
}

.cp-track {
  width: 100%;
  height: 5px;
  background: var(--line);
  border-radius: 3px;
  overflow: hidden;
}

.cp-fill {
  height: 100%;
  background: linear-gradient(90deg, var(--blue), var(--gold));
  border-radius: 3px;
  transition: width 0.35s cubic-bezier(0.25, 1, 0.5, 1);
}

.cp-text {
  font-size: 12px;
  color: var(--t2);
  line-height: 1.5;
}

/* Chips result */
.chips-result {
  display: flex;
  flex-direction: column;
  gap: 5px;
  padding: 11px 14px;
  border: 1px solid var(--line);
  border-radius: 8px;
  margin-bottom: 12px;
}

.chips-result-ok   { border-color: color-mix(in oklch, var(--dn) 50%, var(--line)); background: color-mix(in oklch, var(--dn) 8%, var(--s2)); }
.chips-result-fail { border-color: color-mix(in oklch, var(--up) 50%, var(--line)); background: color-mix(in oklch, var(--up) 8%, var(--s2)); }

.cr-label {
  font-size: 10.5px;
  font-weight: 600;
  letter-spacing: 0.12em;
  text-transform: uppercase;
  color: var(--t3);
}

.cr-value {
  font-size: 13px;
  color: var(--t1);
  line-height: 1.55;
}

/* Chips actions */
.chips-actions {
  display: flex;
  flex-direction: column;
  gap: 10px;
  margin-top: auto;
}

.sec-row {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.sec-btn {
  min-height: 32px;
  padding: 0 12px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  font-family: var(--font);
  font-size: 12px;
  font-weight: 600;
  letter-spacing: 0.04em;
  text-decoration: none;
  cursor: pointer;
  border-radius: 7px;
  transition: all 0.2s;
}

.sec-retry {
  border: 1px solid color-mix(in oklch, var(--dn) 55%, var(--line));
  background: color-mix(in oklch, var(--dn) 8%, var(--s1));
  color: var(--dn);
}
.sec-retry:hover:not(:disabled) {
  border-color: var(--dn);
  background: color-mix(in oklch, var(--dn) 14%, var(--s1));
}
.sec-retry:disabled { opacity: 0.4; cursor: default; }

.sec-link {
  border: 1px solid var(--line2);
  background: transparent;
  color: var(--t2);
}
.sec-link:hover { border-color: var(--gold); color: var(--gold); }

.chips-err {
  font-size: 12px;
  color: var(--up);
  margin-top: 4px;
  line-height: 1.5;
}

/* ═══════════════════════════════════════
   Responsive
═══════════════════════════════════════ */
@media (max-width: 1024px) {
  .header__inner,
  .sync-bar__inner,
  .main { padding-left: 20px; padding-right: 20px; }

  .header-date { display: none; }

  .bento { grid-template-columns: repeat(2, 1fr); }
  .card-overview { grid-column: span 2; }
  .card-chips    { grid-column: span 2; }

  .big-num { font-size: 52px; }
}

@media (max-width: 600px) {
  .bento { grid-template-columns: 1fr; gap: 10px; }
  .card-overview,
  .card-chips { grid-column: span 1; }

  .main { padding: 16px 16px 32px; }

  .card { padding: 18px 18px; }

  .big-num { font-size: 44px; }

  .sync-bar__track,
  .sync-bar__url { display: none; }
}

/* ═══════════════════════════════════════
   Settings Panel
═══════════════════════════════════════ */
.settings-wrap { position: relative; }

.settings-overlay {
  position: fixed;
  inset: 0;
  z-index: 99;
}

.settings-panel {
  position: absolute;
  top: calc(100% + 8px);
  right: 0;
  z-index: 100;
  background: var(--s2);
  border: 1px solid var(--line2);
  border-radius: 12px;
  padding: 16px;
  min-width: 196px;
  box-shadow: 0 8px 32px oklch(0% 0 0 / 0.28);
}

.sp-title {
  font-size: 10.5px;
  font-weight: 700;
  letter-spacing: 0.14em;
  text-transform: uppercase;
  color: var(--t3);
  margin-bottom: 12px;
}

.sp-group { margin-bottom: 12px; }
.sp-group:last-child { margin-bottom: 0; }

.sp-label {
  font-size: 10.5px;
  letter-spacing: 0.10em;
  text-transform: uppercase;
  color: var(--t3);
  margin-bottom: 6px;
}

.sp-btns { display: flex; gap: 6px; }

.sp-btn {
  flex: 1;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 5px;
  font-family: var(--font);
  font-size: 12px;
  font-weight: 600;
  padding: 7px 8px;
  background: transparent;
  border: 1px solid var(--line2);
  border-radius: 7px;
  color: var(--t2);
  cursor: pointer;
  transition: all 0.15s;
  white-space: nowrap;
}
.sp-btn:hover { border-color: var(--t2); color: var(--t1); }
.sp-btn.active {
  background: var(--blue);
  border-color: var(--blue);
  color: oklch(97% 0.01 220);
}

/* ── Classic structural overrides ───────────────────────────── */
.page.classic .bento { gap: 2px; background: var(--line); border-radius: 4px; }
.page.classic .card { border-radius: 4px; box-shadow: none; border-color: var(--line); }
.page.classic .card:hover { box-shadow: none; border-color: var(--line2); }
.page.classic .card-jump { border-radius: 4px; }
.page.classic .jump-field { border-radius: 4px; }
.page.classic .jump-go { border-radius: 0 4px 4px 0; }
.page.classic .action-btn { border-radius: 4px; }
.page.classic .sync-bar__track { border-radius: 4px; }
.page.classic .sync-bar__fill { border-radius: 4px; }

/* ── Classic Header ─────────────────────────────────────────── */
.classic-header {
  background: var(--s1);
  border-bottom: 1px solid var(--line);
  position: sticky;
  top: 0;
  z-index: 50;
}
.classic-header__inner {
  max-width: 1200px;
  margin: 0 auto;
  padding: 0 40px;
  height: 54px;
  display: flex;
  align-items: center;
  justify-content: space-between;
}
.classic-brand { display: flex; align-items: center; gap: 14px; }
.classic-badge {
  font-size: 11px;
  font-weight: 700;
  letter-spacing: 0.14em;
  color: var(--bg);
  background: var(--gold);
  padding: 5px 8px;
  line-height: 1;
  flex-shrink: 0;
}
.classic-brand-text { display: flex; flex-direction: column; gap: 2px; }
.classic-brand-sub { font-size: 10px; letter-spacing: 0.18em; text-transform: uppercase; color: var(--t3); line-height: 1; }
.classic-brand-name { font-size: 16px; font-weight: 600; letter-spacing: 0.02em; color: var(--t1); line-height: 1; }
.classic-header-right { display: flex; align-items: center; gap: 16px; }
.classic-api-status { display: flex; align-items: center; gap: 6px; font-size: 12px; }
.classic-api--ok  .classic-pip { background: var(--dn); }
.classic-api--err .classic-pip { background: var(--up); }
.classic-pip { width: 7px; height: 7px; border-radius: 50%; flex-shrink: 0; }
.classic-date { font-size: 12px; color: var(--t3); font-variant-numeric: tabular-nums; }
.classic-settings-btn {
  background: none;
  border: 1px solid var(--line2);
  color: var(--t2);
  font-size: 14px;
  width: 28px;
  height: 28px;
  display: flex;
  align-items: center;
  justify-content: center;
  cursor: pointer;
  transition: border-color 0.15s, color 0.15s;
}
.classic-settings-btn:hover { border-color: var(--gold); color: var(--gold); }
.classic-toggle-btn {
  background: none;
  border: 1px solid var(--line2);
  color: var(--t2);
  font-size: 15px;
  width: 32px;
  height: 32px;
  display: flex;
  align-items: center;
  justify-content: center;
  cursor: pointer;
  transition: border-color 0.15s, color 0.15s;
}
.classic-toggle-btn:hover { border-color: var(--gold); color: var(--gold); }

/* ════════════════════════════════════════
   Classic Portal  ·  (v-if="isClassic")
   ════════════════════════════════════════ */

.portal {
  max-width: 1200px;
  margin: 0 auto;
  padding: 28px 40px 0;
}

.cards-grid {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 1px;
  background: var(--line);
  border: 1px solid var(--line);
}

/* reset bento card styles inside portal */
.page.classic .portal .card {
  background: var(--s2);
  padding: 24px 28px;
  display: flex;
  flex-direction: column;
  border-radius: 0 !important;
  box-shadow: none !important;
  border: none;
}
.page.classic .portal .card:hover {
  box-shadow: none !important;
  border: none;
  transform: none;
}

.card--overview { grid-column: span 2; min-height: 220px; }
.card--status   { grid-column: span 2; }
.card--action   { grid-column: span 1; min-height: 200px; }
.card--lookup   { grid-column: span 2; position: relative; }
.card--chips    { grid-column: span 2; }

/* ─ Card Eyebrow / Title / Desc ─ */
.card-eyebrow {
  font-size: 11px;
  font-weight: 600;
  letter-spacing: 0.16em;
  text-transform: uppercase;
  color: var(--t3);
  margin-bottom: 14px;
}

.card-title {
  font-size: 20px;
  font-weight: 600;
  letter-spacing: -0.01em;
  color: var(--t1);
  margin-bottom: 10px;
}

.card-desc {
  font-size: 14.5px;
  color: var(--t2);
  line-height: 1.7;
  flex: 1;
  margin-bottom: 20px;
}

/* ─ Overview card ─ */
.overview-body {
  display: flex;
  align-items: flex-end;
  gap: 40px;
  flex: 1;
  padding-bottom: 24px;
}

.overview-left { display: flex; flex-direction: column; gap: 7px; }

.big-num {
  font-size: clamp(52px, 5.5vw, 72px);
  font-weight: 700;
  letter-spacing: -0.05em;
  line-height: 0.88;
  font-variant-numeric: tabular-nums;
  color: var(--t1);
}

.big-label {
  font-size: 12px;
  letter-spacing: 0.10em;
  text-transform: uppercase;
  color: var(--t3);
}

.overview-right {
  display: flex;
  flex-direction: column;
  gap: 16px;
  padding-bottom: 4px;
}

/* override bento .meta-row which is horizontal */
.page.classic .portal .meta-row {
  display: flex;
  flex-direction: column;
  gap: 3px;
}

.meta-key {
  font-size: 11px;
  letter-spacing: 0.14em;
  text-transform: uppercase;
  color: var(--t3);
}

.meta-val {
  font-size: 15px;
  color: var(--t2);
  font-variant-numeric: tabular-nums;
}

/* ─ Ghost button ─ */
.ghost-btn {
  align-self: flex-start;
  font-family: var(--font);
  font-size: 13px;
  font-weight: 600;
  letter-spacing: 0.06em;
  color: var(--gold);
  background: none;
  border: none;
  cursor: pointer;
  padding: 0;
  display: inline-flex;
  align-items: center;
  gap: 6px;
  transition: color 0.15s;
  text-decoration: none;
}
.ghost-btn:hover { color: var(--t1); }
.ghost-btn:hover .ghost-btn__arr { transform: translateX(3px); }
.ghost-btn__arr {
  font-size: 13px;
  transition: transform 0.2s cubic-bezier(0.25, 1, 0.5, 1);
}

/* ─ Status card ─ (names unique to classic, no conflict) */
.status-name { flex: 1; color: var(--t2); font-size: 14.5px; }
.status-val  { font-size: 13px; color: var(--t3); font-variant-numeric: tabular-nums; }

/* ─ Pip double-hyphen variants (unique to classic portal) ─ */
.pip--ok   { background: var(--dn); }
.pip--err  { background: var(--up); }
.pip--warn { background: var(--warn); }
.pip--busy { background: var(--gold); animation: pulse 1.5s ease-in-out infinite; }
.pip--idle { background: var(--line2); }
.pip--fail { background: var(--up); }
.pip--lg   { width: 8px; height: 8px; }

/* ─ Lookup card ─ */
.lookup-wrap { position: relative; }

.lookup-field {
  display: flex;
  border: 1px solid var(--line2);
  transition: border-color 0.15s;
}
.lookup-field:focus-within,
.lookup-field--active { border-color: var(--t1); }

.lookup-input {
  flex: 1;
  padding: 11px 14px;
  background: transparent;
  border: none;
  outline: none;
  font-family: var(--font);
  font-size: 15px;
  color: var(--t1);
  font-variant-numeric: tabular-nums;
}
.lookup-input::placeholder { color: var(--t3); }

.lookup-go {
  padding: 11px 20px;
  background: var(--gold);
  color: var(--bg);
  border: none;
  cursor: pointer;
  font-family: var(--font);
  font-size: 13px;
  font-weight: 700;
  letter-spacing: 0.04em;
  transition: background 0.15s;
}
.lookup-go:hover { background: var(--t1); }

.suggestions {
  position: absolute;
  top: 100%;
  left: 0;
  right: 0;
  background: var(--s1);
  border: 1px solid var(--line2);
  border-top: none;
  list-style: none;
  z-index: 20;
}

.suggestion {
  display: flex;
  align-items: center;
  gap: 14px;
  padding: 11px 14px;
  cursor: pointer;
  font-size: 14.5px;
  border-bottom: 1px solid var(--line);
  transition: background 0.1s;
}
.suggestion:last-child { border-bottom: none; }
.suggestion:hover { background: var(--line); }

.sug-sym  { font-weight: 700; font-variant-numeric: tabular-nums; min-width: 50px; color: var(--gold); }
.sug-name { color: var(--t2); }

/* ─ Chips card ─ */
.chips-body {
  display: flex;
  flex-direction: column;
  gap: 14px;
  margin-bottom: 16px;
}

.chips-fresh-badge {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 10px 14px;
  border: 1px solid var(--line);
  font-size: 13.5px;
  font-weight: 600;
}
.chips-fresh-badge--ok    { border-color: var(--dn);    color: var(--dn); }
.chips-fresh-badge--done  { border-color: color-mix(in oklch, var(--dn) 70%, var(--gold)); color: var(--dn); }
.chips-fresh-badge--fail  { border-color: var(--up);    color: var(--up); }
.chips-fresh-badge--stale { border-color: var(--line2); color: var(--t2); }

.chips-summary {
  display: grid;
  gap: 5px;
  padding: 12px 14px;
  background: color-mix(in oklch, var(--s1) 76%, transparent);
  border-left: 2px solid var(--gold);
}
.chips-summary--done { background: color-mix(in oklch, var(--dn) 10%, var(--s1)); border-left-color: var(--dn); }
.chips-summary--fail { background: color-mix(in oklch, var(--up) 10%, var(--s1)); border-left-color: var(--up); }
.chips-summary__title { font-size: 14px; font-weight: 600; letter-spacing: -0.01em; color: var(--t1); }
.chips-summary__text  { font-size: 12.5px; color: var(--t2); line-height: 1.55; }

.chips-meta { display: flex; flex-direction: column; gap: 6px; }

.chips-progress {
  display: flex;
  flex-direction: column;
  gap: 8px;
}
.chips-progress__track {
  width: 100%;
  height: 6px;
  background: var(--line);
  overflow: hidden;
}
.chips-progress__fill {
  height: 100%;
  background: linear-gradient(90deg, var(--gold), var(--dn));
  transition: width 0.35s cubic-bezier(0.25, 1, 0.5, 1);
}
.chips-progress__text { font-size: 12px; color: var(--t2); line-height: 1.5; }

.chips-result {
  display: grid;
  gap: 6px;
  padding: 12px 14px;
  border: 1px solid var(--line);
}
.chips-result--done { border-color: color-mix(in oklch, var(--dn) 55%, var(--line)); background: color-mix(in oklch, var(--dn) 9%, var(--s1)); }
.chips-result--fail { border-color: color-mix(in oklch, var(--up) 55%, var(--line)); background: color-mix(in oklch, var(--up) 9%, var(--s1)); }
.chips-result__label { font-size: 11px; font-weight: 600; letter-spacing: 0.12em; text-transform: uppercase; color: var(--t3); }
.chips-result__value { font-size: 12.5px; color: var(--t1); line-height: 1.55; }

.chips-actions { display: flex; flex-wrap: wrap; gap: 10px; }

.chips-secondary {
  min-height: 34px;
  padding: 0 12px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  font-family: var(--font);
  font-size: 12px;
  font-weight: 600;
  letter-spacing: 0.05em;
  text-decoration: none;
  cursor: pointer;
  transition: border-color 0.15s, color 0.15s, background 0.15s;
}
.chips-secondary--retry {
  border: 1px solid color-mix(in oklch, var(--dn) 60%, var(--line));
  background: color-mix(in oklch, var(--dn) 9%, var(--s1));
  color: var(--dn);
}
.chips-secondary--retry:hover:not(:disabled) { border-color: var(--dn); background: color-mix(in oklch, var(--dn) 14%, var(--s1)); }
.chips-secondary--retry:disabled { opacity: 0.45; cursor: default; }
.chips-secondary--link { border: 1px solid var(--line2); color: var(--t2); background: transparent; }
.chips-secondary--link:hover { border-color: var(--gold); color: var(--gold); }

.chips-err { font-size: 12px; color: var(--up); margin-top: 8px; line-height: 1.5; }

/* ─ Classic Portal RWD ─ */
@media (max-width: 960px) {
  .portal { padding: 16px 16px 0; }
  .cards-grid { grid-template-columns: repeat(2, 1fr); }
  .card--overview,
  .card--status,
  .card--lookup,
  .card--chips  { grid-column: span 2; }
  .card--action { grid-column: span 1; }
  .page.classic .portal .card { padding: 18px; }
  .big-num { font-size: 48px; }
  .overview-body { flex-direction: column; align-items: flex-start; gap: 16px; padding-bottom: 16px; }
}

@media (max-width: 520px) {
  .cards-grid { grid-template-columns: 1fr; }
  .card--overview,
  .card--status,
  .card--lookup,
  .card--chips,
  .card--action { grid-column: span 1; }
  .page.classic .portal .card { padding: 16px; }
  .card--action { min-height: unset; }
  .big-num { font-size: 40px; }
}

/* ═══════════════════════════════════════
   Test Open Button
═══════════════════════════════════════ */
.test-open-btn {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  height: 28px;
  padding: 0 12px;
  background: transparent;
  border: 1px dashed var(--line2);
  border-radius: 7px;
  color: var(--t3);
  font-size: 11.5px;
  cursor: pointer;
  transition: color 0.18s, border-color 0.18s, background 0.18s, border-style 0.18s;
  /* 與其他按鈕區隔——上方用算區随分隔線，下方等內部間距 */
  margin-top: 0;
  align-self: flex-start;
}
.test-open-btn:hover {
  color: var(--blue);
  border-color: var(--blue);
  border-style: solid;
  background: color-mix(in oklch, var(--blue) 8%, transparent);
}

/* 分隔線：放在 test-open-btn 上方，透過 wrapper div.test-sep 實現 */
.test-sep {
  display: flex;
  flex-direction: column;
  gap: 0;
  margin-top: 14px;
  padding-top: 14px;
  border-top: 1px solid var(--line);
}

/* ═══════════════════════════════════════
   Test Modal
═══════════════════════════════════════ */
.tm-overlay {
  position: fixed;
  inset: 0;
  z-index: 200;
  background: color-mix(in oklch, var(--bg) 72%, transparent);
  backdrop-filter: blur(6px);
  -webkit-backdrop-filter: blur(6px);
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 24px;
  animation: overlay-in 0.16s ease;
}
@keyframes overlay-in {
  from { opacity: 0; }
  to   { opacity: 1; }
}

.tm-panel {
  width: 100%;
  max-width: 600px;
  max-height: 88vh;
  display: flex;
  flex-direction: column;
  background: var(--s1);
  border: 1px solid var(--line2);
  border-radius: 16px;
  overflow: hidden;
  box-shadow: 0 32px 96px color-mix(in oklch, oklch(0% 0 0) 40%, transparent);
  animation: panel-in 0.18s cubic-bezier(0.34, 1.28, 0.64, 1);
}
@keyframes panel-in {
  from { opacity: 0; transform: scale(0.96) translateY(-10px); }
  to   { opacity: 1; transform: none; }
}

.tm-header {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 14px 18px;
  border-bottom: 1px solid var(--line);
  flex-shrink: 0;
  background: var(--bg);
}
.tm-badge {
  padding: 2px 8px;
  background: color-mix(in oklch, var(--gold) 12%, transparent);
  border: 1px solid color-mix(in oklch, var(--gold) 45%, transparent);
  border-radius: 4px;
  font-size: 10px;
  font-family: var(--mono);
  font-weight: 700;
  letter-spacing: 0.1em;
  color: var(--gold);
  flex-shrink: 0;
}
.tm-title {
  flex: 1;
  font-size: 13.5px;
  font-weight: 600;
  color: var(--t1);
  letter-spacing: -0.01em;
}
.tm-close {
  width: 28px; height: 28px;
  display: flex; align-items: center; justify-content: center;
  background: transparent;
  border: 1px solid var(--line);
  border-radius: 7px;
  color: var(--t3);
  cursor: pointer;
  transition: background 0.15s, border-color 0.15s, color 0.15s;
}
.tm-close:hover { background: var(--s2); border-color: var(--line2); color: var(--t1); }

.tm-input-row {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 10px 16px;
  border-bottom: 1px solid var(--line);
  flex-shrink: 0;
  background: var(--s2);
}
.tm-prompt {
  font-family: var(--mono);
  font-size: 16px;
  color: var(--blue);
  flex-shrink: 0;
  user-select: none;
}
.tm-input {
  flex: 1;
  height: 34px;
  padding: 0 12px;
  background: var(--bg);
  border: 1px solid var(--line);
  border-radius: 8px;
  color: var(--t1);
  font-size: 15px;
  font-family: var(--mono);
  letter-spacing: 0.05em;
  outline: none;
  transition: border-color 0.2s;
}
.tm-input:focus { border-color: var(--blue); }
.tm-input::placeholder { color: var(--t3); font-family: var(--font); font-size: 13px; letter-spacing: 0; }
.tm-input:disabled { opacity: 0.45; }
.tm-run-btn {
  height: 34px;
  padding: 0 18px;
  background: var(--blue);
  border: none;
  border-radius: 8px;
  color: oklch(97% 0.005 220);
  font-size: 13px;
  font-weight: 600;
  cursor: pointer;
  white-space: nowrap;
  display: flex;
  align-items: center;
  gap: 5px;
  transition: opacity 0.15s;
}
.tm-run-btn:hover:not(:disabled) { opacity: 0.82; }
.tm-run-btn:disabled { opacity: 0.35; cursor: not-allowed; }

.tm-log {
  flex: 1;
  overflow-y: auto;
  min-height: 160px;
  max-height: 360px;
  padding: 14px 20px;
  background: var(--s3);
  font-family: var(--mono);
  font-size: 12.5px;
  line-height: 1.75;
  scrollbar-width: thin;
  scrollbar-color: var(--line2) transparent;
}
.tm-empty {
  color: var(--t3);
  font-family: var(--font);
  font-size: 13px;
  text-align: center;
  padding: 28px 0;
  line-height: 1.6;
}
.tm-line {
  display: flex;
  align-items: flex-start;
  gap: 10px;
  padding: 1px 0;
  color: var(--t3);
}
.tm-line--running { color: var(--gold); }
.tm-line--done    { color: var(--dn); }
.tm-line--error   { color: var(--up); }
.tm-line--info    { color: var(--t2); }
.tm-icon { flex-shrink: 0; width: 14px; text-align: center; margin-top: 1px; }
.tm-dot  { color: var(--t3); opacity: 0.45; }
.tm-text { flex: 1; word-break: break-all; }
.tm-spin { display: inline-block; animation: spin 1.1s linear infinite; }

.tm-footer {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  padding: 10px 16px;
  border-top: 1px solid var(--line);
  flex-shrink: 0;
  background: var(--s2);
}
.tm-btn-cancel {
  height: 32px;
  padding: 0 16px;
  background: transparent;
  border: 1px solid var(--line2);
  border-radius: 8px;
  color: var(--t2);
  font-size: 12.5px;
  cursor: pointer;
  transition: background 0.15s, color 0.15s;
}
.tm-btn-cancel:hover { background: var(--s1); color: var(--t1); }
.tm-btn-go {
  height: 32px;
  padding: 0 16px;
  background: color-mix(in oklch, var(--blue) 15%, transparent);
  border: 1px solid var(--blue);
  border-radius: 8px;
  color: var(--blue);
  font-size: 12.5px;
  font-weight: 500;
  text-decoration: none;
  display: inline-flex;
  align-items: center;
  transition: background 0.15s, color 0.15s;
}
.tm-btn-go:hover { background: var(--blue); color: oklch(97% 0.005 220); }
</style>
