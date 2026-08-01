<script setup lang="ts">
import { activeEsimProfileDisplayName } from '~/utils/deviceOverviewActiveEsim'
import type { DeviceDetailVM } from '~/types/view-model'

const props = defineProps<{ device: DeviceDetailVM }>()
const emit = defineEmits<{ 'open-operator': []; 'setup-e911': [] }>()

const showSensitive = ref(false)

const m = computed(() => props.device?.modem)

const { simOperatorDisplay } = useOperatorDisplay()

/** SIM 归属运营商（带国旗与 PLMN），与「运营商」的当前驻网不同 */
const simOperator = computed(() => simOperatorDisplay(m.value as never))

const activeEsimProfileName = computed(() => activeEsimProfileDisplayName(props.device as never))

// operating_mode 0/4 是关射频/仅 FTM，VoWiFi 开启时也一定处于飞行
const flightModeEnabled = computed(() => {
  if (props.device?.vowifi_active) return true
  const mode = m.value?.operating_mode
  return mode === 0 || mode === 4
})

const backendModeText = computed(() => {
  const b = props.device?.backend_mode
  if (b === 'qmi') return 'QMI'
  if (b === 'mbim') return 'MBIM'
  if (b === 'at') return 'AT'
  return 'Auto'
})

/** VoWiFi 三态：全部就绪 / 部分就绪 / 未连接 */
/** 五项就绪度：SIM / 接入 / 隧道 / IMS / 短信 */
const readiness = computed(() => {
  const rt = props.device?.vowifi_runtime
  return [
    { key: 'SIM', ready: rt?.sim_ready },
    { key: 'Access', ready: rt?.access_ready },
    { key: 'Tunnel', ready: rt?.tunnel_ready },
    { key: 'IMS', ready: rt?.ims_ready },
    { key: 'SMS', ready: rt?.sms_ready }
  ]
})

const vowifiStatus = computed<'ok' | 'partial' | 'off'>(() => {
  const all = readiness.value.map(i => i.ready)
  if (all.every(Boolean)) return 'ok'
  if (all.some(Boolean)) return 'partial'
  return 'off'
})

const notReady = computed(() => readiness.value.filter(i => !i.ready).map(i => i.key))

/** 有错误时详情自动展开 */
const hasError = computed(() =>
  !!(props.device?.vowifi_runtime?.last_error || props.device?.vowifi_runtime?.last_error_class))
const showVowifiDetail = ref(false)

/** 蜂窝模式下的注册态 */
const isRegistered = computed(() => props.device?.registration_state_label === 'registered')
const cellularText = computed(() => {
  const label = props.device?.registration_state_label
  if (label === 'searching') return '正在搜网'
  if (label === 'denied') return '注册被拒绝'
  if (!props.device?.control_online) return '模组离线'
  return '未注册'
})

const signalDbm = computed(() => m.value?.signal_dbm)

function hasSignal(dbm: number | null | undefined): dbm is number {
  return typeof dbm === 'number' && Number.isFinite(dbm) && dbm !== 0 && dbm !== -999
}
const signalColorClass = computed(() => {
  const v = signalDbm.value
  if (!hasSignal(v)) return 'text-dimmed'
  if (v > -70) return 'text-green-500'
  if (v > -90) return 'text-yellow-500'
  return 'text-red-500'
})
function bars(dbm: number | null | undefined) {
  if (!hasSignal(dbm)) return 0
  if (dbm > -70) return 5
  if (dbm > -80) return 4
  if (dbm > -90) return 3
  if (dbm > -100) return 2
  return 1
}
function barColor(dbm: number | null | undefined) {
  if (!hasSignal(dbm)) return 'bg-muted'
  if (dbm > -70) return 'bg-green-500'
  if (dbm > -90) return 'bg-yellow-500'
  return 'bg-red-500'
}
</script>

