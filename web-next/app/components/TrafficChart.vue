<script setup lang="ts">
import { use } from 'echarts/core'
import { CanvasRenderer } from 'echarts/renderers'
import { LineChart } from 'echarts/charts'
import { GridComponent, TooltipComponent, LegendComponent } from 'echarts/components'
import VChart from 'vue-echarts'
import type { TrafficAnalysis } from '~/services/traffic'

use([CanvasRenderer, LineChart, GridComponent, TooltipComponent, LegendComponent])

const props = defineProps<{ analysis: TrafficAnalysis }>()

const colorMode = useColorMode()
const isDark = computed(() => colorMode.value === 'dark')

function formatBytes(v: number): string {
  if (!Number.isFinite(v) || v <= 0) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  let n = v
  let i = 0
  while (n >= 1024 && i < units.length - 1) {
    n /= 1024
    i++
  }
  return `${n.toFixed(n >= 100 || i === 0 ? 0 : 1)} ${units[i]}`
}

const hasData = computed(() => {
  const chart = props.analysis?.chart
  return !!chart && Array.isArray(chart.timestamps) && chart.timestamps.length > 0
})

const option = computed(() => {
  const chart = props.analysis?.chart
  if (!chart) return {}
  const axis = isDark.value ? '#94a3b8' : '#64748b'
  const split = isDark.value ? 'rgba(148,163,184,0.15)' : 'rgba(100,116,139,0.15)'

  return {
    grid: { left: 8, right: 12, top: 32, bottom: 8, containLabel: true },
    tooltip: {
      trigger: 'axis',
      valueFormatter: (v: number) => formatBytes(v)
    },
    legend: {
      type: 'scroll',
      top: 0,
      textStyle: { color: axis },
      data: chart.devices
    },
    xAxis: {
      type: 'category',
      boundaryGap: false,
      data: chart.timestamps,
      axisLine: { lineStyle: { color: split } },
      axisLabel: { color: axis, fontSize: 11 }
    },
    yAxis: {
      type: 'value',
      axisLabel: { color: axis, fontSize: 11, formatter: (v: number) => formatBytes(v) },
      splitLine: { lineStyle: { color: split } }
    },
    series: chart.devices.map(name => ({
      name,
      type: 'line',
      smooth: true,
      showSymbol: false,
      areaStyle: { opacity: 0.08 },
      lineStyle: { width: 2 },
      data: chart.series?.[name] ?? []
    }))
  }
})
</script>

<template>
  <div
    v-if="!hasData"
    class="flex flex-col items-center justify-center gap-3 py-16 text-center"
  >
    <UIcon name="i-lucide-chart-line" class="size-10 text-dimmed" />
    <p class="text-sm text-muted">暂无流量数据</p>
  </div>
  <ClientOnly v-else>
    <VChart :option="option" autoresize class="h-64 w-full" />
    <template #fallback>
      <USkeleton class="h-64 w-full" />
    </template>
  </ClientOnly>
</template>
