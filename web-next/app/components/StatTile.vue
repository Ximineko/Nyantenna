<script setup lang="ts">
defineProps<{
  label: string
  value: string | number
  icon: string
  tone?: 'default' | 'success' | 'error' | 'primary'
}>()

const toneClass = {
  default: 'text-toned',
  success: 'text-success',
  error: 'text-error',
  primary: 'text-primary'
}
</script>

<template>
  <div class="flex items-center gap-4 px-5 py-4">
    <div
      class="flex size-10 shrink-0 items-center justify-center rounded-lg bg-elevated"
    >
      <UIcon :name="icon" :class="['size-5', toneClass[tone ?? 'default']]" />
    </div>
    <div class="min-w-0">
      <p class="text-xs font-medium uppercase tracking-wide text-dimmed">{{ label }}</p>
      <p class="text-2xl font-semibold tabular-nums mt-0.5 leading-none">{{ value }}</p>
    </div>
  </div>
</template>
