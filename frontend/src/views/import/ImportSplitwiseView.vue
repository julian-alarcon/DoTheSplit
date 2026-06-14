<script setup lang="ts">
import AppLayout from "@/components/AppLayout.vue";
import FullImport from "@/components/FullImport.vue";
</script>

<template>
  <AppLayout :back="{ to: '/import', label: 'Import' }">
    <h1 class="title">Splitwise</h1>
    <p class="lead">
      Upload a Splitwise CSV export. Any group size is supported. We parse the file on the server,
      show you a preview, and let you confirm before any data is written.
    </p>
    <FullImport source="splitwise" />
  </AppLayout>
</template>

<style scoped>
.title {
  margin-bottom: 0.5rem;
  font-size: 1.5rem;
  font-weight: 600;
}
.lead {
  margin-bottom: 1.5rem;
  font-size: 0.875rem;
  color: var(--muted-foreground);
}
</style>
