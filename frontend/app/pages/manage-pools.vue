<script setup lang="ts">
import { ref, computed } from 'vue'
import { useAppPrefs } from '~/composables/useAppPrefs'

const { isDark, isClassic, toggleTheme, setTheme, setStyle } = useAppPrefs()

// ─── Types ─────────────────────────────────────────────────────
interface Tag {
  id: number
  name: string
  color: string
}

interface StockGroup {
  id: number
  name: string
  description: string
  color: string
}

interface Stock {
  id: number
  symbol: string
  name: string
  price: number
  change: number
  change_pct: number
  volume: number
  industry: string
  tags?: Tag[]
  groups?: StockGroup[]
}

// ─── API ────────────────────────────────────────────────────────
const { data: stocks, refresh: refreshStocks } = await useFetch<Stock[]>('/api/stocks')
const { data: groups, refresh: refreshGroups } = await useFetch<StockGroup[]>('/api/groups')
const { data: tags, refresh: refreshTags } = await useFetch<Tag[]>('/api/tags')

// ─── State ──────────────────────────────────────────────────────
const settingsOpen = ref(false)
const selectedStockIds = ref<Set<number>>(new Set())
const searchQuery = ref('')
const selectedIndustry = ref('')
const activeTab = ref<'group' | 'tag'>('group')

// 創建 Group/Tag 對話框
const createDialogOpen = ref(false)
const createName = ref('')
const createDescription = ref('')
const createColor = ref('#3b82f6')
const creating = ref(false)

// 批次操作對話框
const batchDialogOpen = ref(false)
const batchAction = ref<'add' | 'remove'>('add')
const selectedPoolIds = ref<number[]>([])
const batchProcessing = ref(false)

// ─── Computed ───────────────────────────────────────────────────
const industries = computed(() => {
  const set = new Set<string>()
  stocks.value?.forEach(s => s.industry && set.add(s.industry))
  return Array.from(set).sort()
})

const filteredStocks = computed(() => {
  let result = stocks.value ?? []
  
  if (searchQuery.value) {
    const q = searchQuery.value.toLowerCase()
    result = result.filter(s => 
      s.symbol.toLowerCase().includes(q) || 
      s.name.toLowerCase().includes(q)
    )
  }
  
  if (selectedIndustry.value) {
    result = result.filter(s => s.industry === selectedIndustry.value)
  }
  
  return result
})

const allSelected = computed(() => {
  if (filteredStocks.value.length === 0) return false
  return filteredStocks.value.every(s => selectedStockIds.value.has(s.id))
})

const selectedCount = computed(() => selectedStockIds.value.size)

const availablePools = computed(() => {
  return activeTab.value === 'group' 
    ? (groups.value ?? [])
    : (tags.value ?? [])
})

// ─── Actions ────────────────────────────────────────────────────
function toggleSelectAll() {
  if (allSelected.value) {
    filteredStocks.value.forEach(s => selectedStockIds.value.delete(s.id))
  } else {
    filteredStocks.value.forEach(s => selectedStockIds.value.add(s.id))
  }
}

function toggleSelectStock(id: number) {
  if (selectedStockIds.value.has(id)) {
    selectedStockIds.value.delete(id)
  } else {
    selectedStockIds.value.add(id)
  }
}

function clearSelection() {
  selectedStockIds.value.clear()
}

// ─── Create Group/Tag ───────────────────────────────────────────
function openCreateDialog() {
  createName.value = ''
  createDescription.value = ''
  createColor.value = activeTab.value === 'group' ? '#3b82f6' : '#6b7280'
  createDialogOpen.value = true
}

