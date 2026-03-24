<script setup lang="ts">
import { createChart, CandlestickSeries, HistogramSeries } from 'lightweight-charts'

useHead({
  link: [
    { rel: 'preconnect', href: 'https://fonts.googleapis.com' },
    { rel: 'preconnect', href: 'https://fonts.gstatic.com', crossorigin: '' },
    {
      rel: 'stylesheet',
      href: 'https://fonts.googleapis.com/css2?family=DM+Sans:ital,opsz,wght@0,9..40,300;0,9..40,400;0,9..40,500;0,9..40,600;0,9..40,700;1,9..40,400&display=swap',
    },
  ],
})

const route = useRoute()
const symbol = route.params.symbol as string

// ── 型別 ──────────────────────────────────
interface Stock {
  id: number
  symbol: string
  name: string
  updated_at: string
}

interface DailyPrice {
  id: number
  symbol: string
  date: string
  open: number
  high: number
  low: number
  close: number
  volume: number
  tx_value: number
  tx_count: number
}

// ── 資料抓取 ──────────────────────────────
const { data: stock } = await useFetch<Stock>(`/api/stocks/${symbol}`)

const today = new Date()
const from = ref(new Date(today.getFullYear() - 1, today.getMonth(), today.getDate()).toISOString().split('T')[0])
const to = ref(today.toISOString().split('T')[0])

const { data: prices, refresh: refreshPrices } = await useFetch<DailyPrice[]>(
  () => `/api/stocks/${symbol}/prices?from=${from.value}&to=${to.value}&limit=2000`,
)

// ── 主題 ──────────────────────────────────
const isDark = ref(
  typeof localStorage !== 'undefined'
    ? localStorage.getItem('tsm-theme') === 'dark'
    : false
)
function toggleTheme() {
  isDark.value = !isDark.value
  if (typeof localStorage !== 'undefined') {
    localStorage.setItem('tsm-theme', isDark.value ? 'dark' : 'light')
  }
  // 圖表也需要重繪
  nextTick(() => initChart())
}

// ── 圖表 ─────────────────────────────────
const chartContainer = ref<HTMLElement | null>(null)
let chart: ReturnType<typeof createChart> | null = null

function getChartColors() {
  return isDark.value
    ? {
        bg: '#1e2030',      // --s2 暗色
        text: '#f7f5f0',    // --t1 暗色
        grid: '#2d3148',    // --line 暗色
        border: '#3b4060',  // --line2 暗色
      }
    : {
        bg: '#fcfcfa',      // --s2 亮色
        text: '#1f2030',    // --t1 亮色
        grid: '#e8e6e0',    // --line 亮色
        border: '#c8c4b8',  // --line2 亮色
      }
}

function initChart() {
  if (!chartContainer.value) return
  if (chart) { chart.remove(); chart = null }

  const colors = getChartColors()

  chart = createChart(chartContainer.value, {
    layout: {
      background: { color: colors.bg },
      textColor: colors.text,
      fontFamily: "'DM Sans', system-ui, sans-serif",
      fontSize: 12,
    },
    grid: {
      vertLines: { color: colors.grid },
      horzLines: { color: colors.grid },
    },
    timeScale: {
      borderColor: colors.border,
      timeVisible: true,
      fixLeftEdge: false,
    },
    crosshair: { mode: 1 },
    rightPriceScale: { borderColor: colors.border },
    height: 460,
  })

  const candle = chart.addSeries(CandlestickSeries, {
    upColor: '#e05252',
    downColor: '#3daa6b',
    borderUpColor: '#e05252',
    borderDownColor: '#3daa6b',
    wickUpColor: '#e05252',
    wickDownColor: '#3daa6b',
  })

  const volSeries = chart.addSeries(HistogramSeries, {
    color: '#94a3b8',
    priceFormat: { type: 'volume' },
    priceScaleId: 'volume',
  })
  chart.priceScale('volume').applyOptions({ scaleMargins: { top: 0.82, bottom: 0 } })

  if (prices.value && prices.value.length > 0) {
    candle.setData(prices.value.map(p => ({
      time: p.date.split('T')[0] as string,
      open: p.open, high: p.high, low: p.low, close: p.close,
    })))
    volSeries.setData(prices.value.map(p => ({
      time: p.date.split('T')[0] as string,
      value: p.volume / 1000,
      color: p.close >= p.open ? 'rgba(224,82,82,0.35)' : 'rgba(61,170,107,0.35)',
    })))
    chart.timeScale().fitContent()
  }

  const ro = new ResizeObserver(() => {
    if (chart && chartContainer.value)
      chart.applyOptions({ width: chartContainer.value.clientWidth })
  })
  ro.observe(chartContainer.value!)
}

