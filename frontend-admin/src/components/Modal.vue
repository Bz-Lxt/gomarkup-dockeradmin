<script setup lang="ts">
defineProps<{ title: string; open: boolean }>()
const emit = defineEmits<{ close: [] }>()
</script>

<template>
  <Teleport to="body">
    <div v-if="open" class="fixed inset-0 z-50 flex items-center justify-center p-4">
      <div class="absolute inset-0 bg-ink-0/70 backdrop-blur-sm" @click="emit('close')" />
      <div class="relative card w-full max-w-lg p-6 animate-fade-up shadow-2xl" role="dialog" aria-modal="true">
        <div class="flex items-center justify-between mb-5">
          <h3 class="font-display font-semibold text-lg tracking-wide">{{ title }}</h3>
          <button class="text-text-lo hover:text-text-hi transition-colors" aria-label="关闭" @click="emit('close')">
            <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round">
              <path d="M18 6L6 18M6 6l12 12" />
            </svg>
          </button>
        </div>
        <slot />
      </div>
    </div>
  </Teleport>
</template>
