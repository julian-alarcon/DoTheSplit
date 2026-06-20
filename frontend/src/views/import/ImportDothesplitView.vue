<script setup lang="ts">
import AppLayout from "@/components/AppLayout.vue";
import FullImport from "@/components/FullImport.vue";
</script>

<template>
  <AppLayout :back="{ to: '/import', label: 'Import' }">
    <h1 class="mb-2 text-2xl font-semibold">DoTheSplit</h1>
    <p class="mb-6 text-sm text-muted-foreground">
      Upload a CSV produced by Group settings → Export. We parse the file on the server, show you a
      preview, and let you confirm before any data is written. Time, payer, and notes are preserved.
    </p>
    <FullImport source="dothesplit" />
  </AppLayout>
</template>
