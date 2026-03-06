<template>
  <div class="p-6 space-y-6">
    <!-- Header -->
    <div class="flex items-center justify-between flex-wrap gap-3">
      <div>
        <h1 class="text-xl font-bold text-white">看板</h1>
        <p class="text-sm text-gray-500 mt-0.5">接口调用统计 · 实时日志</p>
      </div>

      <!-- Controls Row -->
      <div class="flex items-center gap-2 flex-wrap">
        <!-- Time Range -->
        <div class="flex gap-1 bg-gray-900 border border-gray-800 rounded-lg p-0.5">
          <button
            v-for="r in timeRanges"
            :key="r.value"
            @click="timeRange = r.value"
            class="px-3 py-1 rounded text-xs font-medium transition-all"
            :class="timeRange === r.value
              ? 'bg-blue-600 text-white'
              : 'text-gray-400 hover:text-gray-200'"
          >{{ r.label }}</button>
        </div>

        <!-- Live Toggle -->
        <button
          @click="toggleLive"
          class="flex items-center gap-2 px-3 py-1.5 rounded-lg text-xs font-medium border transition-all"
          :class="liveMode
            ? 'bg-green-900/40 border-green-700 text-green-400'
            : 'bg-gray-800 border-gray-700 text-gray-400 hover:text-gray-200'"
        >
          <span
            class="w-2 h-2 rounded-full"
            :class="liveMode ? 'bg-green-400 animate-pulse' : 'bg-gray-600'"
          ></span>
          {{ liveMode ? '实时 · ' + liveInterval + 's' : '实时' }}
        </button>

        <!-- Manual Refresh -->
        <button
          @click="refresh"
          :disabled="loading"
          class="flex items-center gap-1.5 px-3 py-1.5 bg-gray-800 hover:bg-gray-700 border border-gray-700 text-gray-300 rounded-lg text-xs transition-colors"
        >
          <svg class="w-3.5 h-3.5" :class="{ 'animate-spin': loading }" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15"/>
          </svg>
          刷新
        </button>

        <div class="text-xs text-gray-600 whitespace-nowrap">{{ lastUpdated }}</div>
      </div>
    </div>

    <!-- Channel Tabs -->
    <div class="flex gap-1 bg-gray-900 p-1 rounded-xl border border-gray-800 overflow-x-auto">
      <button
        v-for="ch in channels"
        :key="ch.id"
        @click="selectChannel(ch.id)"
        class="flex items-center gap-2 px-3 py-1.5 rounded-lg text-xs font-medium transition-all whitespace-nowrap"
        :class="activeChannel === ch.id
          ? 'bg-blue-600 text-white shadow-lg shadow-blue-900/40'
          : 'text-gray-400 hover:text-gray-200 hover:bg-gray-800'"
      >
        <span>{{ ch.icon }}</span>
        <span>{{ ch.label }}</span>
        <span v-if="ch.count !== null" class="text-xs px-1.5 py-0.5 rounded-full"
          :class="activeChannel === ch.id ? 'bg-blue-500 text-blue-100' : 'bg-gray-700 text-gray-400'">
          {{ ch.count }}
        </span>
      </button>
    </div>

    <!-- Time Range Banner -->
    <div class="flex items-center gap-2 text-xs text-gray-500">
      <svg class="w-3.5 h-3.5 text-gray-600" fill="none" stroke="currentColor" viewBox="0 0 24 24">
        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8 7V3m8 4V3m-9 8h10M5 21h14a2 2 0 002-2V7a2 2 0 00-2-2H5a2 2 0 00-2 2v12a2 2 0 002 2z"/>
      </svg>
      <span>统计范围：<span class="text-gray-300">{{ currentRangeLabel }}</span></span>
      <span class="text-gray-700">·</span>
      <span>匹配 <span class="text-gray-300 font-medium">{{ rangedLogs.length }}</span> 条 / 共 {{ logs.length }} 条</span>
    </div>

    <!-- Stats Cards -->
    <div class="grid grid-cols-2 lg:grid-cols-4 gap-4">
      <div class="bg-gray-900 border border-gray-800 rounded-xl p-4 space-y-2">
        <div class="flex items-center justify-between">
          <span class="text-xs text-gray-500 font-medium uppercase tracking-wide">总调用</span>
          <div class="w-8 h-8 bg-blue-900/50 rounded-lg flex items-center justify-center">
            <svg class="w-4 h-4 text-blue-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 19v-6a2 2 0 00-2-2H5a2 2 0 00-2 2v6a2 2 0 002 2h2a2 2 0 002-2zm0 0V9a2 2 0 012-2h2a2 2 0 012 2v10m-6 0a2 2 0 002 2h2a2 2 0 002-2m0 0V5a2 2 0 012-2h2a2 2 0 012 2v14a2 2 0 01-2 2h-2a2 2 0 01-2-2z"/>
            </svg>
          </div>
        </div>
        <div class="text-2xl font-bold text-white">{{ stats.total }}</div>
        <div class="text-xs text-gray-500">{{ currentRangeLabel }}</div>
      </div>

      <div class="bg-gray-900 border border-gray-800 rounded-xl p-4 space-y-2">
        <div class="flex items-center justify-between">
          <span class="text-xs text-gray-500 font-medium uppercase tracking-wide">成功率</span>
          <div class="w-8 h-8 bg-green-900/50 rounded-lg flex items-center justify-center">
            <svg class="w-4 h-4 text-green-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z"/>
            </svg>
          </div>
        </div>
        <div class="text-2xl font-bold text-white">{{ stats.successRate }}<span class="text-sm font-normal text-gray-400">%</span></div>
        <div class="text-xs text-gray-500">{{ stats.success }} 成功 / {{ stats.failed }} 失败</div>
      </div>

      <div class="bg-gray-900 border border-gray-800 rounded-xl p-4 space-y-2">
        <div class="flex items-center justify-between">
          <span class="text-xs text-gray-500 font-medium uppercase tracking-wide">平均响应</span>
          <div class="w-8 h-8 bg-purple-900/50 rounded-lg flex items-center justify-center">
            <svg class="w-4 h-4 text-purple-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z"/>
            </svg>
          </div>
        </div>
        <div class="text-2xl font-bold text-white">{{ stats.avgDuration }}<span class="text-sm font-normal text-gray-500 ml-1">ms</span></div>
        <div class="text-xs text-gray-500">最慢 {{ stats.maxDuration }}ms</div>
      </div>

      <div class="bg-gray-900 border border-gray-800 rounded-xl p-4 space-y-2">
        <div class="flex items-center justify-between">
          <span class="text-xs text-gray-500 font-medium uppercase tracking-wide">累计 Tokens</span>
          <div class="w-8 h-8 bg-orange-900/50 rounded-lg flex items-center justify-center">
            <svg class="w-4 h-4 text-orange-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 10V3L4 14h7v7l9-11h-7z"/>
            </svg>
          </div>
        </div>
        <div class="text-2xl font-bold text-white">{{ formatTokens(stats.totalTokens) }}</div>
        <div class="text-xs text-gray-500">↑{{ formatTokens(stats.inputTokens) }} ↓{{ formatTokens(stats.outputTokens) }}</div>
      </div>
    </div>

    <!-- Charts + Account Stats -->
    <div class="grid grid-cols-1 lg:grid-cols-3 gap-4">
      <!-- Trend Chart -->
      <div class="lg:col-span-2 bg-gray-900 border border-gray-800 rounded-xl p-4">
        <div class="flex items-center justify-between mb-4">
          <div class="flex items-center gap-2">
            <h3 class="text-sm font-semibold text-white">调用趋势</h3>
            <span class="text-xs text-gray-600">{{ chartData.length }} 个时间段</span>
          </div>
          <div class="flex gap-4 text-xs text-gray-500">
            <span class="flex items-center gap-1">
              <span class="w-2 h-2 rounded-full bg-green-500 inline-block"></span>成功
            </span>
            <span class="flex items-center gap-1">
              <span class="w-2 h-2 rounded-full bg-red-500 inline-block"></span>失败
            </span>
          </div>
        </div>

        <!-- Stacked bar chart by time bucket -->
        <div class="flex items-end gap-0.5 h-28" v-if="chartData.length">
          <div
            v-for="(bucket, i) in chartData"
            :key="i"
            class="flex-1 flex flex-col items-center group relative"
          >
            <!-- Tooltip -->
            <div class="absolute bottom-full mb-1 left-1/2 -translate-x-1/2 hidden group-hover:block z-10 pointer-events-none">
              <div class="bg-gray-800 border border-gray-700 rounded-lg px-2 py-1.5 text-xs whitespace-nowrap shadow-xl">
                <div class="text-gray-300 font-medium">{{ bucket.label }}</div>
                <div class="text-green-400">✓ {{ bucket.success }}</div>
                <div v-if="bucket.failed > 0" class="text-red-400">✗ {{ bucket.failed }}</div>
                <div class="text-gray-500">avg {{ bucket.avgMs }}ms</div>
              </div>
            </div>

            <!-- Bar -->
            <div class="w-full flex flex-col justify-end" style="height:96px">
              <div
                v-if="bucket.failed > 0"
                class="w-full rounded-t-sm bg-red-500/80"
                :style="{ height: bucket.failedPct + '%', minHeight: bucket.failed ? '3px' : '0' }"
              ></div>
              <div
                class="w-full bg-green-500/80"
                :class="bucket.failed > 0 ? '' : 'rounded-t-sm'"
                :style="{ height: bucket.successPct + '%', minHeight: bucket.success ? '3px' : '0' }"
              ></div>
            </div>
            <div class="text-gray-700 text-center mt-0.5 group-hover:text-gray-500 transition-colors overflow-hidden" style="font-size:8px;width:100%;white-space:nowrap;text-overflow:ellipsis">
              {{ bucket.label }}
            </div>
          </div>
        </div>
        <div v-else class="h-28 flex items-center justify-center text-gray-600 text-sm">
          该时间范围内暂无数据
        </div>
      </div>

      <!-- Account Distribution -->
      <div class="bg-gray-900 border border-gray-800 rounded-xl p-4">
        <h3 class="text-sm font-semibold text-white mb-4">账号调用分布</h3>
        <div class="space-y-3" v-if="accountStats.length">
          <div v-for="acc in accountStats.slice(0, 6)" :key="acc.email" class="space-y-1">
            <div class="flex items-center justify-between text-xs">
              <span class="text-gray-300 truncate max-w-[140px]" :title="acc.email">{{ acc.email }}</span>
              <span class="text-gray-500 ml-2 flex-shrink-0">{{ acc.count }}</span>
            </div>
            <div class="w-full bg-gray-800 rounded-full h-1.5">
              <div
                class="h-1.5 rounded-full bg-gradient-to-r from-blue-500 to-blue-400 transition-all duration-500"
                :style="{ width: acc.pct + '%' }"
              ></div>
            </div>
          </div>
        </div>
        <div v-else class="text-gray-600 text-sm text-center py-8">暂无数据</div>
      </div>
    </div>

    <!-- Model Distribution -->
    <div class="grid grid-cols-2 lg:grid-cols-4 gap-3" v-if="modelStats.length">
      <div
        v-for="m in modelStats"
        :key="m.model"
        class="bg-gray-900 border border-gray-800 rounded-xl p-3 flex items-center gap-3"
      >
        <div class="w-9 h-9 rounded-lg bg-indigo-900/50 flex items-center justify-center flex-shrink-0">
          <svg class="w-4 h-4 text-indigo-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9.663 17h4.673M12 3v1m6.364 1.636l-.707.707M21 12h-1M4 12H3m3.343-5.657l-.707-.707m2.828 9.9a5 5 0 117.072 0l-.548.547A3.374 3.374 0 0014 18.469V19a2 2 0 11-4 0v-.531c0-.895-.356-1.754-.988-2.386l-.548-.547z"/>
          </svg>
        </div>
        <div class="min-w-0">
          <div class="text-white text-sm font-medium truncate">{{ m.model || '未知模型' }}</div>
          <div class="text-gray-500 text-xs">{{ m.count }} 次</div>
        </div>
      </div>
    </div>

    <!-- Log Table -->
    <div class="bg-gray-900 border border-gray-800 rounded-xl overflow-hidden">
      <div class="flex items-center justify-between px-4 py-3 border-b border-gray-800 gap-2 flex-wrap">
        <div class="flex items-center gap-3">
          <h3 class="text-sm font-semibold text-white">调用日志</h3>
          <span class="text-xs text-gray-600">{{ logTotal }} 条，当前显示 {{ filteredLogs.length }} 条</span>
          <!-- Live indicator in table header -->
          <div v-if="liveMode" class="flex items-center gap-1.5 text-xs text-green-400">
            <span class="w-1.5 h-1.5 rounded-full bg-green-400 animate-pulse inline-block"></span>
            实时更新中
          </div>
        </div>
        <div class="flex items-center gap-2 flex-wrap">
          <!-- Status filter -->
          <select
            v-model="statusFilter"
            class="text-xs bg-gray-800 border border-gray-700 text-gray-300 rounded-lg px-2 py-1.5 focus:outline-none focus:border-blue-500"
          >
            <option value="">全部状态</option>
            <option value="2xx">成功 (2xx)</option>
            <option value="4xx">客户端错误 (4xx)</option>
            <option value="5xx">服务端错误 (5xx)</option>
          </select>

          <!-- Live interval selector (only when live is on) -->
          <div v-if="liveMode" class="flex gap-1 bg-gray-800 border border-gray-700 rounded-lg p-0.5">
            <button
              v-for="s in [3, 5, 10]"
              :key="s"
              @click="setLiveInterval(s)"
              class="px-2 py-0.5 rounded text-xs transition-all"
              :class="liveInterval === s ? 'bg-green-700 text-green-100' : 'text-gray-500 hover:text-gray-300'"
            >{{ s }}s</button>
          </div>

          <button
            @click="clearLogs"
            class="text-xs px-3 py-1.5 bg-red-900/30 hover:bg-red-900/50 text-red-400 rounded-lg transition-colors"
          >清空</button>
        </div>
      </div>

      <!-- Table -->
      <div class="overflow-x-auto">
        <table class="w-full text-sm">
          <thead>
            <tr class="text-xs text-gray-500 border-b border-gray-800 bg-gray-900/50">
              <th class="text-left px-4 py-2.5 font-medium">时间</th>
              <th class="text-left px-4 py-2.5 font-medium">账号</th>
              <th class="text-left px-4 py-2.5 font-medium">模型</th>
              <th class="text-left px-4 py-2.5 font-medium">平台</th>
              <th class="text-left px-4 py-2.5 font-medium">状态</th>
              <th class="text-right px-4 py-2.5 font-medium">耗时</th>
              <th class="text-right px-4 py-2.5 font-medium">Tokens</th>
            </tr>
          </thead>
          <tbody>
            <tr
              v-for="(log, i) in filteredLogs"
              :key="log.id"
              class="border-b border-gray-800/40 hover:bg-gray-800/30 transition-colors"
              :class="{ 'log-new': i === 0 && liveMode }"
            >
              <td class="px-4 py-2.5 text-gray-400 text-xs whitespace-nowrap font-mono">{{ formatTime(log.created_at) }}</td>
              <td class="px-4 py-2.5">
                <span class="text-gray-300 text-xs">{{ log.account_email }}</span>
              </td>
              <td class="px-4 py-2.5">
                <span v-if="log.model" class="px-2 py-0.5 bg-indigo-900/40 text-indigo-300 rounded text-xs font-mono">{{ log.model }}</span>
                <span v-else class="text-gray-700 text-xs">—</span>
              </td>
              <td class="px-4 py-2.5">
                <span v-if="log.platform" class="px-2 py-0.5 rounded text-xs" :class="platformClass(log.platform)">{{ platformIcon(log.platform) }} {{ log.platform }}</span>
                <span v-else class="text-gray-700 text-xs">—</span>
              </td>
              <td class="px-4 py-2.5">
                <span class="px-2 py-0.5 rounded text-xs font-medium" :class="getStatusClass(log.status_code)">
                  {{ log.status_code }}
                </span>
              </td>
              <td class="px-4 py-2.5 text-right text-xs font-mono">
                <span :class="log.duration_ms > 10000 ? 'text-red-400' : log.duration_ms > 5000 ? 'text-yellow-400' : 'text-gray-400'">
                  {{ log.duration_ms }}ms
                </span>
              </td>
              <td class="px-4 py-2.5 text-right text-xs">
                <span v-if="log.input_tokens || log.output_tokens" class="text-gray-300 font-mono">
                  {{ log.input_tokens + log.output_tokens }}
                  <span class="text-gray-600 ml-1">({{ log.input_tokens }}+{{ log.output_tokens }})</span>
                </span>
                <span v-else class="text-gray-700">—</span>
              </td>
            </tr>
            <tr v-if="!filteredLogs.length">
              <td colspan="7" class="px-4 py-12 text-center text-gray-600 text-sm">
                {{ logs.length ? '当前筛选条件无匹配数据' : '暂无日志数据' }}
              </td>
            </tr>
          </tbody>
        </table>
      </div>

      <!-- Pagination -->
      <div class="flex items-center justify-between px-4 py-3 border-t border-gray-800">
        <div class="text-xs text-gray-500">
          共 <span class="text-gray-300 font-medium">{{ logTotal }}</span> 条日志，当前第 {{ logPage }} / {{ logTotalPages }} 页
        </div>
        <div class="flex items-center gap-1.5">
          <button
            @click="goLogPage(1)"
            :disabled="logPage <= 1"
            class="px-2 py-1 rounded text-xs bg-gray-800 text-gray-400 hover:bg-gray-700 hover:text-white disabled:opacity-30 disabled:cursor-not-allowed transition-colors"
          >首页</button>
          <button
            @click="goLogPage(logPage - 1)"
            :disabled="logPage <= 1"
            class="px-2.5 py-1 rounded text-xs bg-gray-800 text-gray-400 hover:bg-gray-700 hover:text-white disabled:opacity-30 disabled:cursor-not-allowed transition-colors"
          >上一页</button>

          <template v-for="p in paginationRange" :key="p">
            <span v-if="p === '...'" class="px-1 text-xs text-gray-600">...</span>
            <button
              v-else
              @click="goLogPage(p)"
              class="w-7 h-7 rounded text-xs font-medium transition-colors"
              :class="p === logPage
                ? 'bg-blue-600 text-white'
                : 'bg-gray-800 text-gray-400 hover:bg-gray-700 hover:text-white'"
            >{{ p }}</button>
          </template>

          <button
            @click="goLogPage(logPage + 1)"
            :disabled="logPage >= logTotalPages"
            class="px-2.5 py-1 rounded text-xs bg-gray-800 text-gray-400 hover:bg-gray-700 hover:text-white disabled:opacity-30 disabled:cursor-not-allowed transition-colors"
          >下一页</button>
          <button
            @click="goLogPage(logTotalPages)"
            :disabled="logPage >= logTotalPages"
            class="px-2 py-1 rounded text-xs bg-gray-800 text-gray-400 hover:bg-gray-700 hover:text-white disabled:opacity-30 disabled:cursor-not-allowed transition-colors"
          >末页</button>

          <select
            v-model.number="logPerPage"
            @change="logPage = 1; loadLogs()"
            class="ml-2 text-xs bg-gray-800 border border-gray-700 text-gray-300 rounded-lg px-2 py-1 focus:outline-none focus:border-blue-500"
          >
            <option :value="20">20条/页</option>
            <option :value="50">50条/页</option>
            <option :value="100">100条/页</option>
            <option :value="200">200条/页</option>
          </select>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, watch, onMounted, onUnmounted } from 'vue'
