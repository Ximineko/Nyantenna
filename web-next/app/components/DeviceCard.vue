<script setup lang="ts">
import type { DashboardDevice } from '~/types/api'

const props = defineProps<{
  device: DashboardDevice
  /** 设备管理页的状态比「在线/离线」更细，可覆盖 */
  statusLabel?: string
  statusTone?: 'success' | 'error' | 'warning' | 'neutral' | 'primary'
  /** 设备后端模式，用于区分读卡器与模组 */
  backendMode?: string
}>()
const emit = defineEmits<{ 'open-device': [id: string] }>()

const tone = computed(() => props.statusTone ?? (props.device.healthy ? 'success' : 'error'))
const label = computed(() => props.statusLabel ?? (props.device.healthy ? '在线' : '离线'))
const toneText = computed(() => ({
  success: 'text-success',
  error: 'text-error',
  warning: 'text-warning',
  neutral: 'text-muted',
  primary: 'text-primary'
}[tone.value]))

/** 制式文本：有双工信息时拼成「FDD LTE」这类形式 */
const displayNetworkMode = computed(() => {
  const mode = String(props.device?.network_mode || '').trim()
  const duplex = String(props.device?.network_duplex || '').trim()
  if (!mode) return ''
  return duplex ? `${duplex} ${mode}` : mode
})

const networkIcon = computed(() => {
  if (props.device?.vowifi_active) return 'i-lucide-wifi'
  const m = displayNetworkMode.value.toUpperCase()
  if (!m) return 'i-lucide-signal'
  if (m.includes('5G') || m.includes('NR')) return 'i-lucide-radio-tower'
  if (m.includes('4G') || m.includes('LTE')) return 'i-lucide-signal-high'
  if (m.includes('3G') || m.includes('WCDMA') || m.includes('HSPA') || m.includes('UMTS')) return 'i-lucide-signal-medium'
  return 'i-lucide-signal'
})

const networkColor = computed(() => {
  if (props.device?.vowifi_active) return 'text-emerald-500'
  const m = displayNetworkMode.value.toUpperCase()
  if (!m) return 'text-dimmed'
  if (m.includes('5G') || m.includes('NR')) return 'text-violet-500'
  if (m.includes('4G') || m.includes('LTE')) return 'text-blue-500'
  if (m.includes('3G')) return 'text-orange-500'
  return 'text-dimmed'
})

/** 只取制式本身（去掉 FDD/TDD 前缀），窄屏时 LTE 隐藏以免挤压运营商名 */
const networkModeText = computed(() => {
  const parts = displayNetworkMode.value.trim().split(/\s+/).filter(Boolean)
  if (parts.length <= 1) return parts[0] || ''
  return parts[1] || ''
})
const hideModeOnNarrow = computed(() => networkModeText.value.toUpperCase() === 'LTE')

function hasSignal(dbm: number | null | undefined): dbm is number {
  return typeof dbm === 'number' && Number.isFinite(dbm) && dbm !== 0 && dbm !== -999
}
function signalColor(dbm: number | null | undefined) {
  if (!hasSignal(dbm)) return 'bg-muted'
  if (dbm > -70) return 'bg-green-500'
  if (dbm > -90) return 'bg-yellow-500'
  return 'bg-red-500'
}
function signalBars(dbm: number | null | undefined) {
  if (!hasSignal(dbm)) return 0
  if (dbm > -70) return 4
  if (dbm > -85) return 3
  if (dbm > -100) return 2
  return 1
}
</script>

<template>
  <button
    type="button"
    class="tile-interactive group relative block w-full overflow-hidden text-left
           focus:outline-none focus-visible:ring-2 focus-visible:ring-primary focus-visible:ring-offset-2
           focus-visible:ring-offset-(--ui-bg)"
    @click="emit('open-device', device.id)"
  >
    <div class="relative z-10 p-5">
      <!-- 标题区 -->
      <div class="flex items-start gap-3">
        <div
          class="flex size-10 shrink-0 items-center justify-center rounded-xl
                 bg-elevated text-primary shadow-inner"
        >
          <UIcon
            :name="String(backendMode || '').toLowerCase() === 'pcsc' ? 'i-lucide-scan-line' : 'i-lucide-credit-card'"
            class="size-5"
          />
        </div>
        <div class="min-w-0">
          <h3 class="font-semibold truncate">{{ device.name || device.id }}</h3>
          <div class="mt-0.5 flex flex-wrap items-center gap-1.5">
            <StatusDot :tone="tone" :pulse="device.healthy" />
            <span class="text-xs font-medium" :class="toneText">{{ label }}</span>
            <DeviceKindBadge :backend-mode="backendMode" />
          </div>
        </div>
      </div>

      <!-- 网络状态条 -->
      <div
        class="mt-5 flex items-center justify-between gap-2 rounded-xl border border-default
               bg-elevated/50 px-3 py-2.5"
      >
        <div class="flex min-w-0 items-center gap-2">
          <div class="flex shrink-0 items-center gap-1.5">
            <UIcon :name="networkIcon" :class="['size-4', networkColor]" />
            <span
              v-if="!device.vowifi_active && networkModeText"
              class="text-[11px] font-bold leading-none tracking-tighter"
              :class="hideModeOnNarrow ? 'hidden xl:inline' : ''"
            >{{ networkModeText }}</span>
          </div>
          <span class="min-w-0 flex-1 truncate text-sm font-medium">
            {{ device.vowifi_active ? 'Wi-Fi Calling' : (device.operator || '检测中...') }}
          </span>
        </div>

        <div v-if="!device.vowifi_active" class="flex shrink-0 items-center gap-1" title="信号强度">
          <div class="flex h-3 items-end gap-[2px]">
            <div
              v-for="i in 4"
              :key="i"
              class="w-1 rounded-sm transition-all duration-500"
              :class="signalBars(device.signal_dbm) >= i ? signalColor(device.signal_dbm) : 'bg-accented'"
              :style="{ height: `${i * 25}%` }"
            />
          </div>
          <span class="ml-1 hidden font-mono text-xs text-dimmed xl:inline">
            {{ device.signal_dbm }}dBm
          </span>
        </div>
      </div>

      <!-- 公网 IP -->
      <div class="mt-4 flex items-center justify-between text-sm">
        <span class="flex items-center gap-1.5 text-dimmed">
          <UIcon name="i-lucide-globe" class="size-4" />
          公网 IP
        </span>
        <span class="font-mono font-semibold text-primary">{{ device.public_ip || '—' }}</span>
      </div>
    </div>
  </button>
</template>
