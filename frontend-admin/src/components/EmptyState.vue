<script setup lang="ts">
defineProps<{ icon?: string; text: string; hint?: string }>()
</script>

<template>
  <div class="flex flex-col items-center justify-center py-16 text-center">
    <div class="font-mono text-4xl text-text-lo/40 select-none">{{ icon ?? '[ ]' }}</div>
    <div class="mt-3 text-sm text-text-lo">{{ text }}</div>
    <div v-if="hint" class="mt-1 text-xs text-text-lo/60">{{ hint }}</div>
  </div>
</template>
