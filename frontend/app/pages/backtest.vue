<script setup lang="ts">
useHead({ title: '歷史回測 | 台股分析' })
import { useAppPrefs } from '~/composables/useAppPrefs'
const { isDark, toggleTheme } = useAppPrefs()

// ── 型別 ──────────────────────────────────────────────────────────────────
interface StrategyParams {
  entry_ma_short:    number
  entry_ma_long:     number
  exit_ma_short:     number
  exit_ma_long:      number
  capital_per_trade: number
  max_positions:     number
  stop_loss_pct:     number
  take_profit_pct:   number
  max_hold_days:     number
  fee_rate:          number
  tax_rate:          number
}
interface BacktestJob {
  id:            number
  symbol:        string
  start_date:    string
  end_date:      string
  capital:       number
  status:        string
  progress:      number
  total_return:  number
  annual_return: number
  max_drawdown:  number
  win_rate:      number
  sharpe_ratio:  number
  total_trades:  number
  started_at:    string
  completed_at:  string | null
}
interface Trade {
  id:          number
  symbol:      string
  entry_date:  string
  exit_date:   string
  entry_price: number
  exit_price:  number
  shares:      number
  pnl:         number
  pnl_pct:     number
  hold_days:   number
  exit_reason: string
}
interface EquityPoint {
  date:   string
  equity: number
  cash:   number
}

// ── 策略參數 ────────────────────────────────────────────────────────────────
const symbol = ref('2330')
const startDate = ref(new Date(new Date().getFullYear() - 2, 0, 1).toISOString().split('T')[0])
const endDate   = ref(new Date().toISOString().split('T')[0])
const capital   = ref(1_000_000)

const params = ref<StrategyParams>({
  entry_ma_short:    5,
  entry_ma_long:     20,
  exit_ma_short:     5,
  exit_ma_long:      20,
  capital_per_trade: 0.1,
  max_positions:     5,
  stop_loss_pct:    -0.07,
  take_profit_pct:   0.15,
  max_hold_days:     60,
  fee_rate:          0.001425,
  tax_rate:          0.003,
})

// ── 結果 ─────────────────────────────────────────────────────────────────────
const running    = ref(false)
const error      = ref('')
const resultJob  = ref<BacktestJob | null>(null)
const trades     = ref<Trade[]>([])
const equityCurve = ref<EquityPoint[]>([])

// ── 歷史記錄 ─────────────────────────────────────────────────────────────────
const { data: jobList, refresh: refreshList } = useFetch<BacktestJob[]>(
  '/api/backtest/jobs',
  { default: () => [] }
)

// ── 執行 ──────────────────────────────────────────────────────────────────────
async function runBacktest() {
  if (running.value) return
  running.value = true
  error.value = ''
  resultJob.value = null
  trades.value = []
  equityCurve.value = []

  try {
    const res = await $fetch<{ job: BacktestJob; trades: Trade[]; equity: EquityPoint[] }>(
      '/api/backtest/run',
      {
        method: 'POST',
        body: {
          symbol: symbol.value,
          start_date: startDate.value,
          end_date: endDate.value,
          capital: capital.value,
          params: params.value,
        },
      }
    )
    resultJob.value = res.job
    trades.value    = res.trades ?? []
    equityCurve.value = res.equity ?? []
    refreshList()
  } catch (e: any) {
    error.value = e?.data?.error ?? '執行失敗'
  } finally {
    running.value = false
  }
}

async function loadJob(id: number) {
  error.value = ''
  const res = await $fetch<{ job: BacktestJob; trades: Trade[]; equity: EquityPoint[] }>(
    `/api/backtest/jobs/${id}`
  )
  resultJob.value = res.job
  trades.value    = res.trades ?? []
  equityCurve.value = res.equity ?? []
  // 還原設定
  symbol.value    = res.job.symbol
  startDate.value = res.job.start_date?.split('T')[0] ?? startDate.value
  endDate.value   = res.job.end_date?.split('T')[0] ?? endDate.value
  capital.value   = res.job.capital
}

