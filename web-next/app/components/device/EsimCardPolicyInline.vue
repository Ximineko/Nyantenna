<script setup lang="ts">
import type { CardPolicy } from '~/types/api'
import { cardsService } from '~/services/cards'
import { devicesService } from '~/services/devices'
import { useCardPolicyToggles, type PolicyMirror } from '~/composables/useCardPolicyToggles'

const props = defineProps<{
  deviceId: string
  iccid: string
  isActiveCard: boolean
  deviceOnline: boolean
}>()

const emit = defineEmits<{ policyChanged: [] }>()

const policy = ref<CardPolicy | null>(null)
const loadFailed = ref(false)
const loading = ref(false)

// 激活卡 + 设备在线 → live 热切换；否则只存策略，等激活/上线后生效
const mode = computed<'live' | 'stored'>(() =>
  props.isActiveCard && props.deviceOnline ? 'live' : 'stored'
)

const hint = computed(() => {
  if (mode.value === 'live') return ''
  if (!props.deviceOnline) return '设备离线，改动已保存，激活/上线后生效'
  return '改动将在此卡激活后生效'
})

const mirror = computed<PolicyMirror | null>(() =>
  policy.value
    ? {
        network_enabled: policy.value.network_enabled,
        vowifi_enabled: policy.value.vowifi_enabled,
        airplane_enabled: policy.value.airplane_enabled
      }
    : null
)

async function loadPolicy() {
  loading.value = true
  loadFailed.value = false
  const r = await cardsService.getPolicy(props.iccid)
  loading.value = false
  if (r.ok) policy.value = r.data
  else loadFailed.value = true
}

onMounted(loadPolicy)

// stored 模式下 PUT 互斥后的完整三元组
async function putTriple(next: PolicyMirror): Promise<{ ok: boolean }> {
  const r = await cardsService.putPolicy(props.iccid, {
    network_enabled: next.network_enabled,
    vowifi_enabled: next.vowifi_enabled,
    airplane_enabled: next.airplane_enabled
  })
  return { ok: r.ok }
}

const {
  local,
  networkPending, networkFailed,
  vowifiPending, vowifiFailed,
  airplanePending, airplaneFailed,
  onNetworkToggle, onVoWiFiToggle, onAirplaneToggle
} = useCardPolicyToggles(mirror, {
  async applyNetwork(enabled, next) {
    if (mode.value === 'stored') return putTriple(next)
    const r = enabled
      ? await devicesService.startNetwork(props.deviceId, {
          ip_version: policy.value?.ip_version || 'v4',
          apn: policy.value?.apn || ''
        })
      : await devicesService.stopNetwork(props.deviceId)
    return { ok: r.ok }
  },
  async applyVoWiFi(enabled, next) {
    if (mode.value === 'stored') return putTriple(next)
    const r = enabled
      ? await devicesService.enableVoWiFi(props.deviceId)
      : await devicesService.disableVoWiFi(props.deviceId)
    return { ok: r.ok }
  },
  async applyAirplane(enabled, next) {
    if (mode.value === 'stored') return putTriple(next)
    const r = await devicesService.setFlightMode(props.deviceId, enabled)
    return { ok: r.ok }
  },
  onChanged() { emit('policyChanged') }
})

const switches = computed(() => [
  {
    key: 'network', label: '网络',
    model: 'network_enabled' as const,
    disabled: local.value.vowifi_enabled || local.value.airplane_enabled || networkPending.value,
    pending: networkPending.value, failed: networkFailed.value, on: onNetworkToggle
  },
  {
    key: 'vowifi', label: 'VoWiFi',
    model: 'vowifi_enabled' as const,
    disabled: vowifiPending.value,
    pending: vowifiPending.value, failed: vowifiFailed.value, on: onVoWiFiToggle
  },
  {
    key: 'airplane', label: '飞行',
    model: 'airplane_enabled' as const,
    disabled: local.value.vowifi_enabled || airplanePending.value,
    pending: airplanePending.value, failed: airplaneFailed.value, on: onAirplaneToggle
  }
])
</script>

<template>
  <div class="rounded-lg bg-elevated/50 px-4 py-3">
    <div v-if="loading" class="flex items-center gap-1.5 text-xs text-dimmed">
      <UIcon name="i-lucide-loader-circle" class="size-3.5 animate-spin" />
      正在加载策略…
    </div>

    <div v-else-if="loadFailed" class="flex items-center gap-2 text-xs text-warning">
      策略加载失败
      <UButton size="xs" color="neutral" variant="ghost" label="重试" @click="loadPolicy" />
    </div>

    <template v-else>
      <p v-if="hint" class="mb-2 text-[11px] text-warning">
        {{ hint }}
      </p>
      <div class="grid gap-2 sm:grid-cols-3">
        <div
          v-for="s in switches"
          :key="s.key"
          class="flex items-center justify-between gap-2 rounded-lg bg-default px-3 py-2"
        >
          <span class="text-sm">{{ s.label }}</span>
          <div class="flex shrink-0 items-center gap-2">
            <span v-if="s.failed" class="text-xs text-warning">未生效</span>
            <UIcon v-if="s.pending" name="i-lucide-loader-circle" class="size-3.5 animate-spin text-dimmed" />
            <USwitch
              v-model="local[s.model]"
              :disabled="s.disabled"
              @update:model-value="s.on"
            />
          </div>
        </div>
      </div>
    </template>
  </div>
</template>
