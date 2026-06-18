<script setup lang="ts">
import FlowWorkbenchShell from './flow/FlowWorkbenchShell.vue'
import { useWorkMemoryFlow } from './flow/useWorkMemoryFlow'

useWorkMemoryFlow()
</script>

<template>
  <main class="min-h-screen bg-[var(--background)] text-[var(--foreground)]">
    <div class="app-frame">
      <section class="launcher-shell memory-shell flow-shell" aria-label="Ariadne flow center">
        <FlowWorkbenchShell />
      </section>
    </div>
  </main>
</template>