async function deleteJob(id: number, e: Event) {
  e.stopPropagation()
  await $fetch(`/api/backtest/jobs/${id}`, { method: 'DELETE' })
  if (resultJob.value?.id === id) {
    resultJob.value = null
    trades.value = []
    equityCurve.value = []
  }
  refreshList()
}

// ── 圖表（Canvas 策略淨值曲線）──────────────────────────────────────────────
const canvasRef = ref<HTMLCanvasElement | null>(null)

function drawEquityCurve() {
  const canvas = canvasRef.value
  if (!canvas || equityCurve.value.length < 2) return
  const ctx = canvas.getContext('2d')
  if (!ctx) return

  const dpr = window.devicePixelRatio || 1
  const W   = canvas.clientWidth
  const H   = canvas.clientHeight
  canvas.width  = W * dpr
  canvas.height = H * dpr
  ctx.scale(dpr, dpr)
  ctx.clearRect(0, 0, W, H)

  const data = equityCurve.value
  const padL = 70, padR = 20, padT = 16, padB = 30

  // 找 min/max
  let minY = Infinity, maxY = -Infinity
  for (const d of data) {
    if (d.equity < minY) minY = d.equity
    if (d.equity > maxY) maxY = d.equity
  }
  const rangeY = maxY - minY || 1
  const chartW = W - padL - padR
  const chartH = H - padT - padB

  const toX = (i: number) => padL + (i / (data.length - 1)) * chartW
  const toY = (v: number) => padT + chartH - ((v - minY) / rangeY) * chartH

  // 背景格線
  const lineColor = isDark.value ? 'rgba(255,255,255,0.07)' : 'rgba(0,0,0,0.06)'
  ctx.strokeStyle = lineColor
  ctx.lineWidth = 1
  for (let ri = 0; ri <= 4; ri++) {
    const y = padT + (ri / 4) * chartH
    ctx.beginPath(); ctx.moveTo(padL, y); ctx.lineTo(W - padR, y); ctx.stroke()
  }

  // 初始資金基準線
  if (resultJob.value && resultJob.value.capital) {
    const baseY = toY(resultJob.value.capital)
    ctx.strokeStyle = isDark.value ? 'rgba(255,255,255,0.25)' : 'rgba(0,0,0,0.18)'
    ctx.setLineDash([4, 3])
    ctx.beginPath(); ctx.moveTo(padL, baseY); ctx.lineTo(W - padR, baseY); ctx.stroke()
    ctx.setLineDash([])
  }

  // 填色漸層
  const finalEquity = data[data.length - 1].equity
  const profit = finalEquity > (resultJob.value?.capital ?? finalEquity)
  const baseColor = profit ? '34,197,94' : '239,68,68'
  const grad = ctx.createLinearGradient(0, padT, 0, padT + chartH)
  grad.addColorStop(0, `rgba(${baseColor}, 0.25)`)
  grad.addColorStop(1, `rgba(${baseColor}, 0.0)`)
  ctx.fillStyle = grad
  ctx.beginPath()
  ctx.moveTo(toX(0), toY(data[0].equity))
  for (let i = 1; i < data.length; i++) {
    ctx.lineTo(toX(i), toY(data[i].equity))
  }
  ctx.lineTo(toX(data.length - 1), padT + chartH)
  ctx.lineTo(toX(0), padT + chartH)
  ctx.closePath()
  ctx.fill()

  // 線條
  ctx.strokeStyle = profit ? '#22c55e' : '#ef4444'
  ctx.lineWidth = 2
  ctx.lineJoin = 'round'
  ctx.beginPath()
  ctx.moveTo(toX(0), toY(data[0].equity))
  for (let i = 1; i < data.length; i++) {
    ctx.lineTo(toX(i), toY(data[i].equity))
  }
  ctx.stroke()

  // Y 軸標籤
  ctx.fillStyle = isDark.value ? 'rgba(255,255,255,0.45)' : 'rgba(0,0,0,0.45)'
  ctx.font = `10px 'DM Sans', system-ui`
  ctx.textAlign = 'right'
  for (let ri = 0; ri <= 4; ri++) {
    const v = minY + (1 - ri / 4) * rangeY
    const y = padT + (ri / 4) * chartH
    const label = v >= 1_000_000
      ? `${(v / 1_000_000).toFixed(2)}M`
      : v >= 1_000 ? `${(v / 1_000).toFixed(0)}k` : v.toFixed(0)
    ctx.fillText(label, padL - 5, y + 4)
  }
  // X 軸少量日期
  ctx.textAlign = 'center'
  const step = Math.ceil(data.length / 5)
  for (let i = 0; i < data.length; i += step) {
    ctx.fillText(data[i].date.slice(2), toX(i), H - 8)
  }
}

