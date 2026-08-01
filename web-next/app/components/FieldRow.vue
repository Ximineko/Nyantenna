<script setup lang="ts">
const props = defineProps<{
  label: string
  value?: string | number | null
  monospace?: boolean
  copyable?: boolean
  /** 敏感信息默认模糊，由父级的眼睛开关控制 */
  sensitive?: boolean
}>()

const toast = useToast()
const display = computed(() => {
  const v = props.value
  return v === undefined || v === null || v === '' ? '—' : String(v)
})
const canCopy = computed(() => props.copyable && display.value !== '—')

async function copy() {
  if (!canCopy.value) return
  try {
    await navigator.clipboard.writeText(display.value)
    toast.add({ title: '已复制', description: display.value, color: 'success' })
  } catch {
    toast.add({ title: '复制失败', color: 'error' })
  }
}
</script>

<template>
  <div class="flex w-full min-w-0 items-center justify-between gap-3 overflow-hidden">
    <span class="shrink-0 whitespace-nowrap text-muted">{{ label }}</span>
    <span
      class="block min-w-0 max-w-full flex-1 truncate text-right"
      :class="[
        monospace ? 'font-mono' : '',
        canCopy ? 'cursor-pointer hover:underline' : '',
        sensitive ? 'blur-sm select-none transition-all' : ''
      ]"
      :title="display"
      @click="copy"
    >{{ display }}</span>
  </div>
</template>
