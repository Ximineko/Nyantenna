<script setup lang="ts">
const props = withDefaults(defineProps<{
  /** 设备后端模式：at | qmi | mbim | pcsc */
  backendMode?: string
  size?: 'xs' | 'sm'
  /** 只在需要区分时显示（默认只给 PC/SC 打标，模组设备不加噪音） */
  always?: boolean
}>(), { size: 'xs', always: false })

const mode = computed(() => String(props.backendMode || '').toLowerCase().trim())

// PC/SC 是「只有卡、没有基带」的设备，能力集与模组差别很大，
// 值得在列表里一眼区分；模组之间的 AT/QMI/MBIM 差异对使用者无感，默认不打标。
const meta = computed(() => {
  switch (mode.value) {
    case 'pcsc':
      return { label: '读卡器', icon: 'i-lucide-credit-card', color: 'primary' as const }
    case 'qmi':
      return { label: 'QMI', icon: 'i-lucide-cpu', color: 'neutral' as const }
    case 'mbim':
      return { label: 'MBIM', icon: 'i-lucide-cpu', color: 'neutral' as const }
    case 'at':
      return { label: 'AT', icon: 'i-lucide-cpu', color: 'neutral' as const }
    default:
      return null
  }
})

const visible = computed(() => !!meta.value && (props.always || mode.value === 'pcsc'))
</script>

<template>
  <UBadge
    v-if="visible && meta"
    :size="size"
    :color="meta.color"
    variant="subtle"
    :icon="meta.icon"
    :label="meta.label"
    class="shrink-0"
  />
</template>