import { openaiAPI } from '../api'

// ─── Time ranges ──────────────────────────────────────────────────
const timeRanges = [
  { label: '1h',  value: '1h' },
  { label: '6h',  value: '6h' },
  { label: '24h', value: '24h' },
  { label: '7d',  value: '7d' },
  { label: '30d', value: '30d' },
  { label: '全部', value: 'all' },
]
const timeRange = ref('24h')

const currentRangeLabel = computed(() => timeRanges.find(r => r.value === timeRange.value)?.label || '全部')

function rangeStart(range) {
  const now = Date.now()
  const map = { '1h': 3600, '6h': 21600, '24h': 86400, '7d': 604800, '30d': 2592000 }
  if (range === 'all' || !map[range]) return null
  return new Date(now - map[range] * 1000)
}

// ─── Channels ─────────────────────────────────────────────────────
const channels = ref([
  { id: 'openai',      icon: '🤖', label: 'OpenAI / Codex', count: null },
  { id: 'augment',     icon: '🔑', label: 'Augment',        count: null },
  { id: 'cursor',      icon: '💻', label: 'Cursor',         count: null },
  { id: 'windsurf',    icon: '🏄', label: 'Windsurf',       count: null },
  { id: 'antigravity', icon: '🚀', label: 'Antigravity',    count: null },
  { id: 'claude',      icon: '🧠', label: 'Claude',         count: null },
])
const activeChannel = ref('openai')

