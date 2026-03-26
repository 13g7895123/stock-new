<script setup lang="ts">
useHead({
  title: '系統設定 | Stock',
  link: [
    { rel: 'preconnect', href: 'https://fonts.googleapis.com' },
    { rel: 'preconnect', href: 'https://fonts.gstatic.com', crossorigin: '' },
    {
      rel: 'stylesheet',
      href: 'https://fonts.googleapis.com/css2?family=DM+Sans:ital,opsz,wght@0,9..40,300;0,9..40,400;0,9..40,500;0,9..40,600;0,9..40,700;1,9..40,400&family=Fira+Code:wght@400;500;600&display=swap',
    },
  ],
})

// ── 型別 ──────────────────────────────────────────────────────
interface SchemeInfo {
  id: string
  label: string
  description: string
  need_service?: string
}

interface FeatureConfig {
  primary: string
  fallback_enabled: boolean
  fallback: string
  fallback_trigger: 'error' | 'empty_data'
}

interface Feature {
  id: string
  label: string
  description: string
  category: string
  schemes: SchemeInfo[]
  config: FeatureConfig
}

// ── 狀態 ──────────────────────────────────────────────────────
const features = ref<Feature[]>([])
const selectedId = ref<string | null>(null)
const loading = ref(false)
const saving = ref(false)
const savedId = ref<string | null>(null)
const errorMsg = ref('')

// 每個功能的「草稿設定」（修改中但未儲存）
const drafts = ref<Record<string, FeatureConfig>>({})

// ── API ────────────────────────────────────────────────────────
async function fetchSettings() {
  loading.value = true
  errorMsg.value = ''
  try {
    const res = await fetch('/api/settings/features')
    if (!res.ok) throw new Error(`HTTP ${res.status}`)
    features.value = await res.json()
    // 初始化草稿
    for (const f of features.value) {
      drafts.value[f.id] = { ...f.config }
    }
    if (!selectedId.value && features.value.length) {
      const first = features.value[0]
      if (first) selectedId.value = first.id
    }
  } catch (e: any) {
    errorMsg.value = e.message
  } finally {
    loading.value = false
  }
}

async function saveFeature(id: string) {
  saving.value = true
  savedId.value = null
  errorMsg.value = ''
  try {
    const res = await fetch(`/api/settings/features/${id}`, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(drafts.value[id]),
    })
    if (!res.ok) {
      const data = await res.json()
      throw new Error(data.error || `HTTP ${res.status}`)
    }
    const updated: Feature = await res.json()
    // 更新本地 features 並同步草稿
    const idx = features.value.findIndex(f => f.id === id)
    if (idx !== -1) features.value[idx] = updated
    drafts.value[id] = { ...updated.config }
    savedId.value = id
    setTimeout(() => { savedId.value = null }, 2000)
  } catch (e: any) {
    errorMsg.value = e.message
  } finally {
    saving.value = false
  }
}

// ── 計算屬性 ────────────────────────────────────────────────────
const selectedFeature = computed(() =>
  features.value.find(f => f.id === selectedId.value),
)

const EMPTY_CONFIG: FeatureConfig = { primary: '', fallback_enabled: false, fallback: '', fallback_trigger: 'error' }

const draft = computed<FeatureConfig>(() =>
  selectedId.value ? (drafts.value[selectedId.value] ?? EMPTY_CONFIG) : EMPTY_CONFIG,
)

const isDirty = computed(() => {
  if (!selectedId.value || !selectedFeature.value) return false
  const d = drafts.value[selectedId.value]
  const orig = selectedFeature.value.config
  return JSON.stringify(d) !== JSON.stringify(orig)
})

// fallback 方案的可選選項（排除 primary 本身）
const fallbackOptions = computed<SchemeInfo[]>(() => {
  if (!selectedFeature.value) return []
  return selectedFeature.value.schemes.filter(s => s.id !== draft.value.primary)
})

// 當 primary 改變時，若 fallback 和 primary 相同則清空
watch(
  () => draft.value.primary,
  (newPrimary) => {
    if (!selectedId.value) return
    const d = drafts.value[selectedId.value]
    if (d && d.fallback === newPrimary) {
      d.fallback = fallbackOptions.value[0]?.id ?? ''
    }
  },
)