onMounted(async () => { await nextTick(); initChart() })
watch(prices, async () => { await nextTick(); initChart() })
onBeforeUnmount(() => { chart?.remove() })

// ── 日期範圍快捷 ──────────────────────────
type RangeKey = '1M' | '3M' | '6M' | '1Y' | '2Y'
const activeRange = ref<RangeKey>('1Y')
const ranges: { label: string; key: RangeKey; months: number }[] = [
  { label: '1M', key: '1M', months: 1 },
  { label: '3M', key: '3M', months: 3 },
  { label: '6M', key: '6M', months: 6 },
  { label: '1Y', key: '1Y', months: 12 },
  { label: '2Y', key: '2Y', months: 24 },
]

function setRange(r: typeof ranges[0]) {
  const end = new Date()
  const start = new Date(end.getFullYear(), end.getMonth() - r.months, end.getDate())
  from.value = start.toISOString().split('T')[0]
  to.value   = end.toISOString().split('T')[0]
  activeRange.value = r.key
  refreshPrices()
}

// ── 統計 ──────────────────────────────────
const latest = computed(() => prices.value?.[prices.value.length - 1] ?? null)
const priceChange = computed(() => {
  if (!prices.value || prices.value.length < 2) return null
  const prev = prices.value[prices.value.length - 2]!
  const curr = prices.value[prices.value.length - 1]!
  const diff = curr.close - prev.close
  const pct  = (diff / prev.close) * 100
  return { diff, pct }
})

const periodHigh = computed(() =>
  prices.value?.length ? Math.max(...prices.value.map(p => p.high)).toFixed(2) : '—'
)
const periodLow = computed(() =>
  prices.value?.length ? Math.min(...prices.value.map(p => p.low)).toFixed(2) : '—'
)
const avgVolume = computed(() => {
  if (!prices.value?.length) return '—'
  const avg = prices.value.reduce((a, p) => a + p.volume / 1000, 0) / prices.value.length
  return Math.round(avg).toLocaleString()
})
</script>

