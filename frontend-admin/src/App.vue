<script setup lang="ts">
import AppShell from './components/AppShell.vue'
import ToastContainer from './components/ToastContainer.vue'
</script>

<template>
  <AppShell>
    <router-view v-slot="{ Component }">
      <component :is="Component" />
    </router-view>
  </AppShell>
  <ToastContainer />
</template>
