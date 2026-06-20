<script setup lang="ts">
import AppLayout from "@/components/AppLayout.vue";
import FullImport from "@/components/FullImport.vue";
</script>

<template>
  <AppLayout :back="{ to: '/import', label: 'Import' }">
    <h1 class="mb-2 text-2xl font-semibold">Splitwise</h1>
    <p class="mb-6 text-sm text-muted-foreground">
      Upload a Splitwise CSV export. Any group size is supported. We parse the file on the server,
      show you a preview, and let you confirm before any data is written.
    </p>
    <FullImport source="splitwise" />
  </AppLayout>
</template>