<template>
  <div class="page" :class="{ light: !isDark }">

    <!-- ══ Site Header ══ -->
    <header class="site-header">
      <div class="site-header__inner">
        <div class="brand">
          <span class="brand-badge">TSM</span>
          <div class="brand-text">
            <span class="brand-sub">Taiwan Stock Monitor</span>
            <span class="brand-name">台股監控系統</span>
          </div>
        </div>
        <div class="header-right">
          <NuxtLink to="/" class="back-btn">← 返回首頁</NuxtLink>
          <button class="theme-toggle" :aria-label="isDark ? '切換亮色模式' : '切換暗色模式'" @click="toggleTheme">
            <span v-if="isDark">☀</span>
            <span v-else>☾</span>
          </button>
        </div>
      </div>
    </header>

    <div class="content">

      <!-- ══ Stock Hero ══ -->
      <div class="hero">
        <div class="hero-left">
          <p class="hero-eyebrow">{{ symbol }}</p>
          <h1 class="hero-name">{{ stock?.name ?? symbol }}</h1>
        </div>
        <div v-if="latest" class="hero-price">
          <span class="price-num" :class="priceChange && priceChange.diff >= 0 ? 'col-up' : 'col-dn'">
            {{ latest.close.toFixed(2) }}
          </span>
          <div v-if="priceChange" class="price-delta" :class="priceChange.diff >= 0 ? 'col-up' : 'col-dn'">
            <span>{{ priceChange.diff >= 0 ? '+' : '' }}{{ priceChange.diff.toFixed(2) }}</span>
            <span>{{ priceChange.diff >= 0 ? '+' : '' }}{{ priceChange.pct.toFixed(2) }}%</span>
          </div>
          <span class="price-date">{{ latest.date.split('T')[0] }}</span>
        </div>
      </div>

      <!-- ══ Stat Bar ══ -->
      <div class="stat-bar">
        <div class="stat-item">
          <span class="stat-key">開盤</span>
          <span class="stat-val">{{ latest?.open.toFixed(2) ?? '—' }}</span>
        </div>
        <div class="stat-item">
          <span class="stat-key">最高</span>
          <span class="stat-val col-up">{{ latest?.high.toFixed(2) ?? '—' }}</span>
        </div>
        <div class="stat-item">
          <span class="stat-key">最低</span>
          <span class="stat-val col-dn">{{ latest?.low.toFixed(2) ?? '—' }}</span>
        </div>
        <div class="stat-divider" />
        <div class="stat-item">
          <span class="stat-key">區間高點</span>
          <span class="stat-val">{{ periodHigh }}</span>
        </div>
        <div class="stat-item">
          <span class="stat-key">區間低點</span>
          <span class="stat-val">{{ periodLow }}</span>
        </div>
        <div class="stat-item">
          <span class="stat-key">均成交量（張）</span>
          <span class="stat-val">{{ avgVolume }}</span>
        </div>
      </div>

      <!-- ══ Chart Panel ══ -->
      <div class="chart-panel">
        <div class="chart-toolbar">
          <span class="chart-label">日 K 線圖</span>
          <div class="range-group">
            <button
              v-for="r in ranges"
              :key="r.key"
              class="range-btn"
              :class="{ 'range-btn--active': activeRange === r.key }"
              @click="setRange(r)"
            >{{ r.label }}</button>
          </div>
        </div>

        <div v-if="!prices || prices.length === 0" class="chart-empty">
          此股票尚無日 K 資料，請先在首頁點擊「同步日 K 資料」。
        </div>
        <ClientOnly v-else>
          <div ref="chartContainer" class="chart-container" />
          <template #fallback>
            <div class="chart-container chart-loading">圖表載入中…</div>
          </template>
        </ClientOnly>
      </div>

      <!-- ══ OHLCV Table ══ -->
      <div v-if="prices && prices.length > 0" class="table-panel">
        <div class="table-topbar">
          <h2 class="table-heading">近期日K</h2>
          <span class="table-count">最近 30 筆</span>
        </div>

        <table class="ohlcv-table">
          <thead>
            <tr>
              <th>日期</th>
              <th class="ra">開盤</th>
              <th class="ra">最高</th>
              <th class="ra">最低</th>
              <th class="ra">收盤</th>
              <th class="ra">成交量（張）</th>
              <th class="ra">成交金額（千元）</th>
            </tr>
          </thead>
          <tbody>
            <tr
              v-for="p in [...(prices ?? [])].reverse().slice(0, 30)"
              :key="p.id"
            >
              <td class="td-date">{{ p.date.split('T')[0] }}</td>
              <td class="ra">{{ p.open.toFixed(2) }}</td>
              <td class="ra td-high">{{ p.high.toFixed(2) }}</td>
              <td class="ra td-low">{{ p.low.toFixed(2) }}</td>
              <td class="ra td-close" :class="p.close >= p.open ? 'col-up' : 'col-dn'">{{ p.close.toFixed(2) }}</td>
              <td class="ra td-muted">{{ Math.round(p.volume / 1000).toLocaleString() }}</td>
              <td class="ra td-muted">{{ Math.round(p.tx_value / 1000).toLocaleString() }}</td>
            </tr>
          </tbody>
        </table>
      </div>

    </div>
  </div>
</template>

<style scoped>
/* ── Design Tokens（與首頁一致）──────────── */
.page {
  --bg:    oklch(14.5% 0.016 258);
  --s1:    oklch(19%   0.018 258);
  --s2:    oklch(23%   0.018 258);
  --line:  oklch(28%   0.020 258);
  --line2: oklch(36%   0.020 258);
  --t1:    oklch(97%   0.006 82);
  --t2:    oklch(78%   0.012 258);
  --t3:    oklch(58%   0.014 258);
  --gold:  oklch(76%   0.095 80);
  --up:    oklch(59%   0.18  22);
  --dn:    oklch(62%   0.17  148);
  --font:  'DM Sans', system-ui, 'PingFang TC', 'Microsoft JhengHei', sans-serif;

  min-height: 100vh;
  background: var(--bg);
  color: var(--t1);
  font-family: var(--font);
  font-size: 16px;
  line-height: 1.55;
  -webkit-font-smoothing: antialiased;
}

