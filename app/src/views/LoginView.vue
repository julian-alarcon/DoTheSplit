<script setup lang="ts">
import { ref } from "vue";
import { useRoute, useRouter } from "vue-router";
import { useAuth } from "@/composables/useAuth";
import AppLayout from "@/components/AppLayout.vue";
import Field from "@/components/Field.vue";

const { login } = useAuth();
const router = useRouter();
const route = useRoute();

const email = ref("");
const password = ref("");
const error = ref<string | null>(null);
const submitting = ref(false);

async function onSubmit() {
  error.value = null;
  submitting.value = true;
  const res = await login(email.value, password.value);
  submitting.value = false;
  if (res.ok) {
    const redirect = typeof route.query.redirect === "string" ? route.query.redirect : "/";
    await router.replace(redirect);
  } else {
    error.value =
      res.code === "email_unverified"
        ? "Your email address is not verified yet."
        : "Invalid email or password.";
  }
}
</script>

<template>
  <AppLayout>
    <section class="login">
      <h1 class="login-title">Log in</h1>
      <form class="login-form" @submit.prevent="onSubmit">
        <Field
          v-model="email"
          label="Email"
          type="email"
          required
          autocomplete="username"
          error="Enter a valid email address."
        />
        <Field
          v-model="password"
          label="Password"
          type="password"
          required
          autocomplete="current-password"
          error="Enter your password."
        />
        <p v-if="error" class="login-error" role="alert">{{ error }}</p>
        <button type="submit" class="btn-primary" :disabled="submitting">Log in</button>
      </form>
    </section>
  </AppLayout>
</template>

<style scoped>
.login {
  margin-inline: auto;
  max-width: 24rem;
  padding-block: 1.5rem;
}
.login-title {
  font-size: 1.5rem;
  font-weight: 600;
  margin-bottom: 1.25rem;
}
.login-form {
  display: flex;
  flex-direction: column;
  gap: 1rem;
}
.login-error {
  font-size: 0.875rem;
  color: var(--destructive);
}
</style>