async function createPool() {
  if (!createName.value.trim()) return
  
  creating.value = true
  try {
    const endpoint = activeTab.value === 'group' ? '/api/groups' : '/api/tags'
    const body = activeTab.value === 'group'
      ? { name: createName.value, description: createDescription.value, color: createColor.value }
      : { name: createName.value, color: createColor.value }
    
    await $fetch(endpoint, { method: 'POST', body })
    
    // 刷新資料以便其他頁面（如 watchlist）也能看到新增的群組/標籤
    if (activeTab.value === 'group') {
      await refreshGroups()
    } else {
      await refreshTags()
    }
    await refreshStocks() // 刷新股票資料以更新關聯
    
    createDialogOpen.value = false
  } catch (err) {
    alert('創建失敗：' + (err as Error).message)
  } finally {
    creating.value = false
  }
}

// ─── Batch Update ───────────────────────────────────────────────
function openBatchDialog() {
  if (selectedStockIds.value.size === 0) return
  selectedPoolIds.value = []
  batchAction.value = 'add'
  batchDialogOpen.value = true
}

async function executeBatchUpdate() {
  if (selectedPoolIds.value.length === 0) return
  
  batchProcessing.value = true
  try {
    const selectedStocks = (stocks.value ?? []).filter(s => selectedStockIds.value.has(s.id))
    const isGroup = activeTab.value === 'group'
    
    for (const stock of selectedStocks) {
      // 取得當前的 IDs
      const currentIds = isGroup 
        ? (stock.groups?.map(g => g.id) ?? [])
        : (stock.tags?.map(t => t.id) ?? [])
      
      // 根據 action 計算新的 IDs
      let newIds: number[]
      if (batchAction.value === 'add') {
        // 合併（去重）
        newIds = Array.from(new Set([...currentIds, ...selectedPoolIds.value]))
      } else {
        // 移除
        newIds = currentIds.filter(id => !selectedPoolIds.value.includes(id))
      }
      
      // 更新
      const endpoint = isGroup 
        ? `/api/stocks/${stock.symbol}/groups`
        : `/api/stocks/${stock.symbol}/tags`
      const key = isGroup ? 'group_ids' : 'tag_ids'
      
      await $fetch(endpoint, { 
        method: 'PUT', 
        body: { [key]: newIds }
      })
    }
    
    await refreshStocks()
    batchDialogOpen.value = false
    clearSelection()
  } catch (err) {
    alert('批次更新失敗：' + (err as Error).message)
  } finally {
    batchProcessing.value = false
  }
}

// ─── Delete Pool ────────────────────────────────────────────────
async function deletePool(id: number) {
  if (!confirm(`確定要刪除此${activeTab.value === 'group' ? '群組' : '標籤'}嗎？`)) return
  
  try {
    const endpoint = activeTab.value === 'group' 
      ? `/api/groups/${id}`
      : `/api/tags/${id}`
    
    await $fetch(endpoint, { method: 'DELETE' })
    
    if (activeTab.value === 'group') {
      await refreshGroups()
    } else {
      await refreshTags()
    }
    await refreshStocks()
  } catch (err) {
    alert('刪除失敗：' + (err as Error).message)
  }
}

const today = new Date().toLocaleDateString('zh-TW', {
  year: 'numeric', month: 'long', day: 'numeric', weekday: 'long'
})
</script>

