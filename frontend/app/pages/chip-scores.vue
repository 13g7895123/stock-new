<script setup lang="ts">
useHead({ title: '籌碼評分排行 | 台股分析' })
import { useAppPrefs } from '~/composables/useAppPrefs'
const { isDark, toggleTheme } = useAppPrefs()

interface ScoreRow {
  symbol: string
  name: string
  industry: string
  calc_date: string
  total_score: number
  institutional_score: number
  major_score: number
  chips_pyramid_score: number
  winrate_score: number
}

const { data: result, pending, refresh } = await useFetch<{ data: ScoreRow[]; count: number }>(
  '/api/chip-scores?limit=500',
  { default: () => ({ data: [], count: 0 }) }
)

const calcLoading = ref(false)
const calcMsg = ref('')

async function triggerCalc() {
  calcLoading.value = true
  calcMsg.value = '計算中，全市場約需 1~3 分鐘…'
  try {
    await $fetch('/api/chip-scores/calc', { method: 'POST' })
    setTimeout(async () => {
      await refresh()
      calcMsg.value = '計算完成！'
      calcLoading.value = false
      setTimeout(() => { calcMsg.value = '' }, 3000)
    }, 5000)
  } catch (e: any) {
    calcMsg.value = e?.data?.error ?? '觸發失敗'
    calcLoading.value = false
  }
}

const searchQuery = ref('')
const sortKey = ref<'total_score' | 'symbol' | 'institutional_score' | 'major_score'>('total_score')
const industryFilter = ref('')

const industries = computed(() => {
  const set = new Set((result.value?.data ?? []).map(r => r.industry).filter(Boolean))
  return ['', ...Array.from(set).sort()]
})

const filtered = computed(() => {
  let list = result.value?.data ?? []
  if (searchQuery.value) {
    const q = searchQuery.value.toLowerCase()
    list = list.filter(r => r.symbol.includes(q) || r.name.toLowerCase().includes(q))
  }
  if (industryFilter.value) {
    list = list.filter(r => r.industry === industryFilter.value)
  }
  return [...list].sort((a, b) => {
    if (sortKey.value === 'symbol') return a.symbol.localeCompare(b.symbol)
    return (b[sortKey.value] ?? 0) - (a[sortKey.value] ?? 0)
  })
})

function scoreColor(score: number): string {
  if (score >= 75) return '#22c55e'
  if (score >= 55) return '#f0a842'
  if (score >= 35) return '#fb923c'
  return '#ef4444'
}

function scoreLevel(score: number): string {
  if (score >= 75) return '強'
  if (score >= 55) return '中上'
  if (score >= 35) return '中下'
  return '弱'
}

function barPct(score: number, max: number): string {
  return `${Math.min(100, (score / max) * 100).toFixed(1)}%`
}
</script>

