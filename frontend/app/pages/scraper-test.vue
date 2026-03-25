<script setup lang="ts">
useHead({
  title: '爬蟲測試 | Stock',
  link: [
    { rel: 'preconnect', href: 'https://fonts.googleapis.com' },
    { rel: 'preconnect', href: 'https://fonts.gstatic.com', crossorigin: '' },
    {
      rel: 'stylesheet',
      href: 'https://fonts.googleapis.com/css2?family=DM+Sans:ital,opsz,wght@0,9..40,300;0,9..40,400;0,9..40,500;0,9..40,600;0,9..40,700;1,9..40,400&family=Fira+Code:wght@400;500;600&display=swap',
    },
  ],
})

// ── Tab ───────────────────────────────────────
type ViewTab = 'stream' | 'raw'
const activeTab = ref<ViewTab>('stream')

// ── 型別 (SSE stream) ─────────────────────────

interface SSEEvent {
  stage: string
  message?: string
  progress: number
  url?: string
  total?: number
  synced?: number
  error?: string
}

interface LogEntry {
  ts: string
  stage: string
  message: string
  total?: number
  synced?: number
  url?: string
  elapsed?: number
}

// ── 狀態 ─────────────────────────────────
const symbol = ref('2330')
const running = ref(false)
const logs = ref<LogEntry[]>([])
const progress = ref(0)
const summary = ref<{ records: number; months: number; elapsed: number } | null>(null)
const errorMsg = ref('')

let es: EventSource | null = null
let startTime = 0
let monthCount = 0

// DOM ref for auto-scroll
const logBox = ref<HTMLElement | null>(null)

function nowStr() {
  return new Date().toLocaleTimeString('zh-TW', { hour: '2-digit', minute: '2-digit', second: '2-digit', fractionalSecondDigits: 3 })
}

function elapsedMs() {
  return startTime ? Date.now() - startTime : 0
}

function reset() {
  logs.value = []
  progress.value = 0
  summary.value = null
  errorMsg.value = ''
  monthCount = 0
}

function startFetch() {
  const sym = symbol.value.trim().toUpperCase()
  if (!sym) return
  if (running.value) { es?.close(); es = null; running.value = false; return }

  reset()
  running.value = true
  startTime = Date.now()

  es = new EventSource(`/api/scraper/prices/stock/${sym}`)

  es.onmessage = (e) => {
    let ev: SSEEvent
    try { ev = JSON.parse(e.data) } catch { return }

    progress.value = ev.progress
    const elapsed = elapsedMs()

    const entry: LogEntry = {
      ts: nowStr(),
      stage: ev.stage,
      message: ev.error ?? ev.message ?? ev.stage,
      total: ev.total,
      synced: ev.synced,
      url: ev.url,
      elapsed,
    }
    logs.value.push(entry)

    if (ev.stage === 'fetched') monthCount++

    if (ev.stage === 'done') {
      summary.value = {
        records: ev.synced ?? 0,
        months: monthCount,
        elapsed: elapsed,
      }
      running.value = false
      es?.close()
    }
    if (ev.stage === 'error') {
      errorMsg.value = ev.error ?? '未知錯誤'
      running.value = false
      es?.close()
    }

    nextTick(() => {
      if (logBox.value) logBox.value.scrollTop = logBox.value.scrollHeight
    })
  }

  es.onerror = () => {
    if (running.value) {
      errorMsg.value = '連線中斷'
      logs.value.push({ ts: nowStr(), stage: 'error', message: '連線中斷', elapsed: elapsedMs() })
      running.value = false
    }
    es?.close()
  }
}

const stageLabel: Record<string, string> = {
  start:   'START',
  fetched: 'FETCH',
  warning: 'WARN',
  saving:  'SAVE',
  done:    'DONE',
  error:   'ERROR',
}

// ── 型別 (原始資料解析) ───────────────────────
interface FilterRule {
  name: string
  rule: string
  applied: string
}

interface DebugRow {
  row_num: number
  raw: string[]
  date?: string
  open: number
  high: number
  low: number
  close: number
  volume: number
  tx_value: number
  tx_count: number
  skipped: boolean
  skip_reason?: string
}

interface DebugRawMonth {
  symbol: string
  market: string
  year_month: string
  url: string
  fields: string[]
  raw_count: number
  pass_count: number
  skip_count: number
  skip_reasons: string[]
  rows: DebugRow[]
  filter_rules: FilterRule[]
}

