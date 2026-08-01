<script setup lang="ts">
import { storeToRefs } from 'pinia'
import { useSMSStore } from '~/stores/sms'
import { usePollingScheduler } from '~/composables/usePollingScheduler'
import { estimateSegments } from '~/utils/smsSegments'
import type { SmsThreadVM } from '~/types/view-model'
import type { SMSMessage } from '~/types/api'

const sms = useSMSStore()
const toast = useToast()
const { devices, threads, threadMessages, loading, error } = storeToRefs(sms)

const search = ref('')
const deviceFilter = ref<string>('')
const active = ref<SmsThreadVM | null>(null)
const draft = ref('')
const sending = ref(false)
const messagesEl = ref<HTMLElement | null>(null)

const THREAD_PAGE = 80
const hasMore = ref(false)
const loadingMore = ref(false)

const deviceItems = computed(() => [
  { label: '全部设备', value: '' },
  ...devices.value.map(d => ({ label: d.name || d.id, value: d.id }))
])

const filtered = computed(() => {
  const q = search.value.trim().toLowerCase()
  if (!q) return threads.value
  return threads.value.filter(t => t.peerLower.includes(q) || t.lastMessageLower.includes(q))
})

function parseTs(s: number | string) {
  const ms = new Date(s).getTime()
  return Number.isFinite(ms) ? ms : 0
}

/** 会话列表用的相对格式：今天只给时刻，更早只给日期，一行放得下 */
function formatTime(ts: number | string) {
  const d = new Date(ts)
  if (Number.isNaN(d.getTime())) return String(ts ?? '')
  const now = new Date()
  const sameDay = d.toDateString() === now.toDateString()
  return sameDay
    ? d.toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit' })
    : d.toLocaleDateString('zh-CN', { month: '2-digit', day: '2-digit' })
}

/** 消息气泡用的时刻：恒为 HH:MM。
 *  哪一天由分组分隔条给出，气泡里再重复日期反而把时刻挤掉了。 */
function formatClock(ts: number | string) {
  const d = new Date(ts)
  if (Number.isNaN(d.getTime())) return String(ts ?? '')
  return d.toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit' })
}

function dateKey(ms: number) {
  const t = new Date(ms)
  if (!ms || !Number.isFinite(t.getTime())) return '未知日期'
  return `${t.getFullYear()}-${String(t.getMonth() + 1).padStart(2, '0')}-${String(t.getDate()).padStart(2, '0')}`
}

// type：1=收件，2=发出（与后端 db.go / notify 侧一致）
function isOutgoing(type: number) {
  return Number(type) === 2
}

/** 按自然日分组，日期分隔条插在组首 */
const groups = computed(() => {
  const out: Array<{ date: string, items: SMSMessage[] }> = []
  for (const m of threadMessages.value) {
    const key = dateKey(parseTs(m.timestamp))
    const tail = out[out.length - 1]
    if (!tail || tail.date !== key) out.push({ date: key, items: [m] })
    else tail.items.push(m)
  }
  return out
})

function scrollToBottom() {
  nextTick(() => {
    if (messagesEl.value) messagesEl.value.scrollTop = messagesEl.value.scrollHeight
  })
}

async function openThread(t: SmsThreadVM) {
  active.value = t
  const result = await sms.fetchThread({
    peer: t.peer, limit: THREAD_PAGE, device_id: t.deviceId, imsi: t.imsi
  })
  hasMore.value = result.ok && result.data.length >= THREAD_PAGE
  scrollToBottom()
}

/** 向上翻更早的消息，翻完后把视口锁回原来的位置 */
async function loadMore() {
  const t = active.value
  const oldest = threadMessages.value[0]
  if (!t || !oldest || loadingMore.value) return
  loadingMore.value = true
  const el = messagesEl.value
  const prevHeight = el?.scrollHeight ?? 0
  const result = await sms.loadMoreThread({
    peer: t.peer, limit: THREAD_PAGE, device_id: t.deviceId, imsi: t.imsi,
    before_ts: oldest.timestamp, before_id: oldest.id
  })
  loadingMore.value = false
  hasMore.value = result.ok && result.added >= THREAD_PAGE
  nextTick(() => {
    if (el) el.scrollTop = el.scrollHeight - prevHeight
  })
}