<template>
  <div class="page" :class="{ dark: isDark }">
    <header class="hdr">
      <div class="hdr-inner">
        <div class="hdr-brand">
          <NuxtLink to="/" class="back-link">← 首頁</NuxtLink>
          <span class="sep">/</span>
          <span class="cur-page">籌碼評分</span>
        </div>
        <button class="btn-icon" @click="toggleTheme" :title="isDark ? '切換亮色' : '切換暗色'">
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
          <h1 class="page-title">籌碼評分排行</h1>
          <p class="page-sub">整合三大法人、主力券商、大戶持股、歷史勝率，計算 0~100 分的綜合籌碼健康度</p>
        </div>
        <button class="btn-calc" :disabled="calcLoading" @click="triggerCalc">
          <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" :class="{ spinning: calcLoading }">
            <path d="M21 2v6h-6"/><path d="M3 12a9 9 0 0 1 15-6.7L21 8"/>
            <path d="M3 22v-6h6"/><path d="M21 12a9 9 0 0 1-15 6.7L3 16"/>
          </svg>
          {{ calcLoading ? '計算中…' : '重新計算全市場' }}
        </button>
      </div>
      <div v-if="calcMsg" class="calc-msg">{{ calcMsg }}</div>

      <!-- 分數說明 -->
      <div class="score-legend">
        <div v-for="[label, color, range] in [['強勢 (75~100)','#22c55e',''],['中上 (55~74)','#f0a842',''],['中下 (35~54)','#fb923c',''],['弱勢 (0~34)','#ef4444','']]"
          :key="label" class="legend-item">
          <span class="legend-dot" :style="{ background: color }"></span>
          <span>{{ label }}</span>
        </div>
      </div>

      <!-- 篩選列 -->
      <div class="filter-bar">
        <input v-model="searchQuery" type="text" class="filter-input" placeholder="搜尋股票代碼或名稱…" />
        <select v-model="industryFilter" class="filter-select">
          <option value="">所有產業</option>
          <option v-for="ind in industries.slice(1)" :key="ind" :value="ind">{{ ind }}</option>
        </select>
        <div class="sort-group">
          <span class="sort-label">排序：</span>
          <button class="sort-btn" :class="{ active: sortKey === 'total_score' }" @click="sortKey = 'total_score'">總分</button>
          <button class="sort-btn" :class="{ active: sortKey === 'institutional_score' }" @click="sortKey = 'institutional_score'">法人</button>
          <button class="sort-btn" :class="{ active: sortKey === 'major_score' }" @click="sortKey = 'major_score'">主力</button>
          <button class="sort-btn" :class="{ active: sortKey === 'symbol' }" @click="sortKey = 'symbol'">代碼</button>
        </div>
        <span class="count-tag">{{ filtered.length }} 支</span>
      </div>

      <div v-if="pending" class="loading-wrap">載入中…</div>
      <div v-else-if="filtered.length === 0" class="empty-wrap">
        <p>尚無評分資料，請點擊「重新計算全市場」按鈕</p>
      </div>
      <div v-else class="score-grid">
        <NuxtLink
          v-for="row in filtered"
          :key="row.symbol"
          :to="`/stocks/${row.symbol}`"
          class="score-card"
        >
          <div class="card-top">
            <div class="card-info">
              <span class="card-symbol">{{ row.symbol }}</span>
              <span class="card-name">{{ row.name }}</span>
              <span v-if="row.industry" class="card-industry">{{ row.industry }}</span>
            </div>
            <div class="card-score-wrap">
              <div
                class="score-circle"
                :style="{ '--clr': scoreColor(row.total_score) }"
              >
                <svg viewBox="0 0 36 36" class="circle-svg">
                  <circle cx="18" cy="18" r="15.5" fill="none" stroke="var(--line2)" stroke-width="2.5"/>
                  <circle cx="18" cy="18" r="15.5" fill="none"
                    :stroke="scoreColor(row.total_score)" stroke-width="2.5"
                    stroke-linecap="round"
                    :stroke-dasharray="`${(row.total_score / 100) * 97.4} 97.4`"
                    stroke-dashoffset="24.35"
                    transform="rotate(-90 18 18)"
                  />
                </svg>
                <div class="circle-inner">
                  <span class="circle-score" :style="{ color: scoreColor(row.total_score) }">{{ row.total_score.toFixed(0) }}</span>
                  <span class="circle-level" :style="{ color: scoreColor(row.total_score) }">{{ scoreLevel(row.total_score) }}</span>
                </div>
              </div>
            </div>
          </div>

          <div class="card-bars">
            <div class="bar-row">
              <span class="bar-label">法人</span>
              <div class="bar-track">
                <div class="bar-fill" :style="{ width: barPct(row.institutional_score, 35), background: '#5b9cf6' }"></div>
              </div>
              <span class="bar-val">{{ row.institutional_score.toFixed(1) }}</span>
            </div>
            <div class="bar-row">
              <span class="bar-label">主力</span>
              <div class="bar-track">
                <div class="bar-fill" :style="{ width: barPct(row.major_score, 35), background: '#a78ce8' }"></div>
              </div>
              <span class="bar-val">{{ row.major_score.toFixed(1) }}</span>
            </div>
            <div class="bar-row">
              <span class="bar-label">大戶</span>
              <div class="bar-track">
                <div class="bar-fill" :style="{ width: barPct(row.chips_pyramid_score, 15), background: '#4ecfa8' }"></div>
              </div>
              <span class="bar-val">{{ row.chips_pyramid_score.toFixed(1) }}</span>
            </div>
            <div class="bar-row">
              <span class="bar-label">勝率</span>
              <div class="bar-track">
                <div class="bar-fill" :style="{ width: barPct(row.winrate_score, 15), background: '#f0a842' }"></div>
              </div>
              <span class="bar-val">{{ row.winrate_score.toFixed(1) }}</span>
            </div>
          </div>

          <div class="card-footer">
            <span class="card-date">計算日：{{ row.calc_date?.split?.('T')?.[0] ?? '—' }}</span>
          </div>
        </NuxtLink>
      </div>
    </div>
  </div>
</template>

<style scoped>
*, *::before, *::after { box-sizing: border-box; margin: 0; padding: 0; }

.page {
  --bg: #f5f5f0;
  --surface1: #fff;
  --surface2: #f0efea;
  --t1: #1a1c2e;
  --t2: #4a4d68;
  --t3: #9395a8;
  --line1: #e2e1db;
  --line2: rgba(0,0,0,0.08);
  --gold: #c4922a;
  min-height: 100dvh;
  background: var(--bg);
  color: var(--t1);
  font-family: 'DM Sans', system-ui, sans-serif;
}
.page.dark {
  --bg: #13141f;
  --surface1: #1c1e2f;
  --surface2: #252738;
  --t1: #e8e6d8;
  --t2: #9395a8;
  --t3: #606278;
  --line1: #2a2d42;
  --line2: rgba(255,255,255,0.06);
  --gold: #d4a63a;
}

.hdr { background: var(--surface1); border-bottom: 1px solid var(--line1); position: sticky; top: 0; z-index: 100; }
.hdr-inner { max-width: 1440px; margin: 0 auto; padding: 0 24px; height: 52px; display: flex; align-items: center; justify-content: space-between; }
.hdr-brand { display: flex; align-items: center; gap: 8px; }
.back-link { color: var(--t3); font-size: 13px; text-decoration: none; transition: color 0.15s; }
.back-link:hover { color: var(--t1); }
.sep { color: var(--t3); font-size: 12px; }
.cur-page { font-size: 13px; font-weight: 600; color: var(--t1); }
.btn-icon { background: none; border: none; cursor: pointer; color: var(--t2); padding: 6px; border-radius: 6px; }