<template>
  <div class="page" :class="{ light: !isDark, classic: isClassic }">
    
    <!-- ══ Header ══ -->
    <header v-if="!isClassic" class="site-header">
      <div class="site-header__inner">
        <div class="brand">
          <NuxtLink to="/" class="back-link">
            <svg width="14" height="14" viewBox="0 0 16 16" fill="none">
              <path d="M10 13L5 8l5-5" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round"/>
            </svg>
            首頁
          </NuxtLink>
          <span class="brand-sep">/</span>
          <span class="brand-cur">股池管理</span>
        </div>
        <div class="header-right">
          <span class="header-date">{{ today }}</span>
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
                  <button class="sp-btn" :class="{ active: !isDark }" @click="setTheme(false)">亮色</button>
                  <button class="sp-btn" :class="{ active: isDark }" @click="setTheme(true)">暗色</button>
                </div>
              </div>
              <div class="sp-group">
                <p class="sp-label">版面風格</p>
                <div class="sp-btns">
                  <button class="sp-btn" :class="{ active: isClassic }" @click="setStyle('classic')">Classic</button>
                  <button class="sp-btn" :class="{ active: !isClassic }" @click="setStyle('bento')">Bento</button>
                </div>
              </div>
            </div>
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
      </div>
    </header>

    <!-- ══ Main Content ══ -->
    <div class="content">
      
      <!-- Title Bar -->
      <div class="title-bar">
        <div class="title-bar-left">
          <h1 class="page-title">股池批次管理</h1>
          <p class="page-desc">批次選擇股票，快速更新群組或標籤</p>
        </div>
        <div class="title-bar-right">
          <NuxtLink to="/watchlist" class="action-btn action-btn--secondary">
            <svg width="14" height="14" viewBox="0 0 16 16" fill="none">
              <path d="M2 8h12M8 2v12" stroke="currentColor" stroke-width="1.6" stroke-linecap="round"/>
            </svg>
            前往關注股池
          </NuxtLink>
        </div>
      </div>

      <!-- Tab + Actions -->
      <div class="control-bar">
        <div class="tab-group">
          <button 
            class="tab-btn" 
            :class="{ active: activeTab === 'group' }"
            @click="activeTab = 'group'; clearSelection()"
          >
            群組管理
          </button>
          <button 
            class="tab-btn" 
            :class="{ active: activeTab === 'tag' }"
            @click="activeTab = 'tag'; clearSelection()"
          >
            標籤管理
          </button>
        </div>
        <div class="control-actions">
          <button class="action-btn action-btn--primary" @click="openCreateDialog">
            <svg width="14" height="14" viewBox="0 0 16 16" fill="none">
              <path d="M8 3v10M3 8h10" stroke="currentColor" stroke-width="1.8" stroke-linecap="round"/>
            </svg>
            新增{{ activeTab === 'group' ? '群組' : '標籤' }}
          </button>
        </div>
      </div>

      <!-- Pools List -->
      <div class="pools-section">
        <div class="section-header">
          <h2 class="section-title">{{ activeTab === 'group' ? '群組列表' : '標籤列表' }}</h2>
          <span class="section-count">{{ availablePools.length }} 個</span>
        </div>
        <div v-if="availablePools.length === 0" class="empty-state">
          <p>尚無{{ activeTab === 'group' ? '群組' : '標籤' }}，點擊上方按鈕新增</p>
        </div>
        <div v-else class="pool-grid">
          <div 
            v-for="pool in availablePools" 
            :key="pool.id" 
            class="pool-card"
            :style="{ borderLeftColor: pool.color }"
          >
            <div class="pool-card-header">
              <div class="pool-dot" :style="{ background: pool.color }" />
              <span class="pool-name">{{ pool.name }}</span>
            </div>
            <p v-if="'description' in pool && pool.description" class="pool-desc">{{ pool.description }}</p>
            <div class="pool-card-footer">
              <button class="pool-delete-btn" @click="deletePool(pool.id)">
                <svg width="12" height="12" viewBox="0 0 16 16" fill="none">
                  <path d="M2 4h12M5.5 4V2.5a1 1 0 0 1 1-1h3a1 1 0 0 1 1 1V4M13 4v9.5a1 1 0 0 1-1 1H4a1 1 0 0 1-1-1V4" stroke="currentColor" stroke-width="1.4" stroke-linecap="round" stroke-linejoin="round"/>
                </svg>
                刪除
              </button>
            </div>
          </div>
        </div>
      </div>

      <!-- Stock Selection -->
      <div class="stocks-section">
        <div class="section-header">
          <h2 class="section-title">選擇股票</h2>
          <span v-if="selectedCount > 0" class="section-count selection-count">已選 {{ selectedCount }} 檔</span>
        </div>

        <!-- Filters -->
        <div class="filters">
          <input 
            v-model="searchQuery" 
            type="text" 
            placeholder="搜尋股票代碼或名稱..."
            class="filter-input"
          />
          <select v-model="selectedIndustry" class="filter-select">
            <option value="">所有產業</option>
            <option v-for="ind in industries" :key="ind" :value="ind">{{ ind }}</option>
          </select>
          <button 
            v-if="selectedCount > 0" 
            class="action-btn action-btn--warning"
            @click="openBatchDialog"
          >
            批次更新 ({{ selectedCount }})
          </button>
          <button 
            v-if="selectedCount > 0" 
            class="action-btn action-btn--light"
            @click="clearSelection"
          >
            取消選擇
          </button>
        </div>

        <!-- Stocks Table -->
        <div class="stocks-table-wrap">
          <table class="stocks-table">
            <thead>
              <tr>
                <th class="col-checkbox">
                  <input 
                    type="checkbox" 
                    :checked="allSelected"
                    @change="toggleSelectAll"
                  />
                </th>
                <th>代碼</th>
                <th>名稱</th>
                <th>產業</th>
                <th>收盤</th>
                <th>漲跌</th>
                <th>{{ activeTab === 'group' ? '群組' : '標籤' }}</th>
              </tr>
            </thead>
            <tbody>
              <tr 
                v-for="stock in filteredStocks" 
                :key="stock.id"
                :class="{ selected: selectedStockIds.has(stock.id) }"
              >
                <td class="col-checkbox">
                  <input 
                    type="checkbox" 
                    :checked="selectedStockIds.has(stock.id)"
                    @change="toggleSelectStock(stock.id)"
                  />
                </td>
                <td class="col-symbol">{{ stock.symbol }}</td>
                <td class="col-name">{{ stock.name }}</td>
                <td class="col-industry">{{ stock.industry || '—' }}</td>
                <td class="col-price">{{ stock.price > 0 ? stock.price.toFixed(2) : '—' }}</td>
                <td class="col-change" :class="stock.change > 0 ? 'up' : stock.change < 0 ? 'dn' : ''">
                  {{ stock.change > 0 ? '+' : '' }}{{ stock.change_pct > 0 ? stock.change_pct.toFixed(2) + '%' : '—' }}
                </td>
                <td class="col-pools">
                  <div class="pool-tags">
                    <template v-if="activeTab === 'group'">
                      <span 
                        v-for="g in stock.groups" 
                        :key="g.id" 
                        class="pool-tag"
                        :style="{ background: g.color }"
                      >{{ g.name }}</span>
                    </template>
                    <template v-else>
                      <span 
                        v-for="t in stock.tags" 
                        :key="t.id" 
                        class="pool-tag"
                        :style="{ background: t.color }"
                      >{{ t.name }}</span>
                    </template>
                  </div>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>

    </div>

    <!-- ══ Create Dialog ══ -->
    <Teleport to="body">
      <div v-if="createDialogOpen" class="dialog-overlay" @click="createDialogOpen = false">
        <div class="dialog" @click.stop>
          <div class="dialog-header">
            <h3 class="dialog-title">新增{{ activeTab === 'group' ? '群組' : '標籤' }}</h3>
            <button class="dialog-close" @click="createDialogOpen = false">✕</button>
          </div>
          <div class="dialog-body">
            <div class="form-group">
              <label class="form-label">名稱</label>
              <input 
                v-model="createName" 
                type="text" 
                class="form-input"
                placeholder="輸入名稱"
                @keydown.enter="createPool"
              />
            </div>
            <div v-if="activeTab === 'group'" class="form-group">
              <label class="form-label">說明</label>
              <textarea 
                v-model="createDescription" 
                class="form-textarea"
                placeholder="選填"
                rows="2"
              />
            </div>
            <div class="form-group">
              <label class="form-label">顏色</label>
              <div class="color-picker">
                <input 
                  v-model="createColor" 
                  type="color" 
                  class="color-input"
                />
                <span class="color-value">{{ createColor }}</span>
              </div>
            </div>
          </div>
          <div class="dialog-footer">
            <button class="dialog-btn dialog-btn--cancel" @click="createDialogOpen = false">取消</button>
            <button 
              class="dialog-btn dialog-btn--primary" 
              :disabled="!createName.trim() || creating"
              @click="createPool"
            >
              {{ creating ? '建立中...' : '建立' }}
            </button>
          </div>
        </div>
      </div>
    </Teleport>

    <!-- ══ Batch Update Dialog ══ -->
    <Teleport to="body">
      <div v-if="batchDialogOpen" class="dialog-overlay" @click="batchDialogOpen = false">
        <div class="dialog" @click.stop>
          <div class="dialog-header">
            <h3 class="dialog-title">批次更新 ({{ selectedCount }} 檔股票)</h3>
            <button class="dialog-close" @click="batchDialogOpen = false">✕</button>
          </div>
          <div class="dialog-body">
            <div class="form-group">
              <label class="form-label">操作</label>
              <div class="radio-group">
                <label class="radio-label">
                  <input v-model="batchAction" type="radio" value="add" />
                  <span>新增{{ activeTab === 'group' ? '群組' : '標籤' }}</span>
                </label>
                <label class="radio-label">
                  <input v-model="batchAction" type="radio" value="remove" />
                  <span>移除{{ activeTab === 'group' ? '群組' : '標籤' }}</span>
                </label>
              </div>
            </div>
            <div class="form-group">
              <label class="form-label">選擇{{ activeTab === 'group' ? '群組' : '標籤' }}</label>
              <div class="checkbox-list">
                <label 
                  v-for="pool in availablePools" 
                  :key="pool.id"
                  class="checkbox-label"
                >
                  <input 
                    v-model="selectedPoolIds" 
                    type="checkbox" 
                    :value="pool.id"
                  />
                  <span class="pool-dot" :style="{ background: pool.color }" />
                  <span>{{ pool.name }}</span>
                </label>
              </div>
            </div>
          </div>
          <div class="dialog-footer">
            <button class="dialog-btn dialog-btn--cancel" @click="batchDialogOpen = false">取消</button>
            <button 
              class="dialog-btn dialog-btn--primary" 
              :disabled="selectedPoolIds.length === 0 || batchProcessing"
              @click="executeBatchUpdate"
            >
              {{ batchProcessing ? '處理中...' : '確定' }}
            </button>
          </div>
        </div>
      </div>
    </Teleport>

  </div>
