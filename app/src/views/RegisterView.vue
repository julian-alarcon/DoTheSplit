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
    <section class="card">
      <h1 class="title">Create account</h1>
      <Alert v-if="error" tone="error" class="mb">
        Could not register. The email may already be in use.
      </Alert>
      <form class="form" @submit.prevent="onSubmit">
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
        <button type="submit" class="btn-primary submit" :disabled="submitting">
          Create account
        </button>
      </form>
      <p class="foot">
        Already have one? <RouterLink to="/login" class="link">Log in</RouterLink>.
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