// ── Scheme 顏色 ──────────────────────────────────────────────
const schemeColors: Record<string, string> = {
  go_http:            '#4ecfa8',
  python_http:        '#5b9cf6',
  python_playwright:  '#e07b5a',
  twse_tpex_api:      '#a78ce8',
  broker_api:         '#f0a842',
}
function schemeColor(id: string) {
  return schemeColors[id] ?? '#8b949e'
}

const categoryLabels: Record<string, string> = {
  scraper: '爬蟲',
  sync: '同步',
}

onMounted(fetchSettings)
</script>

<template>
  <div class="settings-page">
    <!-- Header -->
    <header class="header">
      <div class="header-left">
        <span class="header-icon">⚙️</span>
        <h1 class="header-title">系統設定</h1>
        <span class="header-sub">資料取得方案管理</span>
      </div>
      <button class="btn-refresh" :disabled="loading" @click="fetchSettings">
        <span :class="{ spin: loading }">↻</span> 重新整理
      </button>
    </header>

    <!-- Error Banner -->
    <div v-if="errorMsg" class="error-banner">
      ⚠ {{ errorMsg }}
    </div>

    <div class="layout">
      <!-- 左側功能列表 -->
      <aside class="sidebar">
        <div class="sidebar-section-title">功能列表</div>
        <div v-if="loading" class="loading-text">載入中…</div>
        <ul v-else class="feature-list">
          <li
            v-for="f in features"
            :key="f.id"
            class="feature-item"
            :class="{ active: f.id === selectedId }"
            @click="selectedId = f.id"
          >
            <div class="feature-item-top">
              <span class="feature-item-label">{{ f.label }}</span>
              <span class="badge-category">{{ categoryLabels[f.category] ?? f.category }}</span>
            </div>
            <div class="feature-item-scheme">
              <span
                class="scheme-dot"
                :style="{ background: schemeColor(drafts[f.id]?.primary ?? '') }"
              />
              <span class="scheme-id-text">{{ drafts[f.id]?.primary }}</span>
              <span v-if="drafts[f.id]?.fallback_enabled" class="fallback-badge">備案</span>
            </div>
          </li>
        </ul>
      </aside>

      <!-- 右側設定內容 -->
      <main class="content">
        <div v-if="!selectedFeature" class="empty-state">
          <div class="empty-icon">⚙️</div>
          <p>從左側選擇一個功能</p>
        </div>

        <template v-else-if="selectedFeature">
          <!-- 功能標頭 -->
          <div class="content-header">
            <div>
              <h2 class="content-title">{{ selectedFeature.label }}</h2>
              <p class="content-desc">{{ selectedFeature.description }}</p>
            </div>
            <div class="header-actions">
              <span v-if="savedId === selectedFeature.id" class="saved-badge">✓ 已儲存</span>
              <button
                class="btn-save"
                :class="{ dirty: isDirty }"
                :disabled="saving || !isDirty"
                @click="saveFeature(selectedFeature.id)"
              >
                {{ saving ? '儲存中…' : '儲存設定' }}
              </button>
            </div>
          </div>

          <!-- section: 主要方案 -->
          <section class="setting-section">
            <div class="section-title">
              <span class="section-icon">①</span> 主要方案
              <span class="section-hint">正常情況下使用的資料取得方式</span>
            </div>

            <div class="scheme-grid">
              <label
                v-for="s in selectedFeature.schemes"
                :key="s.id"
                class="scheme-card"
                :class="{ selected: draft.primary === s.id }"
                :style="{ '--accent': schemeColor(s.id) }"
              >
                <input
                  v-model="draft.primary"
                  type="radio"
                  :value="s.id"
                  class="scheme-radio"
                />
                <div class="scheme-card-header">
                  <span class="scheme-color-bar" />
                  <span class="scheme-label">{{ s.label }}</span>
                  <span v-if="draft.primary === s.id" class="scheme-active-badge">主要</span>
                </div>
                <p class="scheme-desc">{{ s.description }}</p>
                <div v-if="s.need_service" class="scheme-requires">
                  需要外部服務：<code>{{ s.need_service }}</code>
                </div>
              </label>
            </div>
          </section>

          <!-- section: 備案設定 -->
          <section class="setting-section">
            <div class="section-title">
              <span class="section-icon">②</span> 備案設定
              <span class="section-hint">主方案失敗時的行為</span>
            </div>

            <div class="fallback-row">
              <label class="toggle-label">
                <div
                  class="toggle"
                  :class="{ on: draft.fallback_enabled }"
                  @click="draft.fallback_enabled = !draft.fallback_enabled"
                >
                  <div class="toggle-thumb" />
                </div>
                <span>{{ draft.fallback_enabled ? '啟用備案' : '停用備案（失敗時直接報錯）' }}</span>
              </label>
            </div>

            <template v-if="draft.fallback_enabled">
              <!-- 備案方案選擇 -->
              <div class="fallback-config">
                <div class="fallback-group">
                  <label class="config-label">備案方案</label>
                  <div v-if="fallbackOptions.length === 0" class="no-fallback-hint">
                    沒有其他可用方案（該功能只有一個方案）
                  </div>
                  <div v-else class="scheme-select-row">
                    <label
                      v-for="s in fallbackOptions"
                      :key="s.id"
                      class="scheme-select-item"
                      :class="{ selected: draft.fallback === s.id }"
                      :style="{ '--accent': schemeColor(s.id) }"
                    >
                      <input v-model="draft.fallback" type="radio" :value="s.id" class="scheme-radio" />
                      <span class="scheme-color-dot" />
                      <div>
                        <div class="scheme-select-label">{{ s.label }}</div>
                        <div class="scheme-select-desc">{{ s.description }}</div>
                      </div>
                    </label>
                  </div>
                </div>

                <div class="fallback-group">
                  <label class="config-label">觸發條件</label>
                  <div class="radio-row">
                    <label class="radio-option" :class="{ selected: draft.fallback_trigger === 'error' }">
                      <input v-model="draft.fallback_trigger" type="radio" value="error" class="scheme-radio" />
                      <div>
                        <span class="radio-label">連線失敗時</span>
                        <span class="radio-desc">主方案無法連線或回傳錯誤時啟用備案</span>
                      </div>
                    </label>
                    <label class="radio-option" :class="{ selected: draft.fallback_trigger === 'empty_data' }">
                      <input v-model="draft.fallback_trigger" type="radio" value="empty_data" class="scheme-radio" />
                      <div>
                        <span class="radio-label">資料為空時</span>
                        <span class="radio-desc">主方案成功但資料為空時也啟用備案</span>
                      </div>
                    </label>
                  </div>
                </div>
              </div>

              <!-- 備案流程預覽 -->
              <div class="flow-preview">
                <div class="flow-node primary-node" :style="{ '--c': schemeColor(draft.primary) }">
                  {{ selectedFeature.schemes.find(s => s.id === draft.primary)?.label ?? draft.primary }}
                </div>
                <div class="flow-arrow">
                  <span class="flow-arrow-label">
                    {{ draft.fallback_trigger === 'error' ? '連線失敗 →' : '資料為空 →' }}
                  </span>
                  →
                </div>
                <div
                  v-if="draft.fallback"
                  class="flow-node fallback-node"
                  :style="{ '--c': schemeColor(draft.fallback) }"
                >
                  {{ selectedFeature.schemes.find(s => s.id === draft.fallback)?.label ?? draft.fallback }}
                </div>
                <div v-else class="flow-node empty-node">（未選擇備案）</div>
              </div>
            </template>
          </section>

          <!-- section: 方案說明彙整 -->
          <section class="setting-section">
            <div class="section-title">
              <span class="section-icon">③</span> 所有方案說明
            </div>
            <table class="scheme-table">
              <thead>
                <tr>
                  <th>方案</th>
                  <th>說明</th>
                  <th>依賴</th>
                  <th>狀態</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="s in selectedFeature.schemes" :key="s.id">
                  <td>
                    <div class="scheme-table-id">
                      <span class="scheme-dot" :style="{ background: schemeColor(s.id) }" />
                      <span class="scheme-table-label">{{ s.label }}</span>
                    </div>
                    <code class="scheme-id-code">{{ s.id }}</code>
                  </td>
                  <td class="scheme-table-desc">{{ s.description }}</td>
                  <td>
                    <span v-if="s.need_service" class="need-service-badge">{{ s.need_service }}</span>
                    <span v-else class="no-dep">內建</span>
                  </td>
                  <td>
                    <span v-if="s.id === draft.primary" class="status-primary">主要</span>
                    <span v-else-if="s.id === draft.fallback && draft.fallback_enabled" class="status-fallback">備案</span>
                    <span v-else class="status-idle">備用</span>
                  </td>
                </tr>
              </tbody>
            </table>
          </section>
        </template>
      </main>
    </div>
  </div>
