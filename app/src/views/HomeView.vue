<script setup lang="ts">
import { useAuth } from "@/composables/useAuth";
import AppLayout from "@/components/AppLayout.vue";

const { state } = useAuth();
</script>

<template>
  <AppLayout>
    <section class="home">
      <h1 class="home-title">DoTheSplit</h1>
      <p v-if="state.user">
        Signed in as {{ state.user.display_name }} ({{ state.user.email }})
      </p>
    </section>
  </AppLayout>
</template>

<style scoped>
.home {
  padding-block: 1.5rem;
}
.home-title {
  font-size: 1.5rem;
  font-weight: 600;
}
</style>