// ── 狀態 (原始資料解析) ───────────────────────
const rawSymbol  = ref('2330')
const rawYyyymm  = ref(() => {
  const d = new Date()
  return `${d.getFullYear()}${String(d.getMonth() + 1).padStart(2, '0')}`
}())
const rawLoading = ref(false)
const rawError   = ref('')
const rawData    = ref<DebugRawMonth | null>(null)
const rowFilter  = ref<'all' | 'pass' | 'skip'>('all')
const rawView    = ref<'raw' | 'parsed' | 'both'>('both')

const filteredRows = computed(() => {
  if (!rawData.value) return []
  const rows = rawData.value.rows
  if (rowFilter.value === 'pass') return rows.filter(r => !r.skipped)
  if (rowFilter.value === 'skip') return rows.filter(r => r.skipped)
  return rows
})

async function fetchRaw() {
  const sym = rawSymbol.value.trim().toUpperCase()
  if (!sym) return
  rawLoading.value = true
  rawError.value = ''
  rawData.value = null
  try {
    rawData.value = await $fetch<DebugRawMonth>(`/api/debug/raw-month?symbol=${sym}&yyyymm=${rawYyyymm.value}`)
  } catch (e: unknown) {
    rawError.value = e instanceof Error ? e.message : String(e)
  } finally {
    rawLoading.value = false
  }
}

function fmtNum(n: number | undefined): string {
  if (n === undefined || n === null) return '—'
  return n.toLocaleString()
}
</script>