watch(equityCurve, async () => {
  await nextTick()
  drawEquityCurve()
}, { deep: false })

onMounted(() => {
  const ro = new ResizeObserver(drawEquityCurve)
  if (canvasRef.value?.parentElement) ro.observe(canvasRef.value.parentElement)
})

// ── 統計 helpers ──────────────────────────────────────────────────────────────
function clrReturn(v: number) { return v > 0 ? '#22c55e' : v < 0 ? '#ef4444' : undefined }
function pct(v: number) { return `${v > 0 ? '+' : ''}${v.toFixed(2)}%` }
function fmtPrice(v: number) { return v.toLocaleString('zh-TW', { minimumFractionDigits: 2, maximumFractionDigits: 2 }) }

const winTrades = computed(() => trades.value.filter(t => t.pnl > 0))
const lossTrades = computed(() => trades.value.filter(t => t.pnl <= 0))
const avgPnl = computed(() => {
  if (!trades.value.length) return 0
  return trades.value.reduce((s, t) => s + t.pnl, 0) / trades.value.length
})
</script>

<template>
  <div class="page" :class="{ dark: isDark }">
    <header class="hdr">
      <div class="hdr-inner">
        <div class="hdr-brand">
          <NuxtLink to="/" class="back-link">← 首頁</NuxtLink>
          <span class="sep">/</span>
          <span class="cur-page">歷史回測</span>
        </div>
        <button class="btn-icon" @click="toggleTheme">
          <svg v-if="isDark" width="16" height="16" viewBox="0 0 16 16" fill="none"><circle cx="8" cy="8" r="2.8" fill="currentColor"/><path d="M8 1.5V3M8 13v1.5M1.5 8H3M13 8h1.5M3.4 3.4l1.06 1.06M11.54 11.54l1.06 1.06M3.4 12.6l1.06-1.06M11.54 4.46l1.06-1.06" stroke="currentColor" stroke-width="1.4" stroke-linecap="round"/></svg>
          <svg v-else width="16" height="16" viewBox="0 0 16 16" fill="none"><path d="M13.2 9.3A5.8 5.8 0 0 1 6.7 2.8a.4.4 0 0 0-.46-.5A6.3 6.3 0 1 0 13.7 9.76a.4.4 0 0 0-.5-.46Z" fill="currentColor"/></svg>
        </button>
      </div>
    </header>

    <div class="layout">
      <!-- ── 左側面板：策略設定 + 歷史記錄 ── -->
      <aside class="sidebar">
        <div class="sidebar-section">
          <div class="sidebar-label">策略設定</div>

          <label class="form-field">
            <span>股票代碼</span>
            <input v-model="symbol" type="text" placeholder="e.g. 2330" class="fi" />
          </label>
          <label class="form-field">
            <span>開始日期</span>
            <input v-model="startDate" type="date" class="fi" />
          </label>
          <label class="form-field">
            <span>結束日期</span>
            <input v-model="endDate" type="date" class="fi" />
          </label>
          <label class="form-field">
            <span>初始資金（元）</span>
            <input v-model.number="capital" type="number" step="100000" class="fi" />
          </label>

          <div class="param-section-title">均線訊號（黃金/死叉）</div>
          <label class="form-field">
            <span>進場短均（{{ params.entry_ma_short }}日）</span>
            <input v-model.number="params.entry_ma_short" type="range" min="3" max="60" class="slider" />
          </label>
          <label class="form-field">
            <span>進場長均（{{ params.entry_ma_long }}日）</span>
            <input v-model.number="params.entry_ma_long" type="range" min="5" max="200" class="slider" />
          </label>

          <div class="param-section-title">風控</div>
          <label class="form-field">
            <span>停損 {{ (params.stop_loss_pct * 100).toFixed(0) }}%</span>
            <input v-model.number="params.stop_loss_pct" type="range" min="-0.30" max="-0.01" step="0.01" class="slider" />
          </label>
          <label class="form-field">
            <span>停利 {{ (params.take_profit_pct * 100).toFixed(0) }}%</span>
            <input v-model.number="params.take_profit_pct" type="range" min="0.05" max="1.00" step="0.01" class="slider" />
          </label>
          <label class="form-field">
            <span>最多持倉（{{ params.max_positions }} 支）</span>
            <input v-model.number="params.max_positions" type="range" min="1" max="20" class="slider" />
          </label>
          <label class="form-field">
            <span>每次動用資金 {{ (params.capital_per_trade * 100).toFixed(0) }}%</span>
            <input v-model.number="params.capital_per_trade" type="range" min="0.05" max="1.00" step="0.05" class="slider" />
          </label>
          <label class="form-field">
            <span>最長持倉（{{ params.max_hold_days }} 天）</span>
            <input v-model.number="params.max_hold_days" type="range" min="5" max="365" step="5" class="slider" />
          </label>

          <button class="run-btn" :disabled="running" @click="runBacktest">
            <svg v-if="!running" width="13" height="13" viewBox="0 0 16 16" fill="none"><polygon points="4,2 14,8 4,14" fill="currentColor"/></svg>
            <svg v-else width="13" height="13" viewBox="0 0 16 16" fill="none" class="spinning"><path d="M8 2a6 6 0 1 0 6 6" stroke="currentColor" stroke-width="2" stroke-linecap="round"/></svg>
            {{ running ? '計算中…' : '執行回測' }}
          </button>
          <div v-if="error" class="err-msg">{{ error }}</div>
        </div>

        <!-- 歷史記錄 -->
        <div class="sidebar-section" v-if="jobList && jobList.length">
          <div class="sidebar-label">歷史記錄</div>
          <div
            v-for="job in jobList"
            :key="job.id"
            class="job-item"
            :class="{ active: resultJob?.id === job.id }"
            @click="loadJob(job.id)"
          >
            <div class="job-item-top">
              <span class="job-symbol">{{ job.symbol }}</span>
              <span class="job-return" :style="{ color: clrReturn(job.total_return) }">
                {{ pct(job.total_return) }}
              </span>
              <button class="del-btn" @click="deleteJob(job.id, $event)" title="刪除">×</button>
            </div>
            <div class="job-item-sub">{{ job.start_date?.split('T')[0] }} ~ {{ job.end_date?.split('T')[0] }}</div>
          </div>
        </div>
      </aside>

      <!-- ── 主視覺：結果 ── -->
      <main class="main-area">
        <div v-if="!resultJob && !running" class="empty-result">
          <div class="empty-icon">📈</div>
          <p>設定策略參數後，點擊「執行回測」</p>
          <p class="empty-sub">目前支援：均線黃金/死叉策略，含停損停利、資金管理</p>
        </div>

        <div v-else-if="running" class="empty-result">
          <div class="empty-icon spinning-big">⏳</div>
          <p>計算中…</p>
        </div>

        <div v-else-if="resultJob" class="result-wrap">
          <!-- 標題 -->
          <div class="result-header">
            <div>
              <NuxtLink :to="`/stocks/${resultJob.symbol}`" class="result-symbol">{{ resultJob.symbol }}</NuxtLink>
              <span class="result-period">{{ resultJob.start_date?.split('T')[0] }} ~ {{ resultJob.end_date?.split('T')[0] }}</span>
            </div>
            <span class="result-capital">初始資金 {{ resultJob.capital.toLocaleString() }} 元</span>
          </div>

          <!-- 統計卡 -->
          <div class="stat-cards">
            <div class="stat-card">
              <span class="sc-label">總報酬率</span>
              <span class="sc-val big" :style="{ color: clrReturn(resultJob.total_return) }">{{ pct(resultJob.total_return) }}</span>
            </div>
            <div class="stat-card">
              <span class="sc-label">年化報酬</span>
              <span class="sc-val" :style="{ color: clrReturn(resultJob.annual_return) }">{{ pct(resultJob.annual_return) }}</span>
            </div>
            <div class="stat-card">
              <span class="sc-label">最大回撤</span>
              <span class="sc-val down">-{{ resultJob.max_drawdown.toFixed(2) }}%</span>
            </div>
            <div class="stat-card">
              <span class="sc-label">勝率</span>
              <span class="sc-val">{{ resultJob.win_rate.toFixed(1) }}%</span>
            </div>
            <div class="stat-card">
              <span class="sc-label">夏普比率</span>
              <span class="sc-val" :style="{ color: clrReturn(resultJob.sharpe_ratio) }">{{ resultJob.sharpe_ratio.toFixed(2) }}</span>
            </div>
            <div class="stat-card">
              <span class="sc-label">交易次數</span>
              <span class="sc-val">{{ resultJob.total_trades }}</span>
            </div>
            <div class="stat-card">
              <span class="sc-label">盈虧交易</span>
              <span class="sc-val">
                <span style="color:#22c55e">{{ winTrades.length }}</span>
                <span style="color:var(--t3)">/</span>
                <span style="color:#ef4444">{{ lossTrades.length }}</span>
              </span>
            </div>
            <div class="stat-card">
              <span class="sc-label">均損益</span>
              <span class="sc-val" :style="{ color: clrReturn(avgPnl) }">{{ avgPnl.toFixed(0) }} 元</span>
            </div>
          </div>

          <!-- 淨值曲線 -->
          <div class="equity-panel">
            <div class="panel-title">淨值曲線</div>
            <div class="chart-area" :style="{ height: '240px' }">
              <canvas ref="canvasRef" style="width:100%;height:100%;" />
            </div>
          </div>

          <!-- 交易記錄 -->
          <div class="trades-panel" v-if="trades.length">
            <div class="panel-title">交易記錄（{{ trades.length }} 筆）</div>
            <div class="trades-table-wrap">
              <table class="trades-table">
                <thead>
                  <tr>
                    <th>進場日</th>
                    <th>出場日</th>
                    <th class="ra">進場價</th>
                    <th class="ra">出場價</th>
                    <th class="ra">股數</th>
                    <th class="ra">損益</th>
                    <th class="ra">損益%</th>
                    <th class="ra">持倉天</th>
                    <th>出場原因</th>
                  </tr>
                </thead>
                <tbody>
                  <tr v-for="t in trades" :key="t.id" :class="t.pnl > 0 ? 'tr-win' : 'tr-loss'">
                    <td>{{ t.entry_date?.split('T')[0] }}</td>
                    <td>{{ t.exit_date?.split('T')[0] }}</td>
                    <td class="ra">{{ fmtPrice(t.entry_price) }}</td>
                    <td class="ra">{{ fmtPrice(t.exit_price) }}</td>
                    <td class="ra">{{ t.shares.toLocaleString() }}</td>
                    <td class="ra" :class="t.pnl > 0 ? 'col-green' : 'col-red'">{{ t.pnl.toFixed(0) }}</td>
                    <td class="ra" :class="t.pnl_pct > 0 ? 'col-green' : 'col-red'">{{ pct(t.pnl_pct) }}</td>
                    <td class="ra muted">{{ t.hold_days }}</td>
                    <td class="reason-tag">
                      <span :class="`tag-${t.exit_reason}`">{{ { stop_loss: '停損', take_profit: '停利', max_hold: '到期', signal: '訊號', end_date: '結束' }[t.exit_reason] ?? t.exit_reason }}</span>
                    </td>
                  </tr>
                </tbody>
              </table>
            </div>
          </div>
        </div>
      </main>
    </div>
  </div>