// ─── State ────────────────────────────────────────────────────────
const loading    = ref(false)
const lastUpdated = ref('')
const statusFilter = ref('')
const liveMode   = ref(false)
const liveInterval = ref(5)

const logs        = ref([])
const logPage     = ref(1)
const logPerPage  = ref(50)
const logTotal    = ref(0)
const logTotalPages = ref(1)
const poolAccounts = ref([])

// ─── Derived: logs filtered by time range ─────────────────────────
const rangedLogs = computed(() => {
  const start = rangeStart(timeRange.value)
  if (!start) return logs.value
  return logs.value.filter(l => new Date(l.created_at) >= start)
})

// ─── Stats (over ranged logs) ──────────────────────────────────────
const stats = computed(() => {
  const src = rangedLogs.value
  if (!src.length) return {
    total: 0, success: 0, failed: 0, successRate: 0,
    avgDuration: 0, maxDuration: 0, totalTokens: 0, inputTokens: 0, outputTokens: 0
  }
  const success = src.filter(l => l.status_code >= 200 && l.status_code < 300)
  const durs    = src.map(l => l.duration_ms || 0)
  const totalIn  = src.reduce((s, l) => s + (l.input_tokens  || 0), 0)
  const totalOut = src.reduce((s, l) => s + (l.output_tokens || 0), 0)
  return {
    total:       src.length,
    success:     success.length,
    failed:      src.length - success.length,
    successRate: Math.round(success.length / src.length * 100),
    avgDuration: Math.round(durs.reduce((a, b) => a + b, 0) / durs.length),
    maxDuration: Math.max(...durs),
    totalTokens: totalIn + totalOut,
    inputTokens: totalIn,
    outputTokens: totalOut,
  }
})

