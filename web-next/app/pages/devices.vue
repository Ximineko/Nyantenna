<script setup lang="ts">
import { storeToRefs } from 'pinia'
import { useDevicesStore } from '~/stores/devices'
import { usePollingScheduler } from '~/composables/usePollingScheduler'
import { devicesService } from '~/services/devices'
import { cardsService } from '~/services/cards'
import { trafficService, createEmptyTrafficAnalysis, type TrafficRange } from '~/services/traffic'
import { isWwanQmiControlPath } from '~/utils/deviceBackend'
import type { CardPolicy, DeviceConfigDTO, DiscoveredDevice } from '~/types/api'
import type { DeviceListVM } from '~/types/view-model'

const store = useDevicesStore()
const route = useRoute()
const router = useRouter()
const toast = useToast()

const { list, deviceLimit, detail, config, loading, error, discovered, pcscError } = storeToRefs(store)

const selectedId = ref<string>(String(route.query.device ?? ''))
const busy = ref('')
const selected = computed(() => list.value.find(d => d.id === selectedId.value) ?? null)

const detailTab = ref(String(route.query.tab ?? 'overview'))

/** 仅 PC/SC 读卡器的设备：没有基带，选网/eSIM/USSD/AT 一概不适用 */
const isPCSCDevice = computed(() => String(selected.value?.backend_mode || '').toLowerCase() === 'pcsc')

const detailTabs = computed(() => {
  const tabs = [{ label: '概览', value: 'overview', icon: 'i-lucide-gauge' }]
  if (!isPCSCDevice.value) {
    tabs.push(
      { label: '选网', value: 'operator', icon: 'i-lucide-radio-tower' },
      { label: 'eSIM', value: 'esim', icon: 'i-lucide-credit-card' },
      { label: 'USSD', value: 'ussd', icon: 'i-lucide-message-circle-code' },
      { label: 'AT', value: 'at', icon: 'i-lucide-terminal' }
    )
  }
  tabs.push(
    { label: '卡策略', value: 'card', icon: 'i-lucide-credit-card' },
    { label: '配置', value: 'config', icon: 'i-lucide-settings-2' }
  )
  return tabs
})

// 切到 PC/SC 设备时，之前停留的射频类标签页已不存在，退回概览
watch(isPCSCDevice, (pcsc) => {
  if (pcsc && !['overview', 'card', 'config'].includes(detailTab.value)) {
    detailTab.value = 'overview'
  }
})

async function select(d: DeviceListVM) {
  selectedId.value = d.id
  detailTab.value = 'overview'
  void router.replace({ query: { device: d.id, tab: 'overview' } })
  await Promise.all([store.fetchDetail(d.id), store.fetchConfig(d.id)])
}

function back() {
  selectedId.value = ''
  void router.replace({ query: {} })
}

watch(detailTab, v => {
  if (selectedId.value) void router.replace({ query: { device: selectedId.value, tab: v } })
})

async function copyText(text: string) {
  if (!text) return
  try {
    await navigator.clipboard.writeText(text)
    toast.add({ title: '已复制', description: text, color: 'success' })
  } catch {
    toast.add({ title: '复制失败', color: 'error' })
  }
}

/** 统一包装设备操作：置忙、提示、刷新 */
async function act(id: string, label: string, fn: () => Promise<unknown>) {
  busy.value = id
  try {
    const result = await fn() as { ok?: boolean; error?: { message?: string } }
    toast.add(result?.ok === false
      ? { title: `${label}失败`, description: String(result?.error?.message ?? ''), color: 'error' }
      : { title: `${label}已执行`, color: 'success' })
  } finally {
    busy.value = ''
    await store.fetchList()
    if (selectedId.value) await store.fetchDetail(selectedId.value)
  }
}

/** DeviceCard 用的是仪表盘的数据形状，这里把管理页的嵌套字段摊平 */
function toCardDevice(d: DeviceListVM) {
  return {
    id: d.id,
    name: d.name,
    healthy: d.healthy,
    operator: d.modem?.operator || d.modem?.native_spn || '',
    network_mode: d.modem?.network_mode || '',
    network_duplex: d.modem?.network_duplex || '',
    signal_dbm: d.modem?.signal_dbm ?? 0,
    public_ip: d.public_ip,
    public_ipv6: d.public_ipv6,
    vowifi_active: d.vowifi_enabled === true
  }
}