</template>

<style scoped>
/* ── Design Tokens ──────────────────────────────────────────── */
.page {
  --bg:    oklch(9.5%  0.018 256);
  --s1:    oklch(13%   0.020 257);
  --s2:    oklch(16.5% 0.022 258);
  --s3:    oklch(21%   0.024 258);
  --line:  oklch(22%   0.023 258);
  --line2: oklch(33%   0.023 258);
  --blue:  oklch(63%   0.20  264);
  --gold:  oklch(76%   0.13  82);
  --green: oklch(64%   0.18  148);
  --red:   oklch(62%   0.18  22);
  --t1:    oklch(96%   0.006 218);
  --t2:    oklch(72%   0.013 240);
  --t3:    oklch(50%   0.012 240);
  --font:  'DM Sans', system-ui, sans-serif;

  min-height: 100vh;
  background: var(--bg);
  color: var(--t1);
  font-family: var(--font);
}

.page.light {
  --bg:    oklch(96.5% 0.009 220);
  --s1:    oklch(100%  0     0);
  --s2:    oklch(97%   0.010 220);
  --s3:    oklch(92%   0.014 220);
  --line:  oklch(88%   0.012 220);
  --line2: oklch(72%   0.015 240);
  --blue:  oklch(47%   0.21  264);
  --gold:  oklch(52%   0.16  72);
  --green: oklch(38%   0.20  148);
  --red:   oklch(44%   0.22  22);
  --t1:    oklch(10%   0.018 256);
  --t2:    oklch(35%   0.016 240);
  --t3:    oklch(57%   0.012 240);
}