.page *, .page *::before, .page *::after { box-sizing: border-box; margin: 0; padding: 0; }

/* Light Mode */
.page.light {
  --bg:    oklch(96.5% 0.007 82);
  --s1:    oklch(93%   0.008 82);
  --s2:    oklch(99%   0.004 82);
  --line:  oklch(84%   0.012 258);
  --line2: oklch(68%   0.015 258);
  --t1:    oklch(13%   0.020 258);
  --t2:    oklch(34%   0.016 258);
  --t3:    oklch(54%   0.014 258);
  --gold:  oklch(48%   0.13  60);
  --up:    oklch(44%   0.21  22);
  --dn:    oklch(38%   0.19  148);
}

/* ── Site Header ───────────────────────── */
.site-header {
  background: var(--s1);
  border-bottom: 1px solid var(--line);
  position: sticky;
  top: 0;
  z-index: 50;
}

.site-header__inner {
  max-width: 1200px;
  margin: 0 auto;
  padding: 0 40px;
  height: 54px;
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.brand { display: flex; align-items: center; gap: 14px; }

.brand-badge {
  font-size: 11px;
  font-weight: 700;
  letter-spacing: 0.14em;
  color: var(--bg);
  background: var(--gold);
  padding: 5px 8px;
  line-height: 1;
  flex-shrink: 0;
}

.brand-text { display: flex; flex-direction: column; gap: 2px; }

.brand-sub {
  font-size: 10px;
  letter-spacing: 0.18em;
  text-transform: uppercase;
  color: var(--t3);
  line-height: 1;
}

.brand-name {
  font-size: 16px;
  font-weight: 600;
  letter-spacing: 0.02em;
  color: var(--t1);
  line-height: 1;
}

.header-right { display: flex; align-items: center; gap: 16px; }

.back-btn {
  font-size: 12.5px;
  font-weight: 600;
  color: var(--t3);
  text-decoration: none;
  letter-spacing: 0.02em;
  transition: color 0.15s;
}
.back-btn:hover { color: var(--gold); }

.theme-toggle {
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
  padding: 0;
  flex-shrink: 0;
}
.theme-toggle:hover { border-color: var(--gold); color: var(--gold); }

/* ── Content ───────────────────────────── */
.content {
  max-width: 1200px;
  margin: 0 auto;
  padding: 28px 40px 60px;
  display: flex;
  flex-direction: column;
  gap: 0;
}

/* ── Hero ──────────────────────────────── */
.hero {
  display: flex;
  align-items: flex-end;
  justify-content: space-between;
  flex-wrap: wrap;
  gap: 20px;
  padding-bottom: 20px;
  border-bottom: 1px solid var(--line);
  margin-bottom: 0;
}

.hero-eyebrow {
  font-size: 11px;
  font-weight: 600;
  letter-spacing: 0.18em;
  text-transform: uppercase;
  color: var(--t3);
  margin-bottom: 6px;
}

.hero-name {
  font-size: clamp(24px, 3vw, 36px);
  font-weight: 700;
  letter-spacing: -0.02em;
  color: var(--t1);
  line-height: 1.1;
}

.hero-price {
  display: flex;
  align-items: baseline;
  gap: 14px;
  flex-wrap: wrap;
}

.price-num {
  font-size: clamp(32px, 4vw, 52px);
  font-weight: 700;
  letter-spacing: -0.04em;
  font-variant-numeric: tabular-nums;
  line-height: 1;
}

.price-delta {
  display: flex;
  flex-direction: column;
  gap: 2px;
  font-size: 14px;
  font-weight: 600;
  font-variant-numeric: tabular-nums;
  line-height: 1.3;
}

.price-date {
  font-size: 12px;
  color: var(--t3);
  font-variant-numeric: tabular-nums;
}

.col-up { color: var(--up); }
.col-dn { color: var(--dn); }

/* ── Stat Bar ──────────────────────────── */
.stat-bar {
  display: flex;
  align-items: center;
  gap: 0;
  border-bottom: 1px solid var(--line);
  overflow-x: auto;
  scrollbar-width: none;
}
.stat-bar::-webkit-scrollbar { display: none; }

.stat-item {
  display: flex;
  flex-direction: column;
  gap: 4px;
  padding: 14px 24px 14px 0;
  min-width: 90px;
  flex-shrink: 0;
}

.stat-divider {
  width: 1px;
  height: 32px;
  background: var(--line);
  flex-shrink: 0;
  margin: 0 24px 0 0;
}

.stat-key {
  font-size: 10px;
  font-weight: 600;
  letter-spacing: 0.14em;
  text-transform: uppercase;
  color: var(--t3);
}

.stat-val {
  font-size: 15px;
  font-weight: 600;
  font-variant-numeric: tabular-nums;
  color: var(--t1);
}

/* ── Chart Panel ───────────────────────── */
.chart-panel {
  border: 1px solid var(--line);
  border-top: none;
  background: var(--s2);
  margin-bottom: 1px;
}

.chart-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 12px 20px;
  border-bottom: 1px solid var(--line);
}

