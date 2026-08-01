<script setup lang="ts">
import type { DeviceConfigDTO, DiscoveredDevice } from '~/types/api'
import { isWwanQmiControlPath } from '~/utils/deviceBackend'

const props = defineProps<{
  discovering: boolean
  unconfiguredDiscovered: DiscoveredDevice[]
  addSelected: DiscoveredDevice | null
  addConfig: DeviceConfigDTO
  addSaving: boolean
  pcscError?: string
}>()

const open = defineModel<boolean>('open', { default: false })

const emit = defineEmits<{
  'select-device': [device: DiscoveredDevice]
  'fix-usbnet': [atPort: string]
  rescan: []
  save: []
}>()

function discoveryIdentity(d: DiscoveredDevice | null | undefined): string {
  if (!d) return ''
  return String(d.discovery_key || `${d.usb_path || ''}|${d.at_port || ''}`)
}

function isQmiDiscovery(d: DiscoveredDevice | null | undefined): boolean {
  return String(d?.mode || '').toLowerCase() === 'qmi'
}

function discoveryModeText(d: DiscoveredDevice | null | undefined): string {
  const mode = String(d?.mode || 'unknown').toLowerCase()
  if (mode === 'qmi') return 'QMI'
  if (mode === 'mbim') return 'MBIM'
  if (mode === 'ecm') return 'ECM'
  if (mode === 'rndis') return 'RNDIS'
  if (mode === 'ncm') return 'NCM'
  if (mode === 'pcsc') return 'PC/SC'
  return 'UNKNOWN'
}

// 读卡器只有卡、没有基带：没有网卡与 AT 口，IMEI 也读不到，必须手工填。
const isPCSCReader = computed(() => String(props.addSelected?.mode || '').toLowerCase() === 'pcsc')

// /dev/wwanXqmiY 形态的控制口只能走 QMI；MBIM 设备同理锁死
const isQMIBackendOnly = computed(() =>
  isWwanQmiControlPath(props.addSelected?.control_path || props.addConfig?.control_device)
)
const isMBIMBackendOnly = computed(() => String(props.addSelected?.mode || '').toLowerCase() === 'mbim')

watch(isQMIBackendOnly, (locked) => {
  if (locked && props.addConfig) props.addConfig.device_backend = 'qmi'
}, { immediate: true })

watch(isMBIMBackendOnly, (locked) => {
  if (locked && props.addConfig) props.addConfig.device_backend = 'mbim'
}, { immediate: true })

watch(isPCSCReader, (locked) => {
  if (locked && props.addConfig) props.addConfig.device_backend = 'pcsc'
}, { immediate: true })

const backendItems = computed(() => {
  if (isPCSCReader.value) return [{ label: 'PC/SC', value: 'pcsc' }]
  if (isMBIMBackendOnly.value) return [{ label: 'MBIM', value: 'mbim' }]
  return [
    { label: 'AT', value: 'at', disabled: isQMIBackendOnly.value },
    { label: 'QMI', value: 'qmi', disabled: !props.addConfig?.control_device }
  ]
})

const backendHint = computed(() => {
  if (isPCSCReader.value) return '仅读卡器，无基带；只能用于 VoWiFi'
  if (isQMIBackendOnly.value) return '固定 QMI，AT 口仅用于终端'
  if (isMBIMBackendOnly.value) return '固定 MBIM，AT 口仅用于终端'
  return 'AT=串口 / QMI=纯 QMI'
})
</script>

