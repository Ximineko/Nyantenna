<script setup lang="ts">
import { storeToRefs } from 'pinia'
import { useDevicesStore } from '~/stores/devices'
import { devicesService } from '~/services/devices'
import type { OperatorCandidate, OperatorSelection } from '~/types/api'

const props = defineProps<{ deviceId: string }>()
const store = useDevicesStore()
const toast = useToast()
const { operatorScans } = storeToRefs(store)

const selection = ref<OperatorSelection | null>(null)
const applying = ref('')

const scan = computed(() => operatorScans.value[props.deviceId] ?? null)
const scanning = computed(() => scan.value?.status === 'running')

async function loadSelection() {
  const result = await devicesService.getOperatorSelection(props.deviceId)
  if (result.ok) selection.value = result.data
}

function startScan() {
  store.clearOperatorScan(props.deviceId)
  void store.startOperatorScan(props.deviceId)
}

const statusTone: Record<OperatorCandidate['status'], 'success' | 'neutral' | 'error' | 'warning'> = {
  current: 'success',
  available: 'neutral',
  forbidden: 'error',
  unknown: 'warning'
}
const statusLabel: Record<OperatorCandidate['status'], string> = {
  current: '当前',
  available: '可用',
  forbidden: '禁止',
  unknown: '未知'
}

/** 手动选网；失败时回落到自动，避免设备卡在无服务状态 */
async function apply(c: OperatorCandidate) {
  applying.value = c.plmn
  const result = await devicesService.setOperatorSelection(props.deviceId, {
    mode: 'manual',
    plmn: c.plmn,
    includes_pcs_digit: c.includes_pcs_digit,
    rat: c.rats?.[0]
  })
  applying.value = ''
  if (result.ok === false) {
    toast.add({ title: '选网失败', description: String(result.error?.message ?? ''), color: 'error' })
    return
  }
  selection.value = result.data
  toast.add({ title: `已锁定 ${c.operator_name || c.plmn}`, color: 'success' })
}

async function useAuto() {
  applying.value = 'auto'
  const result = await devicesService.setOperatorSelection(props.deviceId, { mode: 'automatic' })
  applying.value = ''
  if (result.ok === false) {
    toast.add({ title: '切换失败', color: 'error' })
    return
  }
  selection.value = result.data
  toast.add({ title: '已切换为自动选网', color: 'success' })
}

onMounted(() => {
  void loadSelection()
  void store.resumeOperatorScan(props.deviceId)
})
</script>

<template>
  <div class="flex flex-col gap-4">
    <div class="flex flex-wrap items-start justify-between gap-3">
      <div>
        <h2 class="text-sm font-semibold">运营商选择</h2>
        <p class="text-xs text-muted mt-0.5">
          当前模式：{{ selection?.mode === 'manual' ? `手动锁定 ${selection?.plmn ?? ''}` : '自动选网' }}
        </p>
      </div>
      <div class="flex gap-2">
        <UButton
          v-if="selection?.mode === 'manual'"
          size="sm"
          color="neutral"
          variant="outline"
          icon="i-lucide-wand-2"
          label="恢复自动"
          :loading="applying === 'auto'"
          @click="useAuto"
        />
        <UButton
          size="sm"
          icon="i-lucide-radar"
          :label="scanning ? '扫描中' : '扫描网络'"
          :loading="scanning"
          @click="startScan"
        />
      </div>
    </div>

    <UAlert
      color="warning"
      variant="subtle"
      icon="i-lucide-clock"
      title="扫描期间会中断服务"
      description="搜网需要模组遍历频段，通常耗时 30 秒到 2 分钟，期间无法收发数据与短信。"
    />

    <div v-if="scan?.message || scan?.error" class="tile px-4 py-3">
      <p class="text-sm" :class="scan?.error ? 'text-error' : 'text-muted'">
        {{ scan?.error || scan?.message }}
      </p>
    </div>

    <div v-if="scanning && !scan?.candidates?.length" class="flex flex-col gap-2">
      <USkeleton v-for="i in 4" :key="i" class="h-14 w-full rounded-lg" />
    </div>

    <div v-else-if="!scan?.candidates?.length" class="tile">
      <EmptyState
        icon="i-lucide-radio-tower"
        title="还没有扫描结果"
        description="点击扫描网络获取周边可用运营商"
      />
    </div>

    <div v-else class="tile divide-y divide-default">
      <div
        v-for="c in scan.candidates"
        :key="c.plmn"
        class="flex flex-wrap items-center gap-3 px-4 py-3"
      >
        <StatusDot :tone="statusTone[c.status]" :pulse="c.status === 'current'" />
        <div class="min-w-0 flex-1">
          <div class="flex flex-wrap items-center gap-2">
            <span class="font-medium truncate">{{ c.operator_name || c.short_name || c.plmn }}</span>
            <UBadge
              :color="statusTone[c.status]"
              variant="subtle"
              size="sm"
              :label="statusLabel[c.status]"
            />
            <UBadge
              v-for="rat in c.rats ?? []"
              :key="rat"
              color="neutral"
              variant="outline"
              size="sm"
              :label="String(rat)"
            />
          </div>
          <p class="text-xs text-muted font-mono mt-0.5">
            {{ c.plmn }}<span v-if="c.mcc && c.mnc"> · {{ c.mcc }}/{{ c.mnc }}</span>
          </p>
        </div>
        <UButton
          size="xs"
          variant="soft"
          icon="i-lucide-lock"
          label="锁定"
          :disabled="c.status === 'forbidden' || c.status === 'current'"
          :loading="applying === c.plmn"
          @click="apply(c)"
        />
      </div>
    </div>
  </div>
</template>