</template>

<style scoped>
*, *::before, *::after { box-sizing: border-box; margin: 0; padding: 0; }

.page {
  --bg: #f5f5f0;
  --surface: #fff;
  --surface2: #f0efea;
  --t1: #1a1c2e;
  --t2: #4a4d68;
  --t3: #9395a8;
  --line: rgba(0,0,0,0.09);
  --gold: #c4922a;
  min-height: 100dvh;
  background: var(--bg);
  color: var(--t1);
  font-family: 'DM Sans', system-ui, sans-serif;
  display: flex; flex-direction: column;
}
.page.dark {
  --bg: #13141f;
  --surface: #1c1e2f;
  --surface2: #252738;
  --t1: #e8e6d8;
  --t2: #9395a8;
  --t3: #606278;
  --line: rgba(255,255,255,0.07);
  --gold: #d4a63a;
}

.hdr { background: var(--surface); border-bottom: 1px solid var(--line); position: sticky; top: 0; z-index: 100; }
.hdr-inner { max-width: 100%; padding: 0 24px; height: 52px; display: flex; align-items: center; justify-content: space-between; }
.hdr-brand { display: flex; align-items: center; gap: 8px; }
.back-link { color: var(--t3); font-size: 13px; text-decoration: none; }
.back-link:hover { color: var(--t1); }
.sep { color: var(--t3); font-size: 12px; }
.cur-page { font-size: 13px; font-weight: 600; }
.btn-icon { background: none; border: none; cursor: pointer; color: var(--t2); padding: 6px; border-radius: 6px; }

