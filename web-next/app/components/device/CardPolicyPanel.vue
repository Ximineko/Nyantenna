<script setup lang="ts">
import type { CardPolicy } from '~/types/api'
import { devicesService } from '~/services/devices'
import { useCardPolicyToggles, type PolicyMirror } from '~/composables/useCardPolicyToggles'

const props = defineProps<{
  deviceId: string | undefined
  iccid: string | undefined
  policy: CardPolicy | null
  deviceOnline: boolean
}>()

const emit = defineEmits<{ policyChanged: [] }>()

// ip/apn 不进 composable，由本组件独立持有；它们只在下次开网时随请求带出去
const ipVersion = ref<'v4' | 'v6' | 'v4v6'>('v4')
const apn = ref('')

const canToggle = computed(() => props.deviceOnline && !!props.iccid)

const mirror = computed<PolicyMirror | null>(() =>
  props.policy
    ? {
        network_enabled: props.policy.network_enabled,
        vowifi_enabled: props.policy.vowifi_enabled,
        airplane_enabled: props.policy.airplane_enabled
      }
    : null
)

watch(() => props.policy, (p) => {
  if (!p) return
  ipVersion.value = p.ip_version || 'v4'
  apn.value = p.apn || ''
}, { immediate: true })

// 三个开关都是「即时生效」：直接打设备动作端点，不是先存后应用
const {
  local,
  networkPending, networkFailed,
  vowifiPending, vowifiFailed,
  airplanePending, airplaneFailed,
  onNetworkToggle, onVoWiFiToggle, onAirplaneToggle
} = useCardPolicyToggles(mirror, {
  async applyNetwork(enabled) {
    if (!props.deviceId) return { ok: false }
    const r = enabled
      ? await devicesService.startNetwork(props.deviceId, { ip_version: ipVersion.value, apn: apn.value })
      : await devicesService.stopNetwork(props.deviceId)
    return { ok: r.ok }
  },
  async applyVoWiFi(enabled) {
    if (!props.deviceId) return { ok: false }
    const r = enabled
      ? await devicesService.enableVoWiFi(props.deviceId)
      : await devicesService.disableVoWiFi(props.deviceId)
    return { ok: r.ok }
  },
  async applyAirplane(enabled) {
    if (!props.deviceId) return { ok: false }
    const r = await devicesService.setFlightMode(props.deviceId, enabled)
    return { ok: r.ok }
  },
  onChanged() { emit('policyChanged') }
})

const ipVersionItems = [
  { label: 'IPv4', value: 'v4' },
  { label: 'IPv6', value: 'v6' },
  { label: 'IPv4 + IPv6（双栈）', value: 'v4v6' }
]

const sourceLabel = computed(() => {
  if (!props.policy) return ''
  return props.policy.source === 'user' ? '手动设置' : '自动默认'
})
</script>

<template>
  <div class="flex flex-col gap-4">
    <div>
      <h2 class="text-sm font-semibold">
        卡策略
      </h2>
      <p class="text-xs text-muted mt-0.5">
        网络 / VoWiFi 开关跟着 SIM 卡走，切换即时生效
      </p>
    </div>

    <EmptyState
      v-if="!iccid"
      icon="i-lucide-credit-card"
      title="尚未识别到 SIM 卡 ICCID"
      description="策略不可操作"
    />

    <template v-else>
      <UAlert
        v-if="!deviceOnline"
        color="warning"
        variant="subtle"
        icon="i-lucide-triangle-alert"
        title="设备离线"
        description="策略仅展示，切换操作已禁用"
      />

      <div class="tile flex items-center justify-between gap-3 p-3">
        <div class="min-w-0">
          <p class="text-[11px] font-medium uppercase tracking-wider text-dimmed">
            当前卡 ICCID
          </p>
          <p class="mt-0.5 truncate font-mono text-sm">
            {{ iccid }}
          </p>
        </div>
        <UBadge
          v-if="sourceLabel"
          :color="policy?.source === 'user' ? 'primary' : 'neutral'"
          variant="subtle"
          size="sm"
          :label="sourceLabel"
        />
      </div>

      <div class="grid gap-3 lg:grid-cols-2">
        <UFormField label="IP 版本" help="下次开启网络时生效">
          <USelect
            v-model="ipVersion"
            :items="ipVersionItems"
            value-key="value"
            :disabled="!canToggle"
            class="w-full"
          />
        </UFormField>

        <UFormField label="APN（可选）" help="下次开启网络时生效">
          <UInput v-model="apn" placeholder="留空自动识别" :disabled="!canToggle" class="w-full" />
        </UFormField>

        <!-- 开启网络 -->
        <div class="tile flex items-center justify-between gap-3 p-3" :class="local.network_enabled ? 'ring-1 ring-primary/40' : ''">
          <div class="min-w-0">
            <p class="text-sm font-medium">
              开启网络
            </p>
            <p class="text-xs text-muted mt-0.5">
              VoWiFi / 飞行开启时不可用
            </p>
          </div>
          <div class="flex shrink-0 items-center gap-2">
            <span v-if="networkFailed" class="text-xs text-warning">未生效</span>
            <UIcon v-if="networkPending" name="i-lucide-loader-circle" class="size-4 animate-spin text-dimmed" />
            <USwitch
              v-model="local.network_enabled"
              :disabled="!canToggle || local.vowifi_enabled || local.airplane_enabled || networkPending"
              @update:model-value="onNetworkToggle"
            />
          </div>
        </div>

        <!-- VoWiFi -->
        <div class="tile flex items-center justify-between gap-3 p-3" :class="local.vowifi_enabled ? 'ring-1 ring-warning/40' : ''">
          <div class="min-w-0">
            <p class="text-sm font-medium">
              VoWiFi
            </p>
            <p class="text-xs text-muted mt-0.5">
              启用后进飞行模式，不支持国内运营商
            </p>
          </div>
          <div class="flex shrink-0 items-center gap-2">
            <span v-if="vowifiFailed" class="text-xs text-warning">未生效</span>
            <UIcon v-if="vowifiPending" name="i-lucide-loader-circle" class="size-4 animate-spin text-dimmed" />
            <USwitch
              v-model="local.vowifi_enabled"
              :disabled="!canToggle || vowifiPending"
              @update:model-value="onVoWiFiToggle"
            />
          </div>
        </div>

        <!-- 飞行模式 -->
        <div class="tile flex items-center justify-between gap-3 p-3" :class="local.airplane_enabled ? 'ring-1 ring-primary/40' : ''">
          <div class="min-w-0">
            <p class="text-sm font-medium">
              飞行模式
            </p>
            <p class="text-xs text-muted mt-0.5">
              射频关闭，断网；VoWiFi 开启时由其接管
            </p>
          </div>
          <div class="flex shrink-0 items-center gap-2">
            <span v-if="airplaneFailed" class="text-xs text-warning">未生效</span>
            <UIcon v-if="airplanePending" name="i-lucide-loader-circle" class="size-4 animate-spin text-dimmed" />
            <USwitch
              v-model="local.airplane_enabled"
              :disabled="!canToggle || local.vowifi_enabled || airplanePending"
              @update:model-value="onAirplaneToggle"
            />
          </div>
        </div>
      </div>
    </template>
  </div>
</template>
