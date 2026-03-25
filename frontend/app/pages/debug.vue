<script setup lang="ts">
useHead({
  title: 'Debug | Stock',
  link: [
    { rel: 'preconnect', href: 'https://fonts.googleapis.com' },
    { rel: 'preconnect', href: 'https://fonts.gstatic.com', crossorigin: '' },
    {
      rel: 'stylesheet',
      href: 'https://fonts.googleapis.com/css2?family=DM+Sans:ital,opsz,wght@0,9..40,300;0,9..40,400;0,9..40,500;0,9..40,600;0,9..40,700;1,9..40,400&family=Fira+Code:wght@400;500;600&display=swap',
    },
  ],
})

// ─────────────────────────────────────────────────────────────
// 通用型別
// ─────────────────────────────────────────────────────────────
type Tab = 'stock-list' | 'daily-price' | 'single-history' | 'full-backfill' | 'chips'

interface LogEntry {
  ts: string
  stage: string
  source?: string   // TWSE / TPEX / SYSTEM
  message: string
  count?: number    // 本次筆數
  acc?: number      // 累計
  url?: string
  elapsed: number
  isErr?: boolean
  isWarn?: boolean
  isDone?: boolean
}

interface SSEEvent {
  stage: string
  message?: string
  progress: number
  url?: string
  total?: number
  synced?: number
  error?: string
}

// ─────────────────────────────────────────────────────────────
// 全域 utils
// ─────────────────────────────────────────────────────────────
function nowStr() {
  return new Date().toLocaleTimeString('zh-TW', {
    hour: '2-digit', minute: '2-digit', second: '2-digit', fractionalSecondDigits: 3,
  })
}

function makeLog(stage: string, msg: string, extra?: Partial<LogEntry>, elapsed = 0): LogEntry {
  return {
    ts: nowStr(), stage, message: msg, elapsed,
    ...extra,
  }
}

// ─────────────────────────────────────────────────────────────
// TAB
// ─────────────────────────────────────────────────────────────
const activeTab = ref<Tab>('stock-list')
const tabs: { key: Tab; label: string; badge: string; color: string }[] = [
  { key: 'stock-list',     label: '股票清單同步',   badge: 'TWSE+TPEX',    color: '#5b9cf6' },
  { key: 'daily-price',    label: '每日日K同步',     badge: 'TWSE+TPEX',    color: '#a78ce8' },
  { key: 'single-history', label: '單股歷史回填',    badge: 'SSE逐月',      color: '#f0a842' },
  { key: 'full-backfill',  label: '全量日K回填',     badge: 'Worker Pool',  color: '#4ecfa8' },
  { key: 'chips',          label: '籌碼金字塔',      badge: 'Pyramid',      color: '#e07b5a' },
]

// ─────────────────────────────────────────────────────────────
// ① 股票清單同步  GET /api/scraper/stocks  (SSE)
// ─────────────────────────────────────────────────────────────
const sl = reactive({
  running: false,
  progress: 0,
  logs: [] as LogEntry[],
  es: null as EventSource | null,
  startTime: 0,
})
const slLogBox = ref<HTMLElement | null>(null)

function slStart() {
  if (sl.running) { sl.es?.close(); sl.running = false; return }
  sl.logs = []; sl.progress = 0; sl.running = true; sl.startTime = Date.now()
  sl.es = new EventSource('/api/scraper/stocks')
  sl.es.onmessage = (e) => {
    let ev: SSEEvent; try { ev = JSON.parse(e.data) } catch { return }
    sl.progress = ev.progress
    const elapsed = Date.now() - sl.startTime
    const sourceMap: Record<string, string> = {
      fetching_listed: 'TWSE', fetched_listed: 'TWSE',
      fetching_otc: 'TPEX', fetched_otc: 'TPEX',
      saving: 'DB', done: 'SYSTEM', error: 'SYSTEM',
    }
    sl.logs.push(makeLog(ev.stage, ev.error ?? ev.message ?? ev.stage, {
      source: sourceMap[ev.stage] ?? 'SYSTEM',
      count: ev.total, acc: ev.synced, url: ev.url,
      isErr: ev.stage === 'error', isDone: ev.stage === 'done',
    }, elapsed))
    if (ev.stage === 'done' || ev.stage === 'error') { sl.running = false; sl.es?.close() }
    nextTick(() => { if (slLogBox.value) slLogBox.value.scrollTop = slLogBox.value.scrollHeight })
  }
  sl.es.onerror = () => {
    sl.logs.push(makeLog('error', '連線中斷', { isErr: true, source: 'SYSTEM' }, Date.now() - sl.startTime))
    sl.running = false; sl.es?.close()
  }
}

// ─────────────────────────────────────────────────────────────
// ② 每日日K  GET /api/scraper/prices?date=  (SSE)
// ─────────────────────────────────────────────────────────────
const dp = reactive({
  running: false,
  progress: 0,
  date: new Date().toISOString().split('T')[0],
  logs: [] as LogEntry[],
  es: null as EventSource | null,
  startTime: 0,
})
const dpLogBox = ref<HTMLElement | null>(null)

function dpStart() {
  if (dp.running) { dp.es?.close(); dp.running = false; return }
  dp.logs = []; dp.progress = 0; dp.running = true; dp.startTime = Date.now()
  const url = `/api/scraper/prices?date=${dp.date}`
  dp.logs.push(makeLog('start', `連接 ${url}`, { source: 'SYSTEM' }, 0))
  dp.es = new EventSource(url)
  dp.es.onmessage = (e) => {
    let ev: SSEEvent; try { ev = JSON.parse(e.data) } catch { return }
    dp.progress = ev.progress
    const elapsed = Date.now() - dp.startTime
    const sourceMap: Record<string, string> = {
      fetching_twse: 'TWSE', fetched_twse: 'TWSE',
      fetching_tpex: 'TPEX', fetched_tpex: 'TPEX',
      saving: 'DB', done: 'SYSTEM', error: 'SYSTEM',
    }
    dp.logs.push(makeLog(ev.stage, ev.error ?? ev.message ?? ev.stage, {
      source: sourceMap[ev.stage] ?? 'SYSTEM',
      count: ev.total, acc: ev.synced, url: ev.url,
      isErr: ev.stage === 'error', isDone: ev.stage === 'done',
    }, elapsed))
    if (ev.stage === 'done' || ev.stage === 'error') { dp.running = false; dp.es?.close() }
    nextTick(() => { if (dpLogBox.value) dpLogBox.value.scrollTop = dpLogBox.value.scrollHeight })
  }
  dp.es.onerror = () => {
    dp.logs.push(makeLog('error', '連線中斷', { isErr: true, source: 'SYSTEM' }, Date.now() - dp.startTime))
    dp.running = false; dp.es?.close()
  }
}

// ─────────────────────────────────────────────────────────────
// ③ 單股歷史  GET /api/scraper/prices/stock/:symbol  (SSE)
// ─────────────────────────────────────────────────────────────
const sh = reactive({
  running: false,
  progress: 0,
  symbol: '2330',
  logs: [] as LogEntry[],
  es: null as EventSource | null,
  startTime: 0,
  monthCount: 0,
  totalRecords: 0,
  market: '' as 'TWSE' | 'TPEX' | '',
})
const shLogBox = ref<HTMLElement | null>(null)