<template>
  <div class="grid grid-cols-1 gap-4 lg:grid-cols-3">
    <!-- 运行状态 -->
    <div class="rounded-xl border border-default bg-elevated/40 p-4">
      <div class="mb-3 flex items-center justify-between">
        <span class="text-xs font-bold uppercase tracking-wider text-dimmed">运行状态</span>
        <UButton
          icon="i-lucide-settings-2"
          color="neutral"
          variant="ghost"
          size="xs"
          square
          aria-label="网络选择设置"
          @click="emit('open-operator')"
        />
      </div>

      <!-- VoWiFi 状态条 -->
      <div
        v-if="device?.vowifi_enabled"
        class="mb-3 flex items-center gap-2.5 rounded-xl border px-3.5 py-2.5"
        :class="{
          'border-green-500/25 bg-green-500/10': vowifiStatus === 'ok',
          'border-amber-500/25 bg-amber-500/10': vowifiStatus === 'partial',
          'border-red-500/25 bg-red-500/10': vowifiStatus === 'off'
        }"
      >
        <StatusDot
          :tone="vowifiStatus === 'ok' ? 'success' : vowifiStatus === 'partial' ? 'warning' : 'error'"
          :pulse="vowifiStatus !== 'off'"
        />
        <div class="min-w-0">
          <p
            class="text-sm font-bold leading-tight"
            :class="{
              'text-green-600 dark:text-green-400': vowifiStatus === 'ok',
              'text-amber-600 dark:text-amber-400': vowifiStatus === 'partial',
              'text-red-600 dark:text-red-400': vowifiStatus === 'off'
            }"
          >
            <template v-if="vowifiStatus === 'ok'">WiFi-Calling · 全部就绪</template>
            <template v-else-if="vowifiStatus === 'partial'">{{ notReady.join(' · ') }} 未就绪</template>
            <template v-else>VoWiFi 未连接</template>
          </p>
          <p
            v-if="device?.vowifi_runtime?.last_reason"
            class="mt-0.5 truncate text-xs text-muted"
          >{{ device.vowifi_runtime.last_reason }}</p>
        </div>
      </div>

      <!-- 就绪度 -->
      <div v-if="device?.vowifi_enabled" class="mb-3">
        <div class="mb-1 flex gap-1">
          <div
            v-for="item in readiness"
            :key="item.key"
            class="h-1.5 flex-1 rounded-full"
            :class="item.ready === true ? 'bg-green-500'
              : item.ready === false ? 'bg-red-500' : 'bg-accented'"
          />
        </div>
        <div class="flex justify-between">
          <span
            v-for="item in readiness"
            :key="item.key"
            class="flex-1 text-center text-[10px]"
            :class="item.ready === false ? 'font-bold text-error' : 'text-dimmed'"
          >{{ item.key }}</span>
        </div>
      </div>

      <!-- VoWiFi 次要字段：有错误时自动展开 -->
      <div v-if="device?.vowifi_enabled" class="mb-3 overflow-hidden rounded-lg border border-default">
        <button
          type="button"
          class="flex w-full items-center justify-between px-3 py-2 text-xs text-muted transition hover:bg-elevated/60"
          @click="showVowifiDetail = !showVowifiDetail"
        >
          <span class="font-bold uppercase tracking-wider">详情</span>
          <UIcon
            :name="(showVowifiDetail || hasError) ? 'i-lucide-chevron-up' : 'i-lucide-chevron-down'"
            class="size-3.5"
          />
        </button>
        <div
          v-if="showVowifiDetail || hasError"
          class="space-y-1.5 border-t border-default px-3 pb-2 pt-2 text-sm"
        >
          <FieldRow label="数据平面" :value="device?.vowifi_runtime?.dataplane_mode" monospace />
          <FieldRow label="最后原因" :value="device?.vowifi_runtime?.last_reason" />
          <FieldRow label="错误分类" :value="device?.vowifi_runtime?.last_error_class" monospace copyable />
          <FieldRow label="最后错误" :value="device?.vowifi_runtime?.last_error" copyable />
        </div>
      </div>

      <!-- 蜂窝模式的运营商状态条 -->
      <div
        v-if="!device?.vowifi_enabled"
        class="mb-3 flex items-center gap-2.5 rounded-xl border px-3.5 py-2.5"
        :class="isRegistered ? 'border-green-500/25 bg-green-500/10'
          : device?.control_online ? 'border-amber-500/25 bg-amber-500/10'
          : 'border-default bg-elevated'"
      >
        <StatusDot
          :tone="isRegistered ? 'success' : device?.control_online ? 'warning' : 'neutral'"
          :pulse="isRegistered"
        />
        <div class="min-w-0 flex-1">
          <p
            class="text-sm font-bold leading-tight"
            :class="isRegistered ? 'text-green-600 dark:text-green-400'
              : device?.control_online ? 'text-amber-600 dark:text-amber-400' : 'text-muted'"
          >
            <template v-if="isRegistered">
              {{ m?.operator || '--' }}
              <span v-if="m?.network_mode" class="opacity-70">
                · {{ [m?.network_duplex, m?.network_mode].filter(Boolean).join(' ') }}
              </span>
            </template>
            <template v-else>{{ cellularText }}</template>
          </p>
        </div>
      </div>

      <!-- 信号强度 -->
      <div class="mb-3 rounded-xl border border-default px-3.5 py-3">
        <p class="mb-1.5 text-[10px] font-bold uppercase tracking-wider text-dimmed">信号强度</p>
        <div class="flex items-center gap-3">
          <div class="min-w-0">
            <div class="flex items-baseline gap-1">
              <span class="text-2xl font-extrabold leading-none tabular-nums" :class="signalColorClass">
                {{ signalDbm ?? '--' }}
              </span>
              <span class="text-xs text-dimmed">dBm</span>
            </div>
            <p class="mt-1 text-[10px] text-dimmed">
              RSRP {{ m?.signal_rsrp ?? '--' }} · RSRQ {{ m?.signal_rsrq ?? '--' }} · SINR {{ m?.signal_sinr ?? '--' }}<template
                v-if="m?.nr5g_signal_sinr !== undefined"
              > · NR5G SINR {{ m.nr5g_signal_sinr }}</template>
            </p>
          </div>
          <div class="ml-auto flex items-end gap-0.5" style="height: 28px">
            <div
              v-for="i in 5"
              :key="i"
              class="w-1.5 rounded-sm transition-all duration-500"
              :class="bars(signalDbm) >= i ? barColor(signalDbm) : 'bg-accented'"
              :style="{ height: `${i * 20}%` }"
            />
          </div>
        </div>
      </div>

      <div class="space-y-1.5 text-sm">
        <FieldRow label="制式" :value="m?.network_mode" />
        <FieldRow label="频段" :value="m?.radio_band" />
        <FieldRow label="信道" :value="m?.radio_channel" />
        <FieldRow label="注册状态" :value="m?.reg_status_text || m?.reg_status" />
      </div>
    </div>

    <!-- SIM / 设备 -->
    <div class="rounded-xl border border-default bg-elevated/40 p-4">
      <div class="mb-2 flex items-center justify-between">
        <span class="text-xs font-bold uppercase tracking-wider text-dimmed">SIM / 设备</span>
        <UButton
          :icon="showSensitive ? 'i-lucide-eye' : 'i-lucide-eye-off'"
          color="neutral"
          variant="ghost"
          size="xs"
          square
          :aria-label="showSensitive ? '隐藏敏感信息' : '显示敏感信息'"
          @click="showSensitive = !showSensitive"
        />
      </div>
      <div class="space-y-1.5 text-sm">
        <FieldRow label="IMEI" :value="m?.imei" :sensitive="!showSensitive" monospace copyable />
        <FieldRow label="ICCID" :value="m?.iccid" :sensitive="!showSensitive" monospace copyable />
        <FieldRow label="IMSI" :value="m?.imsi" :sensitive="!showSensitive" monospace copyable />
        <FieldRow label="本机号码" :value="device?.local_phone" :sensitive="!showSensitive" monospace copyable />
        <FieldRow label="运营商" :value="m?.operator || m?.native_spn" />
        <FieldRow v-if="activeEsimProfileName" label="当前 eSIM" :value="activeEsimProfileName" monospace copyable />
        <FieldRow label="原运营商" :value="simOperator" copyable />
        <FieldRow label="固件版本" :value="m?.firmware" monospace copyable />
        <FieldRow
          label="归属 PLMN"
          :value="m?.native_mcc && m?.native_mnc ? `${m.native_mcc}/${m.native_mnc}` : undefined"
          monospace
        />
        <div class="flex items-center justify-between gap-3">
          <span class="text-muted">飞行模式</span>
          <span>{{ flightModeEnabled ? '是' : '否' }}</span>
        </div>
        <FieldRow label="运行模式" :value="backendModeText" monospace />
        <FieldRow label="设备 ID" :value="device?.id" monospace copyable />
        <div v-if="device?.e911_setup_available" class="flex items-center justify-between gap-3 pt-1">
          <span class="text-muted">E911 地址</span>
          <UButton size="xs" variant="soft" label="设置" @click="emit('setup-e911')" />
        </div>
      </div>
    </div>

    <!-- 网络 -->
    <div class="rounded-xl border border-default bg-elevated/40 p-4">
      <p class="mb-2 text-xs font-bold uppercase tracking-wider text-dimmed">网络</p>
      <div class="space-y-1.5 text-sm">
        <FieldRow label="内网 IPv4" :value="device?.private_ip" monospace copyable />
        <FieldRow label="内网 IPv6" :value="device?.private_ipv6" monospace copyable />
        <FieldRow label="外网 IPv4" :value="device?.public_ip" monospace copyable />
        <FieldRow label="外网 IPv6" :value="device?.public_ipv6" monospace copyable />
        <FieldRow label="网络接口" :value="device?.traffic_meta?.interface || device?.interface" monospace />
        <FieldRow label="累计上传" :value="device?.traffic?.tx" monospace />
        <FieldRow label="累计下载" :value="device?.traffic?.rx" monospace />
        <FieldRow label="实时速率" :value="device?.traffic?.rate" monospace />
        <FieldRow label="连接数" :value="device?.traffic?.active_conns || device?.traffic?.connections" monospace />
        <FieldRow label="数据连接" :value="device?.network_connected ? '已连接' : '未连接'" />
        <FieldRow label="生命周期" :value="device?.lifecycle_phase" />
      </div>
    </div>
  </div>
</template>
