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
    <section class="mx-auto max-w-96 py-6">
      <h1 class="mb-6 text-2xl font-semibold">Log in</h1>
      <Alert v-if="error" tone="error" class="mb-4">{{ error }}</Alert>
      <form
        class="flex flex-col gap-3 rounded-md border border-border bg-card p-3"
        @submit.prevent="onSubmit"
      >
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
        <p class="text-sm">
          <RouterLink to="/forgot" class="rounded-md underline text-neutral-600 hover:text-neutral-900 dark:text-neutral-400 dark:hover:text-neutral-100 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-neutral-400 dark:focus-visible:ring-neutral-500">Forgot your password?</RouterLink>
        </p>
        <button
          type="submit"
          class="btn-primary mt-2 self-end"
          :disabled="submitting"
        >
          Log in
        </button>
      </form>
      <p class="mt-4 text-sm text-muted-foreground">
        No account? <RouterLink to="/register" class="rounded-md underline text-neutral-600 hover:text-neutral-900 dark:text-neutral-400 dark:hover:text-neutral-100 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-neutral-400 dark:focus-visible:ring-neutral-500">Register</RouterLink>.
      </p>
    </section>
  </AppLayout>
</template>