function shStart() {
  const sym = sh.symbol.trim().toUpperCase()
  if (!sym) return
  if (sh.running) { sh.es?.close(); sh.running = false; return }
  sh.logs = []; sh.progress = 0; sh.monthCount = 0; sh.totalRecords = 0; sh.market = ''
  sh.running = true; sh.startTime = Date.now()
  sh.logs.push(makeLog('start', `開始抓取 ${sym}，從當月往回逐月爬取...`, { source: 'SYSTEM' }, 0))
  sh.es = new EventSource(`/api/scraper/prices/stock/${sym}`)
  sh.es.onmessage = (e) => {
    let ev: SSEEvent; try { ev = JSON.parse(e.data) } catch { return }
    sh.progress = ev.progress
    const elapsed = Date.now() - sh.startTime
    // 從訊息推斷市場（start 事件包含 TWSE/TPEX）
    if (ev.stage === 'start' && ev.message) {
      if (ev.message.includes('TWSE')) sh.market = 'TWSE'
      else if (ev.message.includes('TPEX')) sh.market = 'TPEX'
    }
    if (ev.stage === 'fetched') { sh.monthCount++; sh.totalRecords = ev.synced ?? sh.totalRecords }
    // 從訊息解析年月作為 source 欄位
    let monthLabel = sh.market || 'SRC'
    const ymMatch = ev.message?.match(/\d{6}/)
    if (ymMatch) {
      const ym = ymMatch[0]!
      monthLabel = `${sh.market || 'SRC'} ${ym.substring(0,4)}/${ym.substring(4,6)}`
    }
    sh.logs.push(makeLog(ev.stage, ev.error ?? ev.message ?? ev.stage, {
      source: ev.stage === 'saving' ? 'DB' : ev.stage === 'done' || ev.stage === 'error' ? 'SYSTEM' : monthLabel,
      count: ev.total, acc: ev.synced,
      isErr: ev.stage === 'error', isWarn: ev.stage === 'warning', isDone: ev.stage === 'done',
    }, elapsed))
    if (ev.stage === 'done' || ev.stage === 'error') { sh.running = false; sh.es?.close() }
    nextTick(() => { if (shLogBox.value) shLogBox.value.scrollTop = shLogBox.value.scrollHeight })
  }
  sh.es.onerror = () => {
    sh.logs.push(makeLog('error', '連線中斷', { isErr: true, source: 'SYSTEM' }, Date.now() - sh.startTime))
    sh.running = false; sh.es?.close()
  }
}

// ─────────────────────────────────────────────────────────────
// ④ 全量回填  POST trigger + GET status + POST test
// ─────────────────────────────────────────────────────────────
interface BackfillStatus {
  id?: number; status: string; started_at?: string; completed_at?: string
  total?: number; success?: number; fail?: number; message?: string
}

const fb = reactive({
  status: null as BackfillStatus | null,
  triggering: false,
  testSymbol: '2330',
  testRunning: false,
  testResult: null as { ok: boolean; records?: number; error?: string } | null,
  logs: [] as LogEntry[],
  pollTimer: null as ReturnType<typeof setInterval> | null,
  startTime: 0,
})
const fbLogBox = ref<HTMLElement | null>(null)

async function fbLoadStatus() {
  try {
    const data = await $fetch<BackfillStatus>('/api/scraper/prices/all/status')
    fb.status = data
  } catch { fb.status = { status: 'never' } }
}

async function fbTrigger() {
  if (fb.triggering) return
  fb.triggering = true; fb.logs = []; fb.startTime = Date.now()
  fb.logs.push(makeLog('start', 'POST /api/scraper/prices/all/trigger', { source: 'SYSTEM' }, 0))
  try {
    const r = await $fetch<{ ok: boolean; total: number }>('/api/scraper/prices/all/trigger', { method: 'POST' })
    fb.logs.push(makeLog('start', `已啟動，預計處理 ${r.total} 支股票`, { source: 'SYSTEM', acc: r.total }, Date.now() - fb.startTime))
    fbStartPolling()
  } catch (err: unknown) {
    const msg = (err as { data?: { error?: string } })?.data?.error ?? '啟動失敗'
    fb.logs.push(makeLog('error', msg, { isErr: true, source: 'SYSTEM' }, Date.now() - fb.startTime))
  } finally { fb.triggering = false }
}

function fbStartPolling() {
  fbStopPolling()
  fb.pollTimer = setInterval(async () => {
    await fbLoadStatus()
    const s = fb.status
    if (!s) return
    const elapsed = Date.now() - fb.startTime
    const done = s.status === 'completed' || s.status === 'failed'
    if (s.message) {
      fb.logs.push(makeLog(done ? s.status : 'running', s.message, {
        source: 'WORKER', acc: (s.success ?? 0) + (s.fail ?? 0),
        isDone: s.status === 'completed', isErr: s.status === 'failed',
      }, elapsed))
    }
    if (done) { fbStopPolling() }
  }, 2000)
}

function fbStopPolling() {
  if (fb.pollTimer) { clearInterval(fb.pollTimer); fb.pollTimer = null }
}

async function fbTestSingle() {
  const sym = fb.testSymbol.trim().toUpperCase()
  if (!sym || fb.testRunning) return
  fb.testRunning = true; fb.testResult = null
  fb.logs.push(makeLog('start', `POST /api/scraper/prices/all/test  symbol=${sym}`, { source: 'SYSTEM' }, 0))
  const t0 = Date.now()
  try {
    const r = await $fetch<{ ok: boolean; symbol: string; market: string; records: number }>(
      '/api/scraper/prices/all/test', { method: 'POST', body: { symbol: sym } }
    )
    const elapsed = Date.now() - t0
    fb.testResult = { ok: r.ok, records: r.records }
    fb.logs.push(makeLog('done', `${r.symbol}（${r.market}）完成，共 ${r.records} 筆`, {
      source: r.market, acc: r.records, isDone: true,
    }, elapsed))
  } catch (err: unknown) {
    const msg = (err as { data?: { error?: string } })?.data?.error ?? '失敗'
    fb.testResult = { ok: false, error: msg }
    fb.logs.push(makeLog('error', msg, { isErr: true, source: 'SYSTEM' }, Date.now() - t0))
  } finally { fb.testRunning = false }
  nextTick(() => { if (fbLogBox.value) fbLogBox.value.scrollTop = fbLogBox.value.scrollHeight })
}

const fbProgress = computed(() => {
  const s = fb.status
  if (!s || !s.total) return 0
  const done = (s.success ?? 0) + (s.fail ?? 0)
  return Math.round((done / s.total) * 100)
})

onMounted(() => fbLoadStatus())
onBeforeUnmount(() => fbStopPolling())

// ─────────────────────────────────────────────────────────────
// ⑤ 籌碼  POST trigger-single / GET status
// ─────────────────────────────────────────────────────────────
interface ChipsStatus {
  status: 'never' | 'running' | 'completed' | 'failed'
  is_fresh?: boolean; next_run?: string
  started_at?: string; completed_at?: string
  total?: number; success?: number; fail?: number; message?: string
}

