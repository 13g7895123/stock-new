<script setup lang="ts">
useHead({ title: '產業資金流向 | 台股分析' })
import { useAppPrefs } from '~/composables/useAppPrefs'
const { isDark, toggleTheme } = useAppPrefs()

// ── 資料型別 ─────────────────────────────────────────────────────
interface FlowDay {
  date: string
  foreign_net: number
  trust_net: number
  dealer_net: number
  total_net: number
}
interface IndustryRow {
  industry: string
  days: FlowDay[]
  latest_net: number
}
interface FlowResponse {
  dates: string[]
  data: IndustryRow[]
}

// ── 參數 ──────────────────────────────────────────────────────────
const daysParam = ref(20)
const activeField = ref<'total_net' | 'foreign_net' | 'trust_net' | 'dealer_net'>('total_net')
const expandedIndustry = ref<string | null>(null)

const { data: flowData, pending, refresh } = await useFetch<FlowResponse>(
  () => `/api/industry/flow?days=${daysParam.value}`,
  { default: () => ({ dates: [], data: [] }) }
)

// 展開某產業時取個股資料
interface StockFlowRow {
  symbol: string
  name: string
  date: string
  foreign_net: number
  trust_net: number
  dealer_net: number
  total_net: number
}
interface IndustryDetailResp {
  industry: string
  dates: string[]
  data: StockFlowRow[]
}
const detailData = ref<IndustryDetailResp | null>(null)
const detailLoading = ref(false)

async function toggleExpand(industry: string) {
  if (expandedIndustry.value === industry) {
    expandedIndustry.value = null
    detailData.value = null
    return
  }
  expandedIndustry.value = industry
  detailLoading.value = true
  try {
    detailData.value = await $fetch<IndustryDetailResp>(
      `/api/industry/flow/${encodeURIComponent(industry)}?days=5`
    )
  } catch { detailData.value = null }
  finally { detailLoading.value = false }
}

// ── 格式化 ────────────────────────────────────────────────────────
const dates = computed(() => flowData.value?.dates ?? [])

function getValue(row: IndustryRow, dateIdx: number): number {
  const d = row.days[dateIdx]
  if (!d) return 0
  return d[activeField.value] ?? 0
}

// 找各 column 的最大絕對值（用於顏色強度）
const colMaxAbsMap = computed(() => {
  const data = flowData.value?.data ?? []
  return dates.value.map((_, di) => {
    let max = 1
    for (const row of data) {
      const v = Math.abs(getValue(row, di))
      if (v > max) max = v
    }
    return max
  })
})

function cellColor(val: number, maxAbs: number): string {
  if (maxAbs === 0 || val === 0) return isDark.value ? 'rgba(255,255,255,0.04)' : 'rgba(0,0,0,0.04)'
  const intensity = Math.min(1, Math.abs(val) / maxAbs)
  const alpha = 0.15 + intensity * 0.75
  if (val > 0) return `rgba(34, 197, 94, ${alpha.toFixed(2)})`   // green = 買超
  return `rgba(239, 68, 68, ${alpha.toFixed(2)})`                 // red = 賣超
}

function cellTextColor(val: number): string {
  if (val === 0) return isDark.value ? 'rgba(255,255,255,0.3)' : 'rgba(0,0,0,0.3)'
  return val > 0 ? '#22c55e' : '#ef4444'
}

function formatNet(v: number): string {
  if (v === 0) return '—'
  const abs = Math.abs(v)
  const prefix = v > 0 ? '+' : '-'
  if (abs >= 10000) return `${prefix}${(abs / 10000).toFixed(1)}萬`
  if (abs >= 1000) return `${prefix}${(abs / 1000).toFixed(1)}千`
  return `${prefix}${abs}`
}

function shortDate(d: string) {
  return d.slice(5) // MM-DD
}

// Detail 表格：依 date 分組顯示個股
const detailByDate = computed(() => {
  if (!detailData.value) return []
  const datesArr = detailData.value.dates ?? []
  return datesArr.map(date => ({
    date,
    stocks: detailData.value!.data
      .filter(r => r.date === date)
      .sort((a, b) => b.total_net - a.total_net),
  }))
})
</script>

