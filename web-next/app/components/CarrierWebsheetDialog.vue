<script setup lang="ts">
import { api } from '~/stores/auth'
import type { CarrierWebsheetInfo } from '~/types/api'

const props = defineProps<{
  open: boolean
  websheet: CarrierWebsheetInfo | null
}>()

const emit = defineEmits<{ 'update:open': [value: boolean]; done: [] }>()

const loaded = ref(false)
let completing = false
let channel: BroadcastChannel | null = null

const iframeSrc = computed(() => props.websheet?.embedUrl || '')

/** 运营商页面回调里带 token，用它过滤掉过期会话的消息 */
const websheetToken = computed(() => {
  const raw = props.websheet?.embedUrl || ''
  if (!raw) return ''
  try {
    return new URL(raw, window.location.origin).searchParams.get('token') || ''
  } catch {
    return ''
  }
})

watch(() => props.websheet?.id, () => { loaded.value = false })

async function sendCallback(callback: unknown) {
  const id = props.websheet?.id
  if (!id || !callback || typeof callback !== 'object') return
  try {
    await api.post(`/websheets/${id}/callback`, callback)
  } catch (err) {
    console.error('[CarrierWebsheet] 回调转发失败:', err)
  }
}

async function complete() {
  if (completing) return
  completing = true
  try {
    const id = props.websheet?.id
    if (id) {
      try {
        await api.post(`/websheets/${id}/done`)
      } catch (err) {
        console.error('[CarrierWebsheet] 结束会话失败:', err)
      }
    }
    emit('done')
    emit('update:open', false)
  } finally {
    completing = false
  }
}

/** 只有账户状态变更这一类回调需要继续转发，其余视为流程终止 */
function isTerminal(callback: unknown) {
  if (!callback || typeof callback !== 'object') return true
  const r = callback as { event?: unknown; method?: unknown; resultCode?: unknown }
  const value = String(r.event ?? r.method ?? r.resultCode ?? '').toLowerCase()
  if (!value) return true
  return !value.includes('phoneservicesaccountstatuschanged')
}

function isCurrent(data: unknown): data is { type: string; token?: unknown; callback?: unknown } {
  if (!data || typeof data !== 'object') return false
  const r = data as { type?: unknown; token?: unknown }
  if (r.type !== 'nyantenna-websheet-callback') return false
  const incoming = typeof r.token === 'string' ? r.token : ''
  if (websheetToken.value && incoming && incoming !== websheetToken.value) return false
  return true
}

function handle(data: unknown) {
  if (!props.open || !isCurrent(data)) return
  if (isTerminal(data.callback)) void complete()
  else void sendCallback(data.callback)
}

function onMessage(e: MessageEvent) { handle(e.data) }
function onStorage(e: StorageEvent) {
  if (e.key !== 'nyantenna-websheet-complete' || !e.newValue) return
  try { handle(JSON.parse(e.newValue)) } catch { /* 忽略过期或损坏的完成通知 */ }
}

onMounted(() => {
  window.addEventListener('message', onMessage)
  window.addEventListener('storage', onStorage)
  try {
    channel = new BroadcastChannel('nyantenna-websheet')
    channel.onmessage = e => handle(e.data)
  } catch {
    channel = null
  }
})

onUnmounted(() => {
  window.removeEventListener('message', onMessage)
  window.removeEventListener('storage', onStorage)
  channel?.close()
  channel = null
})
</script>

<template>
  <UModal
    :open="open"
    :title="websheet?.title || '运营商设置'"
    :ui="{ content: 'max-w-3xl' }"
    @update:open="v => emit('update:open', v)"
  >
    <template #body>
      <div class="relative h-[70vh] min-h-96 overflow-hidden rounded-lg border border-default">
        <div
          v-if="!loaded"
          class="absolute inset-0 flex items-center justify-center bg-default"
        >
          <div class="flex flex-col items-center gap-3">
            <UIcon name="i-lucide-loader-circle" class="size-6 animate-spin text-primary" />
            <p class="text-sm text-muted">正在加载运营商页面…</p>
          </div>
        </div>
        <iframe
          v-if="iframeSrc"
          :src="iframeSrc"
          class="size-full"
          allow="clipboard-write"
          @load="loaded = true"
        />
      </div>
    </template>

    <template #footer>
      <div class="flex w-full items-center justify-between gap-3">
        <p class="text-xs text-muted">完成后页面会自动关闭</p>
        <UButton color="neutral" variant="outline" label="我已完成" @click="complete" />
      </div>
    </template>
  </UModal>
</template>