function state(d: DeviceListVM) {
  if (!d.running) return { tone: 'neutral' as const, label: '未运行' }
  if (!d.healthy) return { tone: 'error' as const, label: '异常' }
  if (d.vowifi_enabled) return { tone: 'primary' as const, label: 'VoWiFi' }
  if (d.network_connected) return { tone: 'success' as const, label: '已联网' }
  return { tone: 'warning' as const, label: '待联网' }
}

/* 配置 */
const configDraft = ref<DeviceConfigDTO | null>(null)
const configSaving = ref(false)
const deleting = ref(false)
watch(config, c => { configDraft.value = c ? { ...c } as DeviceConfigDTO : null }, { immediate: true })

async function saveConfig() {
  if (!configDraft.value || !selectedId.value) return
  configSaving.value = true
  const result = await devicesService.updateConfig(selectedId.value, configDraft.value as never)
  configSaving.value = false
  toast.add(result?.ok === false
    ? { title: '保存失败', description: String(result?.error?.message ?? ''), color: 'error' }
    : { title: '配置已保存', color: 'success' })
  if (result?.ok !== false) await store.fetchConfig(selectedId.value)
}

async function removeDevice(id: string) {
  deleting.value = true
  await devicesService.deleteManaged(id)
  deleting.value = false
  back()
  await store.fetchList()
  toast.add({ title: '设备已移除', color: 'neutral' })
}

/* 单设备流量分析 */
const deviceAnalysis = ref(createEmptyTrafficAnalysis())
const deviceAnalysisRange = ref<TrafficRange>('day')
const deviceAnalysisLoading = ref(false)
const deviceAnalysisError = ref<import('~/services/http').AppError | null>(null)

async function fetchDeviceAnalysis() {
  const id = selectedId.value
  if (!id) { deviceAnalysis.value = createEmptyTrafficAnalysis(); return }
  deviceAnalysisLoading.value = true
  const result = await trafficService.getAnalysis(deviceAnalysisRange.value, id)
  deviceAnalysisLoading.value = false
  if (result.ok) {
    deviceAnalysis.value = result.data
    deviceAnalysisError.value = null
  } else {
    deviceAnalysisError.value = result.error
  }
}

function setDeviceAnalysisRange(r: TrafficRange) {
  if (deviceAnalysisRange.value === r) return
  deviceAnalysisRange.value = r
  void fetchDeviceAnalysis()
}

// 只在概览页可见时拉，切设备也要重拉
watch([selectedId, detailTab], ([id, tab]) => {
  if (id && tab === 'overview') void fetchDeviceAnalysis()
}, { immediate: true })

/* 卡策略：跟着 ICCID 走，切设备或换卡都要重新拉 */
const cardPolicy = ref<CardPolicy | null>(null)
const currentICCID = computed(() => String(detail.value?.modem?.iccid || ''))

async function loadCardPolicy() {
  const iccid = currentICCID.value
  if (!iccid) { cardPolicy.value = null; return }
  const result = await cardsService.getPolicy(iccid)
  cardPolicy.value = result.ok ? result.data : null
}

watch(currentICCID, () => { void loadCardPolicy() }, { immediate: true })

async function onCardPolicyChanged() {
  await loadCardPolicy()
  await store.fetchList()
  if (selectedId.value) await store.fetchDetail(selectedId.value)
}

/* E911 运营商网页设置 */
const e911Open = ref(false)
const e911Websheet = ref<import('~/types/api').CarrierWebsheetInfo | null>(null)

async function setupE911() {
  if (!selectedId.value) return
  const result = await devicesService.startE911Websheet(selectedId.value)
  if (result.ok === false) {
    toast.add({ title: 'E911 设置无法启动', description: String(result.error?.message ?? ''), color: 'error' })
    return
  }
  e911Websheet.value = result.data
  e911Open.value = true
}

/* 设备发现与纳管 */
const discoverOpen = ref(false)
const discovering = ref(false)
const addSaving = ref(false)
const addSelected = ref<DiscoveredDevice | null>(null)
const addConfig = ref<DeviceConfigDTO>(emptyAddConfig())

function emptyAddConfig(): DeviceConfigDTO {
  return {
    id: '', name: '', interface: '', modem_imei: '', usb_path: '',
    esim_transport: 'at', at_port: '', control_device: '', device_backend: 'at',
    pcsc_reader: ''
  }
}

/** 只有未纳管、才是可添加的候选 */
const unconfiguredDiscovered = computed(() => discovered.value.filter(d => !d.configured))

async function openDiscover() {
  discoverOpen.value = true
  addSelected.value = null
  addConfig.value = emptyAddConfig()
  await rescanDiscovered(false)
}

