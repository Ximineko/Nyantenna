<script setup lang="ts">
import { use } from 'echarts/core'
import { CanvasRenderer } from 'echarts/renderers'
import { LineChart } from 'echarts/charts'
import { GridComponent, TooltipComponent, LegendComponent } from 'echarts/components'
import VChart from 'vue-echarts'
import type { TrafficAnalysis, TrafficRange } from '~/services/traffic'
import type { AppError } from '~/services/http'

use([CanvasRenderer, LineChart, GridComponent, TooltipComponent, LegendComponent])

const props = withDefaults(defineProps<{
  analysis: TrafficAnalysis
  range: TrafficRange
  /** device 模式下单设备无数据即视为空；overview 模式下只要有时间轴就画 */
  mode?: 'overview' | 'device'
  loading?: boolean
  error?: AppError | null
  title?: string
  subtitle?: string
  disabled?: boolean
}>(), {
  mode: 'overview',
  title: '流量分析',
  subtitle: '数据每分钟采样一次，按日/周/月聚合'
})

const emit = defineEmits<{ 'update:range': [TrafficRange]; refresh: [] }>()

const colorMode = useColorMode()
const isDark = computed(() => colorMode.value === 'dark')

const ranges = [
  { label: '日', value: 'day' as const },
  { label: '周', value: 'week' as const },
  { label: '月', value: 'month' as const }
]

function formatBytes(bytes: unknown) {
  const v = Number(bytes) || 0
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  let val = v
  let i = 0
  while (val >= 1024 && i < units.length - 1) {
    val /= 1024
    i++
  }
  return `${val.toFixed(i === 0 ? 0 : 2)} ${units[i]}`
}

/** 整张图统一换算到同一个单位，避免 y 轴与 tooltip 各说各话 */
function pickUnit(maxBytes: number) {
  const gb = 1024 ** 3
  const mb = 1024 ** 2
  if (maxBytes >= gb) return { label: 'GB', divisor: gb, decimals: 2 }
  if (maxBytes >= mb) return { label: 'MB', divisor: mb, decimals: 2 }
  if (maxBytes >= 1024) return { label: 'KB', divisor: 1024, decimals: 2 }
  return { label: 'B', divisor: 1, decimals: 0 }
}

const buckets = computed(() => props.analysis?.buckets ?? [])

const total = computed(() => {
  const rx = buckets.value.reduce((sum, b) => sum + (Number(b.rx_bytes) || 0), 0)
  const tx = buckets.value.reduce((sum, b) => sum + (Number(b.tx_bytes) || 0), 0)
  return { rx, tx, total: rx + tx }
})

const rangeText = computed(() =>
  ({ day: '本日', week: '本周', month: '本月' } as Record<TrafficRange, string>)[props.range] || '本周期')

function pad2(v: number) { return String(v).padStart(2, '0') }

function parsePeriodStart(value: unknown): Date | null {
  if (typeof value !== 'string' || !value.trim()) return null
  const date = new Date(value)
  return Number.isFinite(date.getTime()) ? date : null
}

// 日视图按小时打标，周/月按月-日
function axisLabel(periodStart: unknown, fallback: string) {
  const d = parsePeriodStart(periodStart)
  if (!d) return fallback
  return props.range === 'day' ? `${pad2(d.getHours())}:00` : `${pad2(d.getMonth() + 1)}-${pad2(d.getDate())}`
}

const snapshot = computed(() => {
  const chart = props.analysis?.chart
  if (!chart) return null
  const timestamps = Array.isArray(chart.timestamps) ? chart.timestamps : []
  const periodStarts = Array.isArray(chart.period_starts) ? chart.period_starts : []
  const devices = Array.isArray(chart.devices) ? chart.devices : []
  return {
    labels: timestamps.map((l, i) => axisLabel(periodStarts[i], String(l || ''))),
    devices,
    series: chart.series ?? {},
    totals: timestamps.map((_, i) =>
      devices.reduce((sum, dev) => sum + Number(chart.series?.[dev]?.[i] || 0), 0))
  }
})

const hasChartData = computed(() => {
  const s = snapshot.value
  if (!s || !s.labels.length) return false
  return props.mode === 'device' ? s.totals.some(v => v > 0) : true
})