<template>
  <div class="page">

    <header class="hd">
      <div class="hd-inner">
        <div class="hd-left">
          <NuxtLink to="/" class="back">← 首頁</NuxtLink>
          <span class="sep">/</span>
          <NuxtLink to="/debug" class="back">Debug</NuxtLink>
          <span class="sep">/</span>
          <span class="cur">爬蟲測試</span>
        </div>
      </div>
    </header>

    <!-- ── Tab 導覽列 ── -->
    <div class="tab-nav">
      <button
        class="tab-btn"
        :class="{ 'tab-btn--active': activeTab === 'stream' }"
        @click="activeTab = 'stream'"
      >SSE 全量串流</button>
      <button
        class="tab-btn"
        :class="{ 'tab-btn--active': activeTab === 'raw' }"
        @click="activeTab = 'raw'"
      >原始資料解析</button>
    </div>

    <div class="body">

      <!-- ════════════════════════════════════════ -->
      <!-- Tab 1: SSE 全量串流                       -->
      <!-- ════════════════════════════════════════ -->
      <template v-if="activeTab === 'stream'">

        <!-- 流程示意 -->
        <div class="flow-diagram">
          <div class="flow-step fs-sys">API /stock/:symbol</div>
          <div class="flow-arrow">→</div>
          <div class="flow-step fs-loop">月份迴圈</div>
          <div class="flow-arrow">→</div>
          <div class="flow-step fs-twse">TWSE / TPEX</div>
          <div class="flow-arrow">→</div>
          <div class="flow-step fs-filter">過濾 &amp; 解析</div>
          <div class="flow-arrow">→</div>
          <div class="flow-step fs-db">DB UPSERT</div>
          <div class="flow-arrow">→</div>
          <div class="flow-step fs-done">done</div>
        </div>

        <!-- 控制列 -->
        <div class="ctrl-row">
          <div class="input-wrap">
            <label class="input-label">股票代號</label>
            <input
              v-model="symbol"
              class="sym-input"
              placeholder="例：2330"
              :disabled="running"
              @keydown.enter="startFetch"
            />
          </div>
          <button class="run-btn" :class="{ 'run-btn--stop': running }" @click="startFetch">
            <span v-if="!running">▶ 開始測試</span>
            <span v-if="running" class="spin">⟳</span>
            <span v-if="running"> 中止</span>
          </button>
        </div>

        <p class="hint">
          呼叫 <code>/api/scraper/prices/stock/:symbol</code>，從當月往回逐月抓取，
          連續 3 個月無資料停止。以下即時顯示每一步 SSE 事件。
        </p>

        <!-- 進度條 -->
        <div v-if="running || progress > 0" class="prog-wrap">
          <div class="prog-bar" :style="{ width: progress + '%' }" :class="{ 'prog-bar--done': !running && progress === 100 }" />
          <span class="prog-label">{{ progress }}%</span>
        </div>

        <!-- 錯誤 -->
        <div v-if="errorMsg" class="error-banner">✖ {{ errorMsg }}</div>

        <!-- 摘要 -->
        <div v-if="summary" class="summary">
          <div class="sum-item">
            <span class="sum-k">取得資料</span>
            <span class="sum-v sum-v--hi">{{ summary.records.toLocaleString() }} 筆</span>
          </div>
          <div class="sum-item">
            <span class="sum-k">月份數</span>
            <span class="sum-v">{{ summary.months }} 個月</span>
          </div>
          <div class="sum-item">
            <span class="sum-k">耗時</span>
            <span class="sum-v">{{ (summary.elapsed / 1000).toFixed(1) }} 秒</span>
          </div>
          <div class="sum-item">
            <span class="sum-k">平均每月</span>
            <span class="sum-v">{{ summary.months ? (summary.elapsed / summary.months / 1000).toFixed(2) : '—' }} 秒</span>
          </div>
        </div>

        <!-- 日誌 -->
        <div class="log-panel">
          <div class="log-header">
            <span class="log-title">事件日誌</span>
            <span class="log-count">{{ logs.length }} 筆</span>
            <button v-if="logs.length" class="clear-btn" @click="reset">清除</button>
          </div>
          <div ref="logBox" class="log-box">
            <div v-if="!logs.length" class="log-empty">尚未開始，請輸入股票代號後按「開始測試」</div>
            <div
              v-for="(entry, i) in logs"
              :key="i"
              class="log-row"
              :class="`log-row--${entry.stage}`"
            >
              <span class="log-ts">{{ entry.ts }}</span>
              <span class="log-badge" :class="`badge--${entry.stage}`">{{ stageLabel[entry.stage] ?? entry.stage.toUpperCase() }}</span>
              <span class="log-msg">{{ entry.message }}</span>
              <span v-if="entry.total" class="log-meta">+{{ entry.total }} 筆</span>
              <span v-if="entry.synced" class="log-meta log-meta--acc">累計 {{ entry.synced?.toLocaleString() }}</span>
              <span class="log-elapsed">+{{ entry.elapsed }}ms</span>
            </div>
          </div>
        </div>

      </template>

      <!-- ════════════════════════════════════════ -->
      <!-- Tab 2: 原始資料解析                        -->
      <!-- ════════════════════════════════════════ -->
      <template v-else>

        <!-- 控制列 -->
        <div class="ctrl-row">
          <div class="input-wrap">
            <label class="input-label">股票代號</label>
            <input v-model="rawSymbol" class="sym-input" placeholder="2330" @keydown.enter="fetchRaw" />
          </div>
          <div class="input-wrap">
            <label class="input-label">年月 YYYYMM</label>
            <input v-model="rawYyyymm" class="sym-input" style="width:110px" placeholder="202503" @keydown.enter="fetchRaw" />
          </div>
          <button class="run-btn" :disabled="rawLoading" @click="fetchRaw">
            <span v-if="!rawLoading">🔍 查詢</span>
            <span v-else class="spin">⟳</span>
          </button>
        </div>

        <p class="hint">
          呼叫 <code>/api/debug/raw-month?symbol=&amp;yyyymm=</code>，
          向 TWSE / TPEX 拉取單月資料，逐列套用過濾規則並回傳解析結果。
        </p>

        <!-- 錯誤 -->
        <div v-if="rawError" class="error-banner">✖ {{ rawError }}</div>

        <!-- ── 結果區 ── -->
        <template v-if="rawData">

          <!-- 統計卡 -->
          <div class="stat-bar">
            <div class="stat-card">
              <span class="stat-k">市場</span>
              <span class="stat-v" :class="rawData.market === 'TWSE' ? 'stat-v--blue' : 'stat-v--purple'">{{ rawData.market }}</span>
            </div>
            <div class="stat-card">
              <span class="stat-k">年月</span>
              <span class="stat-v stat-v--mono">{{ rawData.year_month }}</span>
            </div>
            <div class="stat-card">
              <span class="stat-k">原始列數</span>
              <span class="stat-v stat-v--mono">{{ rawData.raw_count }}</span>
            </div>
            <div class="stat-card">
              <span class="stat-k">通過</span>
              <span class="stat-v stat-v--green">{{ rawData.pass_count }}</span>
            </div>
            <div class="stat-card">
              <span class="stat-k">跳過</span>
              <span class="stat-v stat-v--red">{{ rawData.skip_count }}</span>
            </div>
          </div>

          <!-- 請求 URL -->
          <div class="url-box">
            <span class="url-label">REQUEST URL</span>
            <code class="url-code">{{ rawData.url }}</code>
          </div>

          <!-- API 欄位索引 -->
          <div class="section-title">API 回應欄位</div>
          <div class="fields-row">
            <span v-for="(f, idx) in rawData.fields" :key="idx" class="field-tag">
              <span class="field-idx">[{{ idx }}]</span> {{ f }}
            </span>
          </div>

          <!-- 過濾規則 -->
          <div class="section-title">過濾規則</div>
          <div class="rules-grid">
            <div v-for="(rule, idx) in rawData.filter_rules" :key="idx" class="rule-card">
              <div class="rule-name">
                <span class="rule-num">{{ idx + 1 }}</span>
                {{ rule.name }}
              </div>
              <code class="rule-expr">{{ rule.rule }}</code>
              <div class="rule-desc">{{ rule.applied }}</div>
            </div>
          </div>

          <!-- 跳過原因 -->
          <div v-if="rawData.skip_reasons && rawData.skip_reasons.length" class="skip-badges">
            <span class="section-title" style="margin: 0; font-size: 11px;">跳過原因</span>
            <span v-for="r in rawData.skip_reasons" :key="r" class="skip-badge">{{ r }}</span>
          </div>

          <!-- 資料表格 -->
          <div class="section-title" style="margin-top:20px">
            逐列資料
            <span class="tbl-filter-btns">
              <button v-for="f in ['all','pass','skip'] as const" :key="f" class="filter-btn" :class="{ 'filter-btn--active': rowFilter === f }" @click="rowFilter = f">{{ { all: '全部', pass: '通過', skip: '跳過' }[f] }}</button>
            </span>
            <span class="tbl-view-btns">
              <button v-for="v in ['raw','parsed','both'] as const" :key="v" class="filter-btn filter-btn--sm" :class="{ 'filter-btn--active': rawView === v }" @click="rawView = v">{{ { raw: 'Raw', parsed: '解析', both: '全欄' }[v] }}</button>
            </span>
          </div>

          <div class="tbl-wrap">
            <table class="data-tbl">
              <thead>
                <tr>
                  <th>#</th>
                  <th v-if="rawView === 'raw' || rawView === 'both'" v-for="(f, fi) in rawData.fields" :key="'f'+fi" class="th-raw">[{{ fi }}] {{ f }}</th>
                  <th v-if="rawView === 'parsed' || rawView === 'both'">日期</th>
                  <th v-if="rawView === 'parsed' || rawView === 'both'">開盤</th>
                  <th v-if="rawView === 'parsed' || rawView === 'both'">最高</th>
                  <th v-if="rawView === 'parsed' || rawView === 'both'">最低</th>
                  <th v-if="rawView === 'parsed' || rawView === 'both'">收盤</th>
                  <th v-if="rawView === 'parsed' || rawView === 'both'">成交量</th>
                  <th>狀態</th>
                </tr>
              </thead>
              <tbody>
                <tr
                  v-for="row in filteredRows"
                  :key="row.row_num"
                  :class="{ 'tr-skip': row.skipped }"
                >
                  <td class="td-num">{{ row.row_num }}</td>
                  <td v-if="rawView === 'raw' || rawView === 'both'" v-for="(cell, ci) in row.raw" :key="'c'+ci" class="td-raw" :class="{ 'td-date': ci === 0 }">{{ cell }}</td>
                  <td v-if="rawView === 'parsed' || rawView === 'both'" class="td-date">{{ row.date || '—' }}</td>
                  <td v-if="rawView === 'parsed' || rawView === 'both'" class="td-price">{{ fmtNum(row.open) }}</td>
                  <td v-if="rawView === 'parsed' || rawView === 'both'" class="td-price td-high">{{ fmtNum(row.high) }}</td>
                  <td v-if="rawView === 'parsed' || rawView === 'both'" class="td-price td-low">{{ fmtNum(row.low) }}</td>
                  <td v-if="rawView === 'parsed' || rawView === 'both'" class="td-price">{{ fmtNum(row.close) }}</td>
                  <td v-if="rawView === 'parsed' || rawView === 'both'" class="td-vol">{{ fmtNum(row.volume) }}</td>
                  <td class="td-status">
                    <span v-if="!row.skipped" class="badge-pass">PASS</span>
                    <template v-else>
                      <span class="badge-skip">SKIP</span>
                      <span v-if="row.skip_reason" class="skip-reason">{{ row.skip_reason }}</span>
                    </template>
                  </td>
                </tr>
              </tbody>
            </table>
          </div>

        </template>

        <!-- 空狀態 -->
        <div v-else-if="!rawLoading" class="log-empty" style="padding:60px 0; text-align:center; color:rgba(220,215,200,0.22)">
          輸入股票代號與年月後按「查詢」，即可看到該月原始 API 資料與過濾細節
        </div>
        <div v-else class="log-empty" style="padding:60px 0; text-align:center; color:rgba(220,215,200,0.4)">
          <span class="spin" style="font-size:20px">⟳</span> 載入中…
        </div>

      </template>

    </div>
  </div>
