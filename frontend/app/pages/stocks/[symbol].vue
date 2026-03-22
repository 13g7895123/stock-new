<script setup lang="ts">
import { createChart, CandlestickSeries, HistogramSeries } from 'lightweight-charts'

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

// 預設抓近 1 年資料
const today = new Date()
const from = ref(new Date(today.getFullYear() - 1, today.getMonth(), today.getDate()).toISOString().split('T')[0])
const to = ref(today.toISOString().split('T')[0])

const { data: prices, refresh: refreshPrices } = await useFetch<DailyPrice[]>(
  () => `/api/stocks/${symbol}/prices?from=${from.value}&to=${to.value}&limit=2000`,
)

// ── 圖表 ─────────────────────────────────
const chartContainer = ref<HTMLElement | null>(null)
let chart: ReturnType<typeof createChart> | null = null

function initChart() {
  if (!chartContainer.value) return
  if (chart) {
    chart.remove()
    chart = null
  }

  chart = createChart(chartContainer.value, {
    layout: {
      background: { color: '#ffffff' },
      textColor: '#334155',
    },
    grid: {
      vertLines: { color: '#f1f5f9' },
      horzLines: { color: '#f1f5f9' },
    },
    timeScale: {
      borderColor: '#e2e8f0',
      timeVisible: true,
    },
    crosshair: {
      mode: 1,
    },
    rightPriceScale: {
      borderColor: '#e2e8f0',
    },
    height: 400,
  })

  // 蠟燭圖
  const candle = chart.addSeries(CandlestickSeries, {
    upColor: '#dc2626',
    downColor: '#16a34a',
    borderUpColor: '#dc2626',
    borderDownColor: '#16a34a',
    wickUpColor: '#dc2626',
    wickDownColor: '#16a34a',
  })

  // 成交量
  const volSeries = chart.addSeries(HistogramSeries, {
    color: '#94a3b8',
    priceFormat: { type: 'volume' },
    priceScaleId: 'volume',
  })
  chart.priceScale('volume').applyOptions({ scaleMargins: { top: 0.85, bottom: 0 } })

  if (prices.value && prices.value.length > 0) {
    const candleData = prices.value.map(p => ({
      time: p.date.split('T')[0] as string,
      open: p.open,
      high: p.high,
      low: p.low,
      close: p.close,
    }))
    candle.setData(candleData)

    const volumeData = prices.value.map(p => ({
      time: p.date.split('T')[0] as string,
      value: p.volume / 1000, // 張
      color: p.close >= p.open ? '#fca5a5' : '#86efac',
    }))
    volSeries.setData(volumeData)

    chart.timeScale().fitContent()
  }

  // 自適應寬度
  const ro = new ResizeObserver(() => {
    if (chart && chartContainer.value) {
      chart.applyOptions({ width: chartContainer.value.clientWidth })
    }
  })
  ro.observe(chartContainer.value!)
}

onMounted(async () => {
  await nextTick()
  initChart()
})

watch(prices, async () => {
  await nextTick()
  initChart()
})

onBeforeUnmount(() => {
  chart?.remove()
})

// ── 日期範圍快捷 ──────────────────────────
type RangeKey = '1M' | '3M' | '6M' | '1Y' | '2Y'
const activeRange = ref<RangeKey>('1Y')
const ranges: { label: string; key: RangeKey; months: number }[] = [
  { label: '1 月', key: '1M', months: 1 },
  { label: '3 月', key: '3M', months: 3 },
  { label: '6 月', key: '6M', months: 6 },
  { label: '1 年', key: '1Y', months: 12 },
  { label: '2 年', key: '2Y', months: 24 },
]

function setRange(r: typeof ranges[0]) {
  const end = new Date()
  const start = new Date(end.getFullYear(), end.getMonth() - r.months, end.getDate())
  from.value = start.toISOString().split('T')[0]
  to.value = end.toISOString().split('T')[0]
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
  const pct = (diff / prev.close) * 100
  return { diff, pct }
})
</script>