const option = computed(() => {
  const s = snapshot.value
  if (!s) return {}
  const unit = pickUnit(Math.max(0, ...s.totals))
  const axis = isDark.value ? '#94a3b8' : '#64748b'
  const split = isDark.value ? 'rgba(148,163,184,0.15)' : 'rgba(100,116,139,0.15)'

  // 总流量单独一条线（不参与堆叠），各设备堆叠在下面
  const totalSeries = {
    name: '总流量',
    type: 'line',
    smooth: true,
    showSymbol: false,
    lineStyle: { width: 2 },
    emphasis: { focus: 'series' },
    z: 3,
    data: s.totals.map(v => v / unit.divisor)
  }

  const stacked = s.devices.map(dev => ({
    name: dev,
    type: 'line',
    stack: 'Total',
    smooth: true,
    showSymbol: false,
    areaStyle: { opacity: 0.18 },
    lineStyle: { width: 1 },
    emphasis: { focus: 'series' },
    data: (s.series[dev] || []).map(v => Number(v || 0) / unit.divisor)
  }))

  return {
    grid: { left: 8, right: 12, top: 36, bottom: 8, containLabel: true },
    tooltip: {
      trigger: 'axis',
      valueFormatter: (v: number) => `${Number(v || 0).toFixed(unit.decimals)} ${unit.label}`
    },
    legend: {
      type: 'scroll',
      top: 0,
      textStyle: { color: axis },
      data: ['总流量', ...s.devices]
    },
    xAxis: {
      type: 'category',
      boundaryGap: false,
      data: s.labels,
      axisLine: { lineStyle: { color: split } },
      axisLabel: { color: axis, fontSize: 11 }
    },
    yAxis: {
      type: 'value',
      name: unit.label,
      nameTextStyle: { color: axis, fontSize: 11 },
      axisLabel: { color: axis, fontSize: 11 },
      splitLine: { lineStyle: { color: split } }
    },
    series: [totalSeries, ...stacked]
  }
})
</script>

<template>
  <section>
    <div class="mb-3 flex flex-wrap items-center justify-between gap-3">
      <div>
        <h2 class="text-sm font-semibold">
          {{ title }}
        </h2>
        <p class="text-xs text-muted mt-0.5">
          {{ subtitle }}
        </p>
      </div>
      <div class="flex items-center gap-2">
        <UFieldGroup size="xs">
          <UButton
            v-for="r in ranges"
            :key="r.value"
            :color="range === r.value ? 'primary' : 'neutral'"
            :variant="range === r.value ? 'soft' : 'outline'"
            :disabled="disabled"
            :label="r.label"
            @click="emit('update:range', r.value)"
          />
        </UFieldGroup>
        <UButton
          icon="i-lucide-refresh-cw"
          color="neutral"
          variant="outline"
          size="xs"
          aria-label="刷新流量"
          :loading="loading"
          :disabled="disabled"
          @click="emit('refresh')"
        />
      </div>
    </div>

    <div class="tile p-4">
      <div v-if="disabled" class="py-8 text-center text-sm text-dimmed">
        网络已禁用，暂无流量分析
      </div>

      <template v-else>
        <UAlert
          v-if="error"
          class="mb-4"
          color="error"
          variant="subtle"
          icon="i-lucide-triangle-alert"
          title="流量分析加载失败"
          :description="String(error.message || error)"
          :actions="[{ label: '重试', color: 'error', variant: 'soft', onClick: () => emit('refresh') }]"
        />

        <div class="mb-4 grid gap-3 sm:grid-cols-3">
          <div class="tile px-3 py-2.5">
            <p class="text-xs text-dimmed">
              {{ rangeText }}下载
            </p>
            <p class="mt-1 font-mono text-lg font-semibold tabular-nums">
              {{ formatBytes(total.rx) }}
            </p>
          </div>
          <div class="tile px-3 py-2.5">
            <p class="text-xs text-dimmed">
              {{ rangeText }}上传
            </p>
            <p class="mt-1 font-mono text-lg font-semibold tabular-nums">
              {{ formatBytes(total.tx) }}
            </p>
          </div>
          <div class="tile px-3 py-2.5">
            <p class="text-xs text-dimmed">
              {{ rangeText }}合计
            </p>
            <p class="mt-1 font-mono text-lg font-semibold tabular-nums">
              {{ formatBytes(total.total) }}
            </p>
          </div>
        </div>

        <USkeleton v-if="loading && !hasChartData" class="h-72 w-full" />

        <div
          v-else-if="!hasChartData"
          class="flex flex-col items-center justify-center gap-3 py-16 text-center"
        >
          <UIcon name="i-lucide-chart-line" class="size-10 text-dimmed" />
          <p class="text-sm text-muted">
            暂无流量图表数据
          </p>
        </div>

        <ClientOnly v-else>
          <VChart :option="option" autoresize class="h-72 w-full" />
          <template #fallback>
            <USkeleton class="h-72 w-full" />
          </template>
        </ClientOnly>
      </template>
    </div>
  </section>
</template>