/** 探测口的形态决定后端：MBIM 锁 MBIM，wwanXqmiY 或 QMI+控制口锁 QMI，其余走 AT */
function applyDiscoveredToAddConfig(d: DiscoveredDevice) {
  const mode = String(d.mode || '').toLowerCase()

  // PC/SC 读卡器不是 USB 模组：没有网卡、AT 口与控制设备，
  // 唯一的定位信息是读卡器名；IMEI 保留用户手工输入的值。
  if (mode === 'pcsc') {
    addConfig.value.device_backend = 'pcsc'
    addConfig.value.pcsc_reader = d.control_path || ''
    addConfig.value.interface = ''
    addConfig.value.at_port = ''
    addConfig.value.control_device = ''
    addConfig.value.usb_path = ''
    return
  }

  addConfig.value.pcsc_reader = ''
  addConfig.value.interface = d.net_interface || ''
  addConfig.value.at_port = d.at_port || ''
  addConfig.value.control_device = d.control_path || ''
  addConfig.value.modem_imei = d.imei || ''
  addConfig.value.usb_path = d.usb_path || ''

  if (mode === 'mbim') addConfig.value.device_backend = 'mbim'
  else if (isWwanQmiControlPath(d.control_path) || (mode === 'qmi' && d.control_path)) addConfig.value.device_backend = 'qmi'
  else addConfig.value.device_backend = 'at'
}

function selectDiscovered(d: DiscoveredDevice) {
  if (d.degraded) {
    toast.add({
      title: '无法读取该设备 IMEI',
      description: '控制口可能挂死，请执行 AT!RESET 或切换组态后重试',
      color: 'warning'
    })
    return
  }
  addSelected.value = d
  applyDiscoveredToAddConfig(d)
}

/** rescan=true 时先让后端重新枚举 USB，再拉发现列表 */
async function rescanDiscovered(rescan = true) {
  discovering.value = true
  if (rescan) await devicesService.rescanAll()
  await store.fetchDiscovered()
  discovering.value = false
}

async function addDevice() {
  if (!addSelected.value) {
    toast.add({ title: '请选择一个未配置设备', color: 'warning' })
    return
  }
  if (addConfig.value.device_backend === 'pcsc' && !String(addConfig.value.modem_imei || '').trim()) {
    toast.add({ title: 'PC/SC 设备必须填写 IMEI', description: '读卡器没有基带，无法自动获取', color: 'warning' })
    return
  }
  addSaving.value = true
  applyDiscoveredToAddConfig(addSelected.value)
  const result = await devicesService.addManaged(addConfig.value as never)
  addSaving.value = false
  if (result.ok === false) {
    toast.add({ title: '添加失败', description: String(result.error?.message ?? ''), color: 'error' })
    return
  }
  const warning = result.data?.warning
  toast.add(warning
    ? { title: '设备已添加', description: warning, color: 'warning' }
    : { title: result.data?.started === true ? '设备已添加并开始接管' : '设备配置已添加', color: 'success' })
  discoverOpen.value = false
  await store.fetchList()
}

/** 部分模组的 USB 网络模式不对会导致无法联网，这里提供一键修复 */
async function fixUsbNet(atPort: string) {
  const result = await devicesService.fixDiscoveredUSBNet(atPort)
  toast.add(result.ok === false
    ? { title: '修复失败', description: String(result.error?.message ?? ''), color: 'error' }
    : { title: 'USB 网络模式已修复，请重新扫描', color: 'success' })
  await store.fetchDiscovered()
}

usePollingScheduler(() => store.fetchList(), 5000, {
  immediate: true, maxIntervalMs: 30000, backgroundIntervalMs: 15000
})

onMounted(() => {
  if (selectedId.value) {
    void store.fetchDetail(selectedId.value)
    void store.fetchConfig(selectedId.value)
  }
})
</script>

