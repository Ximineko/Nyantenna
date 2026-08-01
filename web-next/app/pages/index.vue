<script setup lang="ts">
import { storeToRefs } from 'pinia'
import { useDashboardStore } from '~/stores/dashboard'
import { usePollingScheduler } from '~/composables/usePollingScheduler'
import type { TrafficRange } from '~/services/traffic'

const dashboard = useDashboardStore()
const router = useRouter()
const {
  devices, devicesLoading: loading, devicesError,
  analysis, analysisLoading, analysisError
} = storeToRefs(dashboard)

const analysisRange = ref<TrafficRange>('day')

const total = computed(() => devices.value.length)
const online = computed(() => devices.value.filter(d => d?.healthy).length)
const offline = computed(() => Math.max(0, total.value - online.value))

async function fetchDevices() { await dashboard.fetchDevices() }
async function fetchAnalysis() { await dashboard.fetchAnalysis(analysisRange.value) }

function setRange(r: TrafficRange) {
  if (analysisRange.value === r) return
  analysisRange.value = r
  void fetchAnalysis()
}

function openDevice(id: string) {
  const deviceID = String(id || '').trim()
  if (deviceID) void router.push({ path: '/devices', query: { device: deviceID } })
}

usePollingScheduler(fetchDevices, 5000, {
  immediate: true, maxIntervalMs: 30000, backgroundIntervalMs: 15000
})
usePollingScheduler(fetchAnalysis, 60000, {
  immediate: false, maxIntervalMs: 300000, backgroundIntervalMs: 120000
})

onMounted(() => { setTimeout(fetchAnalysis, 800) })
</script>

<template>
  <div>
    <PageHeader title="总览" description="设备状态与流量趋势">
      <template #actions>
        <UButton
          icon="i-lucide-refresh-cw"
          color="neutral"
          variant="outline"
          :loading="loading"
          label="刷新"
          @click="fetchDevices"
        />
      </template>
    </PageHeader>

    <!-- 指标：一条横向分格，不再是三个独立卡片 -->
    <div class="tile grid divide-y divide-default sm:grid-cols-3 sm:divide-x sm:divide-y-0">
      <StatTile label="设备总数" :value="total" icon="i-lucide-cpu" />
      <StatTile label="在线" :value="online" icon="i-lucide-circle-check" tone="success" />
      <StatTile label="离线" :value="offline" icon="i-lucide-circle-slash" tone="error" />
    </div>

    <!-- 设备 -->
    <section class="mt-8">
      <div class="flex items-baseline justify-between gap-4 mb-3">
        <h2 class="text-sm font-semibold">设备</h2>
        <span class="text-xs text-dimmed tabular-nums">{{ total }} 台</span>
      </div>

      <UAlert
        v-if="devicesError"
        color="error"
        variant="subtle"
        icon="i-lucide-triangle-alert"
        title="设备列表加载失败"
        :description="String(devicesError.message || devicesError)"
      />

      <div v-else-if="loading && !devices.length" class="grid gap-3 sm:grid-cols-2 xl:grid-cols-3">
        <USkeleton v-for="i in 6" :key="i" class="h-24 w-full rounded-lg" />
      </div>

      <div v-else-if="!devices.length" class="tile">
        <EmptyState icon="i-lucide-cpu" title="还没有设备" description="接入模组后会自动出现在这里" />
      </div>

      <div v-else class="grid gap-3 sm:grid-cols-2 xl:grid-cols-3">
        <button
          v-for="d in devices"
          :key="d.id"
          type="button"
          class="tile-interactive px-4 py-3.5 text-left"
          @click="openDevice(d.id)"
        >
          <div class="flex items-center gap-2.5">
            <StatusDot :tone="d.healthy ? 'success' : 'error'" :pulse="d.healthy" />
            <span class="font-medium truncate">{{ d.name || d.id }}</span>
            <DeviceKindBadge :backend-mode="d.backend_mode" />
            <UIcon name="i-lucide-chevron-right" class="size-4 text-dimmed ml-auto shrink-0" />
          </div>
          <div class="mt-3 flex items-center justify-between gap-3 text-xs">
            <span class="text-dimmed truncate">{{ d.operator || '未知运营商' }}</span>
            <span class="font-mono text-muted truncate">{{ d.public_ip || '—' }}</span>
          </div>
        </button>
      </div>
    </section>

    <!-- 流量 -->
    <div class="mt-8">
      <TrafficAnalysisPanel
        :analysis="analysis"
        :range="analysisRange"
        mode="overview"
        title="流量趋势"
        :loading="analysisLoading"
        :error="analysisError"
        @update:range="setRange"
        @refresh="fetchAnalysis"
      />
    </div>

  </div>
</template>