</template>

<style scoped>
*, *::before, *::after { box-sizing: border-box; margin: 0; padding: 0; }

.settings-page {
  font-family: 'DM Sans', sans-serif;
  background: #0d1117;
  color: #c9d1d9;
  min-height: 100vh;
  display: flex;
  flex-direction: column;
}

/* ── Header ─────────────────────────────────────────────────── */
.header {
  display: flex; align-items: center; justify-content: space-between;
  padding: 14px 24px;
  background: #161b22; border-bottom: 1px solid #30363d;
  position: sticky; top: 0; z-index: 10;
}
.header-left { display: flex; align-items: center; gap: 10px; }
.header-icon { font-size: 1.3rem; }
.header-title { font-size: 1.1rem; font-weight: 600; color: #e6edf3; }
.header-sub { font-size: 0.78rem; color: #8b949e; }
.btn-refresh {
  display: flex; align-items: center; gap: 6px;
  padding: 6px 14px; border-radius: 6px;
  background: #21262d; border: 1px solid #30363d;
  color: #c9d1d9; font-size: 0.85rem; cursor: pointer;
  transition: background 0.15s;
}
.btn-refresh:hover { background: #30363d; }
.btn-refresh:disabled { opacity: 0.5; cursor: default; }

/* ── Error Banner ───────────────────────────────────────────── */
.error-banner {
  background: #2a1a1a; border-left: 3px solid #f85149;
  padding: 10px 24px; font-size: 0.88rem; color: #f85149;
}

/* ── Layout ─────────────────────────────────────────────────── */
.layout {
  display: flex; flex: 1; overflow: hidden;
  height: calc(100vh - 57px);
}

/* ── Sidebar ─────────────────────────────────────────────────── */
.sidebar {
  width: 260px; min-width: 220px;
  background: #161b22; border-right: 1px solid #30363d;
  overflow-y: auto; padding: 12px 0; flex-shrink: 0;
}
.sidebar-section-title {
  font-size: 0.7rem; font-weight: 600; letter-spacing: 0.08em;
  text-transform: uppercase; color: #8b949e;
  padding: 0 16px 8px;
}
.feature-list { list-style: none; }
.feature-item {
  padding: 10px 16px; cursor: pointer;
  border-left: 3px solid transparent;
  transition: background 0.1s;
}
.feature-item:hover { background: #1c2128; }
.feature-item.active { background: #1c2128; border-left-color: #388bfd; }
.feature-item-top { display: flex; align-items: center; justify-content: space-between; margin-bottom: 5px; }
.feature-item-label { font-size: 0.9rem; font-weight: 500; color: #e6edf3; }
.badge-category {
  font-size: 0.68rem; padding: 1px 7px; border-radius: 10px;
  background: #21262d; color: #8b949e;
}
.feature-item-scheme { display: flex; align-items: center; gap: 6px; }
.scheme-dot { width: 8px; height: 8px; border-radius: 50%; flex-shrink: 0; }
.scheme-id-text { font-size: 0.78rem; font-family: 'Fira Code', monospace; color: #8b949e; }
.fallback-badge {
  font-size: 0.65rem; padding: 1px 5px; border-radius: 8px;
  background: #1a2a1a; color: #56d364; border: 1px solid #238636;
}

/* ── Content ─────────────────────────────────────────────────── */
.content {
  flex: 1; overflow-y: auto;
  padding: 0;
}
.empty-state {
  display: flex; flex-direction: column; align-items: center;
  justify-content: center; height: 100%; gap: 12px; color: #8b949e;
}
.empty-icon { font-size: 3rem; }

/* ── Content Header ─────────────────────────────────────────── */
.content-header {
  display: flex; align-items: flex-start; justify-content: space-between;
  padding: 20px 28px 0;
}
.content-title { font-size: 1.2rem; font-weight: 600; color: #e6edf3; margin-bottom: 4px; }
.content-desc { font-size: 0.85rem; color: #8b949e; }
.header-actions { display: flex; align-items: center; gap: 12px; flex-shrink: 0; margin-left: 16px; }
.saved-badge {
  font-size: 0.82rem; color: #56d364; background: #1a2a1a;
  border: 1px solid #238636; padding: 4px 10px; border-radius: 6px;
}
.btn-save {
  padding: 7px 20px; border-radius: 6px;
  background: #21262d; border: 1px solid #30363d;
  color: #8b949e; font-size: 0.88rem; cursor: pointer;
  transition: all 0.15s;
}
.btn-save.dirty {
  background: #1a3a5c; border-color: #388bfd; color: #58a6ff;
}
.btn-save:disabled { opacity: 0.5; cursor: default; }

/* ── Sections ────────────────────────────────────────────────── */
.setting-section {
  padding: 20px 28px;
  border-bottom: 1px solid #21262d;
}
.section-title {
  display: flex; align-items: center; gap: 8px;
  font-size: 0.88rem; font-weight: 600; color: #e6edf3; margin-bottom: 16px;
}
.section-icon {
  font-size: 0.85rem; background: #21262d;
  width: 22px; height: 22px; border-radius: 50%;
  display: flex; align-items: center; justify-content: center;
  flex-shrink: 0;
}
.section-hint { font-size: 0.78rem; color: #8b949e; font-weight: 400; }

/* ── Scheme Grid (主方案) ────────────────────────────────────── */
.scheme-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(220px, 1fr)); gap: 12px; }
.scheme-card {
  border: 1px solid #30363d; border-radius: 8px;
  background: #161b22; padding: 14px; cursor: pointer;
  transition: all 0.15s; position: relative; overflow: hidden;
}
.scheme-card:hover { border-color: #58a6ff30; background: #1c2128; }
.scheme-card.selected { border-color: var(--accent); background: #1c2128; }
.scheme-card.selected::before {
  content: ''; position: absolute; top: 0; left: 0; right: 0; height: 2px;
  background: var(--accent);
}
.scheme-radio { position: absolute; opacity: 0; pointer-events: none; }
.scheme-card-header { display: flex; align-items: center; gap: 8px; margin-bottom: 8px; }
.scheme-color-bar {
  width: 10px; height: 10px; border-radius: 50%;
  background: var(--accent); flex-shrink: 0;
}
.scheme-label { font-size: 0.9rem; font-weight: 500; color: #e6edf3; flex: 1; }
.scheme-active-badge {
  font-size: 0.68rem; padding: 1px 6px; border-radius: 8px;
  background: color-mix(in srgb, var(--accent) 20%, transparent);
  color: var(--accent); border: 1px solid color-mix(in srgb, var(--accent) 50%, transparent);
}
.scheme-desc { font-size: 0.8rem; color: #8b949e; line-height: 1.5; }
.scheme-requires {
  margin-top: 8px; font-size: 0.75rem; color: #f0a842;
}
.scheme-requires code { font-family: 'Fira Code', monospace; }

/* ── Fallback Controls ───────────────────────────────────────── */
.fallback-row { margin-bottom: 16px; }
.toggle-label { display: flex; align-items: center; gap: 10px; cursor: pointer; user-select: none; }
.toggle {
  width: 38px; height: 20px; border-radius: 10px; background: #30363d;
  position: relative; transition: background 0.2s; cursor: pointer;
}
.toggle.on { background: #238636; }
.toggle-thumb {
  position: absolute; top: 2px; left: 2px;
  width: 16px; height: 16px; border-radius: 50%; background: white;
  transition: left 0.15s;
}
.toggle.on .toggle-thumb { left: 20px; }

.fallback-config { display: flex; flex-direction: column; gap: 16px; }
.fallback-group { }
.config-label { display: block; font-size: 0.8rem; color: #8b949e; margin-bottom: 8px; }
.no-fallback-hint { font-size: 0.82rem; color: #6e7681; font-style: italic; }

.scheme-select-row { display: flex; flex-direction: column; gap: 8px; }
.scheme-select-item {
  display: flex; align-items: flex-start; gap: 10px;
  padding: 10px 12px; border: 1px solid #30363d; border-radius: 6px;
  cursor: pointer; transition: all 0.15s;
}
.scheme-select-item:hover { background: #1c2128; }
.scheme-select-item.selected { border-color: var(--accent); background: #1c2128; }
.scheme-color-dot {
  width: 8px; height: 8px; border-radius: 50%;
  background: var(--accent); flex-shrink: 0; margin-top: 3px;
}
.scheme-select-label { font-size: 0.88rem; color: #e6edf3; font-weight: 500; }
.scheme-select-desc { font-size: 0.78rem; color: #8b949e; }

.radio-row { display: flex; flex-direction: column; gap: 8px; }
.radio-option {
  display: flex; align-items: flex-start; gap: 10px;
  padding: 10px 12px; border: 1px solid #30363d; border-radius: 6px;
  cursor: pointer; transition: all 0.15s;
}
.radio-option:hover { background: #1c2128; }
.radio-option.selected { border-color: #58a6ff; background: #1c2128; }
.radio-label { font-size: 0.88rem; color: #e6edf3; font-weight: 500; display: block; margin-bottom: 2px; }
.radio-desc { font-size: 0.78rem; color: #8b949e; }

/* ── Flow Preview ───────────────────────────────────────────── */
.flow-preview {
  display: flex; align-items: center; gap: 12px;
  margin-top: 16px; padding: 14px 16px; border-radius: 8px;
  background: #0d1117; border: 1px solid #21262d;
}
.flow-node {
  padding: 6px 14px; border-radius: 20px; font-size: 0.82rem; font-weight: 500;
  border: 1px solid color-mix(in srgb, var(--c) 50%, transparent);
  background: color-mix(in srgb, var(--c) 15%, transparent);
  color: var(--c);
}
.primary-node { }
.fallback-node { }
.empty-node { --c: #6e7681; }
.flow-arrow { display: flex; flex-direction: column; align-items: center; gap: 2px; color: #6e7681; font-size: 1rem; }
.flow-arrow-label { font-size: 0.68rem; color: #6e7681; white-space: nowrap; }

/* ── Scheme Table ───────────────────────────────────────────── */
.scheme-table { width: 100%; border-collapse: collapse; font-size: 0.83rem; }
.scheme-table th {
  padding: 8px 12px; text-align: left; font-size: 0.72rem;
  font-weight: 600; letter-spacing: 0.05em; text-transform: uppercase;
  color: #8b949e; border-bottom: 1px solid #30363d;
}
.scheme-table td { padding: 10px 12px; border-bottom: 1px solid #21262d; vertical-align: top; }
.scheme-table tr:hover td { background: #1c2128; }
.scheme-table-id { display: flex; align-items: center; gap: 6px; margin-bottom: 4px; }
.scheme-table-label { font-size: 0.88rem; color: #e6edf3; }
.scheme-id-code {
  font-family: 'Fira Code', monospace; font-size: 0.75rem;
  color: #8b949e; background: #21262d; padding: 1px 6px; border-radius: 4px;
}
.scheme-table-desc { font-size: 0.82rem; color: #8b949e; line-height: 1.5; }
.need-service-badge {
  font-family: 'Fira Code', monospace; font-size: 0.75rem;
  background: #1a2a3a; color: #5b9cf6; padding: 2px 7px; border-radius: 4px;
}
.no-dep { font-size: 0.78rem; color: #6e7681; }
.status-primary {
  font-size: 0.75rem; padding: 2px 8px; border-radius: 10px;
  background: #1a3a5c; color: #58a6ff; border: 1px solid #388bfd;
}
.status-fallback {
  font-size: 0.75rem; padding: 2px 8px; border-radius: 10px;
  background: #1a2a1a; color: #56d364; border: 1px solid #238636;
}
.status-idle { font-size: 0.75rem; color: #6e7681; }

/* ── Misc ────────────────────────────────────────────────────── */
.loading-text { padding: 16px; color: #8b949e; font-size: 0.88rem; }
.spin { display: inline-block; animation: spin 1s linear infinite; }
@keyframes spin { to { transform: rotate(360deg); } }
</style>