/* ── Header ──────────────────────────────────────────────────── */
.site-header {
  background: color-mix(in oklch, var(--s1) 85%, transparent);
  backdrop-filter: blur(16px);
  border-bottom: 1px solid var(--line);
  position: sticky;
  top: 0;
  z-index: 50;
}

.site-header__inner {
  max-width: 1400px;
  margin: 0 auto;
  padding: 0 32px;
  height: 56px;
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.brand { display: flex; align-items: center; gap: 10px; }
.back-link { display: flex; align-items: center; gap: 5px; font-size: 13px; color: var(--t3); text-decoration: none; transition: color 0.15s; }
.back-link:hover { color: var(--gold); }
.brand-sep { color: var(--line2); }
.brand-cur { font-size: 14px; font-weight: 600; color: var(--t1); }
.header-right { display: flex; align-items: center; gap: 10px; }
.header-date { font-size: 12px; color: var(--t3); }

.btn-icon {
  width: 34px; height: 34px;
  display: flex; align-items: center; justify-content: center;
  background: var(--s2); border: 1px solid var(--line); border-radius: 8px;
  color: var(--t2); cursor: pointer; transition: all 0.2s;
}
.btn-icon:hover { background: var(--s3); color: var(--t1); }

.settings-wrap { position: relative; }
.settings-overlay { position: fixed; inset: 0; z-index: 99; }
.settings-panel {
  position: absolute; top: calc(100% + 8px); right: 0; z-index: 100;
  background: var(--s2); border: 1px solid var(--line2); border-radius: 12px;
  padding: 16px; min-width: 196px; box-shadow: 0 8px 32px oklch(0% 0 0 / 0.28);
}
.sp-title { font-size: 10.5px; font-weight: 700; text-transform: uppercase; color: var(--t3); margin-bottom: 12px; }
.sp-group { margin-bottom: 12px; }
.sp-group:last-child { margin-bottom: 0; }
.sp-label { font-size: 10.5px; text-transform: uppercase; color: var(--t3); margin-bottom: 6px; }
.sp-btns { display: flex; gap: 6px; }
.sp-btn {
  flex: 1; font-size: 12px; font-weight: 600; padding: 7px 8px;
  background: transparent; border: 1px solid var(--line2); border-radius: 7px;
  color: var(--t2); cursor: pointer; transition: all 0.15s;
}
.sp-btn:hover { border-color: var(--t2); color: var(--t1); }
.sp-btn.active { background: var(--blue); border-color: var(--blue); color: white; }

/* ── Content ─────────────────────────────────────────────────── */
.content {
  max-width: 1400px;
  margin: 0 auto;
  padding: 28px 40px 60px;
}

/* ── Title Bar ───────────────────────────────────────────────── */
.title-bar {
  display: flex;
  align-items: flex-end;
  justify-content: space-between;
  padding-bottom: 20px;
  border-bottom: 1px solid var(--line);
  margin-bottom: 24px;
}

.page-title {
  font-size: 28px;
  font-weight: 700;
  color: var(--t1);
  margin-bottom: 6px;
}

.page-desc {
  font-size: 13px;
  color: var(--t3);
}

.action-btn {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 8px 16px;
  border-radius: 8px;
  font-size: 13px;
  font-weight: 600;
  border: 1px solid;
  cursor: pointer;
  transition: all 0.15s;
}

.action-btn--primary {
  background: var(--blue);
  border-color: var(--blue);
  color: white;
}
.action-btn--primary:hover { opacity: 0.9; }

.action-btn--secondary {
  background: transparent;
  border-color: var(--line2);
  color: var(--t2);
  text-decoration: none;
}
.action-btn--secondary:hover { border-color: var(--gold); color: var(--gold); }

.action-btn--warning {
  background: var(--gold);
  border-color: var(--gold);
  color: oklch(10% 0.018 256);
}

.action-btn--light {
  background: transparent;
  border-color: var(--line);
  color: var(--t3);
}

/* ── Control Bar ─────────────────────────────────────────────── */
.control-bar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 24px;
}