<template>
  <div class="page" :class="{ dark: isDark }">
    <header class="hdr">
      <div class="hdr-inner">
        <div class="hdr-brand">
          <NuxtLink to="/" class="back-link">← 首頁</NuxtLink>
          <span class="sep">/</span>
          <span class="cur-page">產業資金流向</span>
        </div>
        <button class="btn-icon" @click="toggleTheme">
          <svg v-if="isDark" width="16" height="16" viewBox="0 0 16 16" fill="none">
            <circle cx="8" cy="8" r="2.8" fill="currentColor"/>
            <path d="M8 1.5V3M8 13v1.5M1.5 8H3M13 8h1.5M3.4 3.4l1.06 1.06M11.54 11.54l1.06 1.06M3.4 12.6l1.06-1.06M11.54 4.46l1.06-1.06" stroke="currentColor" stroke-width="1.4" stroke-linecap="round"/>
          </svg>
          <svg v-else width="16" height="16" viewBox="0 0 16 16" fill="none">
            <path d="M13.2 9.3A5.8 5.8 0 0 1 6.7 2.8a.4.4 0 0 0-.46-.5A6.3 6.3 0 1 0 13.7 9.76a.4.4 0 0 0-.5-.46Z" fill="currentColor"/>
          </svg>
        </button>
      </div>
    </header>

    <div class="content">
      <div class="page-head">
        <div>
          <h1 class="page-title">產業資金流向熱力圖</h1>
          <p class="page-sub">以三大法人每日買賣超張數（千張）衡量資金流入產業，綠色=淨買超，紅色=淨賣超</p>
        </div>
        <div class="ctrl-group">
          <div class="field-tabs">
            <button
              v-for="[key, label] in [['total_net','合計'],['foreign_net','外資'],['trust_net','投信'],['dealer_net','自營']]"
              :key="key"
              class="field-tab"
              :class="{ active: activeField === key }"
              @click="activeField = key as typeof activeField"
            >{{ label }}</button>
          </div>
          <div class="day-tabs">
            <button
              v-for="n in [10, 20, 40, 60]"
              :key="n"
              class="day-tab"
              :class="{ active: daysParam === n }"
              @click="daysParam = n; refresh()"
            >{{ n }}日</button>
          </div>
        </div>
      </div>

      <!-- 凡例 -->
      <div class="legend">
        <div class="legend-bar">
          <span class="legend-label">賣超強</span>
          <div class="legend-gradient legend-sell"></div>
          <div class="legend-zero"></div>
          <div class="legend-gradient legend-buy"></div>
          <span class="legend-label">買超強</span>
        </div>
        <span class="legend-note">每格數字單位：千張（負=賣超）</span>
      </div>

      <div v-if="pending" class="loading-wrap">載入中…</div>
      <div v-else-if="!flowData?.data?.length" class="empty-wrap">
        <p>尚無三大法人資料，請先在首頁觸發「三大法人同步」</p>
      </div>
      <div v-else class="heatmap-wrap">
        <div class="heatmap-scroll">
          <table class="heatmap-table">
            <thead>
              <tr>
                <th class="col-ind sticky-col">產業</th>
                <th
                  v-for="(d, di) in dates"
                  :key="d"
                  class="col-date"
                  :class="{ 'th-latest': di === dates.length - 1 }"
                >
                  {{ shortDate(d) }}
                </th>
                <th class="col-sum">近期<br>合計</th>
              </tr>
            </thead>
            <tbody>
              <template v-for="row in flowData.data" :key="row.industry">
                <tr
                  class="ind-row"
                  :class="{ 'ind-row--expanded': expandedIndustry === row.industry }"
                  @click="toggleExpand(row.industry)"
                >
                  <td class="col-ind sticky-col">
                    <div class="ind-name-wrap">
                      <span class="expand-ico">{{ expandedIndustry === row.industry ? '▼' : '▶' }}</span>
                      <span class="ind-name">{{ row.industry }}</span>
                    </div>
                  </td>
                  <td
                    v-for="(d, di) in dates"
                    :key="d"
                    class="cell"
                    :class="{ 'cell-latest': di === dates.length - 1 }"
                    :style="{ background: cellColor(getValue(row, di), colMaxAbsMap[di]) }"
                  >
                    <span :style="{ color: cellTextColor(getValue(row, di)) }">
                      {{ formatNet(getValue(row, di)) }}
                    </span>
                  </td>
                  <td class="col-sum-val">
                    <span :style="{ color: cellTextColor(row.days.reduce((s, d) => s + (d[activeField] ?? 0), 0)) }">
                      {{ formatNet(row.days.reduce((s, d) => s + (d[activeField] ?? 0), 0)) }}
                    </span>
                  </td>
                </tr>

                <!-- 展開：個股明細 -->
                <tr v-if="expandedIndustry === row.industry" class="detail-tr">
                  <td :colspan="dates.length + 2" class="detail-td">
                    <div v-if="detailLoading" class="detail-loading">載入個股資料…</div>
                    <div v-else-if="!detailData" class="detail-loading">無資料</div>
                    <div v-else class="detail-content">
                      <div
                        v-for="group in detailByDate"
                        :key="group.date"
                        class="detail-day"
                      >
                        <div class="detail-day-header">{{ group.date }}</div>
                        <div class="detail-stocks">
                          <NuxtLink
                            v-for="s in group.stocks"
                            :key="s.symbol"
                            :to="`/stocks/${s.symbol}`"
                            class="detail-stock-chip"
                            :class="s.total_net > 0 ? 'chip-buy' : s.total_net < 0 ? 'chip-sell' : 'chip-neutral'"
                          >
                            <span class="chip-symbol">{{ s.symbol }}</span>
                            <span class="chip-name">{{ s.name }}</span>
                            <span class="chip-net">{{ formatNet(s.total_net) }}</span>
                          </NuxtLink>
                        </div>
                      </div>
                    </div>
                  </td>
                </tr>
              </template>
            </tbody>
          </table>
        </div>
      </div>
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
  --col-ind-bg: #f8f7f2;
  min-height: 100dvh;
  background: var(--bg);
  color: var(--t1);
  font-family: 'DM Sans', system-ui, sans-serif;
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
  --col-ind-bg: #1c1e2f;
}

