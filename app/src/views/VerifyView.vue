<script setup lang="ts">
import { computed, ref } from "vue";
import { useRoute, useRouter } from "vue-router";
import { useAuth } from "@/composables/useAuth";
import AppLayout from "@/components/AppLayout.vue";
import Field from "@/components/Field.vue";
import Alert from "@/components/Alert.vue";

const { verifyEmail, resendVerification } = useAuth();
const route = useRoute();
const router = useRouter();

const email = computed(() =>
  typeof route.query.email === "string" ? route.query.email : "",
);

const code = ref("");
const submitting = ref(false);
const errorCode = ref<string | null>(null);
const resent = ref(false);

const errorMessages: Record<string, string> = {
  invalid: "That code didn't match. Try again or request a new one.",
  expired: "Your code expired. Tap 'Send a new code' to receive a fresh one.",
  rate_limited: "Too many attempts. Request a new code and try again.",
};

async function onSubmit() {
  errorCode.value = null;
  submitting.value = true;
  const res = await verifyEmail(email.value, code.value);
  submitting.value = false;
  if (res.ok) {
    await router.replace({ path: "/login", query: { verified: "1" } });
  } else {
    errorCode.value = res.code ?? "invalid";
  }
}

async function onResend() {
  resent.value = false;
  errorCode.value = null;
  await resendVerification(email.value);
  resent.value = true;
}
</script>

<template>
  <AppLayout>
    <section class="card">
      <h1 class="title">Check your inbox</h1>
      <p class="lead">
        We sent a 6-digit code to <strong>{{ email || "your email" }}</strong>.
        Paste it below to finish creating your account.
      </p>

      <Alert v-if="errorCode" tone="error" class="mb">
        {{ errorMessages[errorCode] ?? "Could not verify. Please try again." }}
      </Alert>
      <Alert v-if="resent" tone="success" class="mb">
        New code sent. It may take a moment to arrive.
      </Alert>

      <form class="form" @submit.prevent="onSubmit">
        <Field
          v-model="code"
          label="6-digit code"
          type="text"
          required
          inputmode="numeric"
          autocomplete="one-time-code"
          pattern="[0-9]{6}"
          maxlength="6"
          minlength="6"
          class="code-input"
          error="Enter the 6-digit code from your email"
        />
        <button type="submit" class="btn-primary submit" :disabled="submitting">
          Verify
        </button>
      </form>

      <button type="button" class="resend link" @click="onResend">
        Send a new code
      </button>
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
  margin-bottom: 0.5rem;
}
.lead {
  margin-bottom: 1.5rem;
  font-size: 0.875rem;
  color: var(--muted-foreground);
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
.resend {
  margin-top: 1rem;
  font-size: 0.875rem;
  color: var(--muted-foreground);
  cursor: pointer;
}
.link {
  text-decoration: underline;
}
:deep(.code-input) {
  text-align: center;
  letter-spacing: 0.4em;
}
</style>