.tab-group {
  display: flex;
  gap: 4px;
  background: var(--s2);
  border: 1px solid var(--line);
  border-radius: 8px;
  padding: 4px;
}

.tab-btn {
  padding: 6px 16px;
  border-radius: 6px;
  font-size: 13px;
  font-weight: 600;
  background: transparent;
  border: none;
  color: var(--t3);
  cursor: pointer;
  transition: all 0.15s;
}
.tab-btn.active { background: var(--blue); color: white; }

/* ── Sections ────────────────────────────────────────────────── */
.section-header {
  display: flex;
  align-items: baseline;
  gap: 12px;
  margin-bottom: 16px;
}

.section-title {
  font-size: 16px;
  font-weight: 600;
  color: var(--t1);
}

.section-count {
  font-size: 12px;
  color: var(--t3);
}

.selection-count {
  color: var(--gold);
  font-weight: 600;
}

.pools-section {
  background: var(--s2);
  border: 1px solid var(--line);
  border-radius: 10px;
  padding: 20px;
  margin-bottom: 24px;
}

.pool-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(280px, 1fr));
  gap: 12px;
}

.pool-card {
  background: var(--s1);
  border: 1px solid var(--line);
  border-left-width: 4px;
  border-radius: 8px;
  padding: 14px;
}

