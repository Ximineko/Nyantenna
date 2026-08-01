<script setup lang="ts">
import { storeToRefs } from 'pinia'
import { useLogsStore } from '~/stores/logs'
import { useEventStream } from '~/composables/useEventStream'
import type { LogEntry } from '~/services/logs'

const logsStore = useLogsStore()
const { logs } = storeToRefs(logsStore)
const toast = useToast()

const connected = ref(false)
const paused = ref(false)
const autoScroll = ref(true)
const levelFilter = ref<'all' | 'debug' | 'info' | 'warn' | 'error'>('all')
const searchQuery = ref('')
const maxLogs = 1000

const container = ref<HTMLElement | null>(null)

const levels = [
  { label: '全部', value: 'all' as const },
  { label: 'DEBUG', value: 'debug' as const },
  { label: 'INFO', value: 'info' as const },
  { label: 'WARN', value: 'warn' as const },
  { label: 'ERROR', value: 'error' as const }
]

const filtered = computed(() => {
  let result = logs.value
  if (levelFilter.value !== 'all') {
    result = result.filter(l => l.level?.toLowerCase() === levelFilter.value)
  }
  const q = searchQuery.value.trim().toLowerCase()
  if (q) {
    result = result.filter(l =>
      l.message?.toLowerCase().includes(q) ||
      l.caller?.toLowerCase().includes(q) ||
      (l.fields && String(l.fields).toLowerCase().includes(q))
    )
  }
  return result
})

function scrollToBottom() {
  nextTick(() => {
    if (container.value) container.value.scrollTop = container.value.scrollHeight
  })
}

const stream = useEventStream<LogEntry>({
  path: '/logs/stream',
  eventName: 'log',
  query: { level: '' },
  parse: payload => JSON.parse(payload) as LogEntry,
  onConnected: () => { connected.value = true },
  onEvent: (entry) => {
    if (paused.value) return
    logsStore.append(entry, maxLogs)
    if (autoScroll.value) scrollToBottom()
  },
  onError: () => { connected.value = false }
})

function levelColor(level: string) {
  switch (String(level || '').toLowerCase()) {
    case 'error': return 'error'
    case 'warn': return 'warning'
    case 'debug': return 'neutral'
    default: return 'info'
  }
}

const loadingHistory = ref(false)

/** 重新拉一段历史（流断开后补齐，或想看更早的内容） */
async function loadHistory() {
  loadingHistory.value = true
  await logsStore.fetchHistory(500)
  loadingHistory.value = false
  scrollToBottom()
}

function clearLogs() {
  logsStore.clear()
  toast.add({ title: '已清空当前视图', color: 'neutral' })
}

function download() {
  const text = filtered.value
    .map(l => `[${l.time}] ${l.level} ${l.caller} ${l.message} ${l.fields ?? ''}`.trim())
    .join('\n')
  const blob = new Blob([text], { type: 'text/plain;charset=utf-8' })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = `nyantenna-logs-${new Date().toISOString().replace(/[:.]/g, '-')}.log`
  a.click()
  URL.revokeObjectURL(url)
}

onMounted(async () => {
  await logsStore.fetchHistory(500)
  scrollToBottom()
  stream.connect()
})

onUnmounted(() => {
  stream.disconnect()
})
</script>

<template>
  <div class="flex flex-1 min-h-0 flex-col">
    <PageHeader title="日志" description="实时日志流" />
    <div class="tile flex flex-col flex-1 min-h-0 overflow-hidden">
      <div class="border-b border-default px-4 py-3">
        <div class="flex flex-wrap items-center gap-3">
          <div class="flex items-center gap-2 mr-auto">
            <span
              class="size-2 rounded-full"
              :class="connected ? 'bg-success' : 'bg-error'"
            />
            <span class="text-sm text-muted">
              {{ connected ? '实时连接中' : '连接断开' }}
            </span>
            <UBadge variant="subtle" color="neutral" :label="`${filtered.length} 条`" />
          </div>

          <UInput
            v-model="searchQuery"
            icon="i-lucide-search"
            placeholder="搜索消息 / 调用位置 / 字段"
            size="sm"
            class="w-full sm:w-64"
          />

          <USelect
            v-model="levelFilter"
            :items="levels"
            value-key="value"
            size="sm"
            class="w-28"
          />

          <UFieldGroup size="sm">
            <UButton
              :icon="paused ? 'i-lucide-play' : 'i-lucide-pause'"
              :color="paused ? 'warning' : 'neutral'"
              variant="outline"
              :label="paused ? '继续' : '暂停'"
              @click="paused = !paused"
            />
            <UButton
              icon="i-lucide-arrow-down-to-line"
              :color="autoScroll ? 'primary' : 'neutral'"
              variant="outline"
              label="跟随"
              @click="autoScroll = !autoScroll"
            />
            <UButton
              icon="i-lucide-history"
              color="neutral"
              variant="outline"
              label="历史"
              :loading="loadingHistory"
              @click="loadHistory"
            />
            <UButton
              icon="i-lucide-download"
              color="neutral"
              variant="outline"
              label="导出"
              @click="download"
            />
            <UButton
              icon="i-lucide-trash-2"
              color="neutral"
              variant="outline"
              label="清空"
              @click="clearLogs"
            />
          </UFieldGroup>
        </div>
      </div>

      <div
        ref="container"
        class="flex-1 min-h-0 overflow-auto font-mono text-xs leading-relaxed p-4"
      >
        <EmptyState v-if="!filtered.length" icon="i-lucide-scroll-text" title="暂无日志" />

        <div
          v-for="(log, i) in filtered"
          v-else
          :key="i"
          class="flex gap-3 py-1 border-b border-default/40 last:border-0 hover:bg-elevated/50"
        >
          <span class="text-dimmed shrink-0 tabular-nums">{{ log.time }}</span>
          <UBadge
            :color="levelColor(log.level)"
            variant="subtle"
            size="sm"
            class="shrink-0 w-14 justify-center"
            :label="String(log.level || '').toUpperCase()"
          />
          <span class="text-muted shrink-0 hidden lg:inline max-w-56 truncate">{{ log.caller }}</span>
          <span class="min-w-0 break-all">
            {{ log.message }}
            <span v-if="log.fields" class="text-dimmed">{{ log.fields }}</span>
          </span>
        </div>
      </div>
    </div>
  </div>
</template>
