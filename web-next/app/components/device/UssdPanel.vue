<script setup lang="ts">
import { devicesService } from '~/services/devices'

const props = defineProps<{ deviceId: string }>()
const toast = useToast()

type Turn = { role: 'user' | 'network'; text: string; raw?: string; at: number }

const command = ref('*100#')
const input = ref('')
const turns = ref<Turn[]>([])
const sessionId = ref('')
const status = ref('')
const sending = ref(false)

// USSD 往返可能很慢（网络侧要走信令），给足超时
const timeoutMs = ref(45_000)

const active = computed(() => !!sessionId.value && status.value === 'action-required')

function push(role: Turn['role'], text: string, raw?: string) {
  turns.value.push({ role, text, raw, at: Date.now() })
}

async function start() {
  const cmd = command.value.trim()
  if (!cmd) return
  sending.value = true
  turns.value = []
  push('user', cmd)
  const result = await devicesService.sendUSSD(props.deviceId, { command: cmd, timeout_ms: timeoutMs.value }, timeoutMs.value)
  sending.value = false
  if (result.ok === false) {
    toast.add({ title: 'USSD 发送失败', description: String(result.error?.message ?? ''), color: 'error' })
    return
  }
  sessionId.value = result.data.sessionId
  status.value = String(result.data.status ?? '')
  push('network', result.data.text || '(空响应)', result.data.rawText)
}

async function reply() {
  const text = input.value.trim()
  if (!text || !sessionId.value) return
  sending.value = true
  push('user', text)
  input.value = ''
  const result = await devicesService.continueUSSD(
    props.deviceId,
    { session_id: sessionId.value, input: text, timeout_ms: timeoutMs.value },
    timeoutMs.value
  )
  sending.value = false
  if (result.ok === false) {
    toast.add({ title: '回复失败', description: String(result.error?.message ?? ''), color: 'error' })
    return
  }
  status.value = String(result.data.status ?? '')
  sessionId.value = result.data.sessionId || sessionId.value
  push('network', result.data.text || '(空响应)', result.data.rawText)
}

async function cancel() {
  if (!sessionId.value) return
  await devicesService.cancelUSSD(props.deviceId, sessionId.value)
  sessionId.value = ''
  status.value = ''
  toast.add({ title: '会话已取消', color: 'neutral' })
}

const quick = ['*100#', '*101#', '*111#', '*#06#']
</script>

<template>
  <div class="flex flex-col gap-4">
    <div>
      <h2 class="text-sm font-semibold">USSD 交互</h2>
      <p class="text-xs text-muted mt-0.5">向运营商发送 USSD 指令并进行多轮交互</p>
    </div>

    <div class="tile p-4 flex flex-col gap-3">
      <div class="flex flex-wrap items-end gap-2">
        <UFormField label="指令" class="flex-1 min-w-48">
          <UInput
            v-model="command"
            placeholder="*100#"
            class="w-full font-mono"
            :disabled="active"
            @keydown.enter.prevent="start"
          />
        </UFormField>
        <UFormField label="超时 (ms)" class="w-32">
          <UInputNumber v-model="timeoutMs" :min="5000" :max="180000" :step="5000" class="w-full" />
        </UFormField>
        <UButton
          icon="i-lucide-send"
          label="发送"
          :loading="sending"
          :disabled="active || !command.trim()"
          @click="start"
        />
        <UButton
          v-if="sessionId"
          color="neutral"
          variant="outline"
          icon="i-lucide-x"
          label="结束会话"
          @click="cancel"
        />
      </div>

      <div class="flex flex-wrap gap-1.5">
        <UButton
          v-for="q in quick"
          :key="q"
          size="xs"
          color="neutral"
          variant="soft"
          class="font-mono"
          :label="q"
          :disabled="active"
          @click="command = q"
        />
      </div>
    </div>

    <!-- 会话记录 -->
    <div v-if="turns.length" class="tile flex flex-col divide-y divide-default">
      <div
        v-for="(t, i) in turns"
        :key="i"
        class="flex gap-3 px-4 py-3"
      >
        <UIcon
          :name="t.role === 'user' ? 'i-lucide-arrow-up-right' : 'i-lucide-arrow-down-left'"
          :class="['size-4 shrink-0 mt-0.5', t.role === 'user' ? 'text-primary' : 'text-success']"
        />
        <div class="min-w-0 flex-1">
          <p class="text-sm whitespace-pre-wrap break-words">{{ t.text }}</p>
          <details v-if="t.raw && t.raw !== t.text" class="mt-1.5">
            <summary class="text-xs text-dimmed cursor-pointer select-none">原始响应</summary>
            <pre class="mt-1 text-xs font-mono text-muted whitespace-pre-wrap break-all">{{ t.raw }}</pre>
          </details>
        </div>
      </div>
    </div>

    <!-- 需要继续输入 -->
    <div v-if="active" class="tile p-4">
      <UFormField label="回复内容" help="网络要求继续输入以完成本次会话">
        <div class="flex gap-2">
          <UInput
            v-model="input"
            class="flex-1"
            autofocus
            placeholder="输入选项或内容"
            @keydown.enter.prevent="reply"
          />
          <UButton icon="i-lucide-corner-down-left" label="回复" :loading="sending" @click="reply" />
        </div>
      </UFormField>
    </div>
  </div>
</template>