const cp = reactive({
  status: null as ChipsStatus | null,
  symbol: '2330',
  singleRunning: false,
  logs: [] as LogEntry[],
  pollTimer: null as ReturnType<typeof setInterval> | null,
  startTime: 0,
})
const cpLogBox = ref<HTMLElement | null>(null)

async function cpLoadStatus() {
  try { cp.status = await $fetch<ChipsStatus>('/api/chips/status') } catch {}
}

async function cpTriggerSingle() {
  const sym = cp.symbol.trim().toUpperCase()
  if (!sym || cp.singleRunning) return
  cp.singleRunning = true; cp.startTime = Date.now()
  cp.logs.push(makeLog('start', `POST /api/chips/trigger-single  symbol=${sym}`, { source: 'SYSTEM' }, 0))
  try {
    const r = await $fetch<{ ok: boolean; symbol: string; message?: string }>(
      '/api/chips/trigger-single', { method: 'POST', body: { symbol: sym } }
    )
    const elapsed = Date.now() - cp.startTime
    cp.logs.push(makeLog('done', r.message ?? `${sym} 籌碼更新完成`, { source: 'norway.twsthr.info', isDone: true }, elapsed))
  } catch (err: unknown) {
    const msg = (err as { data?: { error?: string } })?.data?.error ?? '失敗'
    cp.logs.push(makeLog('error', msg, { isErr: true, source: 'SYSTEM' }, Date.now() - cp.startTime))
  } finally { cp.singleRunning = false }
  nextTick(() => { if (cpLogBox.value) cpLogBox.value.scrollTop = cpLogBox.value.scrollHeight })
}

async function cpTriggerAll() {
  if (cp.pollTimer) return
  cp.startTime = Date.now()
  cp.logs.push(makeLog('start', 'POST /api/chips/trigger（全量）', { source: 'SYSTEM' }, 0))
  try {
    const r = await $fetch<{ ok: boolean; total: number }>('/api/chips/trigger', { method: 'POST' })
    cp.logs.push(makeLog('running', `已啟動，預計處理 ${r.total} 支`, { source: 'SYSTEM', acc: r.total }, Date.now() - cp.startTime))
    cpStartPolling()
  } catch (err: unknown) {
    const msg = (err as { data?: { error?: string } })?.data?.error ?? '啟動失敗'
    cp.logs.push(makeLog('error', msg, { isErr: true, source: 'SYSTEM' }, Date.now() - cp.startTime))
  }
}

function cpStartPolling() {
  if (cp.pollTimer) return
  cp.pollTimer = setInterval(async () => {
    await cpLoadStatus()
    const s = cp.status
    if (!s) return
    const elapsed = Date.now() - cp.startTime
    const done = s.status === 'completed' || s.status === 'failed'
    if (s.message) {
      cp.logs.push(makeLog(done ? s.status : 'running', s.message, {
        source: 'WORKER', acc: (s.success ?? 0) + (s.fail ?? 0),
        isDone: s.status === 'completed', isErr: s.status === 'failed',
      }, elapsed))
    }
    if (done) { if (cp.pollTimer) { clearInterval(cp.pollTimer); cp.pollTimer = null } }
  }, 3000)
}

onMounted(() => cpLoadStatus())
onBeforeUnmount(() => { if (cp.pollTimer) { clearInterval(cp.pollTimer); cp.pollTimer = null } })

// ─────────────────────────────────────────────────────────────
// 通用 log 工具
// ─────────────────────────────────────────────────────────────
const stageBadge: Record<string, string> = {
  start: 'START', running: 'RUN', fetching_listed: 'REQ',
  fetched_listed: 'OK', fetching_otc: 'REQ', fetched_otc: 'OK',
  fetching_twse: 'REQ', fetched_twse: 'OK', fetching_tpex: 'REQ', fetched_tpex: 'OK',
  fetched: 'OK', warning: 'WARN', saving: 'SAVE', done: 'DONE', error: 'ERR',
  completed: 'DONE', failed: 'ERR',
}
function badge(stage: string) { return stageBadge[stage] ?? stage.toUpperCase().substring(0, 5) }

const sourceColor: Record<string, string> = {
  TWSE: '#5b9cf6', TPEX: '#a78ce8', DB: '#4ecfa8', WORKER: '#f0a842',
  SYSTEM: 'rgba(220,215,200,0.4)', 'norway.twsthr.info': '#e07b5a',
}
function srcColor(src: string) {
  if (!src) return 'rgba(220,215,200,0.4)'
  if (src.startsWith('TWSE')) return '#5b9cf6'
  if (src.startsWith('TPEX')) return '#a78ce8'
  return sourceColor[src] ?? 'rgba(220,215,200,0.4)'
}
</script>