// ─── Chart (bucket by time range) ─────────────────────────────────
const chartData = computed(() => {
  const src = rangedLogs.value
  if (!src.length) return []

  // Choose bucket size based on range
  const bucketConfig = {
    '1h':  { count: 12, ms: 5 * 60 * 1000,      fmt: (d) => d.toLocaleTimeString('zh', { hour: '2-digit', minute: '2-digit' }) },
    '6h':  { count: 12, ms: 30 * 60 * 1000,     fmt: (d) => d.toLocaleTimeString('zh', { hour: '2-digit', minute: '2-digit' }) },
    '24h': { count: 24, ms: 60 * 60 * 1000,     fmt: (d) => d.getHours() + 'h' },
    '7d':  { count: 7,  ms: 24 * 60 * 60 * 1000, fmt: (d) => (d.getMonth()+1) + '/' + d.getDate() },
    '30d': { count: 30, ms: 24 * 60 * 60 * 1000, fmt: (d) => (d.getMonth()+1) + '/' + d.getDate() },
    'all': { count: 20, ms: 0,                   fmt: (d) => d.toLocaleString('zh', { month: '2-digit', day: '2-digit' }) },
  }

  const cfg = bucketConfig[timeRange.value] || bucketConfig['24h']
  let buckets = []

  if (timeRange.value === 'all' || cfg.ms === 0) {
    // Dynamic buckets for "all"
    if (src.length <= 1) return []
    const oldest = new Date(src[src.length - 1].created_at).getTime()
    const newest = new Date(src[0].created_at).getTime()
    const span = Math.max(newest - oldest, 1)
    const bucketMs = span / 20
    for (let i = 0; i < 20; i++) {
      const from = oldest + i * bucketMs
      const to   = from + bucketMs
      const items = src.filter(l => {
        const t = new Date(l.created_at).getTime()
        return t >= from && t < to
      })
      buckets.push({ from: new Date(from), items })
    }
  } else {
    const start = rangeStart(timeRange.value)
    const now   = Date.now()
    for (let i = 0; i < cfg.count; i++) {
      const from = new Date(start.getTime() + i * cfg.ms)
      const to   = new Date(from.getTime() + cfg.ms)
      const items = src.filter(l => {
        const t = new Date(l.created_at)
        return t >= from && t < to
      })
      buckets.push({ from, to, items })
    }
  }

  const maxTotal = Math.max(...buckets.map(b => b.items.length), 1)

  return buckets.map(b => {
    const success = b.items.filter(l => l.status_code >= 200 && l.status_code < 300).length
    const failed  = b.items.length - success
    const total   = b.items.length
    const durs    = b.items.map(l => l.duration_ms || 0)
    const avgMs   = durs.length ? Math.round(durs.reduce((a,c) => a+c, 0) / durs.length) : 0
    const totalPct   = Math.max(total / maxTotal * 100, total > 0 ? 6 : 0)
    const successPct = total > 0 ? (success / total) * totalPct : 0
    const failedPct  = total > 0 ? (failed  / total) * totalPct : 0
    const cfg2 = bucketConfig[timeRange.value] || bucketConfig['24h']
    return {
      label:      cfg2.fmt(b.from),
      success,
      failed,
      total,
      avgMs,
      successPct: Math.round(successPct),
      failedPct:  Math.round(failedPct),
    }
  })
})

