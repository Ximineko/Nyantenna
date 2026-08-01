<script setup lang="ts">
import { debugCollector } from '~/debug/collector'
import { copyToClipboard } from '~/utils/clipboard'

const open = defineModel<boolean>('open', { default: false })
const toast = useToast()

const currentHref = computed(() => (import.meta.client ? window.location.href : ''))

// 出错时自动弹面板，偏好存在本地
const autoOpen = ref(false)
onMounted(() => { autoOpen.value = localStorage.getItem('debug_panel_auto_open') === '1' })
watch(autoOpen, v => localStorage.setItem('debug_panel_auto_open', v ? '1' : '0'))

watch(debugCollector.openPanelRequestAt, (ts) => {
  if (ts && autoOpen.value) open.value = true
})

function fmtTs(ts: number) {
  return new Date(ts).toLocaleString()
}

async function copySnapshot() {
  const ok = await copyToClipboard(JSON.stringify(debugCollector.sanitizedSnapshot(), null, 2))
  toast.add(ok
    ? { title: '已复制诊断信息', color: 'success' }
    : { title: '浏览器限制，已弹出文本，请手动复制', color: 'warning' })
}

function downloadSnapshot() {
  try {
    const n = new Date()
    const p = (v: number) => String(v).padStart(2, '0')
    const stamp = `${n.getFullYear()}${p(n.getMonth() + 1)}${p(n.getDate())}-${p(n.getHours())}${p(n.getMinutes())}${p(n.getSeconds())}`
    const blob = new Blob([JSON.stringify(debugCollector.sanitizedSnapshot(), null, 2)], {
      type: 'application/json;charset=utf-8'
    })
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = `debug-${stamp}.json`
    document.body.appendChild(a)
    a.click()
    a.remove()
    URL.revokeObjectURL(url)
    toast.add({ title: '已下载诊断文件', color: 'success' })
  } catch {
    toast.add({ title: '导出失败', color: 'error' })
  }
}

function clearAll() {
  debugCollector.clearAll()
  toast.add({ title: '已清空', color: 'neutral' })
}

// 最新的排前面
const routes = computed(() => debugCollector.routes.value.slice().reverse())
const apiErrors = computed(() => debugCollector.apiErrors.value.slice().reverse())
const jsErrors = computed(() => debugCollector.jsErrors.value.slice().reverse())
const authEvents = computed(() => debugCollector.authEvents.value.slice().reverse())
</script>

<template>
  <USlideover v-model:open="open" title="诊断面板" :ui="{ content: 'max-w-xl' }">
    <template #body>
      <div class="flex flex-col gap-5">
        <p class="truncate font-mono text-xs text-dimmed">
          {{ currentHref }}
        </p>

        <div class="flex flex-wrap items-center justify-between gap-2">
          <USwitch v-model="autoOpen" label="错误自动弹出" />
          <div class="flex gap-2">
            <UButton size="xs" color="neutral" variant="outline" label="清空" @click="clearAll" />
            <UButton size="xs" color="neutral" variant="outline" label="导出" @click="downloadSnapshot" />
            <UButton size="xs" label="复制" @click="copySnapshot" />
          </div>
        </div>

        <section>
          <h3 class="mb-2 text-sm font-semibold">
            最近路由
          </h3>
          <p v-if="!routes.length" class="text-xs text-dimmed">
            暂无记录
          </p>
          <div v-else class="tile max-h-56 overflow-auto p-3 flex flex-col gap-2">
            <div v-for="r in routes" :key="r.ts" class="font-mono text-xs">
              <p class="text-dimmed">
                {{ fmtTs(r.ts) }}
              </p>
              <p class="break-words">
                {{ r.from || '-' }} → {{ r.to || '-' }}
                <span v-if="r.name" class="text-dimmed">({{ r.name }})</span>
              </p>
            </div>
          </div>
        </section>

        <section>
          <h3 class="mb-2 text-sm font-semibold">
            最近 API 错误
          </h3>
          <p v-if="!apiErrors.length" class="text-xs text-dimmed">
            暂无记录
          </p>
          <div v-else class="tile max-h-64 overflow-auto p-3 flex flex-col gap-2">
            <div v-for="a in apiErrors" :key="a.ts" class="font-mono text-xs">
              <p class="text-dimmed">
                {{ fmtTs(a.ts) }}
              </p>
              <p class="break-words">
                <span v-if="a.status">HTTP {{ a.status }} · </span>
                <span v-if="a.method">{{ String(a.method).toUpperCase() }} </span>
                <span v-if="a.url">{{ a.url }}</span>
              </p>
              <p class="break-words text-muted">
                {{ a.message }}
              </p>
            </div>
          </div>
        </section>

        <section>
          <h3 class="mb-2 text-sm font-semibold">
            最近前端错误
          </h3>
          <p v-if="!jsErrors.length" class="text-xs text-dimmed">
            暂无记录
          </p>
          <div v-else class="tile max-h-64 overflow-auto p-3 flex flex-col gap-2">
            <div v-for="j in jsErrors" :key="j.ts" class="font-mono text-xs">
              <p class="text-dimmed">
                {{ fmtTs(j.ts) }}<span v-if="j.source"> · {{ j.source }}</span>
              </p>
              <p class="break-words">
                {{ j.message }}
              </p>
              <pre v-if="j.stack" class="whitespace-pre-wrap break-words text-muted">{{ j.stack }}</pre>
            </div>
          </div>
        </section>

        <section>
          <h3 class="mb-2 text-sm font-semibold">
            鉴权事件
          </h3>
          <p v-if="!authEvents.length" class="text-xs text-dimmed">
            暂无记录
          </p>
          <div v-else class="tile max-h-40 overflow-auto p-3 flex flex-col gap-2">
            <div v-for="e in authEvents" :key="e.ts" class="font-mono text-xs">
              <p class="text-dimmed">
                {{ fmtTs(e.ts) }}
              </p>
              <p class="break-words">
                {{ e.kind }}
                <span v-if="e.redirectTo" class="text-dimmed">· redirect={{ e.redirectTo }}</span>
              </p>
            </div>
          </div>
        </section>
      </div>
    </template>
  </USlideover>
</template>