<template>
  <div class="page">

    <!-- ═══════════════ HEADER ═══════════════ -->
    <header class="hd">
      <div class="hd-inner">
        <div class="hd-left">
          <NuxtLink to="/" class="back">← 首頁</NuxtLink>
          <span class="sep">/</span>
          <span class="cur">Debug</span>
        </div>
        <div class="hd-tag">DEV TOOLS</div>
      </div>
    </header>

    <!-- ═══════════════ TAB BAR ═══════════════ -->
    <nav class="tab-bar">
      <div class="tab-bar-inner">
        <button
          v-for="t in tabs" :key="t.key"
          class="tab-btn"
          :class="{ 'tab-btn--active': activeTab === t.key }"
          :style="activeTab === t.key ? { borderColor: t.color, color: t.color } : {}"
          @click="activeTab = t.key"
        >
          <span class="tab-label">{{ t.label }}</span>
          <span class="tab-badge" :style="activeTab === t.key ? { background: t.color + '22', color: t.color } : {}">{{ t.badge }}</span>
        </button>
      </div>
    </nav>

    <div class="body">

      <!-- ══════════════════════════════════════
           ① 股票清單同步
           ══════════════════════════════════════ -->
      <section v-if="activeTab === 'stock-list'" class="panel">
        <div class="panel-head">
          <div>
            <h2 class="panel-title">股票清單同步</h2>
            <p class="panel-sub">呼叫 <code>GET /api/scraper/stocks</code>（SSE），分兩步抓取 TWSE 上市與 TPEX 上櫃清單後合併存入 DB。</p>
          </div>
          <div class="action-row">
            <div class="api-chip api-chip--twse">TWSE OpenAPI</div>
            <div class="api-chip api-chip--tpex">TPEX OpenAPI</div>
            <button class="run-btn" :class="{ 'run-btn--stop': sl.running }" @click="slStart">
              <span v-if="!sl.running">▶ 開始</span>
              <span v-else class="spin">⟳&nbsp;中止</span>
            </button>
          </div>
        </div>

        <div class="flow-steps">
          <div class="flow-step flow-step--twse">
            <span class="fs-num">1</span>
            <div><p class="fs-title">抓取 TWSE 上市清單</p><p class="fs-api">STOCK_DAY_ALL（OpenAPI）</p></div>
          </div>
          <div class="flow-arrow">→</div>
          <div class="flow-step flow-step--tpex">
            <span class="fs-num">2</span>
            <div><p class="fs-title">抓取 TPEX 上櫃清單</p><p class="fs-api">OTC OpenAPI</p></div>
          </div>
          <div class="flow-arrow">→</div>
          <div class="flow-step flow-step--db">
            <span class="fs-num">3</span>
            <div><p class="fs-title">UPSERT 寫入 DB</p><p class="fs-api">ON CONFLICT (symbol)</p></div>
          </div>
        </div>

        <div class="prog-wrap" v-if="sl.running || sl.progress > 0">
          <div class="prog-fill" :class="{ 'prog-fill--done': !sl.running && sl.progress === 100 }" :style="{ width: sl.progress + '%' }" />
        </div>
        <div class="log-panel">
          <div class="log-header">
            <span class="log-title">事件日誌</span>
            <span class="log-count">{{ sl.logs.length }} 筆</span>
            <button v-if="sl.logs.length" class="log-clear" @click="sl.logs = []">清除</button>
          </div>
          <div ref="slLogBox" class="log-box">
            <div v-if="!sl.logs.length" class="log-empty">尚未開始</div>
            <div v-for="(entry, i) in sl.logs" :key="i" class="log-row" :class="`log-row--${entry.stage}`">
              <span class="log-ts">{{ entry.ts }}</span>
              <span class="log-badge" :class="`badge--${entry.stage}`">{{ badge(entry.stage) }}</span>
              <span v-if="entry.source" class="log-src" :style="{ background: srcColor(entry.source) + '1a', color: srcColor(entry.source), border: `1px solid ${srcColor(entry.source)}33` }">{{ entry.source }}</span>
              <span class="log-msg">{{ entry.message }}</span>
              <span v-if="entry.count" class="log-meta">+{{ entry.count.toLocaleString() }}</span>
              <span v-if="entry.acc" class="log-meta log-meta--acc">累計 {{ entry.acc.toLocaleString() }}</span>
              <span class="log-elapsed">+{{ entry.elapsed }}ms</span>
            </div>
          </div>
        </div>
      </section>

      <!-- ══════════════════════════════════════
           ② 每日日K
           ══════════════════════════════════════ -->
      <section v-if="activeTab === 'daily-price'" class="panel">
        <div class="panel-head">
          <div>
            <h2 class="panel-title">每日日K同步</h2>
            <p class="panel-sub">呼叫 <code>GET /api/scraper/prices?date=YYYY-MM-DD</code>（SSE），抓取指定日 TWSE 全市場 + TPEX 全市場 OHLCV 後 Upsert。</p>
          </div>
          <div class="action-row">
            <div class="api-chip api-chip--twse">TWSE STOCK_DAY_ALL</div>
            <div class="api-chip api-chip--tpex">TPEX quotes</div>
          </div>
        </div>

        <div class="ctrl-row">
          <div class="input-wrap">
            <label class="input-label">資料日期</label>
            <input v-model="dp.date" type="date" class="date-input" :disabled="dp.running" />
          </div>
          <button class="run-btn" :class="{ 'run-btn--stop': dp.running }" @click="dpStart">
            <span v-if="!dp.running">▶ 開始</span>
            <span v-else class="spin">⟳&nbsp;中止</span>
          </button>
        </div>

        <div class="flow-steps">
          <div class="flow-step flow-step--twse">
            <span class="fs-num">1</span>
            <div><p class="fs-title">TWSE 全市場最新日K</p><p class="fs-api">exchangeReport/STOCK_DAY_ALL</p></div>
          </div>
          <div class="flow-arrow">→</div>
          <div class="flow-step flow-step--tpex">
            <span class="fs-num">2</span>
            <div><p class="fs-title">TPEX 全市場最新日K</p><p class="fs-api">tpex_mainboard_quotes</p></div>
          </div>
          <div class="flow-arrow">→</div>
          <div class="flow-step flow-step--db">
            <span class="fs-num">3</span>
            <div><p class="fs-title">合併 UPSERT</p><p class="fs-api">ON CONFLICT (symbol, date)</p></div>
          </div>
        </div>

        <div class="prog-wrap" v-if="dp.running || dp.progress > 0">
          <div class="prog-fill" :class="{ 'prog-fill--done': !dp.running && dp.progress === 100 }" :style="{ width: dp.progress + '%' }" />
        </div>
        <div class="log-panel">
          <div class="log-header">
            <span class="log-title">事件日誌</span>
            <span class="log-count">{{ dp.logs.length }} 筆</span>
            <button v-if="dp.logs.length" class="log-clear" @click="dp.logs = []">清除</button>
          </div>
          <div ref="dpLogBox" class="log-box">
            <div v-if="!dp.logs.length" class="log-empty">尚未開始</div>
            <div v-for="(entry, i) in dp.logs" :key="i" class="log-row" :class="`log-row--${entry.stage}`">
              <span class="log-ts">{{ entry.ts }}</span>
              <span class="log-badge" :class="`badge--${entry.stage}`">{{ badge(entry.stage) }}</span>
              <span v-if="entry.source" class="log-src" :style="{ background: srcColor(entry.source) + '1a', color: srcColor(entry.source), border: `1px solid ${srcColor(entry.source)}33` }">{{ entry.source }}</span>
              <span class="log-msg">{{ entry.message }}</span>
              <span v-if="entry.count" class="log-meta">+{{ entry.count.toLocaleString() }}</span>
              <span v-if="entry.acc" class="log-meta log-meta--acc">累計 {{ entry.acc.toLocaleString() }}</span>
              <span class="log-elapsed">+{{ entry.elapsed }}ms</span>
            </div>
          </div>
        </div>
      </section>

      <!-- ══════════════════════════════════════
           ③ 單股歷史回填
           ══════════════════════════════════════ -->
      <section v-if="activeTab === 'single-history'" class="panel">
        <div class="panel-head">
          <div>
            <h2 class="panel-title">單股歷史回填</h2>
            <p class="panel-sub">呼叫 <code>GET /api/scraper/prices/stock/:symbol</code>（SSE），從當月往回逐月抓取，連續 3 個月無資料停止，無月數上限。</p>
          </div>
          <div class="action-row">
            <div class="api-chip api-chip--twse">TWSE STOCK_DAY</div>
            <div class="api-chip api-chip--tpex">TPEX st43</div>
          </div>
        </div>

        <div class="ctrl-row">
          <div class="input-wrap">
            <label class="input-label">股票代號</label>
            <input v-model="sh.symbol" class="sym-input" placeholder="例：2330" :disabled="sh.running" @keydown.enter="shStart" />
          </div>
          <button class="run-btn" :class="{ 'run-btn--stop': sh.running }" @click="shStart">
            <span v-if="!sh.running">▶ 開始</span>
            <span v-else class="spin">⟳&nbsp;中止</span>
          </button>
          <div v-if="sh.market" class="market-badge" :class="`market-badge--${sh.market.toLowerCase()}`">{{ sh.market }}</div>
        </div>

        <div class="flow-steps">
          <div class="flow-step flow-step--src">
            <span class="fs-num">1</span>
            <div><p class="fs-title">查詢市場別</p><p class="fs-api">DB stocks.market</p></div>
          </div>
          <div class="flow-arrow">→</div>
          <div class="flow-step flow-step--twse">
            <span class="fs-num">2</span>
            <div><p class="fs-title">逐月往回抓（TWSE）</p><p class="fs-api">STOCK_DAY?date=YYYYMM01&stockNo=</p></div>
          </div>
          <div class="flow-arrow">or</div>
          <div class="flow-step flow-step--tpex">
            <span class="fs-num">2</span>
            <div><p class="fs-title">逐月往回抓（TPEX）</p><p class="fs-api">st43_result.php?d=民國/MM</p></div>
          </div>
          <div class="flow-arrow">→</div>
          <div class="flow-step flow-step--db">
            <span class="fs-num">3</span>
            <div><p class="fs-title">批次 UPSERT</p><p class="fs-api">500筆/批</p></div>
          </div>
        </div>

        <div v-if="sh.monthCount > 0 || sh.totalRecords > 0" class="mini-summary">
          <span>市場：<b>{{ sh.market || '—' }}</b></span>
          <span class="ms-sep">·</span>
          <span>已抓月份：<b>{{ sh.monthCount }}</b></span>
          <span class="ms-sep">·</span>
          <span>累計：<b>{{ sh.totalRecords.toLocaleString() }}</b> 筆</span>
        </div>

        <div class="prog-wrap" v-if="sh.running || sh.progress > 0">
          <div class="prog-fill" :class="{ 'prog-fill--done': !sh.running && sh.progress === 100 }" :style="{ width: sh.progress + '%' }" />
        </div>
        <div class="log-panel">
          <div class="log-header">
            <span class="log-title">事件日誌</span>
            <span class="log-count">{{ sh.logs.length }} 筆</span>
            <button v-if="sh.logs.length" class="log-clear" @click="sh.logs = []">清除</button>
          </div>
          <div ref="shLogBox" class="log-box">
            <div v-if="!sh.logs.length" class="log-empty">尚未開始</div>
            <div v-for="(entry, i) in sh.logs" :key="i" class="log-row" :class="`log-row--${entry.stage}`">
              <span class="log-ts">{{ entry.ts }}</span>
              <span class="log-badge" :class="`badge--${entry.stage}`">{{ badge(entry.stage) }}</span>
              <span v-if="entry.source" class="log-src" :style="{ background: srcColor(entry.source) + '1a', color: srcColor(entry.source), border: `1px solid ${srcColor(entry.source)}33` }">{{ entry.source }}</span>
              <span class="log-msg">{{ entry.message }}</span>
              <span v-if="entry.count" class="log-meta">+{{ entry.count.toLocaleString() }}</span>
              <span v-if="entry.acc" class="log-meta log-meta--acc">累計 {{ entry.acc.toLocaleString() }}</span>
              <span class="log-elapsed">+{{ entry.elapsed }}ms</span>
            </div>
          </div>
        </div>
      </section>

      <!-- ══════════════════════════════════════
           ④ 全量日K回填
           ══════════════════════════════════════ -->
      <section v-if="activeTab === 'full-backfill'" class="panel">
        <div class="panel-head">
          <div>
            <h2 class="panel-title">全量日K回填</h2>
            <p class="panel-sub">Worker Pool 並行爬取全部股票歷史日K。可觸發全量作業或對單股進行同步測試（BlockingCall，直接回傳結果）。</p>
          </div>
          <div class="action-row">
            <div class="api-chip api-chip--worker">WORKER POOL</div>
            <div class="api-chip api-chip--db">PostgreSQL Upsert</div>
          </div>
        </div>

        <!-- 狀態卡片 -->
        <div v-if="fb.status" class="status-card" :class="`status-card--${fb.status.status}`">
          <div class="sc-row">
            <span class="sc-k">狀態</span>
            <span class="sc-v sc-v--status">{{ fb.status.status }}</span>
            <button class="small-btn" @click="fbLoadStatus">刷新</button>
          </div>
          <div v-if="fb.status.total" class="sc-row">
            <span class="sc-k">進度</span>
            <span class="sc-v">{{ (fb.status.success ?? 0) + (fb.status.fail ?? 0) }} / {{ fb.status.total }}</span>
            <span class="sc-v sc-v--ok">成功 {{ fb.status.success }}</span>
            <span class="sc-v sc-v--err">失敗 {{ fb.status.fail }}</span>
          </div>
          <div v-if="fb.status.message" class="sc-row"><span class="sc-k">訊息</span><span class="sc-v sc-v--msg">{{ fb.status.message }}</span></div>
          <div v-if="fb.status.total && fbProgress > 0" class="sc-prog">
            <div class="sc-prog-bar" :style="{ width: fbProgress + '%' }" :class="fb.status.status === 'completed' ? 'sc-prog-bar--done' : ''" />
          </div>
        </div>

        <div class="flow-steps" style="margin-top:16px;">
          <div class="flow-step flow-step--src">
            <span class="fs-num">1</span>
            <div><p class="fs-title">載入全部股票</p><p class="fs-api">DB: 四碼非零開頭</p></div>
          </div>
          <div class="flow-arrow">→</div>
          <div class="flow-step flow-step--worker">
            <span class="fs-num">2</span>
            <div><p class="fs-title">Worker Pool 並行</p><p class="fs-api">PRICE_SYNC_CONCURRENCY=3</p></div>
          </div>
          <div class="flow-arrow">→</div>
          <div class="flow-step flow-step--twse">
            <span class="fs-num">3a</span>
            <div><p class="fs-title">逐月 TWSE</p><p class="fs-api">STOCK_DAY per month</p></div>
          </div>
          <div class="flow-arrow">or</div>
          <div class="flow-step flow-step--tpex">
            <span class="fs-num">3b</span>
            <div><p class="fs-title">逐月 TPEX</p><p class="fs-api">st43 per month</p></div>
          </div>
          <div class="flow-arrow">→</div>
          <div class="flow-step flow-step--db">
            <span class="fs-num">4</span>
            <div><p class="fs-title">Upsert DB</p><p class="fs-api">500筆/批次</p></div>
          </div>
        </div>

        <div class="ctrl-row" style="margin-top:20px;">
          <button class="run-btn" :disabled="fb.triggering" @click="fbTrigger">
            {{ fb.triggering ? '啟動中...' : '▶ 觸發全量回填' }}
          </button>
          <span class="ctrl-sep">或</span>
          <div class="input-wrap">
            <label class="input-label">測試單股（同步）</label>
            <input v-model="fb.testSymbol" class="sym-input" placeholder="2330" :disabled="fb.testRunning" @keydown.enter="fbTestSingle" />
          </div>
          <button class="run-btn run-btn--alt" :disabled="fb.testRunning" @click="fbTestSingle">
            {{ fb.testRunning ? '執行中...' : '▶ 測試單股' }}
          </button>
          <div v-if="fb.testResult" class="test-result" :class="fb.testResult.ok ? 'test-result--ok' : 'test-result--err'">
            {{ fb.testResult.ok ? `✓ ${fb.testResult.records?.toLocaleString()} 筆` : `✖ ${fb.testResult.error}` }}
          </div>
        </div>

        <div class="log-panel" style="margin-top:16px;">
          <div class="log-header">
            <span class="log-title">事件日誌</span>
            <span class="log-count">{{ fb.logs.length }} 筆</span>
            <button v-if="fb.logs.length" class="log-clear" @click="fb.logs = []">清除</button>
          </div>
          <div ref="fbLogBox" class="log-box">
            <div v-if="!fb.logs.length" class="log-empty">尚未開始</div>
            <div v-for="(entry, i) in fb.logs" :key="i" class="log-row" :class="`log-row--${entry.stage}`">
              <span class="log-ts">{{ entry.ts }}</span>
              <span class="log-badge" :class="`badge--${entry.stage}`">{{ badge(entry.stage) }}</span>
              <span v-if="entry.source" class="log-src" :style="{ background: srcColor(entry.source) + '1a', color: srcColor(entry.source), border: `1px solid ${srcColor(entry.source)}33` }">{{ entry.source }}</span>
              <span class="log-msg">{{ entry.message }}</span>
              <span v-if="entry.count" class="log-meta">+{{ entry.count.toLocaleString() }}</span>
              <span v-if="entry.acc" class="log-meta log-meta--acc">累計 {{ entry.acc.toLocaleString() }}</span>
              <span class="log-elapsed">+{{ entry.elapsed }}ms</span>
            </div>
          </div>
        </div>
      </section>

      <!-- ══════════════════════════════════════
           ⑤ 籌碼金字塔
           ══════════════════════════════════════ -->
      <section v-if="activeTab === 'chips'" class="panel">
        <div class="panel-head">
          <div>
            <h2 class="panel-title">籌碼金字塔</h2>
            <p class="panel-sub">爬取 <code>norway.twsthr.info</code> 股東持股分布資料（Playwright 無頭瀏覽器），可觸發全量或測試單股。</p>
          </div>
          <div class="action-row">
            <div class="api-chip api-chip--chips">norway.twsthr.info</div>
            <div class="api-chip api-chip--db">Playwright</div>
          </div>
        </div>

        <!-- 全局狀態 -->
        <div v-if="cp.status" class="status-card" :class="`status-card--${cp.status.status}`">
          <div class="sc-row">
            <span class="sc-k">狀態</span>
            <span class="sc-v sc-v--status">{{ cp.status.status }}</span>
            <span v-if="cp.status.is_fresh" class="sc-v sc-v--ok">新鮮</span>
            <button class="small-btn" @click="cpLoadStatus">刷新</button>
          </div>
          <div v-if="cp.status.total" class="sc-row">
            <span class="sc-k">進度</span>
            <span class="sc-v">{{ (cp.status.success ?? 0) + (cp.status.fail ?? 0) }} / {{ cp.status.total }}</span>
            <span class="sc-v sc-v--ok">成功 {{ cp.status.success }}</span>
            <span class="sc-v sc-v--err">失敗 {{ cp.status.fail }}</span>
          </div>
          <div v-if="cp.status.message" class="sc-row"><span class="sc-k">訊息</span><span class="sc-v sc-v--msg">{{ cp.status.message }}</span></div>
        </div>

        <div class="flow-steps" style="margin-top:16px;">
          <div class="flow-step flow-step--src">
            <span class="fs-num">1</span>
            <div><p class="fs-title">載入股票清單</p><p class="fs-api">DB stocks</p></div>
          </div>
          <div class="flow-arrow">→</div>
          <div class="flow-step flow-step--chips">
            <span class="fs-num">2</span>
            <div><p class="fs-title">Playwright 爬取</p><p class="fs-api">norway.twsthr.info</p></div>
          </div>
          <div class="flow-arrow">→</div>
          <div class="flow-step flow-step--db">
            <span class="fs-num">3</span>
            <div><p class="fs-title">解析持股分布</p><p class="fs-api">金字塔各層比例</p></div>
          </div>
          <div class="flow-arrow">→</div>
          <div class="flow-step flow-step--db">
            <span class="fs-num">4</span>
            <div><p class="fs-title">Upsert DB</p><p class="fs-api">chip_distributions</p></div>
          </div>
        </div>

        <div class="ctrl-row" style="margin-top:20px;">
          <div class="input-wrap">
            <label class="input-label">測試單股</label>
            <input v-model="cp.symbol" class="sym-input" placeholder="2330" :disabled="cp.singleRunning" @keydown.enter="cpTriggerSingle" />
          </div>
          <button class="run-btn" :disabled="cp.singleRunning" @click="cpTriggerSingle">
            {{ cp.singleRunning ? '執行中...' : '▶ 測試單股' }}
          </button>
          <span class="ctrl-sep">或</span>
          <button class="run-btn run-btn--warn" :disabled="!!cp.pollTimer" @click="cpTriggerAll">
            {{ cp.pollTimer ? '全量執行中...' : '▶ 觸發全量' }}
          </button>
        </div>

        <div class="log-panel" style="margin-top:16px;">
          <div class="log-header">
            <span class="log-title">事件日誌</span>
            <span class="log-count">{{ cp.logs.length }} 筆</span>
            <button v-if="cp.logs.length" class="log-clear" @click="cp.logs = []">清除</button>
          </div>
          <div ref="cpLogBox" class="log-box">
            <div v-if="!cp.logs.length" class="log-empty">尚未開始</div>
            <div v-for="(entry, i) in cp.logs" :key="i" class="log-row" :class="`log-row--${entry.stage}`">
              <span class="log-ts">{{ entry.ts }}</span>
              <span class="log-badge" :class="`badge--${entry.stage}`">{{ badge(entry.stage) }}</span>
              <span v-if="entry.source" class="log-src" :style="{ background: srcColor(entry.source) + '1a', color: srcColor(entry.source), border: `1px solid ${srcColor(entry.source)}33` }">{{ entry.source }}</span>
              <span class="log-msg">{{ entry.message }}</span>
              <span v-if="entry.count" class="log-meta">+{{ entry.count.toLocaleString() }}</span>
              <span v-if="entry.acc" class="log-meta log-meta--acc">累計 {{ entry.acc.toLocaleString() }}</span>
              <span class="log-elapsed">+{{ entry.elapsed }}ms</span>
            </div>
          </div>
        </div>
      </section>

    </div>
  </div>