// ─── Account distribution (ranged) ───────────────────────────────
const accountStats = computed(() => {
  const map = {}
  rangedLogs.value.forEach(l => {
    const k = l.account_email || l.account_id || '未知'
    map[k] = (map[k] || 0) + 1
  })
  const total = rangedLogs.value.length || 1
  return Object.entries(map)
    .map(([email, count]) => ({ email, count, pct: Math.round(count / total * 100) }))
    .sort((a, b) => b.count - a.count)
})

// ─── Model distribution (ranged) ─────────────────────────────────
const modelStats = computed(() => {
  const map = {}
  rangedLogs.value.forEach(l => {
    const k = l.model || ''
    map[k] = (map[k] || 0) + 1
  })
  return Object.entries(map)
    .map(([model, count]) => ({ model, count }))
    .sort((a, b) => b.count - a.count)
    .slice(0, 4)
})

// ─── Filtered logs (ranged + status) ─────────────────────────────
const filteredLogs = computed(() => {
  let src = rangedLogs.value
  if (statusFilter.value === '2xx') src = src.filter(l => l.status_code >= 200 && l.status_code < 300)
  else if (statusFilter.value === '4xx') src = src.filter(l => l.status_code >= 400 && l.status_code < 500)
  else if (statusFilter.value === '5xx') src = src.filter(l => l.status_code >= 500)
  return src
})