.chart-label {
  font-size: 11px;
  font-weight: 600;
  letter-spacing: 0.14em;
  text-transform: uppercase;
  color: var(--t3);
}

.range-group {
  display: flex;
  gap: 2px;
}

.range-btn {
  font-family: var(--font);
  font-size: 11.5px;
  font-weight: 600;
  padding: 5px 14px;
  background: transparent;
  color: var(--t3);
  border: 1px solid transparent;
  cursor: pointer;
  letter-spacing: 0.04em;
  transition: color 0.15s, border-color 0.15s;
}

.range-btn:hover { color: var(--t1); border-color: var(--line2); }
.range-btn--active {
  color: var(--gold);
  border-color: var(--gold);
}

.chart-container {
  width: 100%;
}

.chart-empty {
  padding: 64px 20px;
  text-align: center;
  color: var(--t3);
  font-size: 14.5px;
  line-height: 1.8;
}

.chart-loading {
  height: 460px;
  display: flex;
  align-items: center;
  justify-content: center;
  color: var(--t3);
  font-size: 14px;
}

/* ── OHLCV Table ───────────────────────── */
.table-panel {
  border: 1px solid var(--line);
  border-top: none;
  background: var(--s2);
}

.table-topbar {
  display: flex;
  align-items: baseline;
  gap: 12px;
  padding: 14px 20px 13px;
  border-bottom: 1px solid var(--line);
}

.table-heading {
  font-size: 13px;
  font-weight: 600;
  letter-spacing: -0.01em;
  color: var(--t1);
}

.table-count {
  font-size: 11px;
  color: var(--t3);
}

.ohlcv-table {
  width: 100%;
  border-collapse: collapse;
  font-size: 14.5px;
}

.ohlcv-table th {
  text-align: left;
  font-size: 10px;
  font-weight: 600;
  letter-spacing: 0.12em;
  text-transform: uppercase;
  color: var(--t3);
  padding: 11px 16px 10px 0;
  border-bottom: 1px solid var(--line);
  white-space: nowrap;
}
.ohlcv-table th:first-child { padding-left: 20px; }
.ohlcv-table th.ra { text-align: right; }

.ohlcv-table td {
  padding: 10px 16px 10px 0;
  border-bottom: 1px solid var(--line);
  font-variant-numeric: tabular-nums;
  vertical-align: middle;
  color: var(--t1);
}
.ohlcv-table td:first-child { padding-left: 20px; }
.ohlcv-table tr:last-child td { border-bottom: none; }
.ohlcv-table tbody tr:hover td { background: var(--s1); }

.ra { text-align: right; }

.td-date { color: var(--t2); font-size: 13.5px; }
.td-high { color: var(--up); font-weight: 500; }
.td-low  { color: var(--dn); font-weight: 500; }
.td-close { font-weight: 700; }
.td-muted { color: var(--t3); }

/* ── RWD ───────────────────────────────── */
@media (max-width: 720px) {
  .site-header__inner { padding: 0 16px; }
  .content { padding: 16px 16px 40px; }

  .brand-sub { display: none; }

  .hero { align-items: flex-start; flex-direction: column; }
  .hero-price { gap: 10px; }
  .price-num { font-size: 36px; }

  .stat-item { min-width: 76px; padding-left: 0; }

  .ohlcv-table th:nth-child(7),
  .ohlcv-table td:nth-child(7) { display: none; }
}
</style>