async function refreshThreads() {
  await sms.fetchThreads(deviceFilter.value || undefined)
  if (active.value) {
    const still = threads.value.find(t => t.key === active.value?.key)
    if (still) {
      await sms.fetchThread({
        peer: still.peer,
        limit: Math.max(THREAD_PAGE, threadMessages.value.length),
        device_id: still.deviceId,
        imsi: still.imsi
      })
    }
  }
}

const draftEstimate = computed(() => estimateSegments(draft.value))

async function sendMessage() {
  const text = draft.value.trim()
  if (!text || !active.value) return
  sending.value = true
  const result = await sms.send({
    device_id: active.value.deviceId,
    imsi: active.value.imsi,
    phone: active.value.peer,
    message: text
  })
  sending.value = false
  if (result?.ok === false) {
    toast.add({ title: '发送失败', description: String(result?.error?.message ?? ''), color: 'error' })
    return
  }
  draft.value = ''
  await openThread(active.value)
}

/* 新建短信：可以发给任意号码，不限于已有会话 */
const composeOpen = ref(false)
const composeSending = ref(false)
const compose = ref({ device_id: '', phone: '', message: '' })
const composeEstimate = computed(() => estimateSegments(compose.value.message))
const composeDeviceItems = computed(() => devices.value.map(d => ({ label: d.name || d.id, value: d.id })))

function openCompose() {
  compose.value = {
    device_id: deviceFilter.value || devices.value[0]?.id || '',
    phone: '',
    message: ''
  }
  composeOpen.value = true
}

async function submitCompose() {
  const { device_id, phone, message } = compose.value
  if (!device_id || !phone.trim() || !message.trim()) {
    toast.add({ title: '设备、号码与内容都不能为空', color: 'warning' })
    return
  }
  composeSending.value = true
  const result = await sms.send({ device_id, phone: phone.trim(), message: message.trim() })
  composeSending.value = false
  if (result?.ok === false) {
    toast.add({ title: '发送失败', description: String(result?.error?.message ?? ''), color: 'error' })
    return
  }
  toast.add({ title: '发送成功', color: 'success' })
  composeOpen.value = false
  await refreshThreads()
}

/* 删除：整段会话 or 单条 */
const deleteThreadTarget = ref<SmsThreadVM | null>(null)
const deleteMessageTarget = ref<SMSMessage | null>(null)

async function confirmDeleteThread() {
  const t = deleteThreadTarget.value
  if (!t) return
  deleteThreadTarget.value = null
  const result = await sms.deleteThread({ device_id: t.deviceId, imsi: t.imsi, peer: t.peer })
  if (result?.ok === false) {
    toast.add({ title: '删除失败', description: String(result?.error?.message ?? ''), color: 'error' })
    return
  }
  if (active.value?.key === t.key) active.value = null
  await refreshThreads()
  toast.add({ title: '已删除对话', color: 'neutral' })
}

async function confirmDeleteMessage() {
  const m = deleteMessageTarget.value
  if (!m) return
  deleteMessageTarget.value = null
  const result = await sms.deleteMessage(m.id)
  if (result?.ok === false) {
    toast.add({ title: '删除失败', description: String(result?.error?.message ?? ''), color: 'error' })
    return
  }
  toast.add({ title: '已删除短信', color: 'neutral' })
  // 会话被删空时后端会告知，此时退回列表
  if (result?.ok && (result.data as { thread_empty?: boolean })?.thread_empty) {
    active.value = null
    await refreshThreads()
    return
  }
  if (active.value) await openThread(active.value)
}

watch(deviceFilter, () => { void refreshThreads() })

usePollingScheduler(refreshThreads, 8000, { immediate: true, backgroundIntervalMs: 30000 })
onMounted(() => { void sms.fetchDevices() })
</script>