// ─── Pagination range (smart ellipsis) ─────────────────────────────
const paginationRange = computed(() => {
  const total = logTotalPages.value
  const current = logPage.value
  if (total <= 7) return Array.from({ length: total }, (_, i) => i + 1)
  const pages = []
  pages.push(1)
  if (current > 3) pages.push('...')
  for (let i = Math.max(2, current - 1); i <= Math.min(total - 1, current + 1); i++) {
    pages.push(i)
  }
  if (current < total - 2) pages.push('...')
  pages.push(total)
  return pages
})

// ─── Data fetching ────────────────────────────────────────────────
async function loadLogs() {
  if (activeChannel.value !== 'openai') { logs.value = []; return }
  try {
    const data = await openaiAPI.getCodexLogs({ page: logPage.value, per_page: logPerPage.value })
    if (Array.isArray(data)) {
      logs.value = data
    } else if (data && Array.isArray(data.logs)) {
      logs.value = data.logs
      logTotal.value = data.total || 0
      logTotalPages.value = data.total_pages || 1
    } else {
      logs.value = []
    }
  } catch {
    logs.value = []
  }
}

function goLogPage(p) {
  if (p < 1 || p > logTotalPages.value) return
  logPage.value = p
  loadLogs()
}

async function loadPoolStatus() {
  if (activeChannel.value !== 'openai') return
  try {
    const res  = await fetch('/pool/status')
    const data = await res.json()
    poolAccounts.value = data.accounts || []
    const ch = channels.value.find(c => c.id === 'openai')
    if (ch) ch.count = data.total_accounts
  } catch {
    poolAccounts.value = []
  }
}