.hdr { background: var(--surface); border-bottom: 1px solid var(--line); position: sticky; top: 0; z-index: 200; }
.hdr-inner { max-width: 1600px; margin: 0 auto; padding: 0 24px; height: 52px; display: flex; align-items: center; justify-content: space-between; }
.hdr-brand { display: flex; align-items: center; gap: 8px; }
.back-link { color: var(--t3); font-size: 13px; text-decoration: none; transition: color 0.15s; }
.back-link:hover { color: var(--t1); }
.sep { color: var(--t3); font-size: 12px; }
.cur-page { font-size: 13px; font-weight: 600; color: var(--t1); }
.btn-icon { background: none; border: none; cursor: pointer; color: var(--t2); padding: 6px; border-radius: 6px; }

.content { max-width: 1600px; margin: 0 auto; padding: 24px 24px 60px; }
.page-head { display: flex; align-items: flex-start; justify-content: space-between; gap: 16px; flex-wrap: wrap; margin-bottom: 16px; }
.page-title { font-size: 22px; font-weight: 700; color: var(--t1); margin-bottom: 4px; }
.page-sub { font-size: 13px; color: var(--t3); max-width: 500px; }

.ctrl-group { display: flex; flex-direction: column; gap: 6px; align-items: flex-end; }
.field-tabs, .day-tabs { display: flex; gap: 3px; }
.field-tab, .day-tab {
  font-size: 12px; font-weight: 600;
  padding: 5px 12px; border-radius: 5px;
  background: transparent; border: 1px solid var(--line);
  color: var(--t3); cursor: pointer; transition: all 0.15s;
}
.field-tab.active, .day-tab.active { background: var(--gold); border-color: var(--gold); color: #fff; }

.legend { display: flex; align-items: center; gap: 12px; margin-bottom: 14px; flex-wrap: wrap; }
.legend-bar { display: flex; align-items: center; gap: 4px; }
.legend-label { font-size: 11px; color: var(--t3); }
.legend-gradient { width: 60px; height: 10px; border-radius: 3px; }
.legend-sell { background: linear-gradient(to right, rgba(239,68,68,0.9), rgba(239,68,68,0.15)); }
.legend-zero { width: 8px; height: 10px; background: var(--surface2); border-radius: 2px; }
.legend-buy { background: linear-gradient(to right, rgba(34,197,94,0.15), rgba(34,197,94,0.9)); }
.legend-note { font-size: 11px; color: var(--t3); margin-left: 6px; }

.loading-wrap, .empty-wrap { text-align: center; padding: 60px; color: var(--t3); font-size: 14px; }

.heatmap-wrap { overflow: hidden; border-radius: 10px; border: 1px solid var(--line); background: var(--surface); }
.heatmap-scroll { overflow-x: auto; }

.heatmap-table {
  width: 100%;
  border-collapse: collapse;
  font-size: 11px;
  min-width: 600px;
}

.heatmap-table thead th {
  padding: 8px 6px;
  text-align: center;
  font-weight: 700;
  font-size: 10.5px;
  color: var(--t2);
  background: var(--surface2);
  border-bottom: 1px solid var(--line);
  white-space: nowrap;
  position: sticky;
  top: 0;
  z-index: 10;
}
.col-ind { text-align: left !important; min-width: 120px; }
.col-date { min-width: 56px; }
.col-sum { min-width: 56px; background: var(--col-ind-bg) !important; font-size: 10px; }
.th-latest { font-weight: 800 !important; color: var(--gold) !important; }

.sticky-col {
  position: sticky;
  left: 0;
  z-index: 5;
  background: var(--col-ind-bg);
  border-right: 1px solid var(--line);
}
thead .sticky-col { z-index: 15; }

.ind-row { cursor: pointer; transition: filter 0.12s; }
.ind-row:hover { filter: brightness(1.06); }
.ind-row--expanded .col-ind { font-weight: 700; color: var(--gold); }

.ind-row td { border-bottom: 1px solid var(--line); padding: 6px 6px; }
.col-sum-val { padding: 6px 8px; font-size: 11px; font-weight: 600; text-align: right; background: var(--col-ind-bg); border-left: 1px solid var(--line); }

.ind-name-wrap { display: flex; align-items: center; gap: 6px; padding: 0 8px; }
.expand-ico { font-size: 9px; color: var(--t3); flex-shrink: 0; }
.ind-name { font-size: 12px; font-weight: 600; white-space: nowrap; }

.cell { text-align: center; padding: 5px 4px; transition: background 0.2s; }
.cell-latest { font-weight: 700; outline: 1px solid var(--gold); outline-offset: -1px; }
.cell span { font-size: 10.5px; font-weight: 600; }

/* 展開 detail */
.detail-tr { cursor: default; }
.detail-td { padding: 0 !important; border-bottom: 2px solid var(--gold) !important; }
.detail-loading { padding: 16px 20px; color: var(--t3); font-size: 13px; }
.detail-content { padding: 12px 20px 16px; display: flex; flex-direction: column; gap: 10px; }
.detail-day-header { font-size: 11px; font-weight: 700; color: var(--t2); margin-bottom: 6px; }
.detail-stocks { display: flex; flex-wrap: wrap; gap: 6px; }

.detail-stock-chip {
  display: inline-flex; align-items: center; gap: 5px;
  padding: 4px 10px; border-radius: 6px;
  font-size: 11.5px; font-weight: 600; text-decoration: none;
  color: var(--t1); border: 1px solid var(--line);
  background: var(--surface2); transition: all 0.1s;
}
.detail-stock-chip:hover { transform: translateY(-1px); box-shadow: 0 2px 8px rgba(0,0,0,0.1); }
.chip-buy { border-color: rgba(34,197,94,0.35); background: rgba(34,197,94,0.08); }
.chip-sell { border-color: rgba(239,68,68,0.35); background: rgba(239,68,68,0.08); }
.chip-symbol { font-size: 12px; }
.chip-name { font-size: 10.5px; color: var(--t2); }
.chip-net { font-size: 11px; font-weight: 700; }
.chip-buy .chip-net { color: #22c55e; }
.chip-sell .chip-net { color: #ef4444; }
</style>