<template>
  <UModal v-model:open="open" title="添加设备配置" :ui="{ content: 'max-w-3xl' }">
    <template #body>
      <div class="flex flex-col gap-4">
        <p class="text-xs text-muted">
          选择一个「未配置」的设备，系统将自动填充 AT 端口与识别信息。
        </p>

        <!-- 发现列表 -->
        <div class="max-h-64 overflow-auto">
          <div v-if="discovering" class="flex flex-col items-center gap-2 py-10 text-dimmed">
            <UIcon name="i-lucide-loader-circle" class="size-7 animate-spin" />
            <span class="text-xs">正在探测设备…</span>
          </div>

          <template v-else>
            <!-- 读卡器枚举失败时给出原因，避免只留一句"暂无可添加设备" -->
            <UAlert
              v-if="pcscError"
              class="mb-2"
              color="warning"
              variant="subtle"
              icon="i-lucide-usb"
              title="PC/SC 读卡器不可用"
              :description="pcscError"
            />

            <EmptyState
              v-if="!unconfiguredDiscovered.length"
              icon="i-lucide-search-x"
              title="暂无可添加设备"
              description="系统未发现新的模组"
            />

            <div v-else class="flex flex-col gap-2">
            <button
              v-for="d in unconfiguredDiscovered"
              :key="discoveryIdentity(d)"
              type="button"
              class="w-full rounded-lg border p-3 text-left transition"
              :class="[
                d.degraded
                  ? 'cursor-not-allowed border-warning/40 bg-warning/5 opacity-85'
                  : discoveryIdentity(addSelected) === discoveryIdentity(d)
                    ? 'border-primary bg-primary/5'
                    : 'border-default hover:bg-elevated/60'
              ]"
              :aria-disabled="!!d.degraded"
              @click="emit('select-device', d)"
            >
              <div class="flex flex-wrap items-center gap-2">
                <span class="font-medium">
                  {{ d.mode === 'pcsc' ? d.control_path : `${d.net_interface || '--'} · ${d.driver_name || '--'}` }}
                </span>
                <UBadge
                  size="sm"
                  variant="subtle"
                  :color="d.mode === 'pcsc' ? 'primary' : isQmiDiscovery(d) ? 'success' : 'warning'"
                  :label="discoveryModeText(d)"
                />
              </div>
              <p v-if="d.mode === 'pcsc'" class="mt-0.5 text-xs text-muted">
                PC/SC 读卡器 · 仅供 VoWiFi 使用，需手工填写 IMEI
              </p>
              <p v-else class="mt-0.5 truncate font-mono text-xs text-muted">
                {{ d.control_path }} · AT: {{ d.at_port || '--' }} · IMEI: {{ d.imei || '--' }} · USB: {{ d.usb_path || '--' }}
              </p>
              <div v-if="d.degraded" class="mt-1.5 flex flex-wrap items-center gap-2">
                <span class="text-xs text-warning">无法读取 IMEI（控制口可能挂死），暂不可添加。</span>
                <UButton
                  v-if="d.at_port"
                  size="xs"
                  color="warning"
                  variant="soft"
                  icon="i-lucide-wrench"
                  label="修复 USB 模式"
                  @click.stop="emit('fix-usbnet', d.at_port)"
                />
              </div>
              </button>
            </div>
          </template>
        </div>

        <!-- 选定设备状态 -->
        <div v-if="addSelected" class="tile flex flex-col gap-1.5 p-3">
          <p class="text-[11px] font-medium uppercase tracking-wider text-dimmed">
            选定设备状态
          </p>
          <div class="flex flex-wrap items-center gap-2 text-sm">
            <span class="text-muted">模式:</span>
            <UBadge
              size="sm"
              variant="subtle"
              :color="isQmiDiscovery(addSelected) ? 'success' : 'warning'"
              :label="discoveryModeText(addSelected)"
            />
            <UBadge v-if="isQMIBackendOnly" size="sm" color="success" variant="subtle" label="仅 QMI 后端" />
            <UBadge v-if="isMBIMBackendOnly" size="sm" color="success" variant="subtle" label="仅 MBIM 后端" />
            <UBadge v-if="isPCSCReader" size="sm" color="primary" variant="subtle" label="仅读卡器" />
          </div>
          <p v-if="isQMIBackendOnly" class="text-xs text-success">
            此类 WWAN QMI 设备运行后端固定为 QMI；AT 口仍会保留给 AT 终端。
          </p>
          <p v-if="isPCSCReader" class="text-xs text-muted">
            读卡器只有卡、没有基带：没有数据连接、信号与蜂窝短信，也无法选网或重启模组。
            IMEI 读不到，必须手工填写——IKEv2 与 IMS 都要用它。
          </p>
        </div>

        <!-- 配置字段 -->
        <div class="grid gap-4 sm:grid-cols-2">
          <UFormField label="ID">
            <UInput v-model="addConfig.id" placeholder="例如 ec20_3" class="w-full font-mono" />
          </UFormField>
          <UFormField label="名称">
            <UInput v-model="addConfig.name" placeholder="显示名称（可选）" class="w-full" />
          </UFormField>
          <UFormField
            label="IMEI 绑定"
            :help="isPCSCReader ? '读卡器无法读取 IMEI，必填' : undefined"
            :required="isPCSCReader"
          >
            <UInput
              v-model="addConfig.modem_imei"
              :disabled="!isPCSCReader"
              :placeholder="isPCSCReader ? '手工填写 15 位 IMEI' : '自动识别（从发现设备填充）'"
              class="w-full font-mono"
            />
          </UFormField>

          <UFormField v-if="isPCSCReader" label="读卡器">
            <UInput v-model="addConfig.pcsc_reader" disabled class="w-full font-mono" />
          </UFormField>

          <template v-if="!isPCSCReader">
            <UFormField label="USB 路径">
              <UInput v-model="addConfig.usb_path" disabled class="w-full font-mono" />
            </UFormField>
            <UFormField label="网卡接口">
              <UInput v-model="addConfig.interface" disabled class="w-full font-mono" />
            </UFormField>
            <UFormField label="AT 端口">
              <UInput v-model="addConfig.at_port" disabled class="w-full font-mono" />
            </UFormField>
            <UFormField label="控制设备">
              <UInput v-model="addConfig.control_device" disabled class="w-full font-mono" />
            </UFormField>
          </template>

          <div class="tile flex items-center justify-between gap-3 p-3">
            <div class="min-w-0">
              <p class="text-sm font-medium">
                设备后端模式
              </p>
              <p class="text-xs text-muted mt-0.5">
                {{ backendHint }}
              </p>
            </div>
            <USelect
              v-model="addConfig.device_backend"
              :items="backendItems"
              value-key="value"
              placeholder="AT"
              :disabled="isQMIBackendOnly || isMBIMBackendOnly"
              class="w-24 shrink-0"
            />
          </div>
        </div>
      </div>
    </template>

    <template #footer>
      <div class="flex w-full justify-end gap-2">
        <UButton
          color="neutral"
          variant="outline"
          icon="i-lucide-radar"
          label="重新扫描"
          :loading="discovering"
          @click="emit('rescan')"
        />
        <UButton color="neutral" variant="ghost" label="取消" @click="open = false" />
        <UButton icon="i-lucide-save" label="保存" :loading="addSaving" @click="emit('save')" />
      </div>
    </template>
  </UModal>
</template>
