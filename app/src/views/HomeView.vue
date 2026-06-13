<script setup lang="ts">
import { useRouter } from "vue-router";
import { useAuth } from "@/composables/useAuth";

const { state, logout } = useAuth();
const router = useRouter();

async function onLogout() {
  await logout();
  await router.replace({ name: "login" });
}
</script>

<template>
  <main>
    <h1>DoTheSplit</h1>
    <p v-if="state.user">Signed in as {{ state.user.display_name }} ({{ state.user.email }})</p>
    <button type="button" @click="onLogout">Log out</button>
  </main>
</template>