.pool-card-header {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 8px;
}

.pool-dot {
  width: 10px;
  height: 10px;
  border-radius: 50%;
}

.pool-name {
  font-weight: 600;
  color: var(--t1);
}

.pool-desc {
  font-size: 12px;
  color: var(--t3);
  margin-bottom: 12px;
}

.pool-card-footer {
  display: flex;
  justify-content: flex-end;
}

.pool-delete-btn {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  padding: 4px 10px;
  border-radius: 6px;
  font-size: 11px;
  font-weight: 600;
  background: transparent;
  border: 1px solid transparent;
  color: var(--t3);
  cursor: pointer;
  transition: all 0.15s;
}
.pool-delete-btn:hover { border-color: var(--red); color: var(--red); }

.empty-state {
  padding: 32px;
  text-align: center;
  color: var(--t3);
  font-size: 14px;
}

/* ── Stocks Section ──────────────────────────────────────────── */
.stocks-section {
  background: var(--s2);
  border: 1px solid var(--line);
  border-radius: 10px;
  padding: 20px;
}

.filters {
  display: flex;
  gap: 10px;
  margin-bottom: 16px;
}

.filter-input, .filter-select {
  padding: 8px 12px;
  border-radius: 6px;
  border: 1px solid var(--line);
  background: var(--s1);
  color: var(--t1);
  font-size: 13px;
}

.filter-input {
  flex: 1;
  min-width: 200px;
}

.filter-select {
  min-width: 150px;
}

.stocks-table-wrap {
  overflow-x: auto;
  border: 1px solid var(--line);
  border-radius: 8px;
}

.stocks-table {
  width: 100%;
  border-collapse: collapse;
  font-size: 13px;
}

.stocks-table th {
  background: var(--s3);
  padding: 10px 12px;
  text-align: left;
  font-weight: 600;
  color: var(--t2);
  border-bottom: 1px solid var(--line);
}

.stocks-table td {
  padding: 10px 12px;
  border-bottom: 1px solid var(--line);
}

.stocks-table tbody tr:hover {
  background: var(--s1);
}

.stocks-table tbody tr.selected {
  background: color-mix(in oklch, var(--blue) 15%, transparent);
}

.col-checkbox { width: 40px; text-align: center; }
.col-symbol { font-family: monospace; font-weight: 600; }
.col-name { font-weight: 500; }
.col-price, .col-change { font-variant-numeric: tabular-nums; }
.col-change.up { color: var(--red); }
.col-change.dn { color: var(--green); }

