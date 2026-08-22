<script setup lang="ts">
import Modal from './Modal.vue'

defineProps<{ open: boolean; title: string; message: string; confirmText?: string; busy?: boolean }>()
const emit = defineEmits<{ confirm: []; cancel: [] }>()
</script>

<template>
  <Modal :title="title" :open="open" @close="emit('cancel')">
    <p class="text-sm text-text-lo leading-relaxed">{{ message }}</p>
    <div class="mt-6 flex justify-end gap-3">
      <button class="btn btn-ghost" :disabled="busy" @click="emit('cancel')">取消</button>
      <button class="btn btn-danger" :disabled="busy" @click="emit('confirm')">
        {{ busy ? '执行中…' : (confirmText ?? '确认执行') }}
      </button>
    </div>
  </Modal>
</template>
