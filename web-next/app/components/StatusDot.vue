<script setup lang="ts">
const props = defineProps<{
  tone: 'success' | 'error' | 'warning' | 'neutral' | 'primary'
  pulse?: boolean
}>()

const bg = computed(() => ({
  success: 'bg-success',
  error: 'bg-error',
  warning: 'bg-warning',
  neutral: 'bg-inverted/30',
  primary: 'bg-primary'
}[props.tone]))
</script>

<template>
  <span class="relative flex size-2 shrink-0">
    <span
      v-if="pulse"
      :class="['absolute inline-flex size-full animate-ping rounded-full opacity-60', bg]"
    />
    <span :class="['relative inline-flex size-2 rounded-full', bg]" />
  </span>
</template>
