<script setup lang="ts">
import { ref } from "vue";
import { useRouter } from "vue-router";
import { useAuth } from "@/composables/useAuth";
import { useSetup } from "@/composables/useSetup";
import AppLayout from "@/components/AppLayout.vue";
import Field from "@/components/Field.vue";
import Alert from "@/components/Alert.vue";

const { completeSetup } = useAuth();
const { markLocked } = useSetup();
const router = useRouter();

const token = ref("");
const displayName = ref("");
const email = ref("");
const password = ref("");
const submitting = ref(false);
const errorCode = ref<string | null>(null);

const errorMessages: Record<string, string> = {
  invalid: "The setup token does not match. Check the latest API container log and try again.",
  bad_request: "Check that all fields are filled in and the password is at least 10 characters.",
  email_taken: "That email is already registered.",
  completed: "Setup has already been completed; visit the login page.",
  rate_limited: "Too many attempts. Wait a minute and try again.",
};

async function onSubmit() {
  errorCode.value = null;
  submitting.value = true;
  const res = await completeSetup({
    token: token.value,
    email: email.value,
    password: password.value,
    display_name: displayName.value,
  });
  submitting.value = false;
  if (res.ok) {
    markLocked();
    await router.replace("/groups");
  } else {
    errorCode.value = res.code ?? "bad_request";
  }
}
</script>

<template>
  <AppLayout>
    <div class="wrap">
      <h1 class="title">Welcome to DoTheSplit</h1>
      <p class="lead">
        This is the one-time install ceremony. The first account created here is
        the instance admin. The setup token was printed to the API container log
        on every boot until now: grab it via
        <code class="code">docker compose logs api</code>.
      </p>

      <Alert v-if="errorCode" tone="error" class="mb">
        {{ errorMessages[errorCode] ?? "Could not complete setup." }}
      </Alert>

      <form class="form" @submit.prevent="onSubmit">
        <Field
          v-model="token"
          label="Setup token"
          type="text"
          required
          autocomplete="off"
          spellcheck="false"
          error="Required"
        />
        <Field
          v-model="displayName"
          label="Display name"
          type="text"
          required
          minlength="1"
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
          label="Password"
          type="password"
          required
          minlength="10"
          autocomplete="new-password"
          error="Password must be at least 10 characters"
        />
        <p class="minhint">Minimum 10 characters.</p>
        <button type="submit" class="btn-primary submit" :disabled="submitting">
          Create admin
        </button>
      </form>
    </div>
  </AppLayout>
</template>

<style scoped>
.wrap {
  margin-inline: auto;
  max-width: 28rem;
}
.title {
  font-size: 1.5rem;
  font-weight: 600;
  margin-bottom: 0.5rem;
}
.lead {
  margin-bottom: 1rem;
  font-size: 0.875rem;
  color: var(--muted-foreground);
}
.code {
  border-radius: 0.25rem;
  background: var(--muted);
  padding: 0.125rem 0.25rem;
  font-family: var(--font-mono);
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
.minhint {
  margin-top: -0.25rem;
  font-size: 0.75rem;
  color: var(--muted-foreground);
}
.submit {
  margin-top: 0.5rem;
  align-self: flex-end;
}
</style>
