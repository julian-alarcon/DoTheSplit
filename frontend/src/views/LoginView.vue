<script setup lang="ts">
import { ref } from "vue";
import { RouterLink, useRoute, useRouter } from "vue-router";
import { useAuth } from "@/composables/useAuth";
import AppLayout from "@/components/AppLayout.vue";
import Field from "@/components/Field.vue";
import Alert from "@/components/Alert.vue";

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
    const redirect = typeof route.query.redirect === "string" ? route.query.redirect : "/groups";
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
    <section class="card">
      <h1 class="title">Log in</h1>
      <Alert v-if="error" tone="error" class="mb">{{ error }}</Alert>
      <form class="form" @submit.prevent="onSubmit">
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
          minlength="10"
          autocomplete="current-password"
          error="Password must be at least 10 characters."
        />
        <p class="hint">
          <RouterLink to="/forgot" class="link">Forgot your password?</RouterLink>
        </p>
        <button type="submit" class="btn-primary submit" :disabled="submitting">Log in</button>
      </form>
      <p class="foot">
        No account? <RouterLink to="/register" class="link">Register</RouterLink>.
      </p>
    </section>
  </AppLayout>
</template>

<style scoped>
.card {
  margin-inline: auto;
  max-width: 24rem;
  padding-block: 1.5rem;
}
.title {
  font-size: 1.5rem;
  font-weight: 600;
  margin-bottom: 1.5rem;
}
.mb {
  margin-bottom: 1rem;
}
.form {
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
  border-radius: 0.375rem;
  border: 1px solid var(--border);
  background: var(--card);
  padding: 0.75rem;
}
.hint {
  font-size: 0.875rem;
}
.submit {
  margin-top: 0.5rem;
  align-self: flex-end;
}
.foot {
  margin-top: 1rem;
  font-size: 0.875rem;
  color: var(--muted-foreground);
}
.link {
  text-decoration: underline;
}
</style>