.pool-tags {
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
}

.pool-tag {
  display: inline-block;
  padding: 2px 8px;
  border-radius: 100px;
  font-size: 11px;
  font-weight: 600;
  color: white;
}

/* ── Dialog ──────────────────────────────────────────────────── */
.dialog-overlay {
  position: fixed;
  inset: 0;
  background: oklch(0% 0 0 / 0.5);
  backdrop-filter: blur(4px);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 1000;
}

.dialog {
  background: oklch(100% 0 0); /* 純白色背景，不受主題影響 */
  border: 1px solid oklch(88% 0.012 220);
  border-radius: 12px;
  width: 90%;
  max-width: 500px;
  box-shadow: 0 20px 60px oklch(0% 0 0 / 0.4);
}

.dialog-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 20px 24px;
  border-bottom: 1px solid oklch(88% 0.012 220);
}

.dialog-title {
  font-size: 16px;
  font-weight: 600;
  color: oklch(10% 0.018 256); /* 深色文字 */
}

.dialog-close {
  background: none;
  border: none;
  font-size: 20px;
  color: oklch(50% 0.012 240);
  cursor: pointer;
  padding: 0;
  width: 28px;
  height: 28px;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: 6px;
  transition: all 0.15s;
}
.dialog-close:hover { background: oklch(92% 0.014 220); color: oklch(10% 0.018 256); }

.dialog-body {
  padding: 24px;
}

.form-group {
  margin-bottom: 20px;
}
.form-group:last-child { margin-bottom: 0; }

.form-label {
  display: block;
  font-size: 13px;
  font-weight: 600;
  color: oklch(35% 0.016 240);
  margin-bottom: 8px;
}

.form-input, .form-textarea {
  width: 100%;
  padding: 10px 12px;
  border-radius: 8px;
  border: 1px solid oklch(88% 0.012 220);
  background: oklch(97% 0.010 220);
  color: oklch(10% 0.018 256);
  font-size: 14px;
  font-family: var(--font);
}

.color-picker {
  display: flex;
  align-items: center;
  gap: 10px;
}

.color-input {
  width: 60px;
  height: 40px;
  border: 1px solid oklch(88% 0.012 220);
  border-radius: 6px;
  cursor: pointer;
}

.color-value {
  font-family: monospace;
  font-size: 13px;
  color: oklch(50% 0.012 240);
}

.radio-group {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.radio-label {
  display: flex;
  align-items: center;
  gap: 8px;
  cursor: pointer;
}

.checkbox-list {
  display: flex;
  flex-direction: column;
  gap: 10px;
  max-height: 300px;
  overflow-y: auto;
  padding: 12px;
  background: oklch(97% 0.010 220);
  border: 1px solid oklch(88% 0.012 220);
  border-radius: 8px;
}

.checkbox-label {
  display: flex;
  align-items: center;
  gap: 8px;
  cursor: pointer;
  padding: 6px;
  border-radius: 6px;
  transition: background 0.15s;
}
.checkbox-label:hover { background: oklch(92% 0.014 220); }

.dialog-footer {
  display: flex;
  gap: 10px;
  justify-content: flex-end;
  padding: 20px 24px;
  border-top: 1px solid oklch(88% 0.012 220);
}

.dialog-btn {
  padding: 8px 20px;
  border-radius: 8px;
  font-size: 13px;
  font-weight: 600;
  border: 1px solid;
  cursor: pointer;
  transition: all 0.15s;
}

.dialog-btn--cancel {
  background: transparent;
  border-color: oklch(88% 0.012 220);
  color: oklch(35% 0.016 240);
}
.dialog-btn--cancel:hover { border-color: oklch(72% 0.015 240); }

.dialog-btn--primary {
  background: var(--blue);
  border-color: var(--blue);
  color: white;
}
.dialog-btn--primary:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}
</style>