.layout { display: flex; flex: 1; min-height: 0; }

/* ── Sidebar ── */
.sidebar {
  width: 280px; flex-shrink: 0;
  border-right: 1px solid var(--line);
  background: var(--surface);
  overflow-y: auto;
  display: flex; flex-direction: column; gap: 0;
}
.sidebar-section {
  padding: 16px;
  border-bottom: 1px solid var(--line);
}
.sidebar-label {
  font-size: 10.5px; font-weight: 700; letter-spacing: 0.12em; text-transform: uppercase;
  color: var(--t3); margin-bottom: 12px;
}
.form-field {
  display: flex; flex-direction: column; gap: 4px; margin-bottom: 10px;
}
.form-field > span {
  font-size: 11px; color: var(--t3);
}
.fi {
  padding: 6px 10px; font-size: 13px;
  background: var(--surface2); border: 1px solid var(--line);
  border-radius: 6px; color: var(--t1); outline: none; width: 100%;
}
.fi:focus { border-color: var(--gold); }
.slider { width: 100%; accent-color: var(--gold); cursor: pointer; }

.param-section-title {
  font-size: 10px; font-weight: 700; letter-spacing: 0.1em;
  text-transform: uppercase; color: var(--t3);
  margin: 14px 0 8px; padding-top: 10px; border-top: 1px solid var(--line);
}

