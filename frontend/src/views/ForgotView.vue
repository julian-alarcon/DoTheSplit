<script setup lang="ts">
import { ref } from "vue";
import { RouterLink, useRoute, useRouter } from "vue-router";
import { useAuth } from "@/composables/useAuth";
import AppLayout from "@/components/AppLayout.vue";
import Field from "@/components/Field.vue";

const { requestPasswordReset } = useAuth();
const route = useRoute();
const router = useRouter();

const email = ref(typeof route.query.email === "string" ? route.query.email : "");
const submitting = ref(false);

async function onSubmit() {
  submitting.value = true;
  await requestPasswordReset(email.value);
  submitting.value = false;
  // The API returns 204 either way (no enumeration); always advance to /reset.
  await router.push({ path: "/reset", query: { email: email.value } });
}
</script>

<template>
  <AppLayout>
    <section class="mx-auto max-w-96 py-6">
      <h1 class="mb-2 text-2xl font-semibold">Reset your password</h1>
      <p class="mb-6 text-sm text-muted-foreground">
        Type your email and we'll send you a 6-digit code to set a new password.
        If the email is registered, the code will arrive in a moment.
      </p>
      <form
        class="flex flex-col gap-3 rounded-md border border-border bg-card p-3"
        @submit.prevent="onSubmit"
      >
        <Field
          v-model="email"
          label="Email"
          type="email"
          required
          autocomplete="email"
          error="Enter a valid email address"
        />
        <button
          type="submit"
          class="btn-primary mt-2 self-end"
          :disabled="submitting"
        >
          Send code
        </button>
      </form>
      <p class="mt-4 text-sm">
        <RouterLink to="/login" class="rounded-md underline text-neutral-600 hover:text-neutral-900 dark:text-neutral-400 dark:hover:text-neutral-100 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-neutral-400 dark:focus-visible:ring-neutral-500">Back to login</RouterLink>
      </p>
    </section>
  </AppLayout>
</template>
