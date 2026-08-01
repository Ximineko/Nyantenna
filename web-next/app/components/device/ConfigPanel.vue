<script setup lang="ts">
import { isWwanQmiControlPath } from '~/utils/deviceBackend'
import type { DeviceConfigDTO, DeviceOverviewItem } from '~/types/api'

const props = defineProps<{
  editConfig: DeviceConfigDTO | null
  deviceStatus?: DeviceOverviewItem | null
  saving: boolean
  deleting: boolean
}>()

const emit = defineEmits<{ save: []; delete: [] }>()

// 实际生效的探测值优先于配置里的存量值
const activeControlDevice = computed(() => props.deviceStatus?.control_device || props.editConfig?.control_device)
const activeInterface = computed(() => props.deviceStatus?.interface || props.editConfig?.interface)
const activeATPort = computed(() => props.deviceStatus?.at_port || props.editConfig?.at_port)
const activeUsbPath = computed(() => props.deviceStatus?.usb_path || props.editConfig?.usb_path)

// /dev/wwan0qmi0 形态的控制口只能走 QMI 后端，AT 口仅留给 AT 终端
const isQMIBackendOnly = computed(() => isWwanQmiControlPath(activeControlDevice.value))
const isMBIMBackendOnly = computed(
  () => String(props.editConfig?.device_backend || '').toLowerCase() === 'mbim'
)
// 仅读卡器设备：没有 USB 模组，网卡/AT 口/控制设备一概不存在
const isPCSC = computed(
  () => String(props.editConfig?.device_backend || '').toLowerCase() === 'pcsc'
)

watch(isQMIBackendOnly, (locked) => {
  if (locked && props.editConfig) props.editConfig.device_backend = 'qmi'
}, { immediate: true })

const backendItems = computed(() => {
  if (isPCSC.value) return [{ label: 'PC/SC', value: 'pcsc' }]
  if (isMBIMBackendOnly.value) return [{ label: 'MBIM', value: 'mbim' }]
  return [
    { label: 'AT', value: 'at', disabled: isQMIBackendOnly.value },
    { label: 'QMI', value: 'qmi', disabled: !activeControlDevice.value && props.editConfig?.device_backend !== 'qmi' }
  ]
})

const backendHint = computed(() => {
  if (isPCSC.value) return '仅 PC/SC 读卡器，无基带；只能用于 VoWiFi'
  if (isQMIBackendOnly.value) return '此类设备固定 QMI，AT 口仅用于终端'
  if (isMBIMBackendOnly.value) return '此类设备固定 MBIM，AT 口仅用于终端'
  return 'AT=传统串口 / QMI=纯 QMI'
})
</script>

<template>
  <div class="flex flex-col gap-4">
    <div class="flex flex-wrap items-start justify-between gap-3">
      <div>
        <h2 class="text-sm font-semibold">
          设备配置
        </h2>
        <p class="text-xs text-muted mt-0.5">
          配置存储在数据库中，部分字段可能需要重启生效
        </p>
      </div>
      <div class="flex items-center gap-2">
        <UButton
          icon="i-lucide-trash-2"
          color="error"
          variant="soft"
          label="删除设备"
          :loading="deleting"
          @click="emit('delete')"
        />
        <UButton
          icon="i-lucide-save"
          label="保存配置"
          :loading="saving"
          @click="emit('save')"
        />
      </div>
    </div>

    <div v-if="editConfig" class="grid gap-4 lg:grid-cols-2">
      <UFormField label="ID">
        <UInput :model-value="editConfig.id" disabled class="w-full font-mono" />
      </UFormField>

      <UFormField label="名称">
        <UInput v-model="editConfig.name" placeholder="显示名称" class="w-full" />
      </UFormField>

      <UFormField
        label="IMEI 绑定"
        :help="isPCSC ? '读卡器无法读取 IMEI，由此处提供给 IKEv2 与 IMS' : undefined"
      >
        <UInput
          v-model="editConfig.modem_imei"
          :disabled="!isPCSC"
          :placeholder="isPCSC ? '手工填写 15 位 IMEI' : '自动识别（添加时绑定）'"
          class="w-full font-mono"
        />
      </UFormField>

      <UFormField v-if="isPCSC" label="读卡器" help="留空表示自动选第一个有卡的读卡器">
        <UInput v-model="editConfig.pcsc_reader" placeholder="自动选择" class="w-full font-mono" />
      </UFormField>

      <template v-else>
        <UFormField label="设备路径">
          <UInput :model-value="activeUsbPath || ''" disabled placeholder="由系统自动探测" class="w-full font-mono" />
        </UFormField>

        <UFormField label="网卡接口">
          <UInput :model-value="activeInterface || ''" disabled placeholder="由系统自动探测" class="w-full font-mono" />
        </UFormField>

        <UFormField label="AT 端口">
          <UInput :model-value="activeATPort || ''" disabled placeholder="由系统自动探测" class="w-full font-mono" />
        </UFormField>

        <UFormField label="控制设备">
          <UInput :model-value="activeControlDevice || ''" disabled placeholder="由系统自动探测" class="w-full font-mono" />
        </UFormField>
      </template>

      <div class="tile flex items-center justify-between gap-3 p-3">
        <div class="min-w-0">
          <p class="text-sm font-medium">
            设备运行模式
          </p>
          <p class="text-xs text-muted mt-0.5">
            {{ backendHint }}
          </p>
        </div>
        <USelect
          v-model="editConfig.device_backend"
          :items="backendItems"
          value-key="value"
          placeholder="AT"
          :disabled="isQMIBackendOnly || isMBIMBackendOnly || isPCSC"
          class="w-28 shrink-0"
        />
      </div>
    </div>

    <EmptyState v-else icon="i-lucide-settings-2" title="配置加载中" />
  </div>
</template>