<template>
  <div class="page">
    <!-- 麵包屑 -->
    <nav class="breadcrumb">
      <NuxtLink to="/">台灣股票列表</NuxtLink>
      <span class="sep">›</span>
      <span>{{ symbol }}</span>
    </nav>

    <!-- 標頭：股票資訊 -->
    <header class="stock-header">
      <div class="stock-title">
        <h1>{{ stock?.name ?? symbol }}</h1>
        <span class="symbol-badge">{{ symbol }}</span>
      </div>

      <div v-if="latest" class="price-block">
        <span class="close-price">{{ latest.close.toFixed(2) }}</span>
        <span
          v-if="priceChange"
          class="price-change"
          :class="priceChange.diff >= 0 ? 'up' : 'down'"
        >
          {{ priceChange.diff >= 0 ? '+' : '' }}{{ priceChange.diff.toFixed(2) }}
          ({{ priceChange.diff >= 0 ? '+' : '' }}{{ priceChange.pct.toFixed(2) }}%)
        </span>
        <span class="price-date">{{ latest.date.split('T')[0] }}</span>
      </div>
    </header>

    <!-- 日期範圍 -->
    <div class="range-bar">
      <button
        v-for="r in ranges"
        :key="r.key"
        class="range-btn"
        :class="{ active: activeRange === r.key }"
        @click="setRange(r)"
      >
        {{ r.label }}
      </button>
    </div>

    <!-- K 線圖 -->
    <div v-if="!prices || prices.length === 0" class="no-data">
      此股票尚無日K 資料，請先在首頁點擊「同步日K」。
    </div>
    <ClientOnly v-else>
      <div ref="chartContainer" class="chart-wrap" />
      <template #fallback>
        <div class="chart-wrap chart-loading">圖表載入中...</div>
      </template>
    </ClientOnly>

    <!-- OHLCV 最新資料表 -->
    <div v-if="prices && prices.length > 0" class="ohlcv-table-wrap">
      <h2>近期日K 資料</h2>
      <table>
        <thead>
          <tr>
            <th>日期</th>
            <th>開盤</th>
            <th>最高</th>
            <th>最低</th>
            <th>收盤</th>
            <th>成交量（張）</th>
            <th>成交金額（千元）</th>
          </tr>
        </thead>
        <tbody>
          <tr
            v-for="p in [...(prices ?? [])].reverse().slice(0, 30)"
            :key="p.id"
            :class="p.close >= p.open ? 'day-up' : 'day-down'"
          >
            <td>{{ p.date.split('T')[0] }}</td>
            <td>{{ p.open.toFixed(2) }}</td>
            <td>{{ p.high.toFixed(2) }}</td>
            <td>{{ p.low.toFixed(2) }}</td>
            <td class="close-col">{{ p.close.toFixed(2) }}</td>
            <td>{{ Math.round(p.volume / 1000).toLocaleString() }}</td>
            <td>{{ Math.round(p.tx_value / 1000).toLocaleString() }}</td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>

<style scoped>
* { box-sizing: border-box; }

.page {
  max-width: 1100px;
  margin: 0 auto;
  padding: 24px 16px;
  font-family: system-ui, -apple-system, sans-serif;
}

/* 麵包屑 */
.breadcrumb {
  font-size: 0.85rem;
  color: #64748b;
  margin-bottom: 20px;
  display: flex;
  align-items: center;
  gap: 6px;
}
.breadcrumb a { color: #2563eb; text-decoration: none; }
.breadcrumb a:hover { text-decoration: underline; }
.sep { color: #cbd5e1; }

/* 標頭 */
.stock-header {
  display: flex;
  align-items: flex-end;
  justify-content: space-between;
  flex-wrap: wrap;
  gap: 16px;
  margin-bottom: 20px;
}

.stock-title {
  display: flex;
  align-items: center;
  gap: 12px;
}

h1 {
  font-size: 1.6rem;
  font-weight: 700;
  color: #1a1a2e;
  margin: 0;
}

.symbol-badge {
  background: #e2e8f0;
  color: #475569;
  font-size: 0.85rem;
  padding: 3px 10px;
  border-radius: 99px;
  font-weight: 600;
}

.price-block {
  display: flex;
  align-items: baseline;
  gap: 10px;
  flex-wrap: wrap;
}

.close-price {
  font-size: 2rem;
  font-weight: 700;
  color: #1e293b;
}

.price-change {
  font-size: 1rem;
  font-weight: 600;
}

.price-change.up { color: #dc2626; }
.price-change.down { color: #16a34a; }

.price-date {
  font-size: 0.8rem;
  color: #94a3b8;
}

/* 範圍選擇 */
.range-bar {
  display: flex;
  gap: 6px;
  margin-bottom: 12px;
}

.range-btn {
  padding: 5px 14px;
  border: 1px solid #e2e8f0;
  background: #fff;
  border-radius: 6px;
  font-size: 0.85rem;
  cursor: pointer;
  color: #475569;
  transition: all 0.15s;
}
.range-btn:hover { border-color: #2563eb; color: #2563eb; }
.range-btn.active {
  background: #2563eb;
  border-color: #2563eb;
  color: #fff;
}

/* 圖表 */
.chart-wrap {
  width: 100%;
  border: 1px solid #e2e8f0;
  border-radius: 8px;
  overflow: hidden;
  margin-bottom: 32px;
}

.no-data {
  padding: 48px;
  text-align: center;
  color: #94a3b8;
  border: 1px dashed #e2e8f0;
  border-radius: 8px;
  margin-bottom: 32px;
}

.chart-loading {
  height: 400px;
  display: flex;
  align-items: center;
  justify-content: center;
  color: #94a3b8;
  font-size: 0.9rem;
}

/* OHLCV 表格 */
.ohlcv-table-wrap h2 {
  font-size: 1rem;
  font-weight: 600;
  color: #475569;
  margin: 0 0 12px;
}

table {
  width: 100%;
  border-collapse: collapse;
  font-size: 0.875rem;
}

th {
  text-align: right;
  padding: 8px 12px;
  background: #f8fafc;
  color: #64748b;
  font-weight: 600;
  border-bottom: 2px solid #e2e8f0;
}

th:first-child { text-align: left; }

td {
  text-align: right;
  padding: 8px 12px;
  border-bottom: 1px solid #f1f5f9;
  color: #334155;
}

td:first-child { text-align: left; color: #64748b; }

.day-up .close-col { color: #dc2626; font-weight: 600; }
.day-down .close-col { color: #16a34a; font-weight: 600; }

tr:hover td { background: #f8fafc; }
</style>
