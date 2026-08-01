<script setup lang="ts">
import { AT_TEMPLATES } from '~/constants/atTemplates'
import { devicesService } from '~/services/devices'

const props = defineProps<{
  deviceId: string
  backendMode?: string
  atPort?: string
  running?: boolean
}>()

const toast = useToast()

type Entry = { cmd: string; response: string; ok: boolean; at: number }

const cmd = ref('')
const template = ref('')
const history = ref<Entry[]>([])
const sending = ref(false)
const timeout = ref(10_000)
const logEl = ref<HTMLElement | null>(null)

// USelectMenu 的分组格式：组标题作为 disabled 项插在组员前面
const templateItems = computed(() =>
  AT_TEMPLATES.flatMap(group => [
    { label: group.label, type: 'label' as const },
    ...group.items.map(item => ({ label: item.label, value: item.value }))
  ])
)

const hasATPort = computed(() => String(props.atPort || '').trim().length > 0)
const canUseATTerminal = computed(() => Boolean(props.running) && hasATPort.value)

const unavailableTitle = computed(() => {
  if (!props.running) return '当前设备未运行'
  if (!hasATPort.value) return '当前设备没有可用 AT 口'
  return 'AT 终端暂不可用'
})

const unavailableDescription = computed(() => {
  if (!props.running) {
    return '设备当前未启动，AT 终端暂时不可用。待设备运行后，如果存在可用的 AT 口，即可在这里直接发送 AT 指令。'
  }
  if (!hasATPort.value && props.backendMode === 'qmi') {
    return '设备当前处于纯 QMI 模式，但没有解析到可用的 AT 口，因此无法提供 AT 串口终端。'
  }
  if (!hasATPort.value) {
    return '设备当前没有可用的 AT 口，因此无法提供 AT 串口终端。'
  }
  return '当前设备暂时无法提供 AT 串口终端，请稍后重试。'
})

// 选中模板即填入命令框，之后仍可自由编辑
watch(template, (value) => {
  const next = String(value || '').trim()
  if (next) cmd.value = next
})

async function send() {
  const text = cmd.value.trim()
  if (!text) return
  sending.value = true
  cmd.value = ''
  const result = await devicesService.sendAT(props.deviceId, {
    cmd: text,
    timeout_ms: timeout.value || 10_000
  })
  sending.value = false
  if (result.ok === false) {
    const message = String(result.error?.message ?? '请求异常')
    toast.add({ title: 'AT 命令失败', description: message, color: 'error' })
    history.value.push({ cmd: text, response: message, ok: false, at: Date.now() })
  } else {
    history.value.push({
      cmd: text,
      response: result.data.response,
      ok: result.data.ok !== false,
      at: Date.now()
    })
  }
  nextTick(() => {
    if (logEl.value) logEl.value.scrollTop = logEl.value.scrollHeight
  })
}

function formatTime(ts: number) {
  return new Date(ts).toLocaleTimeString()
}
</script>

<template>
  <div class="flex flex-col gap-4">
    <div class="flex items-start justify-between gap-3">
      <div>
        <h2 class="text-sm font-semibold">
          AT 终端
        </h2>
        <p class="text-xs text-muted mt-0.5">
          发送 AT 指令并查看回显（多行响应会完整返回）
        </p>
      </div>
      <UButton
        v-if="canUseATTerminal && history.length"
        size="sm"
        color="neutral"
        variant="ghost"
        icon="i-lucide-trash-2"
        label="清空"
        @click="history = []"
      />
    </div>

    <template v-if="!canUseATTerminal">
      <div class="tile flex flex-col items-center justify-center gap-2 px-6 py-12 text-center">
        <UIcon name="i-lucide-triangle-alert" class="size-8 text-warning" />
        <p class="text-sm font-semibold">
          {{ unavailableTitle }}
        </p>
        <p class="text-xs text-muted max-w-md leading-relaxed">
          {{ unavailableDescription }}
        </p>
      </div>
    </template>

    <template v-else>
      <UAlert
        color="warning"
        variant="subtle"
        icon="i-lucide-triangle-alert"
        title="谨慎操作"
        description="AT 指令直接作用于模组，错误的写入类指令可能导致模组配置损坏或掉线。"
      />

      <!-- 会话记录 -->
      <div
        ref="logEl"
        class="tile relative h-80 overflow-auto p-4 flex flex-col gap-3"
      >
        <div
          v-if="!history.length && !sending"
          class="absolute inset-0 flex items-center justify-center text-xs text-dimmed"
        >
          暂无 AT 会话记录
        </div>

        <div v-for="(h, i) in history" :key="`${h.at}-${i}`" class="flex flex-col gap-1.5">
          <div class="flex justify-end">
            <div class="max-w-[80%] rounded-2xl rounded-tr-sm bg-primary px-3.5 py-2 text-inverted">
              <div class="font-mono text-xs break-all">
                {{ h.cmd }}
              </div>
              <div class="mt-0.5 text-right text-[10px] opacity-70">
                {{ formatTime(h.at) }}
              </div>
            </div>
          </div>

          <div class="flex justify-start">
            <div
              class="max-w-[80%] rounded-2xl rounded-tl-sm border px-3.5 py-2"
              :class="h.ok
                ? 'border-default bg-elevated/40'
                : 'border-error/30 bg-error/10 text-error'"
            >
              <pre class="font-mono text-xs whitespace-pre-wrap break-all">{{ h.response }}</pre>
              <div class="mt-0.5 text-[10px] text-dimmed">
                {{ formatTime(h.at) }}
              </div>
            </div>
          </div>
        </div>

        <div v-if="sending" class="flex justify-start">
          <div class="flex items-center gap-2 rounded-2xl rounded-tl-sm border border-default bg-elevated/40 px-3.5 py-2.5">
            <span class="flex gap-1">
              <span class="size-1.5 animate-bounce rounded-full bg-primary [animation-delay:-0.3s]" />
              <span class="size-1.5 animate-bounce rounded-full bg-primary [animation-delay:-0.15s]" />
              <span class="size-1.5 animate-bounce rounded-full bg-primary" />
            </span>
            <span class="text-xs text-dimmed">等待模组响应…</span>
          </div>
        </div>
      </div>

      <!-- 输入区 -->
      <div class="tile p-4 flex flex-col gap-3">
        <div class="flex flex-wrap items-end gap-2">
          <UFormField label="快捷指令模板" class="w-64">
            <USelectMenu
              v-model="template"
              :items="templateItems"
              value-key="value"
              placeholder="选择常用命令（可选）"
              class="w-full"
            />
          </UFormField>

          <UFormField label="命令" class="flex-1 min-w-48">
            <UInput
              v-model="cmd"
              class="w-full font-mono"
              placeholder="例如 AT+CSQ（可自由编辑）"
              :disabled="sending"
              @keydown.enter.prevent="send"
            />
          </UFormField>

          <UFormField label="超时 (ms)" class="w-32">
            <UInputNumber v-model="timeout" :min="1000" :max="120000" :step="1000" class="w-full" />
          </UFormField>

          <UButton
            icon="i-lucide-terminal"
            label="发送"
            :loading="sending"
            :disabled="!cmd.trim()"
            @click="send"
          />
        </div>
      </div>
    </template>
  </div>
</template>