<template>
  <div>
    <!-- ============ 详情 ============ -->
    <template v-if="selected">
      <!-- 详情头部卡片 -->
      <div class="tile p-5">
        <div class="flex flex-col gap-4 lg:flex-row lg:items-center lg:justify-between">
          <div class="flex min-w-0 items-center gap-3">
            <UButton
              icon="i-lucide-arrow-left"
              color="neutral"
              variant="ghost"
              square
              aria-label="返回设备列表"
              @click="back"
            />
            <div
              class="flex size-11 shrink-0 items-center justify-center rounded-xl
                     bg-primary/10 text-lg font-extrabold text-primary ring-1 ring-primary/20"
            >
              <UIcon v-if="isPCSCDevice" name="i-lucide-scan-line" class="size-5" />
              <template v-else>N</template>
            </div>
            <div class="min-w-0">
              <div class="flex flex-wrap items-center gap-2">
                <p class="truncate text-xl font-extrabold">{{ selected.name || selected.id }}</p>
                <StatusDot :tone="state(selected).tone" :pulse="selected.healthy" />
                <span class="shrink-0 text-xs text-muted">{{ state(selected).label }}</span>
                <DeviceKindBadge :backend-mode="selected.backend_mode" size="sm" always />
              </div>
              <p class="mt-0.5 truncate text-xs text-muted">
                <span class="cursor-pointer font-mono hover:underline" @click="copyText(selected.id)">
                  {{ selected.id }}
                </span>
                · 公网 IP:
                <span class="cursor-pointer font-mono hover:underline" @click="copyText(selected.public_ip || '')">
                  {{ selected.public_ip || '---' }}
                </span>
              </p>
            </div>
          </div>

          <div class="flex flex-wrap items-center gap-2">
            <UButton
              v-if="selected.vowifi_enabled"
              icon="i-lucide-refresh-ccw"
              color="neutral"
              variant="outline"
              label="重连 VoWiFi"
              :loading="busy === selected.id"
              @click="act(selected.id, 'VoWiFi 重连', () => devicesService.reconnectVoWiFi(selected!.id))"
            />
            <UButton
              v-else-if="!isPCSCDevice"
              icon="i-lucide-refresh-ccw"
              color="neutral"
              variant="outline"
              label="切换 IP"
              :disabled="!selected.network_connected"
              :loading="busy === selected.id"
              @click="act(selected.id, '更换 IP', () => devicesService.rotateIP(selected!.id))"
            />
            <UButton
              v-if="!isPCSCDevice"
              icon="i-lucide-power"
              color="neutral"
              variant="outline"
              label="重启模组"
              class="hover:text-error"
              :loading="busy === selected.id"
              @click="act(selected.id, '重启模组', () => devicesService.rebootModem(selected!.id))"
            />
            <UButton
              icon="i-lucide-mail"
              color="neutral"
              variant="outline"
              label="短信"
              @click="router.push({ path: '/sms', query: { device: selected.id } })"
            />
            <UDropdownMenu
              :items="[[
                ...(isPCSCDevice ? [] : [
                  { label: selected.network_connected ? '断开网络' : '连接网络', icon: selected.network_connected ? 'i-lucide-wifi-off' : 'i-lucide-wifi', onSelect: () => act(selected!.id, selected!.network_connected ? '断开网络' : '连接网络', () => selected!.network_connected ? devicesService.stopNetwork(selected!.id) : devicesService.startNetwork(selected!.id)) }
                ]),
                { label: selected.vowifi_enabled ? '关闭 VoWiFi' : '开启 VoWiFi', icon: 'i-lucide-radio', onSelect: () => act(selected!.id, 'VoWiFi 切换', () => selected!.vowifi_enabled ? devicesService.disableVoWiFi(selected!.id) : devicesService.enableVoWiFi(selected!.id)) },
                { label: '刷新信息', icon: 'i-lucide-rotate-cw', onSelect: () => act(selected!.id, '刷新', () => devicesService.refreshInfo(selected!.id)) },
                ...(isPCSCDevice ? [] : [
                  { label: '切换飞行模式', icon: 'i-lucide-plane', onSelect: () => act(selected!.id, '飞行模式', () => devicesService.setFlightMode(selected!.id, !selected!.network_connected)) }
                ])
              ], [
                { label: '移除设备', icon: 'i-lucide-trash-2', color: 'error', onSelect: () => removeDevice(selected!.id) }
              ]]"
            >
              <UButton icon="i-lucide-ellipsis" color="neutral" variant="outline" square aria-label="更多操作" />
            </UDropdownMenu>
          </div>
        </div>
      </div>

      <div class="tile mt-4 p-5">
        <UTabs v-model="detailTab" :items="detailTabs" :content="false" class="mb-5" />

      <!-- 概览 -->
      <template v-if="detailTab === 'overview'">
        <DeviceOverviewTab
          :device="(detail ?? selected) as never"
          @open-operator="detailTab = 'operator'"
          @setup-e911="setupE911"
        />
        <div class="mt-6">
          <TrafficAnalysisPanel
            :analysis="deviceAnalysis"
            :range="deviceAnalysisRange"
            mode="device"
            title="当前设备流量分析"
            subtitle="数据每分钟采样一次，按日/周/月聚合"
            :loading="deviceAnalysisLoading"
            :error="deviceAnalysisError"
            :disabled="!selected.network_connected"
            @update:range="setDeviceAnalysisRange"
            @refresh="fetchDeviceAnalysis"
          />
        </div>
      </template>

      <!-- 运营商选择 -->
      <DeviceOperatorPanel v-else-if="detailTab === 'operator'" :key="`op-${selected.id}`" :device-id="selected.id" />

      <!-- eSIM -->
      <DeviceEsimPanel
        v-else-if="detailTab === 'esim'"
        :key="`esim-${selected.id}`"
        :device-id="selected.id"
        :device-imei="detail?.modem?.imei || ''"
        :device-online="selected.running === true"
      />

      <!-- USSD -->
      <DeviceUssdPanel v-else-if="detailTab === 'ussd'" :key="`ussd-${selected.id}`" :device-id="selected.id" />

      <!-- AT -->
      <DeviceAtPanel
        v-else-if="detailTab === 'at'"
        :key="`at-${selected.id}`"
        :device-id="selected.id"
        :backend-mode="selected.backend_mode"
        :at-port="selected.at_port"
        :running="selected.running"
      />

      <!-- 卡策略 -->
      <DeviceCardPolicyPanel
        v-else-if="detailTab === 'card'"
        :key="`card-${selected.id}`"
        :device-id="selected.id"
        :iccid="currentICCID"
        :policy="cardPolicy"
        :device-online="selected.running === true"
        @policy-changed="onCardPolicyChanged"
      />

      <!-- 配置 -->
      <DeviceConfigPanel
        v-else
        :edit-config="configDraft"
        :device-status="(detail ?? null) as never"
        :saving="configSaving"
        :deleting="deleting"
        @save="saveConfig"
        @delete="removeDevice(selected.id)"
      />
      </div>
    </template>

    <!-- ============ 列表 ============ -->
    <template v-else>
      <PageHeader
        title="设备"
        :description="deviceLimit ? `共 ${list.length} 台 / 上限 ${deviceLimit}` : `共 ${list.length} 台`"
      >
        <template #actions>
          <UButton
            icon="i-lucide-refresh-cw"
            color="neutral"
            variant="outline"
            :loading="loading"
            aria-label="刷新"
            @click="store.fetchList()"
          />
          <UButton icon="i-lucide-radar" color="neutral" variant="outline" label="扫描" @click="openDiscover" />
        </template>
      </PageHeader>

      <UAlert
        v-if="error"
        color="error"
        variant="subtle"
        icon="i-lucide-triangle-alert"
        title="设备列表加载失败"
        :description="String(error.message || error)"
      />

      <div v-else-if="loading && !list.length" class="grid gap-4 sm:grid-cols-2 xl:grid-cols-3">
        <USkeleton v-for="i in 6" :key="i" class="h-44 w-full rounded-lg" />
      </div>

      <div v-else-if="!list.length" class="tile">
        <EmptyState icon="i-lucide-cpu" title="还没有纳管设备" description="点击扫描发现已接入的模组">
          <UButton size="sm" variant="soft" label="扫描新设备" @click="openDiscover" />
        </EmptyState>
      </div>

      <div v-else class="grid gap-4 sm:grid-cols-2 xl:grid-cols-3">
        <DeviceCard
          v-for="d in list"
          :key="d.id"
          :device="toCardDevice(d) as never"
          :status-label="state(d).label"
          :status-tone="state(d).tone"
          :backend-mode="d.backend_mode"
          @open-device="select(d)"
        />
      </div>
    </template>

    <!-- E911 运营商页面 -->
    <CarrierWebsheetDialog
      v-model:open="e911Open"
      :websheet="e911Websheet"
      @done="store.fetchDetail(selectedId)"
    />

    <!-- 设备发现与添加 -->
    <DeviceAddDialog
      v-model:open="discoverOpen"
      :discovering="discovering"
      :unconfigured-discovered="unconfiguredDiscovered"
      :add-selected="addSelected"
      :add-config="addConfig"
      :add-saving="addSaving"
      :pcsc-error="pcscError"
      @select-device="selectDiscovered"
      @fix-usbnet="fixUsbNet"
      @rescan="rescanDiscovered()"
      @save="addDevice"
    />

  </div>
</template>