</template>

<style scoped>
*, *::before, *::after { box-sizing: border-box; margin: 0; padding: 0; }

.page { min-height: 100vh; background: #0f1117; color: #e2e0d8; font-family: 'DM Sans', system-ui, sans-serif; }

/* ── header ── */
.hd { border-bottom: 1px solid rgba(255,255,255,0.07); }
.hd-inner { max-width: 1120px; margin: 0 auto; padding: 13px 24px; display: flex; align-items: center; justify-content: space-between; }
.hd-left { display: flex; align-items: center; gap: 8px; font-size: 13px; }
.back { color: rgba(220,215,200,0.45); text-decoration: none; }
.back:hover { color: #e2e0d8; }
.sep { color: rgba(220,215,200,0.22); }
.cur { color: #e2e0d8; font-weight: 500; }
.hd-tag { font-size: 10px; font-weight: 700; letter-spacing: .1em; color: rgba(91,156,246,0.55); padding: 3px 8px; border: 1px solid rgba(91,156,246,0.2); border-radius: 4px; }

/* ── tab bar ── */
.tab-bar { border-bottom: 1px solid rgba(255,255,255,0.07); background: rgba(255,255,255,0.015); }
.tab-bar-inner { max-width: 1120px; margin: 0 auto; padding: 0 24px; display: flex; gap: 2px; overflow-x: auto; }
.tab-btn {
  padding: 12px 18px; background: none; border: none; border-bottom: 2px solid transparent;
  color: rgba(220,215,200,0.42); font-size: 13px; font-weight: 500; cursor: pointer;
  display: flex; align-items: center; gap: 8px; white-space: nowrap; transition: color .15s, border-color .15s;
}
.tab-btn:hover { color: rgba(220,215,200,0.75); }
.tab-btn--active { color: #e2e0d8; }
.tab-badge {
  font-size: 10px; padding: 2px 6px; border-radius: 4px;
  background: rgba(255,255,255,0.06); color: rgba(220,215,200,0.35);
  font-family: 'Fira Code', monospace; letter-spacing: .03em;
}

/* ── body / panel ── */
.body { max-width: 1120px; margin: 0 auto; padding: 28px 24px 48px; }
/* ── panel head ── */
.panel-head { display: flex; justify-content: space-between; align-items: flex-start; gap: 16px; flex-wrap: wrap; }
.panel-title { font-size: 18px; font-weight: 600; color: #e2e0d8; }
.panel-sub { margin-top: 4px; font-size: 12.5px; color: rgba(220,215,200,0.4); line-height: 1.6; }
.panel-sub code { font-family: 'Fira Code', monospace; color: rgba(91,156,246,0.75); font-size: .93em; }
.action-row { display: flex; align-items: center; gap: 8px; flex-wrap: wrap; }

/* ── api chips ── */
.api-chip { font-size: 11px; font-family: 'Fira Code', monospace; padding: 4px 10px; border-radius: 5px; font-weight: 500; white-space: nowrap; }
.api-chip--twse   { background: rgba(91,156,246,0.12);  color: #5b9cf6;  border: 1px solid rgba(91,156,246,0.25); }
.api-chip--tpex   { background: rgba(167,140,232,0.12); color: #a78ce8; border: 1px solid rgba(167,140,232,0.25); }
.api-chip--db     { background: rgba(78,207,168,0.12);  color: #4ecfa8;  border: 1px solid rgba(78,207,168,0.25); }
.api-chip--worker { background: rgba(240,168,66,0.12);  color: #f0a842;  border: 1px solid rgba(240,168,66,0.25); }
.api-chip--chips  { background: rgba(224,123,90,0.12);  color: #e07b5a;  border: 1px solid rgba(224,123,90,0.25); }

/* ── flow steps ── */
.flow-steps { display: flex; align-items: center; gap: 0; margin-top: 20px; flex-wrap: wrap; gap: 6px; }
.flow-step {
  padding: 10px 14px; border-radius: 8px; border: 1px solid rgba(255,255,255,0.07);
  display: flex; align-items: flex-start; gap: 10px; min-width: 130px;
}
.flow-step--twse   { background: rgba(91,156,246,0.07);  border-color: rgba(91,156,246,0.2); }
.flow-step--tpex   { background: rgba(167,140,232,0.07); border-color: rgba(167,140,232,0.2); }
.flow-step--db     { background: rgba(78,207,168,0.07);  border-color: rgba(78,207,168,0.2); }
.flow-step--src    { background: rgba(220,215,200,0.04); border-color: rgba(220,215,200,0.12); }
.flow-step--worker { background: rgba(240,168,66,0.07);  border-color: rgba(240,168,66,0.2); }
.flow-step--chips  { background: rgba(224,123,90,0.07);  border-color: rgba(224,123,90,0.2); }
.fs-num { font-size: 10px; font-weight: 700; color: rgba(220,215,200,0.3); min-width: 14px; padding-top: 1px; }
.fs-title { font-size: 12px; font-weight: 600; color: rgba(220,215,200,0.75); }
.fs-api { font-size: 10.5px; font-family: 'Fira Code', monospace; color: rgba(220,215,200,0.3); margin-top: 2px; }
.flow-arrow { color: rgba(220,215,200,0.2); font-size: 12px; padding: 0 2px; align-self: center; }

/* ── ctrl row ── */
.ctrl-row { display: flex; align-items: flex-end; gap: 12px; margin-top: 20px; flex-wrap: wrap; }
.ctrl-sep { font-size: 12px; color: rgba(220,215,200,0.3); }
.input-wrap { display: flex; flex-direction: column; gap: 5px; }
.input-label { font-size: 10.5px; color: rgba(220,215,200,0.38); letter-spacing: .05em; text-transform: uppercase; }
.sym-input, .date-input {
  height: 36px; padding: 0 12px;
  background: rgba(255,255,255,0.04); border: 1px solid rgba(255,255,255,0.1);
  border-radius: 7px; color: #e2e0d8;
  font-family: 'Fira Code', monospace; font-size: 14px; outline: none;
  transition: border-color .15s;
}
.sym-input { width: 120px; }
.date-input { width: 160px; color-scheme: dark; }
.sym-input:focus, .date-input:focus { border-color: rgba(91,156,246,0.5); }
.sym-input:disabled, .date-input:disabled { opacity: .45; cursor: not-allowed; }

/* ── run btn ── */
.run-btn {
  height: 36px; padding: 0 18px;
  background: rgba(91,156,246,0.12); border: 1px solid rgba(91,156,246,0.3);
  border-radius: 7px; color: #5b9cf6; font-size: 13px; font-weight: 500; cursor: pointer;
  transition: background .15s; white-space: nowrap; display: flex; align-items: center; gap: 6px;
}
.run-btn:hover:not(:disabled) { background: rgba(91,156,246,0.22); }
.run-btn:disabled { opacity: .5; cursor: not-allowed; }
.run-btn--stop { background: rgba(224,82,82,0.1); border-color: rgba(224,82,82,0.28); color: #e05252; }
.run-btn--alt  { background: rgba(78,207,168,0.1); border-color: rgba(78,207,168,0.28); color: #4ecfa8; }
.run-btn--warn { background: rgba(240,168,66,0.1); border-color: rgba(240,168,66,0.28); color: #f0a842; }
@keyframes spin { to { transform: rotate(360deg); } }
.spin { display: inline-block; animation: spin .7s linear infinite; }

/* ── market badge ── */
.market-badge { padding: 4px 10px; border-radius: 6px; font-size: 12px; font-weight: 600; font-family: 'Fira Code', monospace; }
.market-badge--twse { background: rgba(91,156,246,0.15); color: #5b9cf6; border: 1px solid rgba(91,156,246,0.3); }
.market-badge--tpex { background: rgba(167,140,232,0.15); color: #a78ce8; border: 1px solid rgba(167,140,232,0.3); }

/* ── mini summary ── */
.mini-summary { margin-top: 14px; font-size: 12.5px; color: rgba(220,215,200,0.5); display: flex; gap: 8px; align-items: center; }
.mini-summary b { color: rgba(220,215,200,0.85); }
.ms-sep { color: rgba(220,215,200,0.2); }

/* ── prog bar wrapper (inline for now) ── */
.prog-wrap { margin-top: 14px; height: 5px; background: rgba(255,255,255,0.06); border-radius: 99px; overflow: hidden; position: relative; }
.prog-fill { height: 100%; background: linear-gradient(90deg, #5b9cf6, #a78ce8); border-radius: 99px; transition: width .3s; }
.prog-fill--done { background: linear-gradient(90deg, #4ecfa8, #5b9cf6); }

/* ── status card ── */
.status-card {
  margin-top: 20px; padding: 14px 18px; border-radius: 10px;
  border: 1px solid rgba(255,255,255,0.08); background: rgba(255,255,255,0.025);
  display: flex; flex-direction: column; gap: 8px;
}
.status-card--running  { border-color: rgba(240,168,66,0.25); background: rgba(240,168,66,0.04); }
.status-card--completed { border-color: rgba(78,207,168,0.25); background: rgba(78,207,168,0.04); }
.status-card--failed   { border-color: rgba(224,82,82,0.25);  background: rgba(224,82,82,0.04); }
.sc-row { display: flex; align-items: center; gap: 14px; flex-wrap: wrap; }
.sc-k { font-size: 11px; color: rgba(220,215,200,0.3); text-transform: uppercase; letter-spacing: .05em; min-width: 36px; }
.sc-v { font-size: 13px; font-family: 'Fira Code', monospace; color: rgba(220,215,200,0.7); }
.sc-v--status { font-weight: 700; color: rgba(220,215,200,0.9); }
.sc-v--ok { color: #4ecfa8; }
.sc-v--err { color: #e05252; }
.sc-v--msg { color: rgba(220,215,200,0.55); font-family: 'DM Sans', sans-serif; font-size: 12.5px; }
.small-btn { margin-left: auto; font-size: 11px; color: rgba(220,215,200,0.35); background: rgba(255,255,255,0.05); border: 1px solid rgba(255,255,255,0.08); border-radius: 5px; padding: 3px 10px; cursor: pointer; }
.small-btn:hover { color: rgba(220,215,200,0.7); }
.sc-prog { height: 4px; background: rgba(255,255,255,0.06); border-radius: 99px; overflow: hidden; margin-top: 2px; }
.sc-prog-bar { height: 100%; background: linear-gradient(90deg, #f0a842, #5b9cf6); transition: width .4s; }
.sc-prog-bar--done { background: linear-gradient(90deg, #4ecfa8, #5b9cf6); }

/* ── test result ── */
.test-result { font-size: 13px; font-family: 'Fira Code', monospace; padding: 6px 12px; border-radius: 6px; }
.test-result--ok  { background: rgba(78,207,168,0.1);  color: #4ecfa8; border: 1px solid rgba(78,207,168,0.25); }
.test-result--err { background: rgba(224,82,82,0.1);   color: #e05252; border: 1px solid rgba(224,82,82,0.25); }

/* ════════════════════════════════════════
   LOG PANEL  (duplicated in template with v-scope trick)
   ════════════════════════════════════════ */
.log-panel { border: 1px solid rgba(255,255,255,0.07); border-radius: 10px; overflow: hidden; margin-top: 16px; }
.log-header { display: flex; align-items: center; gap: 10px; padding: 9px 14px; background: rgba(255,255,255,0.025); border-bottom: 1px solid rgba(255,255,255,0.055); }
.log-title { font-size: 11px; font-weight: 600; color: rgba(220,215,200,0.4); text-transform: uppercase; letter-spacing: .07em; }
.log-count { font-family: 'Fira Code', monospace; font-size: 11px; color: rgba(220,215,200,0.25); }
.log-clear { margin-left: auto; font-size: 11px; color: rgba(220,215,200,0.3); background: none; border: none; cursor: pointer; padding: 2px 6px; border-radius: 4px; }
.log-clear:hover { color: rgba(220,215,200,0.65); background: rgba(255,255,255,0.05); }
.log-box { max-height: 480px; overflow-y: auto; padding: 6px 0; font-family: 'Fira Code', monospace; font-size: 12px; scroll-behavior: smooth; }
.log-box::-webkit-scrollbar { width: 5px; }
.log-box::-webkit-scrollbar-thumb { background: rgba(255,255,255,0.08); border-radius: 99px; }
.log-empty { padding: 36px; text-align: center; color: rgba(220,215,200,0.2); font-size: 12.5px; font-family: 'DM Sans', sans-serif; }
.log-row { display: flex; align-items: baseline; gap: 8px; padding: 3.5px 14px; border-left: 2px solid transparent; }
.log-row:hover { background: rgba(255,255,255,0.02); }
.log-row--done    { border-left-color: rgba(78,207,168,0.5);  background: rgba(78,207,168,0.03); }
.log-row--completed { border-left-color: rgba(78,207,168,0.5); background: rgba(78,207,168,0.03); }
.log-row--error, .log-row--failed { border-left-color: rgba(224,82,82,0.5); background: rgba(224,82,82,0.04); }
.log-row--warning  { border-left-color: rgba(240,168,66,0.4); }
.log-row--saving   { border-left-color: rgba(167,140,232,0.4); }
.log-ts { color: rgba(220,215,200,0.22); font-size: 10.5px; min-width: 86px; white-space: nowrap; }
.log-badge { display: inline-block; font-size: 9.5px; font-weight: 700; padding: 1px 5px; border-radius: 3px; min-width: 40px; text-align: center; letter-spacing: .04em; flex-shrink: 0; }
.badge--start    { background: rgba(220,215,200,0.08); color: rgba(220,215,200,0.5); }
.badge--running  { background: rgba(240,168,66,0.12);  color: #f0a842; }
.badge--ok, .badge--fetched, .badge--fetched_listed, .badge--fetched_otc, .badge--fetched_twse, .badge--fetched_tpex
                 { background: rgba(91,156,246,0.12);  color: #5b9cf6; }
.badge--warning  { background: rgba(240,168,66,0.12);  color: #f0a842; }
.badge--saving   { background: rgba(167,140,232,0.12); color: #a78ce8; }
.badge--done, .badge--completed { background: rgba(78,207,168,0.12); color: #4ecfa8; }
.badge--error, .badge--failed   { background: rgba(224,82,82,0.12);  color: #e05252; }
.badge--req, .badge--fetching_listed, .badge--fetching_otc, .badge--fetching_twse, .badge--fetching_tpex
                 { background: rgba(220,215,200,0.06); color: rgba(220,215,200,0.4); }
.log-src { font-size: 10px; padding: 1px 6px; border-radius: 3px; flex-shrink: 0; font-weight: 600; }
.log-msg { flex: 1; color: rgba(220,215,200,0.65); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.log-row--done .log-msg, .log-row--completed .log-msg { color: #4ecfa8; }
.log-row--error .log-msg, .log-row--failed .log-msg   { color: #e05252; }
.log-row--warning .log-msg { color: #f0a842; }
.log-meta     { font-size: 10.5px; color: rgba(91,156,246,0.6);   white-space: nowrap; flex-shrink: 0; }
.log-meta--acc { color: rgba(78,207,168,0.55); }
.log-elapsed  { font-size: 10px; color: rgba(220,215,200,0.18); white-space: nowrap; flex-shrink: 0; }
</style>