</template>

<style scoped>
*, *::before, *::after { box-sizing: border-box; margin: 0; padding: 0; }

.page {
  min-height: 100vh;
  background: #0f1117;
  color: #e2e0d8;
  font-family: 'DM Sans', system-ui, sans-serif;
}

/* ── header ── */
.hd { border-bottom: 1px solid rgba(255,255,255,0.07); }
.hd-inner { max-width: 1000px; margin: 0 auto; padding: 14px 24px; display: flex; align-items: center; }
.hd-left { display: flex; align-items: center; gap: 8px; font-size: 13px; }
.back { color: rgba(220,215,200,0.55); text-decoration: none; }
.back:hover { color: #e2e0d8; }
.sep { color: rgba(220,215,200,0.25); }
.cur { color: #e2e0d8; font-weight: 500; }

/* ── body ── */
.body { max-width: 1000px; margin: 0 auto; padding: 32px 24px; }

/* ── 控制列 ── */
.ctrl-row { display: flex; gap: 12px; align-items: flex-end; flex-wrap: wrap; }
.input-wrap { display: flex; flex-direction: column; gap: 6px; }
.input-label { font-size: 11px; color: rgba(220,215,200,0.45); letter-spacing: .06em; text-transform: uppercase; }
.sym-input {
  height: 38px; padding: 0 14px;
  background: rgba(255,255,255,0.05);
  border: 1px solid rgba(255,255,255,0.1);
  border-radius: 8px;
  color: #e2e0d8;
  font-family: 'Fira Code', monospace;
  font-size: 15px;
  width: 130px;
  outline: none;
  transition: border-color .15s;
}
.sym-input:focus { border-color: rgba(91,156,246,0.6); }
.sym-input:disabled { opacity: .5; cursor: not-allowed; }

.run-btn {
  height: 38px; padding: 0 20px;
  background: rgba(91,156,246,0.15);
  border: 1px solid rgba(91,156,246,0.35);
  border-radius: 8px;
  color: #5b9cf6;
  font-size: 14px; font-weight: 500;
  cursor: pointer;
  transition: background .15s, border-color .15s;
  display: flex; align-items: center; gap: 6px;
}
.run-btn:hover { background: rgba(91,156,246,0.25); border-color: rgba(91,156,246,0.6); }
.run-btn--stop { background: rgba(224,82,82,0.12); border-color: rgba(224,82,82,0.3); color: #e05252; }
.run-btn--stop:hover { background: rgba(224,82,82,0.22); }
@keyframes spin { to { transform: rotate(360deg); } }
.spin { display: inline-block; animation: spin .8s linear infinite; }

/* ── hint ── */
.hint { margin-top: 16px; font-size: 12.5px; color: rgba(220,215,200,0.38); line-height: 1.6; }
.hint code { font-family: 'Fira Code', monospace; color: rgba(91,156,246,0.75); font-size: .95em; }

/* ── 進度條 ── */
.prog-wrap {
  margin-top: 20px;
  height: 6px;
  background: rgba(255,255,255,0.07);
  border-radius: 99px;
  position: relative;
  overflow: hidden;
}
.prog-bar {
  height: 100%;
  background: linear-gradient(90deg, #5b9cf6, #a78ce8);
  border-radius: 99px;
  transition: width .3s ease;
}
.prog-bar--done { background: linear-gradient(90deg, #4ecfa8, #5b9cf6); }
.prog-label {
  position: absolute;
  right: 0; top: -18px;
  font-size: 11px;
  color: rgba(220,215,200,0.4);
}

/* ── error ── */
.error-banner {
  margin-top: 16px;
  padding: 10px 16px;
  background: rgba(224,82,82,0.1);
  border: 1px solid rgba(224,82,82,0.25);
  border-radius: 8px;
  color: #e05252;
  font-size: 13px;
}

/* ── summary ── */
.summary {
  margin-top: 20px;
  display: flex; gap: 0;
  background: rgba(78,207,168,0.06);
  border: 1px solid rgba(78,207,168,0.18);
  border-radius: 10px;
  overflow: hidden;
}
.sum-item {
  flex: 1;
  display: flex; flex-direction: column;
  padding: 14px 18px;
  gap: 4px;
  border-right: 1px solid rgba(78,207,168,0.12);
}
.sum-item:last-child { border-right: none; }
.sum-k { font-size: 11px; color: rgba(220,215,200,0.38); text-transform: uppercase; letter-spacing: .06em; }
.sum-v { font-family: 'Fira Code', monospace; font-size: 17px; color: rgba(220,215,200,0.85); }
.sum-v--hi { color: #4ecfa8; }

/* ── 日誌 ── */
.log-panel {
  margin-top: 24px;
  border: 1px solid rgba(255,255,255,0.07);
  border-radius: 12px;
  overflow: hidden;
}
.log-header {
  display: flex; align-items: center; gap: 10px;
  padding: 10px 16px;
  background: rgba(255,255,255,0.03);
  border-bottom: 1px solid rgba(255,255,255,0.06);
}
.log-title { font-size: 12px; font-weight: 600; color: rgba(220,215,200,0.55); text-transform: uppercase; letter-spacing: .06em; }
.log-count { font-family: 'Fira Code', monospace; font-size: 11px; color: rgba(220,215,200,0.3); }
.clear-btn { margin-left: auto; font-size: 11px; color: rgba(220,215,200,0.35); background: none; border: none; cursor: pointer; padding: 2px 6px; border-radius: 4px; }
.clear-btn:hover { color: rgba(220,215,200,0.7); background: rgba(255,255,255,0.06); }

.log-box {
  max-height: 520px;
  overflow-y: auto;
  padding: 8px 0;
  font-family: 'Fira Code', monospace;
  font-size: 12.5px;
  scroll-behavior: smooth;
}
.log-box::-webkit-scrollbar { width: 6px; }
.log-box::-webkit-scrollbar-track { background: transparent; }
.log-box::-webkit-scrollbar-thumb { background: rgba(255,255,255,0.1); border-radius: 99px; }

.log-empty { padding: 40px; text-align: center; color: rgba(220,215,200,0.22); font-size: 13px; font-family: 'DM Sans', sans-serif; }

.log-row {
  display: flex; align-items: baseline; gap: 10px;
  padding: 4px 16px;
  border-left: 2px solid transparent;
  transition: background .1s;
}
.log-row:hover { background: rgba(255,255,255,0.025); }

.log-row--fetched { border-left-color: rgba(91,156,246,0.4); }
.log-row--done    { border-left-color: rgba(78,207,168,0.6); background: rgba(78,207,168,0.04); }
.log-row--error   { border-left-color: rgba(224,82,82,0.6);  background: rgba(224,82,82,0.05); }
.log-row--warning { border-left-color: rgba(240,168,66,0.5); }
.log-row--saving  { border-left-color: rgba(167,140,232,0.5); }
.log-row--start   { border-left-color: rgba(220,215,200,0.2); }

.log-ts { color: rgba(220,215,200,0.28); font-size: 11px; white-space: nowrap; min-width: 88px; }

.log-badge {
  display: inline-block;
  font-size: 10px; font-weight: 600;
  padding: 1px 6px;
  border-radius: 4px;
  min-width: 46px; text-align: center;
  letter-spacing: .04em;
  flex-shrink: 0;
}
.badge--start   { background: rgba(220,215,200,0.1);  color: rgba(220,215,200,0.55); }
.badge--fetched { background: rgba(91,156,246,0.15); color: #5b9cf6; }
.badge--warning { background: rgba(240,168,66,0.15); color: #f0a842; }
.badge--saving  { background: rgba(167,140,232,0.15); color: #a78ce8; }
.badge--done    { background: rgba(78,207,168,0.15);  color: #4ecfa8; }
.badge--error   { background: rgba(224,82,82,0.15);   color: #e05252; }

.log-msg { flex: 1; color: rgba(220,215,200,0.72); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.log-row--done .log-msg   { color: #4ecfa8; font-weight: 500; }
.log-row--error .log-msg  { color: #e05252; }
.log-row--warning .log-msg { color: #f0a842; }

.log-meta {
  font-size: 11px;
  color: rgba(91,156,246,0.65);
  white-space: nowrap;
  flex-shrink: 0;
}
.log-meta--acc { color: rgba(78,207,168,0.55); }

.log-elapsed { font-size: 11px; color: rgba(220,215,200,0.2); white-space: nowrap; flex-shrink: 0; }
</style>