.run-btn {
  display: flex; align-items: center; justify-content: center; gap: 7px;
  width: 100%; margin-top: 14px; padding: 10px 0;
  background: var(--gold); color: #fff; border: none; border-radius: 8px;
  font-size: 13px; font-weight: 700; cursor: pointer; transition: opacity 0.15s;
}
.run-btn:disabled { opacity: 0.55; cursor: not-allowed; }
.err-msg { margin-top: 8px; font-size: 12px; color: #ef4444; text-align: center; }

.job-item {
  padding: 9px 10px; border-radius: 8px; cursor: pointer;
  transition: background 0.12s; margin-bottom: 4px;
  border: 1px solid transparent;
}
.job-item:hover { background: var(--surface2); }
.job-item.active { border-color: var(--gold); background: rgba(196,146,42,0.07); }
.job-item-top { display: flex; align-items: center; gap: 8px; }
.job-symbol { font-size: 13px; font-weight: 700; flex: 1; }
.job-return { font-size: 13px; font-weight: 700; }
.del-btn { background: none; border: none; color: var(--t3); cursor: pointer; font-size: 14px; padding: 0 2px; transition: color 0.12s; }
.del-btn:hover { color: #ef4444; }
.job-item-sub { font-size: 10.5px; color: var(--t3); margin-top: 3px; }

/* ── Main ── */
.main-area { flex: 1; overflow-y: auto; padding: 24px; }

.empty-result { text-align: center; padding: 80px 20px; color: var(--t3); }
.empty-icon { font-size: 48px; margin-bottom: 16px; }
.empty-result p { font-size: 15px; margin-bottom: 6px; color: var(--t2); }
.empty-sub { font-size: 12px; color: var(--t3); }

.result-wrap { display: flex; flex-direction: column; gap: 20px; }
.result-header { display: flex; align-items: baseline; justify-content: space-between; flex-wrap: wrap; gap: 8px; }
.result-symbol { font-size: 22px; font-weight: 800; color: var(--gold); text-decoration: none; }
.result-symbol:hover { text-decoration: underline; }
.result-period { font-size: 13px; color: var(--t3); margin-left: 10px; }
.result-capital { font-size: 12px; color: var(--t3); }

.stat-cards { display: grid; grid-template-columns: repeat(auto-fill, minmax(130px, 1fr)); gap: 10px; }
.stat-card {
  background: var(--surface); border: 1px solid var(--line); border-radius: 10px;
  padding: 14px 16px; display: flex; flex-direction: column; gap: 4px;
}
.sc-label { font-size: 10px; font-weight: 700; letter-spacing: 0.1em; text-transform: uppercase; color: var(--t3); }
.sc-val { font-size: 18px; font-weight: 700; }
.sc-val.big { font-size: 22px; }
.sc-val.down { color: #ef4444; }

.equity-panel, .trades-panel {
  background: var(--surface); border: 1px solid var(--line); border-radius: 10px; overflow: hidden;
}
.panel-title { padding: 12px 18px; font-size: 12px; font-weight: 700; letter-spacing: 0.1em; text-transform: uppercase; color: var(--t2); border-bottom: 1px solid var(--line); }
.chart-area { padding: 12px 8px; }

.trades-table-wrap { overflow-x: auto; }
.trades-table { width: 100%; border-collapse: collapse; font-size: 12px; }
.trades-table th { padding: 8px 10px; font-weight: 700; font-size: 10.5px; text-align: left; color: var(--t2); background: var(--surface2); border-bottom: 1px solid var(--line); }
.trades-table td { padding: 7px 10px; border-bottom: 1px solid var(--line); }
.tr-win > td:first-child { border-left: 2px solid #22c55e; }
.tr-loss > td:first-child { border-left: 2px solid #ef4444; }
.ra { text-align: right; }
.muted { color: var(--t3); }
.col-green { color: #22c55e; font-weight: 600; }
.col-red   { color: #ef4444; font-weight: 600; }

.reason-tag { text-align: center; }
.tag-stop_loss   { background: rgba(239,68,68,0.12); color: #ef4444; font-size:10px; font-weight:700; padding:1px 6px; border-radius:4px; }
.tag-take_profit { background: rgba(34,197,94,0.12);  color: #22c55e; font-size:10px; font-weight:700; padding:1px 6px; border-radius:4px; }
.tag-signal      { background: rgba(91,156,246,0.12); color: #5b9cf6; font-size:10px; font-weight:700; padding:1px 6px; border-radius:4px; }
.tag-max_hold    { background: rgba(240,168,66,0.12); color: #f0a842; font-size:10px; font-weight:700; padding:1px 6px; border-radius:4px; }
.tag-end_date    { background: rgba(147,149,168,0.12); color: var(--t3); font-size:10px; font-weight:700; padding:1px 6px; border-radius:4px; }

@keyframes spin { to { transform: rotate(360deg); } }
.spinning { animation: spin 1s linear infinite; }
.spinning-big { display: inline-block; animation: spin 2s linear infinite; }

@media (max-width: 640px) {
  .sidebar { width: 100%; border-right: none; border-bottom: 1px solid var(--line); }
  .layout { flex-direction: column; }
}
</style>
