<script setup lang="ts">
defineProps<{
  icon: string
  title: string
  description?: string
}>()
</script>

<template>
  <div class="flex flex-col items-center justify-center gap-3 px-6 py-16 text-center">
    <div class="flex size-12 items-center justify-center rounded-full bg-elevated">
      <UIcon :name="icon" class="size-6 text-dimmed" />
    </div>
    <div>
      <p class="text-sm font-medium">{{ title }}</p>
      <p v-if="description" class="text-sm text-muted mt-1">{{ description }}</p>
    </div>
    <slot />
  </div>
</template>
