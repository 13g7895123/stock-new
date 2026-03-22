<script setup lang="ts">
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

const syncing = ref(false)
const syncState = ref<SyncState | null>(null)

function syncStocks() {
  if (syncing.value) return
  syncing.value = true
  syncState.value = null

  const es = new EventSource('/api/scraper/stocks')

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
        stage: 'error',
        message: '',
        progress: 0,
        url: '',
        total: 0,
        synced: 0,
        error: '連線失敗，請確認後端服務是否正常',
      }
    }
  }
}
</script>

<template>
  <div class="container">
    <header>
      <h1>台灣股票列表</h1>
      <div class="toolbar">
        <span class="count">共 {{ stocks?.length ?? 0 }} 支股票</span>
        <button :disabled="syncing" class="sync-btn" @click="syncStocks">
          {{ syncing ? '同步中...' : '同步最新清單' }}
        </button>
      </div>

      <!-- 進度區塊 -->
      <div v-if="syncState" class="sync-panel" :class="syncState.stage">
        <!-- 來源網址 -->
        <div v-if="syncState.url" class="source-url">
          資料來源：<a :href="syncState.url" target="_blank" rel="noopener">{{ syncState.url }}</a>
        </div>

        <!-- 進度條 -->
        <div v-if="syncState.stage !== 'error'" class="progress-wrap">
          <div class="progress-bar">
            <div class="progress-fill" :style="{ width: syncState.progress + '%' }" />
          </div>
          <span class="progress-pct">{{ syncState.progress }}%</span>
        </div>

        <!-- 狀態訊息 -->
        <p class="sync-message">
          <span v-if="syncState.stage === 'error'" class="error-icon">✕</span>
          <span v-else-if="syncState.stage === 'done'" class="done-icon">✓</span>
          {{ syncState.stage === 'error' ? syncState.error : syncState.message }}
        </p>
      </div>
    </header>

    <div v-if="status === 'pending'" class="loading">載入中...</div>

    <table v-else-if="stocks && stocks.length > 0">
      <thead>
        <tr>
          <th>代號</th>
          <th>名稱</th>
          <th>股價</th>
          <th>漲跌</th>
          <th>漲跌幅</th>
          <th>成交量</th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="stock in stocks" :key="stock.id">
          <td class="symbol">{{ stock.symbol }}</td>
          <td>{{ stock.name }}</td>
          <td>{{ stock.price > 0 ? stock.price.toFixed(2) : '-' }}</td>
          <td :class="stock.change > 0 ? 'up' : stock.change < 0 ? 'down' : ''">
            {{ stock.price > 0 ? (stock.change > 0 ? '+' : '') + stock.change.toFixed(2) : '-' }}
          </td>
          <td :class="stock.change_pct > 0 ? 'up' : stock.change_pct < 0 ? 'down' : ''">
            {{ stock.price > 0 ? (stock.change_pct > 0 ? '+' : '') + stock.change_pct.toFixed(2) + '%' : '-' }}
          </td>
          <td>{{ stock.volume > 0 ? stock.volume.toLocaleString() : '-' }}</td>
        </tr>
      </tbody>
    </table>

    <div v-else class="empty">
      尚無股票資料，請點擊「同步最新清單」從 TWSE 抓取。
    </div>
  </div>
</template>

<style scoped>
* {
  box-sizing: border-box;
}

.container {
  max-width: 1100px;
  margin: 0 auto;
  padding: 24px 16px;
  font-family: system-ui, -apple-system, sans-serif;
}

header {
  margin-bottom: 24px;
}

h1 {
  font-size: 1.75rem;
  font-weight: 700;
  margin: 0 0 12px;
  color: #1a1a2e;
}

.toolbar {
  display: flex;
  align-items: center;
  gap: 16px;
  margin-bottom: 12px;
}

.count {
  color: #666;
  font-size: 0.9rem;
}

.sync-btn {
  padding: 8px 18px;
  background: #2563eb;
  color: #fff;
  border: none;
  border-radius: 6px;
  cursor: pointer;
  font-size: 0.9rem;
  font-weight: 500;
  transition: background 0.15s;
}

.sync-btn:hover:not(:disabled) {
  background: #1d4ed8;
}

.sync-btn:disabled {
  background: #93c5fd;
  cursor: not-allowed;
}

/* 進度面板 */
.sync-panel {
  border: 1px solid #e2e8f0;
  border-radius: 8px;
  padding: 12px 16px;
  background: #f8fafc;
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.sync-panel.error {
  border-color: #fca5a5;
  background: #fff5f5;
}

.sync-panel.done {
  border-color: #86efac;
  background: #f0fdf4;
}

.source-url {
  font-size: 0.8rem;
  color: #64748b;
}

.source-url a {
  color: #2563eb;
  text-decoration: none;
  word-break: break-all;
}

.source-url a:hover {
  text-decoration: underline;
}

.progress-wrap {
  display: flex;
  align-items: center;
  gap: 10px;
}

.progress-bar {
  flex: 1;
  height: 8px;
  background: #e2e8f0;
  border-radius: 4px;
  overflow: hidden;
}

.progress-fill {
  height: 100%;
  background: #2563eb;
  border-radius: 4px;
  transition: width 0.4s ease;
}

.sync-panel.done .progress-fill {
  background: #16a34a;
}

.progress-pct {
  font-size: 0.8rem;
  color: #475569;
  width: 36px;
  text-align: right;
}

.sync-message {
  margin: 0;
  font-size: 0.875rem;
  color: #334155;
  display: flex;
  align-items: center;
  gap: 6px;
}

.done-icon {
  color: #16a34a;
  font-weight: 700;
}

.error-icon {
  color: #dc2626;
  font-weight: 700;
}

.loading,
.empty {
  padding: 48px;
  text-align: center;
  color: #888;
}

table {
  width: 100%;
  border-collapse: collapse;
  font-size: 0.9rem;
}

th {
  text-align: left;
  padding: 10px 12px;
  background: #f1f5f9;
  color: #475569;
  font-weight: 600;
  border-bottom: 2px solid #e2e8f0;
}

td {
  padding: 10px 12px;
  border-bottom: 1px solid #e2e8f0;
  color: #1e293b;
}

tr:hover td {
  background: #f8fafc;
}

.symbol {
  font-weight: 600;
  color: #2563eb;
}

.up {
  color: #dc2626;
}

.down {
  color: #16a34a;
}
</style>