<template>
  <div class="flex flex-1 min-h-0 flex-col">
    <PageHeader title="短信" description="按号码聚合的会话视图">
      <template #actions>
        <UButton
          icon="i-lucide-refresh-cw"
          color="neutral"
          variant="outline"
          :loading="loading"
          aria-label="刷新"
          @click="refreshThreads"
        />
        <UButton icon="i-lucide-send" label="新建短信" @click="openCompose" />
      </template>
    </PageHeader>

    <UAlert
      v-if="error"
      color="error"
      variant="subtle"
      icon="i-lucide-triangle-alert"
      title="短信加载失败"
      :description="String(error.message || error)"
    />

    <div class="grid gap-4 lg:grid-cols-[1fr_19rem] flex-1 min-h-0">
      <!-- 会话详情 -->
      <div class="tile flex flex-col min-h-0 overflow-hidden">
        <div v-if="active" class="border-b border-default px-4 py-3">
          <div class="flex items-center justify-between gap-4">
            <div class="min-w-0">
              <p class="font-semibold truncate">
                {{ active.peer }}
              </p>
              <p class="text-xs text-muted truncate mt-0.5">
                {{ active.lastDeviceName || active.deviceId || active.imsi }}
              </p>
            </div>
            <UButton
              icon="i-lucide-trash-2"
              color="error"
              variant="ghost"
              size="sm"
              aria-label="删除对话"
              @click="deleteThreadTarget = active"
            />
          </div>
        </div>

        <div v-if="!active" class="flex flex-1 items-center justify-center">
          <EmptyState icon="i-lucide-message-square" title="选择右侧会话开始查看" />
        </div>

        <template v-else>
          <div ref="messagesEl" class="flex-1 min-h-0 overflow-auto p-4 flex flex-col gap-4">
            <div v-if="hasMore" class="flex justify-center">
              <UButton
                size="xs"
                color="neutral"
                variant="ghost"
                label="加载更多"
                :loading="loadingMore"
                @click="loadMore"
              />
            </div>

            <div v-for="g in groups" :key="g.date" class="flex flex-col gap-3">
              <div class="flex justify-center">
                <span class="rounded-full bg-elevated px-3 py-0.5 text-[11px] text-dimmed">
                  {{ g.date }}
                </span>
              </div>

              <div
                v-for="m in g.items"
                :key="m.id"
                class="group/msg flex items-end gap-1.5"
                :class="isOutgoing(m.type) ? 'justify-end' : 'justify-start'"
              >
                <UButton
                  v-if="isOutgoing(m.type)"
                  size="xs"
                  icon="i-lucide-trash-2"
                  color="neutral"
                  variant="ghost"
                  class="opacity-0 transition group-hover/msg:opacity-100"
                  aria-label="删除短信"
                  @click="deleteMessageTarget = m"
                />
                <div
                  class="max-w-[75%] rounded-lg px-3 py-2 text-sm break-words"
                  :class="isOutgoing(m.type) ? 'bg-primary text-inverted' : 'bg-elevated text-default'"
                >
                  <p class="whitespace-pre-wrap">
                    {{ m.content }}
                  </p>
                  <p
                    class="text-[10px] mt-1 opacity-70 tabular-nums"
                    :class="isOutgoing(m.type) ? 'text-right' : ''"
                  >
                    {{ formatClock(m.timestamp) }}
                    <span v-if="isOutgoing(m.type) && m.status === 2"> · 已送达</span>
                    <span v-else-if="isOutgoing(m.type) && m.status === 3"> · 失败</span>
                  </p>
                </div>
                <UButton
                  v-if="!isOutgoing(m.type)"
                  size="xs"
                  icon="i-lucide-trash-2"
                  color="neutral"
                  variant="ghost"
                  class="opacity-0 transition group-hover/msg:opacity-100"
                  aria-label="删除短信"
                  @click="deleteMessageTarget = m"
                />
              </div>
            </div>

            <div
              v-if="!threadMessages.length"
              class="flex-1 flex items-center justify-center text-sm text-muted"
            >
              这个会话还没有消息
            </div>
          </div>

          <div class="border-t border-default p-3">
            <div class="flex items-end gap-2">
              <UTextarea
                v-model="draft"
                placeholder="输入短信内容，Enter 发送，Shift+Enter 换行"
                :rows="1"
                autoresize
                :maxrows="5"
                class="flex-1"
                @keydown.enter.exact.prevent="sendMessage"
              />
              <UButton
                icon="i-lucide-send"
                :loading="sending"
                :disabled="!draft.trim()"
                @click="sendMessage"
              />
            </div>
            <p v-if="draft" class="mt-1.5 text-right text-[11px] text-dimmed tabular-nums">
              {{ draftEstimate.encoding }} · 预计 {{ draftEstimate.parts }} 段 · {{ draftEstimate.units }} {{ draftEstimate.unitName }}
            </p>
          </div>
        </template>
      </div>

      <!-- 会话列表 -->
      <div class="tile flex flex-col min-h-0 overflow-hidden">
        <div class="border-b border-default p-3">
          <div class="flex flex-col gap-2">
            <UInput
              v-model="search"
              icon="i-lucide-search"
              placeholder="搜索号码或内容"
              size="sm"
            />
            <USelect
              v-model="deviceFilter"
              :items="deviceItems"
              value-key="value"
              size="sm"
            />
          </div>
        </div>

        <div class="flex-1 min-h-0 overflow-auto">
          <div v-if="loading && !filtered.length" class="flex flex-col gap-2 p-3">
            <USkeleton v-for="i in 6" :key="i" class="h-14 w-full" />
          </div>

          <EmptyState
            v-else-if="!filtered.length"
            icon="i-lucide-message-square-off"
            title="暂无会话"
            description="等待设备收到短信，或点击「新建短信」"
          />

          <button
            v-for="t in filtered"
            v-else
            :key="t.key"
            type="button"
            class="w-full text-left px-4 py-3 border-b border-default/60 last:border-0 transition hover:bg-elevated/60"
            :class="active?.key === t.key ? 'bg-elevated border-l-2 border-l-primary' : 'border-l-2 border-l-transparent'"
            @click="openThread(t)"
          >
            <div class="flex items-baseline justify-between gap-2">
              <span class="font-medium truncate">{{ t.peer }}</span>
              <span class="text-xs text-dimmed shrink-0">{{ formatTime(t.lastTs) }}</span>
            </div>
            <p class="text-sm text-muted truncate mt-1">
              {{ t.lastMessage }}
            </p>
            <p v-if="t.lastDeviceName" class="text-xs text-dimmed truncate mt-0.5">
              {{ t.lastDeviceName }}
            </p>
          </button>
        </div>
      </div>
    </div>

    <!-- 新建短信 -->
    <UModal v-model:open="composeOpen" title="发送短信">
      <template #body>
        <div class="flex flex-col gap-4">
          <UFormField label="发送设备" required>
            <USelect
              v-model="compose.device_id"
              :items="composeDeviceItems"
              value-key="value"
              placeholder="选择设备"
              class="w-full"
            />
          </UFormField>
          <UFormField label="目标号码" required>
            <UInput v-model="compose.phone" placeholder="+86138…" class="w-full font-mono" />
          </UFormField>
          <UFormField label="短信内容" required>
            <UTextarea v-model="compose.message" :rows="4" autoresize :maxrows="10" class="w-full" />
          </UFormField>
          <p class="text-right text-[11px] text-dimmed tabular-nums">
            {{ composeEstimate.encoding }} · 预计 {{ composeEstimate.parts }} 段 · {{ composeEstimate.units }} {{ composeEstimate.unitName }}
          </p>
        </div>
      </template>
      <template #footer>
        <div class="flex w-full justify-end gap-2">
          <UButton color="neutral" variant="ghost" label="取消" @click="composeOpen = false" />
          <UButton icon="i-lucide-send" label="发送" :loading="composeSending" @click="submitCompose" />
        </div>
      </template>
    </UModal>

    <!-- 删除对话 -->
    <UModal
      :open="!!deleteThreadTarget"
      title="删除对话"
      @update:open="v => { if (!v) deleteThreadTarget = null }"
    >
      <template #body>
        <p class="text-sm">
          将删除与 <span class="font-mono font-medium">{{ deleteThreadTarget?.peer }}</span> 的全部短信历史，无法恢复。
        </p>
        <p class="mt-2 text-sm text-muted">
          仅删除短信中心的历史记录，不影响运营商侧。
        </p>
      </template>
      <template #footer>
        <div class="flex w-full justify-end gap-2">
          <UButton color="neutral" variant="ghost" label="取消" @click="deleteThreadTarget = null" />
          <UButton color="error" label="确认删除" @click="confirmDeleteThread" />
        </div>
      </template>
    </UModal>

    <!-- 删除单条 -->
    <UModal
      :open="!!deleteMessageTarget"
      title="删除短信"
      @update:open="v => { if (!v) deleteMessageTarget = null }"
    >
      <template #body>
        <p class="text-sm">
          删除这条短信？删除后无法恢复。
        </p>
        <p class="mt-2 rounded-lg bg-elevated px-3 py-2 text-sm text-muted">
          {{ deleteMessageTarget?.content }}
        </p>
      </template>
      <template #footer>
        <div class="flex w-full justify-end gap-2">
          <UButton color="neutral" variant="ghost" label="取消" @click="deleteMessageTarget = null" />
          <UButton color="error" label="确认删除" @click="confirmDeleteMessage" />
        </div>
      </template>
    </UModal>
  </div>
</template>