.content { max-width: 1440px; margin: 0 auto; padding: 28px 24px 60px; }
.page-head { display: flex; align-items: flex-start; justify-content: space-between; gap: 16px; margin-bottom: 20px; flex-wrap: wrap; }
.page-title { font-size: 22px; font-weight: 700; color: var(--t1); margin-bottom: 4px; }
.page-sub { font-size: 13px; color: var(--t3); max-width: 500px; }

.btn-calc {
  display: inline-flex; align-items: center; gap: 7px;
  padding: 9px 18px; border-radius: 8px;
  background: var(--gold); color: #fff; border: none; cursor: pointer;
  font-size: 13px; font-weight: 600; white-space: nowrap;
  transition: opacity 0.15s;
}
.btn-calc:disabled { opacity: 0.6; cursor: not-allowed; }
.calc-msg { font-size: 13px; color: var(--t2); margin-bottom: 12px; padding: 10px 14px; background: var(--surface2); border-radius: 8px; }

.score-legend { display: flex; gap: 16px; margin-bottom: 16px; flex-wrap: wrap; }
.legend-item { display: flex; align-items: center; gap: 6px; font-size: 12px; color: var(--t3); }
.legend-dot { width: 8px; height: 8px; border-radius: 50%; flex-shrink: 0; }

.filter-bar { display: flex; align-items: center; gap: 10px; margin-bottom: 20px; flex-wrap: wrap; }
.filter-input {
  padding: 7px 12px; font-size: 13px; background: var(--surface1);
  border: 1px solid var(--line1); border-radius: 8px; color: var(--t1);
  outline: none; width: 200px;
}
.filter-input:focus { border-color: var(--gold); }
.filter-select {
  padding: 7px 12px; font-size: 13px; background: var(--surface1);
  border: 1px solid var(--line1); border-radius: 8px; color: var(--t1); cursor: pointer;
  outline: none;
}
.sort-group { display: flex; align-items: center; gap: 4px; }
.sort-label { font-size: 12px; color: var(--t3); }
.sort-btn {
  padding: 5px 10px; font-size: 12px; font-weight: 600;
  background: transparent; border: 1px solid var(--line1); border-radius: 5px;
  color: var(--t3); cursor: pointer; transition: all 0.15s;
}
.sort-btn.active { background: var(--gold); border-color: var(--gold); color: #fff; }
.count-tag { font-size: 12px; color: var(--t3); margin-left: 4px; }

.loading-wrap, .empty-wrap { text-align: center; padding: 60px; color: var(--t3); font-size: 14px; }

.score-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(280px, 1fr));
  gap: 14px;
}

.score-card {
  display: flex; flex-direction: column; gap: 12px;
  background: var(--surface1); border: 1px solid var(--line1);
  border-radius: 12px; padding: 16px;
  text-decoration: none; color: inherit;
  transition: all 0.15s; cursor: pointer;
}
.score-card:hover { border-color: var(--gold); box-shadow: 0 4px 16px rgba(0,0,0,0.08); transform: translateY(-2px); }

.card-top { display: flex; align-items: flex-start; justify-content: space-between; gap: 12px; }
.card-info { display: flex; flex-direction: column; gap: 2px; min-width: 0; }
.card-symbol { font-size: 15px; font-weight: 700; color: var(--t1); }
.card-name { font-size: 13px; color: var(--t2); white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.card-industry { font-size: 11px; color: var(--t3); background: var(--surface2); border-radius: 4px; padding: 1px 6px; align-self: flex-start; margin-top: 2px; }

.score-circle { position: relative; width: 60px; height: 60px; flex-shrink: 0; }
.circle-svg { width: 100%; height: 100%; }
.circle-inner { position: absolute; inset: 0; display: flex; flex-direction: column; align-items: center; justify-content: center; }
.circle-score { font-size: 14px; font-weight: 700; line-height: 1; }
.circle-level { font-size: 9px; font-weight: 600; margin-top: 1px; }

.card-bars { display: flex; flex-direction: column; gap: 5px; }
.bar-row { display: flex; align-items: center; gap: 6px; }
.bar-label { font-size: 10.5px; color: var(--t3); width: 24px; flex-shrink: 0; }
.bar-track { flex: 1; height: 5px; background: var(--surface2); border-radius: 3px; overflow: hidden; }
.bar-fill { height: 100%; border-radius: 3px; transition: width 0.5s ease; }
.bar-val { font-size: 10.5px; color: var(--t2); width: 28px; text-align: right; flex-shrink: 0; }

.card-footer { margin-top: 2px; }
.card-date { font-size: 10.5px; color: var(--t3); }

@keyframes spin { to { transform: rotate(360deg); } }
.spinning { animation: spin 1s linear infinite; }
</style>
