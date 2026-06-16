<script setup lang="ts">
import { ref } from "vue";
import { RouterLink, useRouter } from "vue-router";
import { useAuth } from "@/composables/useAuth";
import AppLayout from "@/components/AppLayout.vue";
import Field from "@/components/Field.vue";
import Alert from "@/components/Alert.vue";

const { register } = useAuth();
const router = useRouter();

const displayName = ref("");
const email = ref("");
const password = ref("");
const submitting = ref(false);
const error = ref(false);

async function onSubmit() {
  error.value = false;
  submitting.value = true;
  const res = await register({
    email: email.value,
    password: password.value,
    display_name: displayName.value,
  });
  submitting.value = false;
  if (!res.ok) {
    error.value = true;
    return;
  }
  if (res.verificationRequired) {
    await router.push({ path: "/verify", query: { email: email.value } });
  } else {
    await router.replace("/groups");
  }
}
</script>

<template>
  <AppLayout>
    <section class="mx-auto max-w-96 py-6">
      <h1 class="mb-6 text-2xl font-semibold">Create account</h1>
      <Alert v-if="error" tone="error" class="mb-4">
        Could not register. The email may already be in use.
      </Alert>
      <form
        class="flex flex-col gap-3 rounded-md border border-border bg-card p-3"
        @submit.prevent="onSubmit"
      >
        <Field
          v-model="displayName"
          label="Display name"
          type="text"
          required
          maxlength="80"
          error="Required"
        />
        <Field
          v-model="email"
          label="Email"
          type="email"
          required
          autocomplete="email"
          error="Enter a valid email address"
        />
        <Field
          v-model="password"
          label="Password (min 10 characters)"
          type="password"
          required
          minlength="10"
          autocomplete="new-password"
          error="Password must be at least 10 characters"
        />
        <button
          type="submit"
          class="btn-primary mt-2 self-end"
          :disabled="submitting"
        >
          Create account
        </button>
      </form>
      <p class="mt-4 text-sm text-muted-foreground">
        Already have one? <RouterLink to="/login" class="rounded-md underline text-neutral-600 hover:text-neutral-900 dark:text-neutral-400 dark:hover:text-neutral-100 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-neutral-400 dark:focus-visible:ring-neutral-500">Log in</RouterLink>.
      </p>
    </section>
  </AppLayout>
</template>