async function refresh() {
  loading.value = true
  await Promise.all([loadLogs(), loadPoolStatus()])
  loading.value = false
  lastUpdated.value = new Date().toLocaleTimeString('zh')
}

function selectChannel(id) {
  activeChannel.value = id
  refresh()
}

async function clearLogs() {
  if (!confirm('确认清空所有日志？')) return
  try {
    await openaiAPI.clearCodexLogs()
    logs.value = []
    logTotal.value = 0
    logTotalPages.value = 1
    logPage.value = 1
  } catch {}
}

// ─── Live mode ────────────────────────────────────────────────────
let liveTimer = null

function startLive() {
  stopLive()
  liveTimer = setInterval(refresh, liveInterval.value * 1000)
}
function stopLive() {
  if (liveTimer) { clearInterval(liveTimer); liveTimer = null }
}
function toggleLive() {
  liveMode.value = !liveMode.value
  liveMode.value ? startLive() : stopLive()
}
function setLiveInterval(s) {
  liveInterval.value = s
  if (liveMode.value) startLive()
}

// ─── Helpers ──────────────────────────────────────────────────────
function formatTime(ts) {
  if (!ts) return '—'
  const d = new Date(ts)
  if (d.toDateString() === new Date().toDateString()) {
    return d.toLocaleTimeString('zh', { hour: '2-digit', minute: '2-digit', second: '2-digit' })
  }
  return d.toLocaleString('zh', { month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit' })
}

function formatTokens(n) {
  if (!n) return '0'
  if (n >= 1_000_000) return (n / 1_000_000).toFixed(1) + 'M'
  if (n >= 1_000)     return (n / 1_000).toFixed(1) + 'K'
  return String(n)
}

function getStatusClass(code) {
  if (code >= 200 && code < 300) return 'bg-green-900/50 text-green-400'
  if (code >= 400 && code < 500) return 'bg-yellow-900/50 text-yellow-400'
  if (code >= 500)               return 'bg-red-900/50 text-red-400'
  return 'bg-gray-800 text-gray-400'
}

function platformIcon(p) {
  const map = { 'macOS': '🍎', 'Windows': '🪟', 'Linux': '🐧', 'iOS': '📱', 'Android': '🤖', 'Codex CLI': '⌨️' }
  return map[p] || '💻'
}

function platformClass(p) {
  const map = {
    'macOS':     'bg-gray-700/50 text-gray-300',
    'Windows':   'bg-blue-900/40 text-blue-300',
    'Linux':     'bg-orange-900/40 text-orange-300',
    'iOS':       'bg-purple-900/40 text-purple-300',
    'Android':   'bg-green-900/40 text-green-300',
    'Codex CLI': 'bg-cyan-900/40 text-cyan-300',
  }
  return map[p] || 'bg-gray-800 text-gray-400'
}

// ─── Lifecycle ────────────────────────────────────────────────────
let passiveTimer = null
onMounted(() => {
  refresh()
  passiveTimer = setInterval(refresh, 30000)
})
onUnmounted(() => {
  if (passiveTimer) clearInterval(passiveTimer)
  stopLive()
})
</script>

<style scoped>
@keyframes fadeIn {
  from { opacity: 0; background-color: rgba(59,130,246,0.08); }
  to   { opacity: 1; background-color: transparent; }
}
.log-new {
  animation: fadeIn 1s ease;
}
</style>
